package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// capacityFake fakes the capacity contract on top of the screening fake.
type capacityFake struct {
	*fakeRepository
	state        domain.CapacityState
	capacityErr  error
	getCalls     int
	getRecruiter uuid.UUID
	getPosting   uuid.UUID
}

func (f *capacityFake) GetCapacity(_ context.Context, recruiterID, postingID uuid.UUID) (domain.CapacityState, error) {
	f.getCalls++
	f.getRecruiter, f.getPosting = recruiterID, postingID
	if f.capacityErr != nil {
		return domain.CapacityState{}, f.capacityErr
	}
	return f.state, nil
}

func newCapacityService(t *testing.T, fake *capacityFake) *Service {
	t.Helper()
	if fake.fakeRepository == nil {
		fake.fakeRepository = &fakeRepository{}
	}
	svc, err := New(fake, &fakeCVLinker{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return svc
}

func TestGetCapacityDelegatesForRecruiter(t *testing.T) {
	want := domain.NewCapacityState(domain.CapacityInput{PostingStatus: "open", InterviewSlot: ptr(int32(4)), CVApprovals: 1})
	fake := &capacityFake{state: want}
	got, err := newCapacityService(t, fake).GetCapacity(context.Background(), recruiterID, "recruiter", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AvailableToApprove != want.AvailableToApprove || got.InterviewSlot == nil || *got.InterviewSlot != 4 {
		t.Fatalf("GetCapacity() = %+v, want %+v", got, want)
	}
	if fake.getCalls != 1 || fake.getRecruiter != recruiterID || fake.getPosting != postingID {
		t.Fatalf("repository called %d times with recruiter %s posting %s, want the caller's IDs",
			fake.getCalls, fake.getRecruiter, fake.getPosting)
	}
}

func TestOnlyRecruitersReadCapacity(t *testing.T) {
	for _, role := range []string{"candidate", "admin", "", "Recruiter"} {
		fake := &capacityFake{}
		_, err := newCapacityService(t, fake).GetCapacity(context.Background(), recruiterID, role, postingID)
		assertCode(t, err, apperror.CodeForbidden)
		if fake.getCalls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestGetCapacityPassesRepositoryErrorsThrough(t *testing.T) {
	want := apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
	_, err := newCapacityService(t, &capacityFake{capacityErr: want}).GetCapacity(context.Background(), recruiterID, "recruiter", postingID)
	assertCode(t, err, want.Code)
}

func TestGetCapacityNeedsCapacityRepository(t *testing.T) {
	// The screening-only repository does not implement the capacity contract.
	_, err := newService(t, &fakeRepository{}).GetCapacity(context.Background(), recruiterID, "recruiter", postingID)
	assertCode(t, err, apperror.CodeInternal)
}
