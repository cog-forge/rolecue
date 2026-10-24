package domain

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

func slot(n int32) *int32 { return &n }

// The Screening response keeps every existing key and adds job_posting_status (KTD5).
func TestScreeningJSONContract(t *testing.T) {
	deadline := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	reason := "Missing Go experience"
	body, err := json.Marshal(Screening{
		ID: uuid.MustParse("33333333-3333-4333-8333-333333333333"), JobPostingID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		Status: StatusRejected, JobPostingStatus: "intake_closed",
		InterviewDeadline: &deadline, RejectReason: &reason, UpdatedAt: deadline,
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"id", "interview_deadline", "job_posting_id", "job_posting_status", "reject_reason", "status", "updated_at"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("Screening keys = %v, want %v", keys, want)
	}
	var postingStatus string
	if err := json.Unmarshal(fields["job_posting_status"], &postingStatus); err != nil || postingStatus != "intake_closed" {
		t.Fatalf("job_posting_status = %s, want intake_closed", fields["job_posting_status"])
	}
}

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
