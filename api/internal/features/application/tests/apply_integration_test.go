//go:build integration

package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestApplyHappyPathStoresPendingAndShowsItToTheRecruiter(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	store := &fakeCVStore{}
	svc := newApplyService(t, pool, store)
	owner, candidate := s.user("recruiter"), s.user("candidate")
	postingID := s.posting(owner, "open", slot(3))

	got, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(64))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.JobPostingID != postingID || got.Status != domain.StatusPending || got.ID == uuid.Nil {
		t.Fatalf("Apply() = %+v, want a pending Application on the posting", got)
	}

	row, n := s.applicationRowFor(t, postingID, candidate)
	if n != 1 {
		t.Fatalf("stored Applications = %d, want 1", n)
	}
	if row.status != "pending" || row.cvPath == nil || *row.cvPath == "" {
		t.Fatalf("stored row = %+v, want pending with cv_path set", row)
	}
	if store.uploadCount() != 1 || *row.cvPath != store.uploads[0] {
		t.Fatalf("cv_path = %v, want the uploaded key %v", row.cvPath, store.uploads)
	}
	if len(store.deleted()) != 0 {
		t.Fatalf("a successful apply deleted %v, want nothing", store.deleted())
	}

	// The owning Recruiter sees the Application at once, without an interview.
	items, total, err := svc.ListOwn(ctx, owner, "recruiter", domain.ListFilter{JobPostingID: &postingID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != got.ID || items[0].Status != domain.StatusPending {
		t.Fatalf("recruiter list = %+v (total %d), want the new pending Application", items, total)
	}
}

func TestSecondApplyToSamePostingIsAlreadyApplied(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	store := &fakeCVStore{}
	svc := newApplyService(t, pool, store)
	candidate := s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", slot(3))

	if _, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(32)); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(32))
	assertCode(t, err, apperror.CodeAlreadyApplied)

	if n := applicationCount(t, s, postingID); n != 1 {
		t.Fatalf("stored Applications = %d, want 1", n)
	}
	// The rejected second CV must not stay in storage.
	if deleted := store.deleted(); len(deleted) != 1 || deleted[0] != store.uploads[1] {
		t.Fatalf("deleted = %v, want only the rejected second upload %v", deleted, store.uploads)
	}
}

func TestConcurrentApplySameCandidateStoresOneRow(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	store := &fakeCVStore{}
	svc := newApplyService(t, pool, store)
	candidate := s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", slot(3))

	const attempts = 6
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(32))
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
		case apperror.CodeAlreadyApplied:
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("succeeded = %d, want exactly one apply to win", succeeded)
	}
	if n := applicationCount(t, s, postingID); n != 1 {
		t.Fatalf("stored Applications = %d, want 1", n)
	}
	// Every losing upload was deleted, so exactly one CV object remains.
	if got := store.uploadCount() - len(store.deleted()); got != 1 {
		t.Fatalf("CV objects left = %d, want 1 (uploads %d, deletes %d)", got, store.uploadCount(), len(store.deleted()))
	}
}

func TestThirtyCandidatesApplyBeyondInterviewSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newApplyService(t, pool, &fakeCVStore{})
	postingID := s.posting(s.user("recruiter"), "open", slot(3))

	for i := 0; i < 30; i++ {
		if _, err := svc.Apply(ctx, s.user("candidate"), "candidate", postingID, pdfBytes(16)); err != nil {
			t.Fatalf("candidate %d: Apply() error = %v, want success (submissions are not capped by interview_slot)", i, err)
		}
	}
	if n := applicationCount(t, s, postingID); n != 30 {
		t.Fatalf("stored Applications = %d, want 30", n)
	}
	if got := countApplications(t, s, postingID, "pending"); got != 30 {
		t.Fatalf("pending Applications = %d, want 30", got)
	}
}

func TestApplyNeedsAnOpenPosting(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.user("recruiter")

	for _, status := range []string{"intake_closed", "closed"} {
		store := &fakeCVStore{}
		svc := newApplyService(t, pool, store)
		postingID := s.posting(owner, status, slot(3))
		_, err := svc.Apply(ctx, s.user("candidate"), "candidate", postingID, pdfBytes(16))
		assertCode(t, err, apperror.CodeIntakeNotOpen)
		if n := applicationCount(t, s, postingID); n != 0 {
			t.Fatalf("%s posting stored %d Applications, want 0", status, n)
		}
		// The CV uploaded before the refused insert must be removed again.
		if store.uploadCount() != 1 || len(store.deleted()) != 1 || store.deleted()[0] != store.uploads[0] {
			t.Fatalf("%s posting: uploads %v, deletes %v, want the upload deleted", status, store.uploads, store.deleted())
		}
	}
}

// Approval opens a posting, but Candidates can only apply once the Recruiter funds interview slots.
// The refusal comes before the CV is stored, so nothing is uploaded and no row is written; the
// Recruiter keeps seeing the posting as open without capacity. Funding then lets the same Candidate in.
func TestApplyToAnApprovedUnfundedPostingIsRefusedUntilFunded(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	store := &fakeCVStore{}
	svc := newApplyService(t, pool, store)
	owner, candidate := s.user("recruiter"), s.user("candidate")
	s.wallet(owner, 20)
	postingID := s.posting(owner, "open", nil)

	_, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(16))
	assertCode(t, err, apperror.CodeIntakeNotOpen)
	if n := applicationCount(t, s, postingID); n != 0 {
		t.Fatalf("unfunded posting stored %d Applications, want 0", n)
	}
	if store.uploadCount() != 0 || len(store.deleted()) != 0 {
		t.Fatalf("uploads %v, deletes %v, want no CV stored at all", store.uploads, store.deleted())
	}

	// The Recruiter's view is unchanged: still open, no capacity, no Applications.
	state, err := svc.GetCapacity(ctx, owner, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if state.PostingStatus != "open" || state.Funded || state.InterviewSlot != nil {
		t.Fatalf("recruiter capacity = %+v, want open and unfunded", state)
	}
	items, total, err := svc.ListOwn(ctx, owner, "recruiter", domain.ListFilter{JobPostingID: &postingID, Limit: 20})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("recruiter list = %+v (total %d, err %v), want empty", items, total, err)
	}

	if _, err := svc.FundCapacity(ctx, owner, "recruiter", postingID, 3); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(16))
	if err != nil {
		t.Fatalf("Apply() after funding error = %v", err)
	}
	if got.JobPostingID != postingID || got.Status != domain.StatusPending {
		t.Fatalf("Apply() = %+v, want a pending Application", got)
	}
	if _, n := s.applicationRowFor(t, postingID, candidate); n != 1 || store.uploadCount() != 1 {
		t.Fatalf("Applications = %d, uploads = %d, want 1 each", n, store.uploadCount())
	}
}

// The locked check in the insert is the authoritative one: a posting that is unfunded when the insert
// runs is refused even if the pre-check was bypassed.
func TestInsertCandidateApplicationRefusesAnUnfundedPostingUnderTheLock(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	postingID := s.posting(s.user("recruiter"), "open", nil)

	_, err := repo.InsertCandidateApplication(ctx, s.user("candidate"), postingID, "cvs/direct.pdf")
	assertCode(t, err, apperror.CodeIntakeNotOpen)
	if n := applicationCount(t, s, postingID); n != 0 {
		t.Fatalf("stored %d Applications, want 0", n)
	}
}

func TestApplyHidesPendingRejectedAndLockedPostings(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.user("recruiter")

	for _, status := range []string{"pending", "rejected"} {
		store := &fakeCVStore{}
		svc := newApplyService(t, pool, store)
		postingID := s.posting(owner, status, nil)
		_, err := svc.Apply(ctx, s.user("candidate"), "candidate", postingID, pdfBytes(16))
		assertCode(t, err, apperror.CodeJobPostingNotFound)
		if n := applicationCount(t, s, postingID); n != 0 {
			t.Fatalf("%s posting stored %d Applications, want 0", status, n)
		}
		if len(store.deleted()) != 1 {
			t.Fatalf("%s posting: deleted %v, want the uploaded CV removed", status, store.deleted())
		}
	}

	// A locked Recruiter's open posting is hidden from applicants, as on the job board.
	locked := s.user("recruiter")
	lockedPosting := s.posting(locked, "open", slot(3))
	s.lockRecruiter(locked)
	svc := newApplyService(t, pool, &fakeCVStore{})
	_, err := svc.Apply(ctx, s.user("candidate"), "candidate", lockedPosting, pdfBytes(16))
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestApplyRejectsNonPDFAndStoresNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	postingID := s.posting(s.user("recruiter"), "open", nil)

	tooLarge := append([]byte("%PDF-"), make([]byte, domain.MaxCVBytes)...)
	cases := map[string][]byte{
		"plain text":  []byte("not a pdf at all"),
		"empty":       {},
		"missing":     nil,
		"over 5 MB":   tooLarge,
		"wrong magic": []byte("PK\x03\x04zip"),
	}
	for name, data := range cases {
		store := &fakeCVStore{}
		svc := newApplyService(t, pool, store)
		candidate := s.user("candidate")
		_, err := svc.Apply(ctx, candidate, "candidate", postingID, data)
		assertCode(t, err, apperror.CodeValidation)
		if store.uploadCount() != 0 {
			t.Fatalf("%s: uploaded %d objects, want none", name, store.uploadCount())
		}
		if _, n := s.applicationRowFor(t, postingID, candidate); n != 0 {
			t.Fatalf("%s: stored %d Applications, want none", name, n)
		}
	}
}

func TestApplyFailedUploadStoresNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	postingID := s.posting(s.user("recruiter"), "open", slot(3))
	candidate := s.user("candidate")
	store := &fakeCVStore{uploadErr: context.DeadlineExceeded}
	svc := newApplyService(t, pool, store)

	_, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(16))
	assertCode(t, err, apperror.CodeInternal)
	if n := applicationCount(t, s, postingID); n != 0 {
		t.Fatalf("stored %d Applications after a failed upload, want 0", n)
	}
	if len(store.deleted()) != 0 {
		t.Fatalf("deleted %v after a failed upload, want nothing (no object exists)", store.deleted())
	}
}

func TestApplyForbiddenForRecruiterAndAdmin(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	store := &fakeCVStore{}
	svc := newApplyService(t, pool, store)
	owner := s.user("recruiter")
	postingID := s.posting(owner, "open", nil)

	for _, role := range []string{"recruiter", "admin"} {
		_, err := svc.Apply(ctx, owner, role, postingID, pdfBytes(16))
		assertCode(t, err, apperror.CodeForbidden)
	}
	if store.uploadCount() != 0 || applicationCount(t, s, postingID) != 0 {
		t.Fatalf("a forbidden apply uploaded %d objects or stored Applications", store.uploadCount())
	}
}

func TestCandidateListIsScopedPaginatedNewestFirst(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newApplyService(t, pool, &fakeCVStore{})
	mine, other := s.user("candidate"), s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", nil)
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	first := s.application(postingID, mine)
	second := s.application(s.posting(s.user("recruiter"), "open", nil), mine)
	third := s.application(s.posting(s.user("recruiter"), "open", nil), mine)
	s.submittedAt(first, base)
	s.submittedAt(second, base.Add(time.Minute))
	s.submittedAt(third, base.Add(2*time.Minute))
	s.application(postingID, other) // another Candidate's Application must not appear

	page1, err := svc.ListMine(ctx, mine, "candidate", domain.CandidateListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 3 || page1.Page != 1 || page1.PageSize != 2 || len(page1.Items) != 2 {
		t.Fatalf("page 1 = %+v, want total 3 with 2 items", page1)
	}
	if page1.Items[0].ID != third || page1.Items[1].ID != second {
		t.Fatalf("page 1 order = %v, %v, want newest first (%v, %v)", page1.Items[0].ID, page1.Items[1].ID, third, second)
	}
	page2, err := svc.ListMine(ctx, mine, "candidate", domain.CandidateListFilter{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page2.Total != 3 || len(page2.Items) != 1 || page2.Items[0].ID != first {
		t.Fatalf("page 2 = %+v, want the oldest Application alone", page2)
	}
	for _, item := range append(page1.Items, page2.Items...) {
		if item.JobPostingTitle != "app posting" {
			t.Fatalf("item title = %q, want the posting title", item.JobPostingTitle)
		}
	}

	empty, err := svc.ListMine(ctx, other, "candidate", domain.CandidateListFilter{Page: 1, PageSize: 10})
	if err != nil || empty.Total != 1 || len(empty.Items) != 1 {
		t.Fatalf("other Candidate's list = %+v, %v, want only their own single Application", empty, err)
	}

	_, err = svc.ListMine(ctx, mine, "candidate", domain.CandidateListFilter{Page: 0, PageSize: 10})
	assertCode(t, err, apperror.CodeValidation)
	_, err = svc.ListMine(ctx, mine, "candidate", domain.CandidateListFilter{Page: 1, PageSize: 101})
	assertCode(t, err, apperror.CodeValidation)
	_, err = svc.ListMine(ctx, mine, "recruiter", domain.CandidateListFilter{Page: 1, PageSize: 10})
	assertCode(t, err, apperror.CodeForbidden)
}

func TestCandidateDetailOfAnotherCandidateIsNotFound(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newApplyService(t, pool, &fakeCVStore{})
	owner, candidate, stranger := s.user("candidate"), s.user("candidate"), s.user("candidate")
	appID := s.application(s.posting(s.user("recruiter"), "open", nil), owner)

	got, err := svc.GetMine(ctx, owner, "candidate", appID)
	if err != nil || got.ID != appID {
		t.Fatalf("own GetMine() = %+v, %v", got, err)
	}
	_, err = svc.GetMine(ctx, stranger, "candidate", appID)
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.GetMine(ctx, candidate, "candidate", uuid.New())
	assertCode(t, err, apperror.CodeApplicationNotFound)
	_, err = svc.GetMine(ctx, owner, "recruiter", appID)
	assertCode(t, err, apperror.CodeForbidden)
}

func TestCanStartInterviewOnlyWhenEligibleBeforeDeadline(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newApplyService(t, pool, &fakeCVStore{})
	candidate := s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", nil)

	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Minute)
	cases := []struct {
		name     string
		status   string
		deadline *time.Time
		want     bool
	}{
		{"eligible before deadline", "interview_eligible", &future, true},
		{"eligible after deadline", "interview_eligible", &past, false},
		{"eligible without deadline", "interview_eligible", nil, false},
		{"pending", "pending", nil, false},
		{"rejected", "rejected", nil, false},
		{"approved", "approved", &future, false},
		{"interviewed", "interviewed", &future, false},
	}
	for _, tc := range cases {
		appID := s.application(postingID, candidate)
		setStatusAndDeadline(s, appID, tc.status, tc.deadline)

		got, err := svc.GetMine(ctx, candidate, "candidate", appID)
		if err != nil {
			t.Fatalf("%s: GetMine() error = %v", tc.name, err)
		}
		if got.CanStartInterview != tc.want {
			t.Fatalf("%s: GetMine().can_start_interview = %v, want %v", tc.name, got.CanStartInterview, tc.want)
		}
		page, err := svc.ListMine(ctx, candidate, "candidate", domain.CandidateListFilter{Page: 1, PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range page.Items {
			if item.ID == appID {
				found = true
				if item.CanStartInterview != tc.want {
					t.Fatalf("%s: ListMine().can_start_interview = %v, want %v", tc.name, item.CanStartInterview, tc.want)
				}
			}
		}
		if !found {
			t.Fatalf("%s: Application missing from the Candidate's list", tc.name)
		}
	}
}

// The CV storage key must never reach the Candidate's own views.
func TestCandidateViewsNeverExposeTheCVKey(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	store := &fakeCVStore{}
	svc := newApplyService(t, pool, store)
	candidate := s.user("candidate")
	postingID := s.posting(s.user("recruiter"), "open", slot(3))
	sub, err := svc.Apply(ctx, candidate, "candidate", postingID, pdfBytes(16))
	if err != nil {
		t.Fatal(err)
	}
	key := store.uploads[0]

	got, err := svc.GetMine(ctx, candidate, "candidate", sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.ListMine(ctx, candidate, "candidate", domain.CandidateListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{marshalJSON(t, sub), marshalJSON(t, got), marshalJSON(t, page)} {
		if strings.Contains(body, key) {
			t.Fatalf("a Candidate view leaked the CV storage key: %s", body)
		}
	}
}
