package domain

import (
	"errors"
	"testing"
)

func slot(n int32) *int32 { return &n }

func TestCheckApproval(t *testing.T) {
	for _, tc := range []struct {
		name     string
		capacity InterviewCapacity
		want     error
	}{
		{"free slot", InterviewCapacity{PostingStatus: "open", InterviewSlot: slot(2), CVApprovals: 1}, nil},
		{"last slot", InterviewCapacity{PostingStatus: "open", InterviewSlot: slot(1), CVApprovals: 0}, nil},
		{"full", InterviewCapacity{PostingStatus: "open", InterviewSlot: slot(2), CVApprovals: 2}, ErrCapacityFull},
		{"over full", InterviewCapacity{PostingStatus: "open", InterviewSlot: slot(1), CVApprovals: 3}, ErrCapacityFull},
		{"zero slots", InterviewCapacity{PostingStatus: "open", InterviewSlot: slot(0)}, ErrCapacityFull},
		{"not funded", InterviewCapacity{PostingStatus: "open"}, ErrCapacityNotFunded},
		{"pending posting", InterviewCapacity{PostingStatus: "pending", InterviewSlot: slot(5)}, ErrIntakeNotOpen},
		{"rejected posting", InterviewCapacity{PostingStatus: "rejected", InterviewSlot: slot(5)}, ErrIntakeNotOpen},
		{"intake closed", InterviewCapacity{PostingStatus: "intake_closed", InterviewSlot: slot(5)}, ErrIntakeNotOpen},
		{"closed", InterviewCapacity{PostingStatus: "closed", InterviewSlot: slot(5)}, ErrIntakeNotOpen},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.capacity.CheckApproval(); !errors.Is(got, tc.want) {
				t.Fatalf("CheckApproval() = %v, want %v", got, tc.want)
			}
		})
	}
}
