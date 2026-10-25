//go:build integration

// Package tests holds the slow, opt-in tests of CV screening. The interview_slot cap, the posting
// row lock, ownership and the 24-hour deadline live in SQL, so only a real database can prove them.
// Until Applications can be submitted (KAN-112) the tests insert them directly. They need a migrated
// PostgreSQL database:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/application/tests -count=1
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return ctx, pool
}

// seed inserts rows and deletes exactly those rows afterwards, in reverse order.
type seed struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	undo []func()
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	return s
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) later(sql string, args ...any) {
	s.undo = append(s.undo, func() {
		if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	})
}

func (s *seed) user(role string) uuid.UUID {
	id := uuid.New()
	var company *string
	if role == "recruiter" {
		acme := "Acme"
		company = &acme
	}
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, $4, true, $5)`, id, "app "+role, fmt.Sprintf("app-%s@example.test", id), role, company)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

// posting adds a posting; slot nil means interview capacity was never funded.
func (s *seed) posting(recruiterID uuid.UUID, status string, slot *int32) uuid.UUID {
	s.t.Helper()
	var levelID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	jdID, id := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		jdID, "job-postings/"+jdID.String()+".pdf", levelID)
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, interview_slot)
		VALUES ($1, $2::public.job_posting_status, $3, $4, 'app posting', $5)`, id, status, jdID, recruiterID, slot)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	// Applications made through the service (apply) are not registered one by one; clear them before the posting.
	s.later(`DELETE FROM public.applications WHERE job_posting_id = $1`, id)
	return id
}

// application adds a pending Application (what KAN-112 will create from a CV submission).
func (s *seed) application(postingID, candidateID uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path) VALUES ($1, $2, $3, 'cvs/test.pdf')`,
		id, postingID, candidateID)
	s.later(`DELETE FROM public.applications WHERE id = $1`, id)
	return id
}

type stored struct {
	status   string
	deadline *time.Time
	reason   *string
}

func (s *seed) stored(applicationID uuid.UUID) stored {
	s.t.Helper()
	var got stored
	if err := s.pool.QueryRow(s.ctx, `SELECT status::text, interview_deadline, reject_reason FROM public.applications WHERE id = $1`,
		applicationID).Scan(&got.status, &got.deadline, &got.reason); err != nil {
		s.t.Fatal(err)
	}
	return got
}

func slot(n int32) *int32 { return &n }

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if got := codeOf(err); got != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func newService(t *testing.T, pool *pgxpool.Pool) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), &stubCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestApproveCVGrantsEligibilityWithA24HourDeadline(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, candidate := s.user("recruiter"), s.user("candidate")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, candidate)

	before := time.Now()
	got, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != appID || got.JobPostingID != postingID || got.Status != domain.StatusInterviewEligible || got.InterviewDeadline == nil {
		t.Fatalf("ApproveCV() = %+v", got)
	}
	// The database clock sets the deadline; allow for clock skew between the test host and Postgres.
	if d := got.InterviewDeadline.Sub(before); d < domain.InterviewWindow-time.Minute || d > domain.InterviewWindow+time.Minute {
		t.Fatalf("deadline is %v after approval, want about 24h", d)
	}
	row := s.stored(appID)
	if row.status != "interview_eligible" || row.deadline == nil || !row.deadline.Equal(*got.InterviewDeadline) {
		t.Fatalf("stored = %+v, want interview_eligible with the returned deadline", row)
	}

	// CV approval consumes no interview slot: nothing touches interviews or the posting.
	var interviews int
	var postingStatus string
	var postingSlot *int32
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.interviews WHERE application_id = $1`, appID).Scan(&interviews); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status::text, interview_slot FROM public.job_postings WHERE id = $1`, postingID).Scan(&postingStatus, &postingSlot); err != nil {
		t.Fatal(err)
	}
	if interviews != 0 || postingStatus != "open" || postingSlot == nil || *postingSlot != 3 {
		t.Fatalf("interviews=%d posting=%s slot=%v, want no interview and the posting untouched", interviews, postingStatus, postingSlot)
	}

	_, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, appID)
	assertCode(t, err, apperror.CodeApplicationNotPending)
}

func TestApprovalsStopAtInterviewSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(2))
	apps := []uuid.UUID{}
	for i := 0; i < 4; i++ {
		apps = append(apps, s.application(postingID, s.user("candidate")))
	}

	for _, id := range apps[:2] {
		if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, id); err != nil {
			t.Fatal(err)
		}
	}
	// The approval that takes the last slot closes intake and rejects what is still pending (KAN-111).
	var postingStatus string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM public.job_postings WHERE id = $1`, postingID).Scan(&postingStatus); err != nil {
		t.Fatal(err)
	}
	if postingStatus != "intake_closed" {
		t.Fatalf("posting status = %s, want intake_closed after the last slot", postingStatus)
	}
	for _, id := range apps[2:] {
		if row := s.stored(id); row.status != "rejected" || row.deadline != nil {
			t.Fatalf("pending Application = %+v, want it rejected by the intake close", row)
		}
	}
	// With nothing pending, approving or rejecting again reports the Application as already screened;
	// a missing Application still gets its own error.
	_, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, apps[2])
	assertCode(t, err, apperror.CodeApplicationNotPending)
	_, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, apps[0])
	assertCode(t, err, apperror.CodeApplicationNotPending)
	_, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, uuid.New())
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.RejectCV(ctx, owner, "recruiter", postingID, apps[3], nil)
	assertCode(t, err, apperror.CodeApplicationNotPending)
}

func TestEveryPastApprovalCountsTowardTheSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(2))
	missed := s.application(postingID, s.user("candidate"))
	decided := s.application(postingID, s.user("candidate"))
	next := s.application(postingID, s.user("candidate"))
	for _, id := range []uuid.UUID{missed, decided} {
		if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, id); err != nil {
			t.Fatal(err)
		}
	}
	// One Candidate missed the deadline (KAN-214) and one was approved after interviewing (KAN-115).
	s.exec(`UPDATE public.applications SET status = 'rejected' WHERE id = $1`, missed)
	s.exec(`UPDATE public.applications SET status = 'approved' WHERE id = $1`, decided)
	// The second approval closed intake and rejected `next`. Put both back to the state an open posting
	// with a pending Application would have, to prove the count itself still refuses a third approval.
	s.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, postingID)
	s.exec(`UPDATE public.applications SET status = 'pending', reject_reason = NULL WHERE id = $1`, next)

	_, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, next)
	assertCode(t, err, apperror.CodeInterviewCapacityFull)
}

func TestApprovalNeedsAnOpenFundedPosting(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")

	unfunded := s.posting(owner, "open", nil)
	_, err := svc.ApproveCV(ctx, owner, "recruiter", unfunded, s.application(unfunded, s.user("candidate")))
	assertCode(t, err, apperror.CodeInterviewCapacityNotFunded)

	for _, status := range []string{"pending", "rejected", "intake_closed", "closed"} {
		postingID := s.posting(owner, status, slot(5))
		appID := s.application(postingID, s.user("candidate"))
		_, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, appID)
		assertCode(t, err, apperror.CodeIntakeNotOpen)
		if row := s.stored(appID); row.status != "pending" {
			t.Fatalf("%s posting: Application = %+v, want it untouched", status, row)
		}
	}
}

func TestOwnershipAndMissingApplications(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, other := s.user("recruiter"), s.user("recruiter")
	postingID := s.posting(owner, "open", slot(5))
	otherPosting := s.posting(other, "open", slot(5))
	appID := s.application(postingID, s.user("candidate"))
	foreignApp := s.application(otherPosting, s.user("candidate"))

	// Another Recruiter cannot reach this posting, and the Application is untouched.
	_, err := svc.ApproveCV(ctx, other, "recruiter", postingID, appID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.RejectCV(ctx, other, "recruiter", postingID, appID, nil)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	if row := s.stored(appID); row.status != "pending" {
		t.Fatalf("Application = %+v after another Recruiter's attempts, want pending", row)
	}

	// An Application of another posting is not found through this one, even by its owner's ID.
	_, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, foreignApp)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.RejectCV(ctx, owner, "recruiter", postingID, foreignApp, nil)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	if row := s.stored(foreignApp); row.status != "pending" {
		t.Fatalf("foreign Application = %+v, want pending", row)
	}

	_, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, uuid.New())
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.ApproveCV(ctx, owner, "recruiter", uuid.New(), appID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestRejectCV(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(1))
	withReason := s.application(postingID, s.user("candidate"))
	withoutReason := s.application(postingID, s.user("candidate"))
	approved := s.application(postingID, s.user("candidate"))

	reason := "  Missing Go experience "
	got, err := svc.RejectCV(ctx, owner, "recruiter", postingID, withReason, &reason)
	if err != nil || got.Status != domain.StatusRejected || got.RejectReason == nil || *got.RejectReason != "Missing Go experience" || got.InterviewDeadline != nil {
		t.Fatalf("RejectCV() = %+v, %v", got, err)
	}
	if row := s.stored(withReason); row.status != "rejected" || row.reason == nil || *row.reason != "Missing Go experience" || row.deadline != nil {
		t.Fatalf("stored = %+v", row)
	}
	if _, err := svc.RejectCV(ctx, owner, "recruiter", postingID, withoutReason, nil); err != nil {
		t.Fatal(err)
	}
	if row := s.stored(withoutReason); row.status != "rejected" || row.reason != nil {
		t.Fatalf("stored = %+v, want rejected with no reason", row)
	}

	// Rejection is terminal, and a CV-approved Application cannot be rejected through screening.
	_, err = svc.RejectCV(ctx, owner, "recruiter", postingID, withReason, nil)
	assertCode(t, err, apperror.CodeApplicationNotPending)
	_, err = svc.ApproveCV(ctx, owner, "recruiter", postingID, withReason)
	assertCode(t, err, apperror.CodeApplicationNotPending)
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, approved); err != nil {
		t.Fatal(err)
	}
	_, err = svc.RejectCV(ctx, owner, "recruiter", postingID, approved, nil)
	assertCode(t, err, apperror.CodeApplicationNotPending)
	if row := s.stored(approved); row.status != "interview_eligible" {
		t.Fatalf("approved Application = %+v, want it still eligible", row)
	}
	// A rejected CV never counted toward the slot, so the one slot was free for the approval above.
}

func TestConcurrentApprovalsNeverExceedTheSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(2))
	const attempts = 8
	apps := make([]uuid.UUID, attempts)
	for i := range apps {
		apps[i] = s.application(postingID, s.user("candidate"))
	}

	var wg sync.WaitGroup
	errs := make([]error, attempts)
	start := make(chan struct{})
	for i := range apps {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.ApproveCV(ctx, owner, "recruiter", postingID, apps[i])
		}(i)
	}
	close(start)
	wg.Wait()

	succeeded := 0
	for _, err := range errs {
		switch codeOf(err) {
		case "":
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			succeeded++
		case apperror.CodeInterviewCapacityFull, apperror.CodeApplicationNotPending:
			// The approval that took the last slot closed intake and rejected the rest.
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	var eligible int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.applications WHERE job_posting_id = $1 AND status = 'interview_eligible'`,
		postingID).Scan(&eligible); err != nil {
		t.Fatal(err)
	}
	if succeeded != 2 || eligible != 2 {
		t.Fatalf("succeeded=%d eligible=%d, want exactly interview_slot (2)", succeeded, eligible)
	}
}

func TestInterviewEligibility(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner, candidate, stranger := s.user("recruiter"), s.user("candidate"), s.user("candidate")
	postingID := s.posting(owner, "open", slot(5))
	appID := s.application(postingID, candidate)

	_, err := svc.InterviewEligibility(ctx, candidate, "candidate", appID)
	assertCode(t, err, apperror.CodeApplicationNotEligible) // still pending

	approved, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.InterviewEligibility(ctx, candidate, "candidate", appID)
	if err != nil || got.ApplicationID != appID || got.JobPostingID != postingID || !got.InterviewDeadline.Equal(*approved.InterviewDeadline) {
		t.Fatalf("InterviewEligibility() = %+v, %v, want eligible until the approval deadline", got, err)
	}

	// Another Candidate cannot learn whether the Application exists.
	_, err = svc.InterviewEligibility(ctx, stranger, "candidate", appID)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.InterviewEligibility(ctx, candidate, "candidate", uuid.New())
	assertCode(t, err, apperror.CodeApplicationNotFound)

	// Eligibility survives intake close (the Candidate keeps the 24-hour window).
	s.exec(`UPDATE public.job_postings SET status = 'intake_closed' WHERE id = $1`, postingID)
	if _, err := svc.InterviewEligibility(ctx, candidate, "candidate", appID); err != nil {
		t.Fatalf("after intake close: %v, want still eligible", err)
	}

	s.exec(`UPDATE public.applications SET interview_deadline = now() - interval '1 second' WHERE id = $1`, appID)
	_, err = svc.InterviewEligibility(ctx, candidate, "candidate", appID)
	assertCode(t, err, apperror.CodeApplicationNotEligible)

	s.exec(`UPDATE public.applications SET interview_deadline = now() + interval '1 hour', status = 'interviewed' WHERE id = $1`, appID)
	_, err = svc.InterviewEligibility(ctx, candidate, "candidate", appID)
	assertCode(t, err, apperror.CodeApplicationNotEligible)
}
