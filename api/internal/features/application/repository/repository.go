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
// The approval that takes the last slot closes intake in the same transaction (KTD2, R7): the
// remaining pending Applications are rejected, and the terminal evaluation runs after the close. The
// returned JobPostingStatus is the status after that.
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
		postingStatus := posting.Status
		// CheckApproval guarantees approvals < slot, so this approval fills the slot exactly when
		// approvals+1 reaches it.
		if posting.InterviewSlot != nil && approvals+1 >= int64(*posting.InterviewSlot) {
			if _, err := closeIntake(ctx, q, postingID); err != nil {
				return err
			}
			status, err := evaluateTerminalClose(ctx, q, postingID, "intake_closed")
			if err != nil {
				return err
			}
			postingStatus = status
		}
		screening = toScreening(row.ID, row.JobPostingID, row.Status, postingStatus, row.InterviewDeadline, row.RejectReason, row.UpdatedAt)
		return nil
	})
	if err != nil {
		return domain.Screening{}, internal(err)
	}
	return screening, nil
}

// RejectCV makes a pending Application terminally rejected. It takes the same posting lock as
// ApproveCV, so a rejection never races an approval or a later intake close on that posting. It
// never closes intake, and the posting status it returns is the locked one.
func (r *Repository) RejectCV(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID, reason *string) (domain.Screening, error) {
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
		row, err := q.RejectCV(ctx, RejectCVParams{ID: applicationID, JobPostingID: postingID, RejectReason: reason})
		if err != nil {
			return internal(err)
		}
		screening = toScreening(row.ID, row.JobPostingID, row.Status, posting.Status, row.InterviewDeadline, row.RejectReason, row.UpdatedAt)
		return nil
	})
	if err != nil {
		return domain.Screening{}, internal(err)
	}
	return screening, nil
}

// CloseIntake stops a posting from taking more CVs: an open posting becomes intake_closed and its
// pending Applications are rejected in the same transaction. Closing an intake_closed or closed
// posting changes nothing and reports zero rejections, so a retry after a timeout succeeds. The
// terminal close runs last, so a posting with no live Application (including one that never had an
// Application) closes at once.
func (r *Repository) CloseIntake(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.IntakeClosure, error) {
	closure := domain.IntakeClosure{JobPostingID: postingID}
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		posting, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: postingID, RecruiterID: recruiterID})
		if err != nil {
			return postingError(err)
		}
		switch posting.Status {
		case "open":
			rejected, err := closeIntake(ctx, q, postingID)
			if err != nil {
				return err
			}
			closure.RejectedCount = rejected
			closure.Status = "intake_closed"
		case "intake_closed", "closed":
			closure.Status = posting.Status
		default:
			return domain.ErrIntakeNotOpen
		}
		status, err := evaluateTerminalClose(ctx, q, postingID, closure.Status)
		if err != nil {
			return err
		}
		closure.Status = status
		return nil
	})
	if err != nil {
		return domain.IntakeClosure{}, internal(err)
	}
	return closure, nil
}

// EvaluateTerminalClose applies the terminal close rule to one posting for system callers, which
// have no Recruiter to own the posting. It returns the posting status afterwards. A posting that is
// not intake_closed, or is already closed, is left unchanged.
func (r *Repository) EvaluateTerminalClose(ctx context.Context, postingID uuid.UUID) (string, error) {
	var status string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		current, err := q.LockPostingByID(ctx, postingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
		}
		if err != nil {
			return internal(err)
		}
		status, err = evaluateTerminalClose(ctx, q, postingID, current)
		return err
	})
	if err != nil {
		return "", internal(err)
	}
	return status, nil
}

// closeIntake moves an open posting to intake_closed and rejects its pending Applications with the
// fixed reason. The caller holds the posting lock.
func closeIntake(ctx context.Context, q *Queries, postingID uuid.UUID) (int64, error) {
	if _, err := q.SetPostingIntakeClosed(ctx, postingID); err != nil {
		return 0, internal(err)
	}
	reason := domain.IntakeClosedRejectReason
	rejected, err := q.RejectPendingApplications(ctx, RejectPendingApplicationsParams{RejectReason: &reason, JobPostingID: postingID})
	if err != nil {
		return 0, internal(err)
	}
	return rejected, nil
}

// evaluateTerminalClose moves an intake_closed posting to closed when no Application is still
// pending, interview_eligible or interviewed; a posting with no Applications qualifies. The close
// also credits back the unused interview capacity. It takes the posting status the caller holds the
// lock for and returns the status afterwards. Any other status is returned as is.
func evaluateTerminalClose(ctx context.Context, q *Queries, postingID uuid.UUID, status string) (string, error) {
	if status != "intake_closed" {
		return status, nil
	}
	live, err := q.CountLiveApplications(ctx, postingID)
	if err != nil {
		return "", internal(err)
	}
	if live > 0 {
		return status, nil
	}
	closed, err := q.SetPostingClosed(ctx, postingID)
	if err != nil {
		return "", internal(err)
	}
	// The refund rides on the one close that changed the row, so it runs once per posting (KAN-119).
	if closed > 0 {
		if err := refundUnusedCapacity(ctx, q, postingID); err != nil {
			return "", err
		}
	}
	return "closed", nil
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

// ListApplications returns one page of the Recruiter's Applications, newest first, and the total
// that matches the filters. A posting filter on a posting the Recruiter does not own is not found,
// the same answer as a posting that does not exist.
func (r *Repository) ListApplications(ctx context.Context, recruiterID uuid.UUID, filter domain.ListFilter) ([]domain.ListItem, int64, error) {
	if filter.JobPostingID != nil {
		owned, err := r.queries.IsPostingOwned(ctx, IsPostingOwnedParams{ID: *filter.JobPostingID, RecruiterID: recruiterID})
		if err != nil {
			return nil, 0, internal(err)
		}
		if !owned {
			return nil, 0, apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
		}
	}
	var status *string
	if filter.Status != nil {
		value := string(*filter.Status)
		status = &value
	}
	rows, err := r.queries.ListRecruiterApplications(ctx, ListRecruiterApplicationsParams{
		RecruiterID: recruiterID, JobPostingID: filter.JobPostingID, Status: status, Search: filter.Search,
		PageLimit: filter.Limit, PageOffset: filter.Offset,
	})
	if err != nil {
		return nil, 0, internal(err)
	}
	total, err := r.queries.CountRecruiterApplications(ctx, CountRecruiterApplicationsParams{
		RecruiterID: recruiterID, JobPostingID: filter.JobPostingID, Status: status, Search: filter.Search,
	})
	if err != nil {
		return nil, 0, internal(err)
	}
	items := make([]domain.ListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.ListItem{
			ID: row.ID, JobPostingID: row.JobPostingID, JobPostingTitle: row.JobPostingTitle,
			Status: domain.ApplicationStatus(row.Status), SubmittedAt: row.CreatedAt.Time,
			Candidate: domain.ListCandidate{
				ID: row.CandidateID, Name: row.CandidateName, Email: row.CandidateEmail, Image: row.CandidateImage,
			},
			InterviewEligible: row.InterviewEligible, InterviewDeadline: timePtr(row.InterviewDeadline),
			RejectReason: row.RejectReason, UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return items, total, nil
}

// GetApplicationDetail reads one Application of a posting the Recruiter owns. A posting that is not
// theirs, or does not exist, is job posting not found; an Application that is not on this posting is
// application not found.
func (r *Repository) GetApplicationDetail(ctx context.Context, recruiterID, postingID, applicationID uuid.UUID) (domain.DetailRecord, error) {
	owned, err := r.queries.IsPostingOwned(ctx, IsPostingOwnedParams{ID: postingID, RecruiterID: recruiterID})
	if err != nil {
		return domain.DetailRecord{}, internal(err)
	}
	if !owned {
		return domain.DetailRecord{}, apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
	}
	row, err := r.queries.GetRecruiterApplication(ctx, GetRecruiterApplicationParams{
		ID: applicationID, JobPostingID: postingID, RecruiterID: recruiterID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DetailRecord{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.DetailRecord{}, internal(err)
	}
	return domain.DetailRecord{
		Item: domain.ListItem{
			ID: row.ID, JobPostingID: row.JobPostingID, JobPostingTitle: row.JobPostingTitle,
			Status: domain.ApplicationStatus(row.Status), SubmittedAt: row.CreatedAt.Time,
			Candidate: domain.ListCandidate{
				ID: row.CandidateID, Name: row.CandidateName, Email: row.CandidateEmail, Image: row.CandidateImage,
			},
			InterviewEligible: row.InterviewEligible, InterviewDeadline: timePtr(row.InterviewDeadline),
			RejectReason: row.RejectReason, UpdatedAt: row.UpdatedAt.Time,
		},
		CVPath:        row.CvPath,
		PostingStatus: row.PostingStatus,
		InterviewSlot: row.InterviewSlot,
		CVApprovals:   row.CvApprovals,
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

func toScreening(id, postingID uuid.UUID, status, postingStatus string, deadline pgtype.Timestamptz, reason *string, updatedAt pgtype.Timestamptz) domain.Screening {
	return domain.Screening{
		ID: id, JobPostingID: postingID, Status: domain.ApplicationStatus(status), JobPostingStatus: postingStatus,
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
