//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func answer(text string) *string { return &text }

func TestRubricCatalogAndGradedSaveAgainstPostgres(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	levelID := s.level("qb-"+s.tag, "qb level description")
	recruiter, other := s.user("recruiter"), s.user("recruiter")

	// Rubrics are seeded first so their cleanup runs after the questions that reference them are gone.
	mine := s.rubric("qb mine "+s.tag, &recruiter, true)
	mineArchived := s.rubric("qb mine archived "+s.tag, &recruiter, false)
	theirs := s.rubric("qb theirs "+s.tag, &other, true)
	systemActive := s.rubric("qb system "+s.tag, nil, true)
	systemArchived := s.rubric("qb system archived "+s.tag, nil, false)

	jd, postingID := s.jdWithPosting(levelID, recruiter, true)
	repo := qbrepository.NewRepository(pool)

	t.Run("catalog offers system and own active rubrics only", func(t *testing.T) {
		catalog, err := repo.RubricCatalog(ctx, recruiter)
		if err != nil {
			t.Fatal(err)
		}
		offered := map[uuid.UUID]domain.RubricOption{}
		for _, option := range catalog {
			offered[option.ID] = option
		}
		for _, want := range []uuid.UUID{mine, systemActive} {
			if _, ok := offered[want]; !ok {
				t.Errorf("rubric %s should be offered", want)
			}
		}
		for name, id := range map[string]uuid.UUID{"another recruiter's": theirs, "own archived": mineArchived, "archived system": systemArchived} {
			if _, ok := offered[id]; ok {
				t.Errorf("%s rubric was offered", name)
			}
		}
		if len(catalog) > domain.MaxPromptRubrics {
			t.Fatalf("catalog has %d rubrics, cap is %d", len(catalog), domain.MaxPromptRubrics)
		}
		if got := offered[mine]; len(got.Criteria) != 1 || got.Criteria[0].Metric != "Technical Accuracy" || got.Criteria[0].MaxPoints != 3 {
			t.Fatalf("criteria were not summarized: %+v", got.Criteria)
		}
		// System templates come first, so a Recruiter with many rubrics never loses them to the cap.
		seenOwn := false
		for _, option := range catalog {
			own := option.ID == mine
			if seenOwn && !own && strings.HasPrefix(option.Name, "qb system") {
				t.Fatal("a system rubric was listed after an own rubric")
			}
			seenOwn = seenOwn || own
		}
	})

	t.Run("a generated bank stores rubric and expected answer, and reads them back", func(t *testing.T) {
		saved, err := repo.SaveForPosting(ctx, recruiter, postingID, []domain.GeneratedQuestion{
			{Content: "Q-own?", RubricID: &mine, ExpectedAnswer: answer("own answer")},
			{Content: "Q-system?", RubricID: &systemActive, ExpectedAnswer: answer("system answer")},
			{Content: "Q-none?", ExpectedAnswer: answer("only an answer")},
			{Content: "Q-nothing?"},
			{Content: "Q-foreign?", RubricID: &theirs, ExpectedAnswer: answer("foreign")},
			{Content: "Q-archived?", RubricID: &mineArchived, ExpectedAnswer: answer("archived")},
			{Content: "Q-archived-system?", RubricID: &systemArchived, ExpectedAnswer: answer("archived system")},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(saved) != 7 {
			t.Fatalf("saved %d questions", len(saved))
		}
		listed, err := repo.ListBank(ctx, jd, recruiter)
		if err != nil {
			t.Fatal(err)
		}
		byContent := map[string]domain.BankQuestion{}
		for _, q := range listed {
			byContent[q.Content] = q
		}
		type want struct {
			rubric     *uuid.UUID
			answer     string
			incomplete bool
		}
		for content, w := range map[string]want{
			"Q-own?":             {&mine, "own answer", false},
			"Q-system?":          {&systemActive, "system answer", false},
			"Q-none?":            {nil, "only an answer", true},
			"Q-nothing?":         {nil, "", true},
			"Q-foreign?":         {nil, "foreign", true},
			"Q-archived?":        {nil, "archived", true},
			"Q-archived-system?": {nil, "archived system", true},
		} {
			got := byContent[content]
			if (got.RubricID == nil) != (w.rubric == nil) || (w.rubric != nil && *got.RubricID != *w.rubric) {
				t.Errorf("%s: rubric = %v, want %v (a rubric the Recruiter may not use must not be stored)", content, got.RubricID, w.rubric)
			}
			if (got.ExpectedAnswer == nil) != (w.answer == "") || (w.answer != "" && *got.ExpectedAnswer != w.answer) {
				t.Errorf("%s: expected answer = %v, want %q", content, got.ExpectedAnswer, w.answer)
			}
			if got.GradingIncomplete != w.incomplete {
				t.Errorf("%s: grading_incomplete = %v, want %v", content, got.GradingIncomplete, w.incomplete)
			}
		}
	})

	t.Run("the interview-facing read carries no grading data", func(t *testing.T) {
		listed, err := repo.List(ctx, jd)
		if err != nil || len(listed) != 7 {
			t.Fatalf("got %v, %v", listed, err)
		}
		body, err := json.Marshal(listed)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"rubric", "expected_answer", "grading", "answer"} {
			if strings.Contains(string(body), forbidden) {
				t.Fatalf("CoreQuestion JSON exposes %q: %s", forbidden, body)
			}
		}
	})

	t.Run("editing the text keeps the grading setup and a manual question starts without one", func(t *testing.T) {
		listed, _ := repo.ListBank(ctx, jd, recruiter)
		var own domain.BankQuestion
		for _, q := range listed {
			if q.Content == "Q-own?" {
				own = q
			}
		}
		updated, err := repo.Update(ctx, recruiter, postingID, own.ID, "Q-own edited?")
		if err != nil || updated.RubricID == nil || *updated.RubricID != mine || updated.ExpectedAnswer == nil || updated.GradingIncomplete {
			t.Fatalf("edit changed the grading setup: %+v, %v", updated, err)
		}
		added, err := repo.Add(ctx, recruiter, postingID, "Q-manual?")
		if err != nil || added.RubricID != nil || added.ExpectedAnswer != nil || !added.GradingIncomplete {
			t.Fatalf("manual question = %+v, %v", added, err)
		}
	})

	t.Run("a bank that exists is never replaced", func(t *testing.T) {
		_, err := repo.SaveForPosting(ctx, recruiter, postingID, []domain.GeneratedQuestion{{Content: "again?", RubricID: &mine, ExpectedAnswer: answer("x")}})
		if codeOf(err) != apperror.CodeQuestionBankExists {
			t.Fatalf("err = %v", err)
		}
	})
}

// An archive that commits between generation and save must not leave a question bound to the archived rubric.
func TestRubricArchivedBeforeSaveIsNotBound(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	levelID := s.level("qb-"+s.tag, "d")
	recruiter := s.user("recruiter")
	mine := s.rubric("qb mine "+s.tag, &recruiter, true)
	jd, postingID := s.jdWithPosting(levelID, recruiter, true)
	repo := qbrepository.NewRepository(pool)

	s.exec(`UPDATE public.rubrics SET is_active = false WHERE id = $1`, mine)
	saved, err := repo.SaveForPosting(ctx, recruiter, postingID, []domain.GeneratedQuestion{{Content: "Q?", RubricID: &mine, ExpectedAnswer: answer("a")}})
	if err != nil || len(saved) != 1 || saved[0].RubricID != nil || !saved[0].GradingIncomplete {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	_ = jd
}

// The whole service on the real repository, with a scripted model: one call, validated, stored, readable.
func TestServiceGeneratesGradedBankAgainstPostgres(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	levelID := s.level("qb-"+s.tag, "d")
	recruiter, other := s.user("recruiter"), s.user("recruiter")
	mine := s.rubric("qb mine "+s.tag, &recruiter, true)
	theirs := s.rubric("qb theirs "+s.tag, &other, true)
	jd, postingID := s.jdWithPosting(levelID, recruiter, true)
	repo := qbrepository.NewRepository(pool)

	gen := &scriptedGenerator{}
	svc, err := qbservice.New(repo, gen, 0, 3)
	if err != nil {
		t.Fatal(err)
	}

	// First attempt guesses another Recruiter's private rubric: refused, nothing stored.
	gen.reply = func(domain.GenerationInput) domain.GenerationCandidate {
		return domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{
			{Content: "A?", RubricID: &theirs, ExpectedAnswer: answer("a")},
			{Content: "B?", ExpectedAnswer: answer("b")}, {Content: "C?", ExpectedAnswer: answer("c")}}}
	}
	_, err = svc.Generate(ctx, recruiter, "recruiter", postingID)
	if codeOf(err) != apperror.CodeInvalidQuestionGenerationOutput && codeOf(err) != apperror.CodeQuestionGenerationFailed {
		t.Fatalf("a foreign rubric id was not refused: %v", err)
	}
	if left, _ := repo.List(ctx, jd); len(left) != 0 {
		t.Fatalf("%d questions stored from a refused proposal", len(left))
	}
	foreignSeen := false
	for _, option := range gen.lastInput.Rubrics {
		foreignSeen = foreignSeen || option.ID == theirs
	}
	if foreignSeen || !gen.lastInput.Grading {
		t.Fatalf("the prompt catalog must hold only the owner's allowed rubrics: %+v", gen.lastInput.Rubrics)
	}

	gen.calls = 0
	gen.reply = func(domain.GenerationInput) domain.GenerationCandidate {
		return domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{
			{Content: "A?", RubricID: &mine, ExpectedAnswer: answer("a")},
			{Content: "B?", ExpectedAnswer: answer("b")}, {Content: "C?", ExpectedAnswer: answer("c")}}}
	}
	got, err := svc.Generate(ctx, recruiter, "recruiter", postingID)
	if err != nil || len(got) != 3 || gen.calls != 1 {
		t.Fatalf("got %d questions, %d model calls, %v", len(got), gen.calls, err)
	}
	bank, err := svc.List(ctx, recruiter, "recruiter", postingID)
	if err != nil || len(bank.Questions) != 3 {
		t.Fatal(bank, err)
	}
	incomplete := 0
	for _, q := range bank.Questions {
		if q.GradingIncomplete {
			incomplete++
		}
	}
	if incomplete != 2 {
		t.Fatalf("%d incomplete questions, want 2 (those without a suitable rubric)", incomplete)
	}
}

type scriptedGenerator struct {
	calls     int
	lastInput domain.GenerationInput
	reply     func(domain.GenerationInput) domain.GenerationCandidate
}

func (g *scriptedGenerator) Generate(_ context.Context, in domain.GenerationInput, _ int) (domain.GenerationCandidate, error) {
	g.calls++
	g.lastInput = in
	return g.reply(in), nil
}
