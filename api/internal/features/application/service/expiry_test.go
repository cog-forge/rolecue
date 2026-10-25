package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// expiryApp is one Application as the expiry fake stores it.
type expiryApp struct {
	status    domain.ApplicationStatus
	deadline  *time.Time
	completed bool
	reason    string
}

// expiryFake applies the expiry rule to in-memory Applications at the time it is given, so the fake
// clock decides what is due. locked stands for an advisory lock another instance holds.
type expiryFake struct {
	fakeRepository
	apps   []*expiryApp
	locked bool
	calls  int
}

func (f *expiryFake) AutoRejectExpired(_ context.Context, now time.Time) (int, error) {
	f.calls++
	if f.locked {
		return 0, nil
	}
	rejected := 0
	for _, app := range f.apps {
		if domain.ExpiredForRejection(app.status, app.deadline, app.completed, now) {
			app.status = domain.StatusRejected
			app.reason = domain.ExpiryRejectReason
			rejected++
		}
	}
	return rejected, nil
}

func deadlineAt(t time.Time) *time.Time { return &t }

func TestAutoRejectExpiredFollowsTheClock(t *testing.T) {
	deadline := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	app := &expiryApp{status: domain.StatusInterviewEligible, deadline: deadlineAt(deadline)}
	repo := &expiryFake{apps: []*expiryApp{app}}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}

	for _, at := range []time.Time{deadline.Add(-time.Second), deadline} {
		rejected, err := svc.AutoRejectExpired(context.Background(), at)
		if err != nil {
			t.Fatal(err)
		}
		if rejected != 0 || app.status != domain.StatusInterviewEligible {
			t.Fatalf("at %s rejected %d, status %s; want nothing before or at the deadline", at, rejected, app.status)
		}
	}

	rejected, err := svc.AutoRejectExpired(context.Background(), deadline.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if rejected != 1 || app.status != domain.StatusRejected || app.reason != domain.ExpiryRejectReason {
		t.Fatalf("after the deadline rejected %d, status %s, reason %q", rejected, app.status, app.reason)
	}
}

func TestAutoRejectExpiredSparesCompletedAndDecidedApplications(t *testing.T) {
	deadline := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	completed := &expiryApp{status: domain.StatusInterviewEligible, deadline: deadlineAt(deadline), completed: true}
	interviewed := &expiryApp{status: domain.StatusInterviewed, deadline: deadlineAt(deadline), completed: true}
	pending := &expiryApp{status: domain.StatusPending, deadline: deadlineAt(deadline)}
	startedOnly := &expiryApp{status: domain.StatusInterviewEligible, deadline: deadlineAt(deadline)}
	repo := &expiryFake{apps: []*expiryApp{completed, interviewed, pending, startedOnly}}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := svc.AutoRejectExpired(context.Background(), deadline.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Only the started-but-not-completed interview is rejected: the rule counts it as expired.
	if rejected != 1 {
		t.Fatalf("rejected %d, want 1 (the started-only interview)", rejected)
	}
	if completed.status != domain.StatusInterviewEligible || interviewed.status != domain.StatusInterviewed || pending.status != domain.StatusPending {
		t.Fatal("a completed interview, a decided Application or a pending CV was touched")
	}
	if startedOnly.status != domain.StatusRejected {
		t.Fatalf("started-only status %s, want rejected", startedOnly.status)
	}
}

func TestAutoRejectExpiredRepeatRunRejectsNothingNew(t *testing.T) {
	deadline := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	repo := &expiryFake{apps: []*expiryApp{{status: domain.StatusInterviewEligible, deadline: deadlineAt(deadline)}}}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	at := deadline.Add(time.Hour)
	if first, _ := svc.AutoRejectExpired(context.Background(), at); first != 1 {
		t.Fatalf("first pass rejected %d, want 1", first)
	}
	if second, _ := svc.AutoRejectExpired(context.Background(), at); second != 0 {
		t.Fatalf("second pass rejected %d, want 0", second)
	}
}

func TestAutoRejectExpiredDoesNothingWhileAnotherInstanceHoldsTheLock(t *testing.T) {
	deadline := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	app := &expiryApp{status: domain.StatusInterviewEligible, deadline: deadlineAt(deadline)}
	repo := &expiryFake{apps: []*expiryApp{app}, locked: true}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := svc.AutoRejectExpired(context.Background(), deadline.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if rejected != 0 || app.status != domain.StatusInterviewEligible {
		t.Fatalf("busy lock rejected %d, status %s; want no work", rejected, app.status)
	}
}

func TestAutoRejectExpiredNeedsTheExpiryRepository(t *testing.T) {
	svc, err := New(&fakeRepository{}, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AutoRejectExpired(context.Background(), time.Now())
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != apperror.CodeInternal {
		t.Fatalf("got %v, want INTERNAL_ERROR", err)
	}
}
