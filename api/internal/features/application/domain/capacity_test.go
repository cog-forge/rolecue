package domain

import (
	"encoding/json"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestNewCapacityStateUnfunded(t *testing.T) {
	got := NewCapacityState(CapacityInput{PostingStatus: "open", CVApprovals: 0, Consumed: 0})
	if got.Funded || got.InterviewSlot != nil {
		t.Fatalf("unfunded state = %+v, want funded=false and no slot", got)
	}
	if got.AvailableToApprove != 0 || got.Refundable != 0 || got.Refunded != nil {
		t.Fatalf("unfunded state = %+v, want no slot figures and refunded null", got)
	}
	if got.PricePerSlot != PricePerInterviewSlot {
		t.Fatalf("price = %d, want %d", got.PricePerSlot, PricePerInterviewSlot)
	}
}

func TestNewCapacityStateCountsSlots(t *testing.T) {
	// Funded with 5: two approvals leave three open for approval; one interview leaves four refundable.
	got := NewCapacityState(CapacityInput{PostingStatus: "open", InterviewSlot: slot(5), CVApprovals: 2, Consumed: 1})
	if got.AvailableToApprove != 3 || got.Refundable != 4 {
		t.Fatalf("available=%d refundable=%d, want 3 and 4", got.AvailableToApprove, got.Refundable)
	}
	if got.Refunded != nil {
		t.Fatalf("refunded = %v before close, want null", *got.Refunded)
	}
}

func TestNewCapacityStateSubmittedCVsAreNotCapacity(t *testing.T) {
	// The count of submitted CVs is not an input at all: with no approvals every slot stays open.
	got := NewCapacityState(CapacityInput{PostingStatus: "open", InterviewSlot: slot(5)})
	if got.AvailableToApprove != 5 {
		t.Fatalf("available = %d, want the full slot count 5", got.AvailableToApprove)
	}
}

func TestNewCapacityStateFloorsAtZero(t *testing.T) {
	got := NewCapacityState(CapacityInput{PostingStatus: "intake_closed", InterviewSlot: slot(2), CVApprovals: 4, Consumed: 3})
	if got.AvailableToApprove != 0 || got.Refundable != 0 {
		t.Fatalf("available=%d refundable=%d, want both floored to 0", got.AvailableToApprove, got.Refundable)
	}
}

func TestNewCapacityStateRefundOnlyWhenClosed(t *testing.T) {
	closed := NewCapacityState(CapacityInput{PostingStatus: "closed", InterviewSlot: slot(5), Consumed: 2})
	if closed.Refunded == nil || *closed.Refunded != 3 {
		t.Fatalf("refunded = %v, want 3 coins (3 unused slots at 1 coin)", closed.Refunded)
	}

	unfundedClosed := NewCapacityState(CapacityInput{PostingStatus: "closed"})
	if unfundedClosed.Refunded == nil || *unfundedClosed.Refunded != 0 {
		t.Fatalf("unfunded closed refunded = %v, want 0", unfundedClosed.Refunded)
	}
}

func TestCapacityStateJSONContract(t *testing.T) {
	body, err := json.Marshal(NewCapacityState(CapacityInput{PostingStatus: "open"}))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"interview_slot", "funded", "price_per_slot", "cv_approvals", "consumed",
		"available_to_approve", "refundable", "posting_status", "refunded"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("CapacityState JSON lacks %q: %s", key, body)
		}
	}
	if string(fields["interview_slot"]) != "null" || string(fields["refunded"]) != "null" {
		t.Fatalf("unfunded open state must send interview_slot and refunded as null: %s", body)
	}
}

func TestValidateSlots(t *testing.T) {
	for _, slots := range []int32{0, -1, 10001} {
		assertDomainCode(t, ValidateSlots(slots), apperror.CodeValidation)
	}
	for _, slots := range []int32{1, 5, 10000} {
		if err := ValidateSlots(slots); err != nil {
			t.Fatalf("ValidateSlots(%d) = %v, want nil", slots, err)
		}
	}
}

func assertDomainCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if !apperror.IsCode(err, want) {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}
