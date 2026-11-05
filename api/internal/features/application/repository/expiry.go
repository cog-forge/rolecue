package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// applicationExpiryLockKey names the advisory lock of the expiry pass. Only one pass runs at a time
// across API instances (KAN-214).
const applicationExpiryLockKey = "application-expiry"

// AutoRejectExpired runs one expiry pass and returns how many Applications it rejected. It is
// AutoRejectExpiredIDs without the ids.
func (r *Repository) AutoRejectExpired(ctx context.Context, now time.Time) (int, error) {
	ids, err := r.AutoRejectExpiredIDs(ctx, now)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// AutoRejectExpiredIDs rejects the interview_eligible Applications whose deadline is before now and whose
// interview has not completed, then closes the postings that became terminal (refund included). The
// whole pass is one transaction under a transaction-scoped try-lock: when another instance holds the
// lock this pass does nothing and returns no ids. Postings are locked in id order, each before its
// Applications, the order the screening and interview paths use. It returns the ids of the Applications
// it rejected (KAN-219).
func (r *Repository) AutoRejectExpiredIDs(ctx context.Context, now time.Time) ([]uuid.UUID, error) {
	var rejected []uuid.UUID
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
			ids, err := q.RejectExpiredApplications(ctx, RejectExpiredApplicationsParams{
				RejectReason: &reason, JobPostingID: postingID, Now: at,
			})
			if err != nil {
				return internal(err)
			}
			rejected = append(rejected, ids...)
			if len(ids) > 0 {
				logger.FromContext(ctx).Info("applications expiry applied in transaction", logger.Op("application.expiry"),
					zap.String("posting_id", postingID.String()), zap.Int("count", len(ids)),
					zap.String("reason", domain.ExpiryRejectReason))
			}
			if _, err := evaluateTerminalClose(ctx, q, postingID, status); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, internal(err)
	}
	return rejected, nil
}
