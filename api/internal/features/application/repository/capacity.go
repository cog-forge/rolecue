package repository

import (
	"context"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetCapacity reads the interview capacity of a posting the Recruiter owns. It takes the posting
// lock, so the slot, the approvals and the consumed count are read from one consistent state. A
// posting that is not theirs, or does not exist, is job posting not found.
func (r *Repository) GetCapacity(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.CapacityState, error) {
	var state domain.CapacityState
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		posting, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return postingError(err)
		}
		state, err = capacityState(ctx, q, postingID, posting.Status, posting.InterviewSlot)
		return err
	})
	if err != nil {
		return domain.CapacityState{}, internal(err)
	}
	return state, nil
}

// FundCapacity buys slots interview slots for a posting the Recruiter owns, at one coin each, in one
// transaction with the slot write (R2). Under the posting lock:
//   - a posting already funded with the same count returns its current state and charges nothing;
//   - a posting funded with another count is INTERVIEW_CAPACITY_ALREADY_FUNDED;
//   - a posting that is not open is INTAKE_NOT_OPEN;
//   - otherwise the Recruiter's wallet is debited first, then interview_slot is set. A low balance
//     returns INSUFFICIENT_BALANCE and rolls the whole call back.
func (r *Repository) FundCapacity(ctx context.Context, recruiterID, postingID uuid.UUID, slots int32) (domain.CapacityState, error) {
	var state domain.CapacityState
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		posting, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return postingError(err)
		}
		status, slot := posting.Status, posting.InterviewSlot
		if slot != nil {
			if *slot != slots {
				return domain.ErrCapacityAlreadyFunded
			}
		} else {
			if status != "open" {
				return domain.ErrIntakeNotOpen
			}
			description := fmt.Sprintf("Interview capacity of job posting %s, %d slots", postingID, slots)
			if err := ledger.Debit(ctx, tx, recruiterID, slots*domain.PricePerInterviewSlot, description); err != nil {
				return err
			}
			written, err := q.SetInterviewSlot(ctx, SetInterviewSlotParams{ID: postingID, InterviewSlot: &slots})
			if err != nil {
				return internal(err)
			}
			if written != 1 {
				return apperror.New(apperror.CodeInternal, "interview capacity was not written")
			}
			slot = &slots
		}
		state, err = capacityState(ctx, q, postingID, status, slot)
		return err
	})
	if err != nil {
		return domain.CapacityState{}, internal(err)
	}
	return state, nil
}

// capacityState counts the approvals and consumed interviews of a locked posting and builds the
// state from them. The caller holds the posting lock.
func capacityState(ctx context.Context, q *Queries, postingID uuid.UUID, status string, slot *int32) (domain.CapacityState, error) {
	approvals, err := q.CountCVApprovals(ctx, postingID)
	if err != nil {
		return domain.CapacityState{}, internal(err)
	}
	consumed, err := q.CountConsumedInterviews(ctx, postingID)
	if err != nil {
		return domain.CapacityState{}, internal(err)
	}
	return domain.NewCapacityState(domain.CapacityInput{
		PostingStatus: status, InterviewSlot: slot, CVApprovals: approvals, Consumed: consumed,
	}), nil
}
