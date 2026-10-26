package domain

import (
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// PricePerInterviewSlot is what one interview slot costs in coins (KTD8). The price is fixed at
// 1 coin system-wide (team decision), so a refund uses the same constant.
const PricePerInterviewSlot int32 = 1

// MinInterviewSlots and MaxInterviewSlots bound one funding purchase (KTD8).
const (
	MinInterviewSlots int32 = 1
	MaxInterviewSlots int32 = 10000
)

// PostingStatusClosed is the terminal Job Posting status. Only it carries a refunded amount.
const PostingStatusClosed = "closed"

// CapacityState is the interview capacity of one Job Posting as its owner reads it (KAN-118). The
// submitted CV count never counts as capacity.
type CapacityState struct {
	// InterviewSlot is nil until the Recruiter funds capacity.
	InterviewSlot *int32 `json:"interview_slot"`
	Funded        bool   `json:"funded"`
	PricePerSlot  int32  `json:"price_per_slot"`
	// CVApprovals counts every Application ever CV-approved on the posting.
	CVApprovals int64 `json:"cv_approvals"`
	// Consumed counts interviews that used a slot.
	Consumed int64 `json:"consumed"`
	// AvailableToApprove is the number of slots no CV approval has taken yet.
	AvailableToApprove int64 `json:"available_to_approve"`
	// Refundable is the slots not consumed by an interview, in slots.
	Refundable    int64  `json:"refundable"`
	PostingStatus string `json:"posting_status"`
	// Refunded is the coins credited back at terminal close. It is non-null only when the posting is
	// closed, and then equals Refundable times PricePerSlot at close.
	Refunded *int64 `json:"refunded"`
}

// CapacityInput is the posting state CapacityState is built from, read under the posting lock.
type CapacityInput struct {
	PostingStatus string
	InterviewSlot *int32
	CVApprovals   int64
	Consumed      int64
}

// NewCapacityState builds the capacity state from the locked posting and its counts. It is pure:
// the counts floor at zero, and an unfunded posting reports zero for every slot-based figure.
func NewCapacityState(in CapacityInput) CapacityState {
	state := CapacityState{
		InterviewSlot: in.InterviewSlot,
		Funded:        in.InterviewSlot != nil,
		PricePerSlot:  PricePerInterviewSlot,
		CVApprovals:   in.CVApprovals,
		Consumed:      in.Consumed,
		PostingStatus: in.PostingStatus,
	}
	if state.Funded {
		slot := int64(*in.InterviewSlot)
		state.AvailableToApprove = max(slot-in.CVApprovals, 0)
		state.Refundable = max(slot-in.Consumed, 0)
	}
	if in.PostingStatus == PostingStatusClosed {
		refunded := state.Refundable * int64(state.PricePerSlot)
		state.Refunded = &refunded
	}
	return state
}

// ValidateSlots checks the slot count of a funding purchase.
func ValidateSlots(slots int32) error {
	if slots < MinInterviewSlots || slots > MaxInterviewSlots {
		return apperror.New(apperror.CodeValidation, "slots must be between 1 and 10000")
	}
	return nil
}

// ErrCapacityAlreadyFunded is returned when a posting is funded with a different slot count than the
// one it already has. Funding is one-time (R3).
var ErrCapacityAlreadyFunded = apperror.New(apperror.CodeInterviewCapacityAlreadyFunded,
	"job posting interview capacity is already funded with a different slot count")
