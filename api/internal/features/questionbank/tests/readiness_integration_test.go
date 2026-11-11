//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	moderationdomain "github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	moderationrepository "github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	jobpostingrepository "github.com/cog-forge/rolecue/api/internal/features/jobposting/repository"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// KAN-331: readiness, withdrawal of a submission, and approval versus edit, on the real repositories.

type gradingFixture struct {
	s         *seed
	recruiter uuid.UUID
	rubric    uuid.UUID
	posting   uuid.UUID
	jd        uuid.UUID
	bank      *qbrepository.Repository
	jobs      *jobpostingrepository.Repository
	moderate  *moderationrepository.Repository
}

func newGradingFixture(t *testing.T, ctx context.Context, s *seed) gradingFixture {
	t.Helper()
	level := s.level("qb-"+s.tag, "d")
	recruiter := s.user("recruiter")
	rubric := s.rubric("qb readiness "+s.tag, &recruiter, true) // before the questions: cleanup runs in reverse
	jd, posting := s.jdWithPosting(level, recruiter, true)
	s.presentation(recruiter, posting)
	return gradingFixture{s: s, recruiter: recruiter, rubric: rubric, posting: posting, jd: jd,
		bank:     qbrepository.NewRepository(s.pool),
		jobs:     jobpostingrepository.NewRepository(s.pool),
		moderate: moderationrepository.NewRepository(s.pool)}
}

func (f gradingFixture) fill(t *testing.T, ctx context.Context, questions ...domain.GeneratedQuestion) []domain.BankQuestion {
	t.Helper()
	saved, err := f.bank.SaveForPosting(ctx, f.recruiter, f.posting, questions)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func (f gradingFixture) goodQuestion(content string) domain.GeneratedQuestion {
	return domain.GeneratedQuestion{Content: content, RubricID: &f.rubric, ExpectedAnswer: answer("Key points of a strong answer.")}
}

func (f gradingFixture) state(t *testing.T, ctx context.Context) (status string, submitted bool) {
	t.Helper()
	if err := f.s.pool.QueryRow(ctx, `SELECT status::text, review_submitted FROM public.job_postings WHERE id = $1`, f.posting).Scan(&status, &submitted); err != nil {
		t.Fatal(err)
	}
	return status, submitted
}

// Every way a question can be incomplete is seen the same way by the Recruiter's bank view, by Submit and by
// the Admin's approval: they apply one rule.
func TestReadinessRuleIsTheSameInTheBankViewSubmitAndApproval(t *testing.T) {
	ctx, pool := openPool(t, 90*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	other := s.user("recruiter")
	theirs := s.rubric("qb theirs "+s.tag, &other, true)
	archived := func(f gradingFixture) uuid.UUID {
		return s.rubric("qb archived "+s.tag+uuid.NewString()[:4], &f.recruiter, false)
	}
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

	type tc struct {
		name   string
		build  func(f gradingFixture) domain.GeneratedQuestion
		issues []string
	}
	cases := []tc{
		{"complete", func(f gradingFixture) domain.GeneratedQuestion { return f.goodQuestion("Q?") }, nil},
		{"no rubric", func(f gradingFixture) domain.GeneratedQuestion {
			return domain.GeneratedQuestion{Content: "Q?", ExpectedAnswer: answer("a")}
		}, []string{domain.IssueMissingRubric}},
		{"no expected answer", func(f gradingFixture) domain.GeneratedQuestion {
			return domain.GeneratedQuestion{Content: "Q?", RubricID: &f.rubric}
		}, []string{domain.IssueMissingExpectedAnswer}},
		{"neither", func(f gradingFixture) domain.GeneratedQuestion { return domain.GeneratedQuestion{Content: "Q?"} },
			[]string{domain.IssueMissingRubric, domain.IssueMissingExpectedAnswer}},
		{"expected answer at the word limit", func(f gradingFixture) domain.GeneratedQuestion {
			return domain.GeneratedQuestion{Content: "Q?", RubricID: &f.rubric, ExpectedAnswer: answer(words(domain.MaxExpectedAnswerWords))}
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newGradingFixture(t, ctx, s)
			f.fill(t, ctx, c.build(f))
			assertReadiness(t, ctx, f, c.issues)
		})
	}

	// States that cannot be produced through the generator are set directly, as a later edit, an archive or
	// bad data would.
	direct := []struct {
		name   string
		mutate func(f gradingFixture)
		issues []string
	}{
		{"rubric archived after assignment", func(f gradingFixture) {
			s.exec(`UPDATE public.rubrics SET is_active = false WHERE id = $1`, f.rubric)
		}, []string{domain.IssueRubricArchived}},
		{"rubric of another recruiter", func(f gradingFixture) {
			s.exec(`UPDATE public.core_questions SET rubric_id = $2 WHERE jd_id = $1`, f.jd, theirs)
		}, []string{domain.IssueRubricUnavailable}},
		{"blank expected answer", func(f gradingFixture) {
			s.exec(`UPDATE public.core_questions SET expected_answer = E'  \n ' WHERE jd_id = $1`, f.jd)
		}, []string{domain.IssueMissingExpectedAnswer}},
		{"expected answer over the word limit", func(f gradingFixture) {
			s.exec(`UPDATE public.core_questions SET expected_answer = $2 WHERE jd_id = $1`, f.jd, words(domain.MaxExpectedAnswerWords+1))
		}, []string{domain.IssueExpectedAnswerInvalid}},
		{"expected answer over the character limit", func(f gradingFixture) {
			s.exec(`UPDATE public.core_questions SET expected_answer = $2 WHERE jd_id = $1`, f.jd, strings.Repeat("界", domain.MaxExpectedAnswerRunes+1))
		}, []string{domain.IssueExpectedAnswerInvalid}},
		{"expected answer at the character limit", func(f gradingFixture) {
			s.exec(`UPDATE public.core_questions SET expected_answer = $2 WHERE jd_id = $1`, f.jd, strings.Repeat("界", domain.MaxExpectedAnswerRunes))
		}, nil},
	}
	_ = archived
	for _, c := range direct {
		t.Run(c.name, func(t *testing.T) {
			f := newGradingFixture(t, ctx, s)
			f.fill(t, ctx, f.goodQuestion("Q?"))
			c.mutate(f)
			assertReadiness(t, ctx, f, c.issues)
		})
	}
}

func assertReadiness(t *testing.T, ctx context.Context, f gradingFixture, wantIssues []string) {
	t.Helper()
	listed, err := f.bank.ListBank(ctx, f.jd, f.recruiter)
	if err != nil || len(listed) != 1 {
		t.Fatalf("bank view: %v, %v", listed, err)
	}
	if got := strings.Join(listed[0].GradingIssues, ","); got != strings.Join(wantIssues, ",") || listed[0].GradingIncomplete != (len(wantIssues) > 0) {
		t.Fatalf("issues = %q (incomplete=%v), want %q", got, listed[0].GradingIncomplete, wantIssues)
	}
	// The pure rule must agree with what the database says about the same row.
	var rubricID *uuid.UUID
	var expected *string
	var active, permitted bool
	if err := f.s.pool.QueryRow(ctx, `SELECT cq.rubric_id, cq.expected_answer, COALESCE(r.is_active, false), (r.id IS NOT NULL AND (r.scope = 'system' OR r.owner_id = $2))
		FROM public.core_questions cq LEFT JOIN public.rubrics r ON r.id = cq.rubric_id WHERE cq.jd_id = $1`, f.jd, f.recruiter).Scan(&rubricID, &expected, &active, &permitted); err != nil {
		t.Fatal(err)
	}
	if got := domain.GradingIssuesOf(rubricID, domain.RubricState{Active: active, Permitted: permitted}, expected); strings.Join(got, ",") != strings.Join(wantIssues, ",") {
		t.Fatalf("GradingIssuesOf = %v, want %v", got, wantIssues)
	}
	submitErr := f.jobs.Submit(ctx, f.recruiter, f.posting)
	if len(wantIssues) > 0 {
		if codeOf(submitErr) != apperror.CodeGradingConfigIncomplete {
			t.Fatalf("submit = %v, want GRADING_CONFIG_INCOMPLETE", submitErr)
		}
		if _, submitted := f.state(t, ctx); submitted {
			t.Fatal("an incomplete posting reached the Admin queue")
		}
		// Force it into the queue (as old data could be) to prove the approval applies the rule on its own.
		f.s.exec(`UPDATE public.job_postings SET review_submitted = true WHERE id = $1`, f.posting)
		if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); !errors.Is(err, moderationdomain.ErrGradingIncomplete) {
			t.Fatalf("approval = %v, want ErrGradingIncomplete", err)
		}
		if status, _ := f.state(t, ctx); status != "pending" {
			t.Fatalf("status = %s after a refused approval", status)
		}
		// Rejecting never depends on grading.
		if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusRejected, answer("fix it")); err != nil {
			t.Fatalf("reject: %v", err)
		}
		return
	}
	if submitErr != nil {
		t.Fatalf("submit of a complete posting: %v", submitErr)
	}
	if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); err != nil {
		t.Fatalf("approval of a complete posting: %v", err)
	}
}

// Any change to a question withdraws a submitted posting; saving the same text again does not.
func TestBankChangesWithdrawTheSubmission(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()

	type step struct {
		name   string
		act    func(f gradingFixture, q domain.BankQuestion) error
		change bool
	}
	steps := []step{
		{"edit the text", func(f gradingFixture, q domain.BankQuestion) error {
			_, err := f.bank.Update(ctx, f.recruiter, f.posting, q.ID, "A different question?")
			return err
		}, true},
		{"save the same text", func(f gradingFixture, q domain.BankQuestion) error {
			_, err := f.bank.Update(ctx, f.recruiter, f.posting, q.ID, q.Content)
			return err
		}, false},
		{"add a question", func(f gradingFixture, q domain.BankQuestion) error {
			_, err := f.bank.Add(ctx, f.recruiter, f.posting, "Added by hand?")
			return err
		}, true},
		{"delete a question", func(f gradingFixture, q domain.BankQuestion) error {
			return f.bank.Delete(ctx, f.recruiter, f.posting, q.ID)
		}, true},
	}
	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			f := newGradingFixture(t, ctx, s)
			saved := f.fill(t, ctx, f.goodQuestion("Q1?"), f.goodQuestion("Q2?"))
			if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
				t.Fatal(err)
			}
			if err := st.act(f, saved[0]); err != nil {
				t.Fatal(err)
			}
			_, submitted := f.state(t, ctx)
			if submitted == st.change {
				t.Fatalf("review_submitted = %v after %q, want %v", submitted, st.name, !st.change)
			}
			if st.change {
				// The Admin cannot approve what the Recruiter changed after submitting.
				if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); !errors.Is(err, moderationdomain.ErrNotPending) {
					t.Fatalf("approval of a withdrawn submission = %v, want ErrNotPending", err)
				}
			}
		})
	}
}

// Approval and an edit run one after the other, never interleaved: either the approval wins and the edit is
// refused as locked, or the edit wins and withdraws the submission so the approval is refused. The approved
// bank never contains an edit made after the Admin's decision, and an edit made before it is never approved
// unseen.
func TestApprovalRacingAnEditHasOneConsistentOutcome(t *testing.T) {
	ctx, pool := openPool(t, 3*time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	approvedFirst, editedFirst := 0, 0
	for i := 0; i < 25; i++ {
		f := newGradingFixture(t, ctx, s)
		saved := f.fill(t, ctx, f.goodQuestion("Original?"))
		if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
			t.Fatal(err)
		}
		var approveErr, editErr error
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			approveErr = f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, editErr = f.bank.Update(ctx, f.recruiter, f.posting, saved[0].ID, "Edited?")
		}()
		close(start)
		wg.Wait()

		status, submitted := f.state(t, ctx)
		var content string
		if err := pool.QueryRow(ctx, `SELECT content FROM public.core_questions WHERE id = $1`, saved[0].ID).Scan(&content); err != nil {
			t.Fatal(err)
		}
		switch {
		case approveErr == nil && codeOf(editErr) == apperror.CodeQuestionBankLocked:
			approvedFirst++
			if status != "open" || content != "Original?" {
				t.Fatalf("approval won but status=%s content=%q", status, content)
			}
		case editErr == nil && errors.Is(approveErr, moderationdomain.ErrNotPending):
			editedFirst++
			if status != "pending" || submitted || content != "Edited?" {
				t.Fatalf("edit won but status=%s submitted=%v content=%q", status, submitted, content)
			}
		default:
			t.Fatalf("iteration %d: approve=%v edit=%v (status=%s content=%q)", i, approveErr, editErr, status, content)
		}
	}
	t.Logf("approval first %d times, edit first %d times", approvedFirst, editedFirst)
}

// An archive racing an approval: whichever commits first is a legal order, and no foreign key is ever broken.
func TestApprovalRacingAnArchiveIsConsistent(t *testing.T) {
	ctx, pool := openPool(t, 3*time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	approved, refused := 0, 0
	for i := 0; i < 20; i++ {
		f := newGradingFixture(t, ctx, s)
		f.fill(t, ctx, f.goodQuestion("Q?"))
		if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
			t.Fatal(err)
		}
		var approveErr error
		var archiveErr error
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			approveErr = f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, archiveErr = pool.Exec(ctx, `UPDATE public.rubrics SET is_active = false WHERE id = $1`, f.rubric)
		}()
		close(start)
		wg.Wait()
		if archiveErr != nil {
			t.Fatal(archiveErr)
		}
		status, _ := f.state(t, ctx)
		var rubricID *uuid.UUID
		if err := pool.QueryRow(ctx, `SELECT rubric_id FROM public.core_questions WHERE jd_id = $1`, f.jd).Scan(&rubricID); err != nil || rubricID == nil || *rubricID != f.rubric {
			t.Fatalf("the question lost its rubric: %v, %v", rubricID, err)
		}
		switch {
		case approveErr == nil && status == "open":
			approved++ // approved first, archived afterwards: the reference is kept
		case errors.Is(approveErr, moderationdomain.ErrGradingIncomplete) && status == "pending":
			refused++ // archived first: not approvable until the Recruiter picks an active rubric
		default:
			t.Fatalf("iteration %d: approve=%v status=%s", i, approveErr, status)
		}
	}
	t.Logf("approved first %d times, refused after the archive %d times", approved, refused)
}

// Concurrent approvals and rejections of one submitted posting have exactly one winner.
func TestConcurrentDecisionsHaveOneWinner(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	f := newGradingFixture(t, ctx, s)
	f.fill(t, ctx, f.goodQuestion("Q?"))
	if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
		t.Fatal(err)
	}
	results := make([]error, 6)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status := moderationdomain.PostingStatusOpen
			if i%2 == 1 {
				status = moderationdomain.PostingStatusRejected
			}
			results[i] = f.moderate.Decide(ctx, f.posting, status, answer(fmt.Sprintf("reason %d", i)))
		}()
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, err := range results {
		switch {
		case err == nil:
			wins++
		case !errors.Is(err, moderationdomain.ErrNotPending):
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d decisions won, want exactly one", wins)
	}
}

// A submission that the Recruiter withdrew by editing is submitted again explicitly; a posting that went to
// the Admin queue is visible there only while submitted.
func TestWithdrawnPostingCanBeSubmittedAgain(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	f := newGradingFixture(t, ctx, s)
	saved := f.fill(t, ctx, f.goodQuestion("Q?"))
	if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
		t.Fatal(err)
	}
	if _, err := f.bank.Update(ctx, f.recruiter, f.posting, saved[0].ID, "Better wording?"); err != nil {
		t.Fatal(err)
	}
	if _, submitted := f.state(t, ctx); submitted {
		t.Fatal("the edit did not withdraw the submission")
	}
	if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
		t.Fatalf("submit again: %v", err)
	}
	if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); err != nil {
		t.Fatalf("approval after the new submission: %v", err)
	}
	// Once approved, the bank is locked: no add, edit or delete, grading data included.
	if _, err := f.bank.Update(ctx, f.recruiter, f.posting, saved[0].ID, "Too late?"); codeOf(err) != apperror.CodeQuestionBankLocked {
		t.Fatalf("edit after approval: %v", err)
	}
	if err := f.bank.Delete(ctx, f.recruiter, f.posting, saved[0].ID); codeOf(err) != apperror.CodeQuestionBankLocked {
		t.Fatalf("delete after approval: %v", err)
	}
	if _, err := f.bank.Add(ctx, f.recruiter, f.posting, "Too late?"); codeOf(err) != apperror.CodeQuestionBankLocked {
		t.Fatalf("add after approval: %v", err)
	}
}

// Several drafts may share one rubric; deleting it is refused while any of them references it, archiving is
// not, and the references survive.
func TestMultipleDraftsShareARubricSafely(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	f1 := newGradingFixture(t, ctx, s)
	level := s.level("qb2-"+s.tag, "d")
	jd2, posting2 := s.jdWithPosting(level, f1.recruiter, true)
	f1.fill(t, ctx, f1.goodQuestion("Draft one?"))
	if _, err := f1.bank.SaveForPosting(ctx, f1.recruiter, posting2, []domain.GeneratedQuestion{f1.goodQuestion("Draft two?")}); err != nil {
		t.Fatal(err)
	}
	var refs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.core_questions WHERE rubric_id = $1`, f1.rubric).Scan(&refs); err != nil || refs != 2 {
		t.Fatalf("references = %d, %v", refs, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.rubrics WHERE id = $1`, f1.rubric); err == nil {
		t.Fatal("a rubric referenced by two drafts was hard-deleted")
	}
	s.exec(`UPDATE public.rubrics SET is_active = false WHERE id = $1`, f1.rubric)
	for _, jd := range []uuid.UUID{f1.jd, jd2} {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.core_questions WHERE jd_id = $1 AND rubric_id = $2`, jd, f1.rubric).Scan(&n); err != nil || n != 1 {
			t.Fatalf("a draft lost its reference after the archive: %d, %v", n, err)
		}
	}
	// Archived: a new generation no longer offers it, and archiving did not edit any question.
	catalog, err := f1.bank.RubricCatalog(ctx, f1.recruiter)
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range catalog {
		if option.ID == f1.rubric {
			t.Fatal("an archived rubric is still offered to new generations")
		}
	}
	_ = posting2
}
