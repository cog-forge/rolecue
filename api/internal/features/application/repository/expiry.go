package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// applicationExpiryLockKey names the advisory lock of the expiry pass. Only one pass runs at a time
// across API instances (KAN-214).
const applicationExpiryLockKey = "application-expiry"

// AutoRejectExpired rejects the interview_eligible Applications whose deadline is before now and whose
// interview has not completed, then closes the postings that became terminal (refund included). The
// whole pass is one transaction under a transaction-scoped try-lock: when another instance holds the
// lock this pass does nothing and reports zero. Postings are locked in id order, each before its
// Applications, the order the screening and interview paths use. It returns how many Applications it
// rejected.
func (r *Repository) AutoRejectExpired(ctx context.Context, now time.Time) (int, error) {
	rejected := 0
	at := pgtype.Timestamptz{Time: now, Valid: true}
	reason := domain.ExpiryRejectReason
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		acquired, err := q.TryApplicationExpiryLock(ctx, applicationExpiryLockKey)
		if err != nil {
			return internal(err)
		}
		if !acquired {
			return nil
		}
		postings, err := q.ListPostingsWithExpiredApplications(ctx, at)
		if err != nil {
			return internal(err)
		}
		for _, postingID := range postings {
			status, err := q.LockPostingByID(ctx, postingID)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return internal(err)
			}
			if _, err := q.LockUnfinishedInterviewsForExpiry(ctx, LockUnfinishedInterviewsForExpiryParams{
				JobPostingID: postingID, Now: at,
			}); err != nil {
				return internal(err)
			}
			count, err := q.RejectExpiredApplications(ctx, RejectExpiredApplicationsParams{
				RejectReason: &reason, JobPostingID: postingID, Now: at,
			})
			if err != nil {
				return internal(err)
			}
			rejected += int(count)
			if _, err := evaluateTerminalClose(ctx, q, postingID, status); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, internal(err)
	}
	return rejected, nil
}
