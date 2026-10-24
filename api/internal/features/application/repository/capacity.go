package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
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
