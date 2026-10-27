package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DecideApplication records the Recruiter's final decision on an interviewed Application (KAN-115).
// Lock order: the posting row (NO KEY UPDATE, as screening takes it), then the Application row. The
// interview start locks the posting first too, so the two never wait on each other the other way.
// Under the locks the Application must be interviewed and the review window must have opened, the
// latest interview deadline of the posting, read on the database clock like the interview start. Then the decision is written and the terminal close is
// evaluated in the same transaction, so a posting never waits in intake_closed with nothing left live.
// A posting that is not the Recruiter's answers application not found, the same as a missing
// Application.
func (r *Repository) DecideApplication(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID, decision domain.Decision, reason *string) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		posting, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: postingID, RecruiterID: recruiterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrApplicationNotFound
		}
		if err != nil {
			return internal(err)
		}
		status, err := q.LockApplication(ctx, LockApplicationParams{ID: applicationID, JobPostingID: postingID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrApplicationNotFound
		}
		if err != nil {
			return internal(err)
		}
		if domain.ApplicationStatus(status) != domain.StatusInterviewed {
			return domain.ErrApplicationNotInterviewed
		}
		gate, err := q.GetFinalReviewOpensAt(ctx, postingID)
		if err != nil {
			return internal(err)
		}
		if !domain.CanDecide(domain.StatusInterviewed, timePtr(gate.OpensAt), gate.DbNow.Time) {
			return domain.ErrFinalReviewLocked
		}
		written, err := q.SetApplicationDecision(ctx, SetApplicationDecisionParams{
			Status: string(decision.Status()), RejectReason: reason, ID: applicationID, JobPostingID: postingID,
		})
		if err != nil {
			return internal(err)
		}
		if written != 1 {
			return apperror.New(apperror.CodeInternal, "application decision was not written")
		}
		_, err = evaluateTerminalClose(ctx, q, postingID, posting.Status)
		return err
	})
	if err != nil {
		return internal(err)
	}
	return nil
}
