//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// stubCVLinker stands in for object storage. The links it signs do not carry the storage key.
type stubCVLinker struct {
	keys []string
	ttls []time.Duration
	err  error
}

func (l *stubCVLinker) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	l.keys = append(l.keys, key)
	l.ttls = append(l.ttls, ttl)
	if l.err != nil {
		return storage.SignedRequest{}, l.err
	}
	return storage.SignedRequest{
		URL: "https://storage.example.test/signed?sig=test", Method: http.MethodGet, ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func detailService(t *testing.T, pool *pgxpool.Pool, linker *stubCVLinker) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), linker)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// sortedKeys returns the keys of a JSON object, sorted.
func sortedKeys(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("not a JSON object: %s", raw)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func assertKeys(t *testing.T, what string, raw json.RawMessage, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := sortedKeys(t, raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s keys = %v, want %v", what, got, want)
	}
}

func TestApplicationDetailForTheOwner(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	linker := &stubCVLinker{}
	svc := detailService(t, pool, linker)
	owner, candidate := s.user("recruiter"), s.user("candidate")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, candidate) // stored with CV path cvs/test.pdf

	before := time.Now()
	got, err := svc.GetDetail(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != appID || got.JobPostingID != postingID || got.JobPostingTitle != "app posting" ||
		got.Status != domain.StatusPending || got.Candidate.ID != candidate || got.Candidate.Email == "" {
		t.Fatalf("list fields = %+v", got.ListItem)
	}
	if got.JobPosting.ID != postingID || got.JobPosting.Title != "app posting" || got.JobPosting.Status != "open" ||
		got.JobPosting.InterviewSlot == nil || *got.JobPosting.InterviewSlot != 3 || got.JobPosting.CVApprovals != 0 {
		t.Fatalf("JobPosting = %+v", got.JobPosting)
	}
	if got.CV == nil || got.CV.URL == "" {
		t.Fatalf("CV = %+v, want a signed link", got.CV)
	}
	if d := got.CV.ExpiresAt.Sub(before); d < 4*time.Minute || d > 6*time.Minute {
		t.Fatalf("CV expires %v after the read, want about five minutes", d)
	}
	if len(linker.keys) != 1 || linker.keys[0] != "cvs/test.pdf" || linker.ttls[0] != 5*time.Minute {
		t.Fatalf("signed %v for %v, want cvs/test.pdf for 5m", linker.keys, linker.ttls)
	}
}

func TestApplicationDetailWithoutACVIsNull(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	linker := &stubCVLinker{}
	svc := detailService(t, pool, linker)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))
	s.exec(`UPDATE public.applications SET cv_path = NULL WHERE id = $1`, appID)

	got, err := svc.GetDetail(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CV != nil || len(linker.keys) != 0 {
		t.Fatalf("CV = %+v with %d signings, want null and no signing", got.CV, len(linker.keys))
	}
}

func TestApplicationDetailNotFoundCodes(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	linker := &stubCVLinker{}
	svc := detailService(t, pool, linker)
	owner, other := s.user("recruiter"), s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	otherPosting := s.posting(other, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))
	foreignApp := s.application(otherPosting, s.user("candidate"))

	// Another Recruiter's posting is not found, even with a real Application ID.
	_, err := svc.GetDetail(ctx, other, "recruiter", postingID, appID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)
	_, err = svc.GetDetail(ctx, owner, "recruiter", uuid.New(), appID)
	assertCode(t, err, apperror.CodeJobPostingNotFound)

	// An Application of another posting, or a random ID, is not on this posting.
	_, err = svc.GetDetail(ctx, owner, "recruiter", postingID, foreignApp)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.GetDetail(ctx, owner, "recruiter", postingID, uuid.New())
	assertCode(t, err, apperror.CodeApplicationNotFound)

	_, err = svc.GetDetail(ctx, owner, "candidate", postingID, appID)
	assertCode(t, err, apperror.CodeForbidden)
	if len(linker.keys) != 0 {
		t.Fatalf("a refused read signed %v", linker.keys)
	}
}

func TestApplicationDetailCountsRejectedAfterApproval(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := detailService(t, pool, &stubCVLinker{})
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	missed := s.application(postingID, s.user("candidate"))
	pending := s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, missed); err != nil {
		t.Fatal(err)
	}
	// The Candidate missed the deadline: the Application is rejected but was CV-approved once.
	s.exec(`UPDATE public.applications SET status = 'rejected' WHERE id = $1`, missed)

	got, err := svc.GetDetail(ctx, owner, "recruiter", postingID, pending)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobPosting.CVApprovals != 1 {
		t.Fatalf("cv_approvals = %d, want 1 (the rejected-after-approval Application counts)", got.JobPosting.CVApprovals)
	}
	rejected, err := svc.GetDetail(ctx, owner, "recruiter", postingID, missed)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != domain.StatusRejected || rejected.InterviewEligible || rejected.InterviewDeadline == nil {
		t.Fatalf("missed Application = %+v, want rejected with its deadline and not eligible", rejected.ListItem)
	}
}

func TestApplicationDetailEligibilityEndsAtTheDeadline(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := detailService(t, pool, &stubCVLinker{})
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, appID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetDetail(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.InterviewEligible || got.InterviewDeadline == nil || got.Status != domain.StatusInterviewEligible {
		t.Fatalf("before the deadline = %+v, want eligible with a deadline", got.ListItem)
	}

	s.exec(`UPDATE public.applications SET interview_deadline = now() - interval '1 second' WHERE id = $1`, appID)
	got, err = svc.GetDetail(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	if got.InterviewEligible || got.InterviewDeadline == nil {
		t.Fatalf("after the deadline = %+v, want not eligible with the deadline kept", got.ListItem)
	}
}

func TestApplicationDetailJSONHasTheDocumentedKeysOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := detailService(t, pool, &stubCVLinker{})
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, s.user("candidate"))
	if _, err := svc.ApproveCV(ctx, owner, "recruiter", postingID, appID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetDetail(ctx, owner, "recruiter", postingID, appID)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	assertKeys(t, "detail", body,
		"id", "job_posting_id", "job_posting_title", "status", "submitted_at", "candidate",
		"interview_eligible", "interview_deadline", "reject_reason", "updated_at", "job_posting", "cv",
		"interview", "final_review")

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, "candidate", fields["candidate"], "id", "name", "email", "image")
	assertKeys(t, "job_posting", fields["job_posting"], "id", "title", "status", "interview_slot", "cv_approvals")
	assertKeys(t, "cv", fields["cv"], "url", "expires_at")

	// The storage key and the column that holds it never leave the service.
	text := string(body)
	for _, forbidden := range []string{"cv_path", "cvs/test.pdf", "transcript", "recording", "evaluation"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("response contains %q: %s", forbidden, text)
		}
	}
}
