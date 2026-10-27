//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	moderationdomain "github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	moderationrepository "github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	bankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	bankrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	bankservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type deliveryGenerator func(context.Context, bankdomain.GenerationInput, int) (bankdomain.GenerationCandidate, error)

func (g deliveryGenerator) Generate(ctx context.Context, input bankdomain.GenerationInput, count int) (bankdomain.GenerationCandidate, error) {
	return g(ctx, input, count)
}

func generatedQuestions(count int) bankdomain.GenerationCandidate {
	questions := make([]string, count)
	for i := range questions {
		questions[i] = fmt.Sprintf("Explain technical topic %d.", i)
	}
	return bankdomain.GenerationCandidate{Questions: questions}
}

func deliveryService(t *testing.T, f fixture, size int, generator deliveryGenerator) *bankservice.Service {
	t.Helper()
	svc, err := bankservice.New(bankrepository.NewRepository(f.seed.pool), generator, 0, size)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func alwaysGenerate(_ context.Context, _ bankdomain.GenerationInput, count int) (bankdomain.GenerationCandidate, error) {
	return generatedQuestions(count), nil
}

// A new posting is prepared first: generating its bank does not submit it, and only a submitted
// posting with questions reaches the Admin.
func TestDeliveryGenerateReviewThenSubmit(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Delivery")
	calls := 0
	svc := deliveryService(t, f, 5, func(ctx context.Context, in bankdomain.GenerationInput, count int) (bankdomain.GenerationCandidate, error) {
		calls++
		if calls == 1 {
			return bankdomain.GenerationCandidate{}, errors.New("upstream unavailable")
		}
		return alwaysGenerate(ctx, in, count)
	})
	moderation := moderationrepository.NewRepository(f.seed.pool)

	if err := f.repo.Submit(ctx, f.recruiter, id); codeOf(err) != apperror.CodeQuestionBankNotReady {
		t.Fatalf("submit before generation: %v", err)
	}
	if _, err := svc.Add(ctx, f.recruiter, "recruiter", id, "Skip generation?"); codeOf(err) != apperror.CodeQuestionBankNotReady {
		t.Fatalf("manual question before generation: %v", err)
	}
	if _, err := svc.Generate(ctx, f.recruiter, "recruiter", id); codeOf(err) != apperror.CodeQuestionGenerationFailed {
		t.Fatalf("generation failure: %v", err)
	}
	questions, err := svc.Generate(ctx, f.recruiter, "recruiter", id)
	if err != nil || len(questions) != 5 {
		t.Fatalf("retry: questions=%d err=%v", len(questions), err)
	}
	if _, err := svc.Add(ctx, f.recruiter, "recruiter", id, "What is a goroutine leak?"); err != nil {
		t.Fatalf("add after generation: %v", err)
	}
	posting, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil || posting.CoreQuestionCount != 6 || posting.ReviewSubmitted {
		t.Fatalf("generated bank must await explicit submission: %+v %v", posting.JobPosting, err)
	}
	if err := moderation.Decide(ctx, id, moderationdomain.PostingStatusOpen, nil); !errors.Is(err, moderationdomain.ErrNotPending) {
		t.Fatalf("unsubmitted posting was decided: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := f.repo.Submit(ctx, f.recruiter, id); err != nil {
			t.Fatalf("submission %d: %v", i, err)
		}
	}
	if err := moderation.Decide(ctx, id, moderationdomain.PostingStatusOpen, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(ctx, f.recruiter, "recruiter", id, "Too late"); codeOf(err) != apperror.CodeQuestionBankLocked {
		t.Fatalf("edit after approval: %v", err)
	}
}

// Deleting the last question takes a submitted posting out of the queue.
func TestDeliveryLastDeleteWithdrawsSubmission(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Last question")
	svc := deliveryService(t, f, 1, alwaysGenerate)
	questions, err := svc.Generate(ctx, f.recruiter, "recruiter", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Submit(ctx, f.recruiter, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, f.recruiter, "recruiter", id, questions[0].ID); err != nil {
		t.Fatal(err)
	}
	posting, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil || posting.ReviewSubmitted || posting.CoreQuestionCount != 0 {
		t.Fatalf("emptied bank stayed submitted: %+v %v", posting.JobPosting, err)
	}
	if err := moderationrepository.NewRepository(f.seed.pool).Decide(ctx, id, moderationdomain.PostingStatusOpen, nil); err == nil {
		t.Fatal("approved a posting with an empty bank")
	}
}

// A rejected posting is corrected while rejected: changing a generation input clears the bank, the
// Recruiter generates and edits it again, then resubmits.
func TestDeliveryRejectedCorrectionAndResubmission(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Correct a rejected posting")
	var notes []string
	svc := deliveryService(t, f, 3, func(ctx context.Context, in bankdomain.GenerationInput, count int) (bankdomain.GenerationCandidate, error) {
		notes = append(notes, in.RefinementNote)
		return alwaysGenerate(ctx, in, count)
	})
	if _, err := svc.Generate(ctx, f.recruiter, "recruiter", id); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Submit(ctx, f.recruiter, id); err != nil {
		t.Fatal(err)
	}
	reason := "Clarify the role"
	if err := moderationrepository.NewRepository(f.seed.pool).Decide(ctx, id, moderationdomain.PostingStatusRejected, &reason); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Submit(ctx, f.recruiter, id); codeOf(err) != apperror.CodeJobPostingNotPending {
		t.Fatalf("submit instead of resubmit: %v", err)
	}
	note := "Focus on Java"
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{RefinementNote: domain.StringChange{Present: true, Value: &note}}, nil); err != nil {
		t.Fatal(err)
	}
	posting, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil || posting.CoreQuestionCount != 0 || posting.ReviewSubmitted || posting.RefinementNote != note {
		t.Fatalf("input change kept the old bank: %+v %v", posting.JobPosting, err)
	}
	if err := f.repo.Resubmit(ctx, f.recruiter, id); codeOf(err) != apperror.CodeQuestionBankNotReady {
		t.Fatalf("resubmitted without a bank: %v", err)
	}
	questions, err := svc.Generate(ctx, f.recruiter, "recruiter", id)
	if err != nil {
		t.Fatalf("generate while rejected: %v", err)
	}
	if _, err := svc.Update(ctx, f.recruiter, "recruiter", id, questions[0].ID, "Refined question"); err != nil {
		t.Fatalf("edit while rejected: %v", err)
	}
	if err := f.repo.Resubmit(ctx, f.recruiter, id); err != nil {
		t.Fatal(err)
	}
	posting, err = f.repo.Get(ctx, f.recruiter, id)
	if err != nil || posting.Status != domain.StatusPending || !posting.ReviewSubmitted || posting.RejectReason != nil {
		t.Fatalf("resubmission: %+v %v", posting.JobPosting, err)
	}
	if len(notes) != 2 || notes[1] != note {
		t.Fatalf("regeneration did not use the new refinement note: %q", notes)
	}
}

func TestDeliveryLockedStates(t *testing.T) {
	ctx, f := newFixture(t)
	svc := deliveryService(t, f, 3, func(context.Context, bankdomain.GenerationInput, int) (bankdomain.GenerationCandidate, error) {
		t.Error("a locked posting reached the LLM")
		return bankdomain.GenerationCandidate{}, nil
	})
	for _, status := range []string{"open", "intake_closed", "closed"} {
		id := f.create(t, ctx, status)
		f.seed.exec(`UPDATE public.job_postings SET status = $2::public.job_posting_status WHERE id = $1`, id, status)
		if _, err := svc.Generate(ctx, f.recruiter, "recruiter", id); codeOf(err) != apperror.CodeQuestionBankLocked {
			t.Fatalf("%s generation: %v", status, err)
		}
		if err := f.repo.Submit(ctx, f.recruiter, id); codeOf(err) != apperror.CodeJobPostingNotEditable {
			t.Fatalf("%s submission: %v", status, err)
		}
	}
}

// The backfill in migration 000013 keeps pending postings that have a bank in the Admin queue and
// sends pending postings without questions back to preparation. Its UPDATE runs here, scoped to
// this test's postings and rolled back, so the shared database is left as it was.
func TestDeliveryMigrationBackfill(t *testing.T) {
	ctx, f := newFixture(t)
	source, err := os.ReadFile("../../../../migrations/000013_posting_question_bank_delivery.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	backfill := ""
	for _, statement := range strings.Split(string(source), ";") {
		if i := strings.Index(statement, "UPDATE public.job_postings"); i >= 0 {
			backfill = statement[i:]
		}
	}
	if !strings.Contains(backfill, "WHERE ") {
		t.Fatal("backfill UPDATE not found in migration 000013")
	}
	backfill = strings.Replace(backfill, "WHERE ", "WHERE jp.id = ANY($1) AND ", 1)

	cases := map[string]struct {
		status    string
		questions bool
		submitted bool
	}{
		"pending with bank":  {"pending", true, true},
		"pending empty bank": {"pending", false, false},
		"rejected with bank": {"rejected", true, false},
	}
	ids := map[string]uuid.UUID{}
	all := []uuid.UUID{}
	for name, c := range cases {
		id := f.create(t, ctx, name)
		ids[name] = id
		all = append(all, id)
		f.seed.exec(`UPDATE public.job_postings SET status = $2::public.job_posting_status, review_submitted = false WHERE id = $1`, id, c.status)
		if c.questions {
			f.seed.exec(`INSERT INTO public.core_questions (jd_id, content) SELECT jd_id, 'Existing question' FROM public.job_postings WHERE id = $1`, id)
		}
	}
	tx, err := f.seed.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, backfill, all); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	for name, c := range cases {
		var submitted bool
		if err := tx.QueryRow(ctx, `SELECT review_submitted FROM public.job_postings WHERE id = $1`, ids[name]).Scan(&submitted); err != nil {
			t.Fatal(err)
		}
		if submitted != c.submitted {
			t.Fatalf("%s: submitted=%t, want %t", name, submitted, c.submitted)
		}
	}
}

// waitForLockWait polls until another session is blocked on a row lock, so a test can commit the
// transaction that holds it only once the competing call is really waiting (KAN-211).
func waitForLockWait(t *testing.T, ctx context.Context, f fixture) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := f.seed.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the competing call never waited on the posting lock")
}

// KAN-211, approval first: an approval that is still committing holds the posting row, so deleting
// the last question waits for it, then finds an open posting and is refused. The posting keeps its
// question instead of opening with an empty bank.
func TestDeliveryDeleteWaitsForApprovalAndIsRefused(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Approval first")
	svc := deliveryService(t, f, 1, alwaysGenerate)
	questions, err := svc.Generate(ctx, f.recruiter, "recruiter", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Submit(ctx, f.recruiter, id); err != nil {
		t.Fatal(err)
	}

	approval, err := f.seed.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = approval.Rollback(ctx) }()
	if _, err := approval.Exec(ctx, `UPDATE public.job_postings SET status = 'open' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- svc.Delete(ctx, f.recruiter, "recruiter", id, questions[0].ID) }()
	waitForLockWait(t, ctx, f)
	if err := approval.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; codeOf(err) != apperror.CodeQuestionBankLocked {
		t.Fatalf("delete after approval: %v", err)
	}
	posting, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil || posting.Status != domain.StatusOpen || posting.CoreQuestionCount != 1 {
		t.Fatalf("open posting lost its question: %+v %v", posting.JobPosting, err)
	}
}

// KAN-211, edit first: while the Recruiter's transaction holds the posting row and empties the bank,
// the approval waits, then rechecks the withdrawn submission and refuses to open the posting.
func TestDeliveryApprovalWaitsForLastDeleteAndIsRefused(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Delete first")
	svc := deliveryService(t, f, 1, alwaysGenerate)
	questions, err := svc.Generate(ctx, f.recruiter, "recruiter", id)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Submit(ctx, f.recruiter, id); err != nil {
		t.Fatal(err)
	}

	// The same statements Delete runs, held open so the approval has to queue behind them.
	edit, err := f.seed.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = edit.Rollback(ctx) }()
	if _, err := edit.Exec(ctx, `SELECT jd_id FROM public.job_postings WHERE id = $1 FOR UPDATE`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := edit.Exec(ctx, `DELETE FROM public.core_questions WHERE id = $1`, questions[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := edit.Exec(ctx, `UPDATE public.job_postings SET review_submitted = false WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- moderationrepository.NewRepository(f.seed.pool).Decide(ctx, id, moderationdomain.PostingStatusOpen, nil)
	}()
	waitForLockWait(t, ctx, f)
	if err := edit.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, moderationdomain.ErrNotPending) {
		t.Fatalf("approval after the bank emptied: %v", err)
	}
	posting, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil || posting.Status != domain.StatusPending || posting.ReviewSubmitted || posting.CoreQuestionCount != 0 {
		t.Fatalf("emptied posting was opened or stayed queued: %+v %v", posting.JobPosting, err)
	}
}
