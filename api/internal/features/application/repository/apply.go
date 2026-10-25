package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InsertCandidateApplication stores a pending Application for the Candidate on an open posting.
// Under the advisory lock for the (posting, Candidate) pair and the posting's key-share lock it
// checks that the posting is visible and open, and that the Candidate has no Application there yet
// (KTD6). cvKey is the storage key of the uploaded CV. The caller deletes that object when this
// returns an error.
func (r *Repository) InsertCandidateApplication(ctx context.Context, candidateID, postingID uuid.UUID, cvKey string) (domain.Submission, error) {
	var submission domain.Submission
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if err := q.LockApplyKey(ctx, applyLockKey(postingID, candidateID)); err != nil {
			return internal(err)
		}
		posting, err := q.LockApplyPosting(ctx, postingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrJobPostingNotFound
		}
		if err != nil {
			return internal(err)
		}
		if err := applyVisible(posting.Status, posting.RecruiterLocked); err != nil {
			return err
		}
		applied, err := q.CandidateHasApplication(ctx, CandidateHasApplicationParams{JobPostingID: postingID, CandidateID: candidateID})
		if err != nil {
			return internal(err)
		}
		if applied {
			return domain.ErrAlreadyApplied
		}
		row, err := q.InsertCandidateApplication(ctx, InsertCandidateApplicationParams{
			JobPostingID: postingID, CandidateID: candidateID, CvPath: &cvKey,
		})
		if err != nil {
			return internal(err)
		}
		submission = domain.Submission{
			ID: row.ID, JobPostingID: row.JobPostingID, Status: domain.ApplicationStatus(row.Status), CreatedAt: row.CreatedAt.Time,
		}
		return nil
	})
	if err != nil {
		return domain.Submission{}, internal(err)
	}
	return submission, nil
}

// applyVisible mirrors the job board: only an open posting whose Recruiter is not locked accepts an
// Application. A closed intake is reported as such; anything else is not found.
func applyVisible(status string, recruiterLocked bool) error {
	if recruiterLocked {
		return domain.ErrJobPostingNotFound
	}
	switch status {
	case "open":
		return nil
	case "intake_closed", "closed":
		return domain.ErrIntakeNotOpen
	default:
		return domain.ErrJobPostingNotFound
	}
}

func applyLockKey(postingID, candidateID uuid.UUID) string {
	return fmt.Sprintf("candidate-apply:%s:%s", postingID, candidateID)
}

// ListCandidateApplications returns one page of the Candidate's own Applications, newest first, and
// the total count.
func (r *Repository) ListCandidateApplications(ctx context.Context, candidateID uuid.UUID, limit, offset int32) ([]domain.CandidateApplication, int64, error) {
	rows, err := r.queries.ListCandidateApplications(ctx, ListCandidateApplicationsParams{
		CandidateID: candidateID, PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		return nil, 0, internal(err)
	}
	total, err := r.queries.CountCandidateApplications(ctx, candidateID)
	if err != nil {
		return nil, 0, internal(err)
	}
	items := make([]domain.CandidateApplication, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.CandidateApplication{
			ID: row.ID, JobPostingID: row.JobPostingID, JobPostingTitle: row.JobPostingTitle,
			Status: domain.ApplicationStatus(row.Status), RejectReason: row.RejectReason,
			InterviewDeadline: timePtr(row.InterviewDeadline), CreatedAt: row.CreatedAt.Time,
			Interview: candidateInterview(row.InterviewID, row.InterviewCompletedAt, row.InterviewEvaluationStatus, row.InterviewScore),
		})
	}
	return items, total, nil
}

// GetCandidateApplication reads one of the Candidate's own Applications. Another Candidate's
// Application is application not found.
func (r *Repository) GetCandidateApplication(ctx context.Context, candidateID, applicationID uuid.UUID) (domain.CandidateApplication, error) {
	row, err := r.queries.GetCandidateApplication(ctx, GetCandidateApplicationParams{ID: applicationID, CandidateID: candidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CandidateApplication{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.CandidateApplication{}, internal(err)
	}
	return domain.CandidateApplication{
		ID: row.ID, JobPostingID: row.JobPostingID, JobPostingTitle: row.JobPostingTitle,
		Status: domain.ApplicationStatus(row.Status), RejectReason: row.RejectReason,
		InterviewDeadline: timePtr(row.InterviewDeadline), CreatedAt: row.CreatedAt.Time,
		Interview: candidateInterview(row.InterviewID, row.InterviewCompletedAt, row.InterviewEvaluationStatus, row.InterviewScore),
	}, nil
}
