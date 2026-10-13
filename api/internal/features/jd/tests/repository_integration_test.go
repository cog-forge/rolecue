//go:build integration

package tests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/internal/features/jd/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jd/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func record(contentPath string) domain.Record {
	return domain.Record{
		ContentPath:    contentPath,
		Title:          "Backend Engineer",
		Seniority:      domain.Senior,
		RefinementNote: strPtr("Exclude C#"),
		Requirements: []domain.RequirementRow{
			{Name: "Go", Target: "programming_language", Type: domain.Required, EvidenceText: strPtr("5+ years")},
			{Name: "Kafka", Target: domain.TargetTechnologies, Type: domain.Preferred},
		},
	}
}

func TestConfirmedJDRoundTrip(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	owner := s.user()
	repo := repository.NewRepository(pool)

	created, err := repo.Create(ctx, owner, record(s.key("1")))
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, owner, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentPath != s.key("1") || got.Title != "Backend Engineer" || got.Seniority != domain.Senior || got.HasQuestionBank {
		t.Fatalf("got %+v", got)
	}
	if got.RefinementNote == nil || *got.RefinementNote != "Exclude C#" {
		t.Fatalf("note = %v", got.RefinementNote)
	}
	if len(got.Requirements) != 2 || got.Requirements[0].Name != "Go" || got.Requirements[0].EvidenceText == nil ||
		*got.Requirements[0].EvidenceText != "5+ years" || got.Requirements[1].EvidenceText != nil {
		t.Fatalf("requirements = %+v (blank evidence must read back as NULL)", got.Requirements)
	}
	items, total, err := repo.List(ctx, owner, 10, 0)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("list = %+v, %d, %v", items, total, err)
	}
}

func TestOwnershipThroughRoleProfile(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	owner, other := s.user(), s.user()
	repo := repository.NewRepository(pool)
	created, err := repo.Create(ctx, owner, record(s.key("2")))
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Get(ctx, other, created.ID)
	wantCode(t, err, apperror.CodeJDNotFound)
	_, err = repo.Update(ctx, other, created.ID, record(""))
	wantCode(t, err, apperror.CodeJDNotFound)
	wantCode(t, repo.Delete(ctx, other, created.ID), apperror.CodeJDNotFound)
	if _, total, _ := repo.List(ctx, other, 10, 0); total != 0 {
		t.Fatal("another Candidate sees the JD in their list")
	}
	if got, err := repo.Get(ctx, owner, created.ID); err != nil || got.Title != "Backend Engineer" {
		t.Fatalf("the owner's JD changed after another user's attempts: %+v %v", got, err)
	}
}

func TestFailedConfirmWritesNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	owner := s.user()
	repo := repository.NewRepository(pool)

	// The last write of the transaction fails (an enum value PostgreSQL rejects), so the JD and
	// role profile written before it must roll back too.
	bad := record(s.key("3"))
	bad.Requirements = append(bad.Requirements, domain.RequirementRow{Name: "X", Target: "tool", Type: "mandatory"})
	_, err := repo.Create(ctx, owner, bad)
	wantCode(t, err, apperror.CodeInternal)

	// A level that isn't seeded fails before any insert.
	unknown := record(s.key("4"))
	unknown.Seniority = domain.Seniority("lvl-" + s.tag) // a name no levels row can have
	_, err = repo.Create(ctx, owner, unknown)
	wantCode(t, err, apperror.CodeInternal)

	if n := s.count(`SELECT count(*) FROM public.role_profiles WHERE candidate_id = $1`, owner); n != 0 {
		t.Fatalf("%d role profiles left after failed confirms", n)
	}
	if n := s.count(`SELECT count(*) FROM public.job_descriptions WHERE content_path LIKE $1 || '/%'`, s.tag); n != 0 {
		t.Fatalf("%d job descriptions left after failed confirms", n)
	}
}

func TestUpdateReplacesAndLocksAfterTheBank(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	owner := s.user()
	repo := repository.NewRepository(pool)
	created, err := repo.Create(ctx, owner, record(s.key("5")))
	if err != nil {
		t.Fatal(err)
	}

	change := record("")
	change.Title, change.Seniority, change.RefinementNote = "Lead Backend", domain.Lead, nil
	change.Requirements = change.Requirements[:1]
	updated, err := repo.Update(ctx, owner, created.ID, change)
	if err != nil || updated.ContentPath != s.key("5") {
		t.Fatalf("update: %+v %v", updated, err)
	}
	got, _ := repo.Get(ctx, owner, created.ID)
	if got.Title != "Lead Backend" || got.Seniority != domain.Lead || got.RefinementNote != nil || len(got.Requirements) != 1 {
		t.Fatalf("after update: %+v", got)
	}

	s.exec(`INSERT INTO public.core_questions (jd_id, content) VALUES ($1, 'q')`, created.ID)
	_, err = repo.Update(ctx, owner, created.ID, record(""))
	wantCode(t, err, apperror.CodeJDRequirementsLocked)
	got, _ = repo.Get(ctx, owner, created.ID)
	if !got.HasQuestionBank || got.Title != "Lead Backend" || len(got.Requirements) != 1 {
		t.Fatalf("a locked JD changed: %+v", got)
	}
}

func TestDeleteConflictRollsBack(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	owner := s.user()
	repo := repository.NewRepository(pool)
	created, err := repo.Create(ctx, owner, record(s.key("6")))
	if err != nil {
		t.Fatal(err)
	}
	s.exec(`INSERT INTO public.core_questions (jd_id, content) VALUES ($1, 'q')`, created.ID)
	s.exec(`INSERT INTO public.interviews (role_profile_id, type) SELECT id, 'practice' FROM public.role_profiles WHERE jd_id = $1`, created.ID)

	wantCode(t, repo.Delete(ctx, owner, created.ID), apperror.CodeJDInUse)
	got, err := repo.Get(ctx, owner, created.ID)
	if err != nil || len(got.Requirements) != 2 || !got.HasQuestionBank {
		t.Fatalf("a refused delete must keep every row: %+v %v", got, err)
	}

	s.exec(`DELETE FROM public.interviews WHERE role_profile_id IN (SELECT id FROM public.role_profiles WHERE jd_id = $1)`, created.ID)
	if err := repo.Delete(ctx, owner, created.ID); err != nil {
		t.Fatal(err)
	}
	left := s.count(`SELECT (SELECT count(*) FROM public.job_descriptions WHERE id = $1)
		+ (SELECT count(*) FROM public.role_profiles WHERE jd_id = $1)
		+ (SELECT count(*) FROM public.requirements WHERE jd_id = $1)
		+ (SELECT count(*) FROM public.core_questions WHERE jd_id = $1)`, created.ID)
	if left != 0 {
		t.Fatalf("%d rows left after delete", left)
	}
}

// The full confirm path against the real schema: the bank exists afterwards, the requirements
// are locked, and no Candidate-facing response ever carries the questions.
func TestConfirmGeneratesBankWithoutExposingIt(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	owner := s.user()
	extraction, err := service.New(noExtractor{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	app, err := service.NewApplication(extraction, repository.NewRepository(pool), &memoryStore{prefix: s.tag, objects: map[string]string{}}, insertingBank{pool: pool}, nil)
	if err != nil {
		t.Fatal(err)
	}
	created, err := app.Create(ctx, owner, domain.CreateInput{RawText: strings.Repeat("Backend role with Go. ", 10), StructuredJD: reviewed(), RefinementNote: "Exclude C#"})
	if err != nil {
		t.Fatal(err)
	}
	if created.QuestionBankReady {
		t.Fatal("confirm answers before the background generation finishes")
	}
	// Wait for the background generation, the way the frontend polls GET /jds/{id}.
	var got domain.JD
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err = app.Get(ctx, owner, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.QuestionBankReady || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !got.QuestionBankReady {
		t.Fatal("the bank was not generated in the background")
	}
	// Guards against a field that carries questions ever being added to the JD.
	for _, view := range []domain.JD{created, got} {
		data, _ := json.Marshal(view)
		if strings.Contains(string(data), "SECRET QUESTION") {
			t.Fatalf("a Candidate response contains question-bank data: %s", data)
		}
	}
	_, err = app.Update(ctx, owner, created.ID, domain.UpdateInput{StructuredJD: reviewed()})
	wantCode(t, err, apperror.CodeJDRequirementsLocked)
	_, err = app.Get(ctx, uuid.New(), created.ID)
	wantCode(t, err, apperror.CodeJDNotFound)
}
