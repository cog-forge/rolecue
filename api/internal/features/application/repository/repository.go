package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository stores CV screening decisions. Each decision runs in one transaction that holds the
// posting row lock, so the interview_slot check and the write cannot interleave with another one.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

// ApproveCV grants interview eligibility when the posting is open and has a free interview slot.
func (r *Repository) ApproveCV(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID) (domain.Screening, error) {
	var screening domain.Screening
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		posting, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return postingError(err)
		}
		if err := lockPendingApplication(ctx, q, postingID, applicationID); err != nil {
			return err
		}
		approvals, err := q.CountCVApprovals(ctx, postingID)
		if err != nil {
			return internal(err)
		}
		capacity := domain.InterviewCapacity{PostingStatus: posting.Status, InterviewSlot: posting.InterviewSlot, CVApprovals: approvals}
		if err := capacity.CheckApproval(); err != nil {
			return err
		}
		row, err := q.ApproveCV(ctx, ApproveCVParams{ID: applicationID, JobPostingID: postingID})
		if err != nil {
			return internal(err)
		}
		screening = toScreening(row.ID, row.JobPostingID, row.Status, row.InterviewDeadline, row.RejectReason, row.UpdatedAt)
		return nil
	})
	if err != nil {
		return domain.Screening{}, internal(err)
	}
	return screening, nil
}

// RejectCV makes a pending Application terminally rejected. It takes the same posting lock as
// ApproveCV, so a rejection never races an approval or a later intake close on that posting.
func (r *Repository) RejectCV(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID, reason *string) (domain.Screening, error) {
	var screening domain.Screening
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if _, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: postingID, RecruiterID: recruiterID}); err != nil {
			return postingError(err)
		}
		if err := lockPendingApplication(ctx, q, postingID, applicationID); err != nil {
			return err
		}
		row, err := q.RejectCV(ctx, RejectCVParams{ID: applicationID, JobPostingID: postingID, RejectReason: reason})
		if err != nil {
			return internal(err)
		}
		screening = toScreening(row.ID, row.JobPostingID, row.Status, row.InterviewDeadline, row.RejectReason, row.UpdatedAt)
		return nil
	})
	if err != nil {
		return domain.Screening{}, internal(err)
	}
	return screening, nil
}

// CandidateEligibility reads the Candidate's own Application. Another Candidate's Application is
// reported as not found.
func (r *Repository) CandidateEligibility(ctx context.Context, candidateID, applicationID uuid.UUID) (domain.EligibilityRecord, error) {
	row, err := r.queries.GetCandidateEligibility(ctx, GetCandidateEligibilityParams{ID: applicationID, CandidateID: candidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EligibilityRecord{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.EligibilityRecord{}, internal(err)
	}
	return domain.EligibilityRecord{
		ApplicationID: row.ID, JobPostingID: row.JobPostingID, Status: domain.ApplicationStatus(row.Status),
		InterviewDeadline: timePtr(row.InterviewDeadline), WithinDeadline: row.WithinDeadline,
	}, nil
}

func postingError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	return internal(err)
}

// lockPendingApplication checks the Application before the capacity rule, so a missing or already
// screened Application gets its own error even when the posting is full. Under the lock the
// guarded update that follows always matches.
func lockPendingApplication(ctx context.Context, q *Queries, postingID, applicationID uuid.UUID) error {
	status, err := q.LockApplication(ctx, LockApplicationParams{ID: applicationID, JobPostingID: postingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrApplicationNotFound
	}
	if err != nil {
		return internal(err)
	}
	if domain.ApplicationStatus(status) != domain.StatusPending {
		return domain.ErrApplicationNotPending
	}
	return nil
}

// internal keeps AppErrors raised inside a transaction and maps anything else, such as a failed
// begin or commit, to an internal error.
func internal(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "application screening persistence failed", err)
}

func toScreening(id, postingID uuid.UUID, status string, deadline pgtype.Timestamptz, reason *string, updatedAt pgtype.Timestamptz) domain.Screening {
	return domain.Screening{
		ID: id, JobPostingID: postingID, Status: domain.ApplicationStatus(status),
		InterviewDeadline: timePtr(deadline), RejectReason: reason, UpdatedAt: updatedAt.Time,
	}
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
