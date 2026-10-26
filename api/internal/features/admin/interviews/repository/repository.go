package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads interviews for the Admin. It only reads: no write path exists for interview
// operational metadata here.
type Repository struct {
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{queries: New(pool)}
}

// List returns one page of interviews and the total that match the filter. The page offset comes
// from the filter's page and page size, which the service has already validated.
func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.InterviewRow, int64, error) {
	args := ListAdminInterviewsParams{
		InterviewType: typeArg(filter.Type),
		CandidateID:   filter.CandidateID,
		JobPostingID:  filter.JobPostingID,
		PageLimit:     filter.PageSize,
		PageOffset:    (filter.Page - 1) * filter.PageSize,
	}
	rows, err := r.queries.ListAdminInterviews(ctx, args)
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountAdminInterviews(ctx, CountAdminInterviewsParams{
		InterviewType: args.InterviewType,
		CandidateID:   args.CandidateID,
		JobPostingID:  args.JobPostingID,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.InterviewRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.InterviewRow{
			ID:   row.ID,
			Type: row.Type,
			Owner: domain.OwnerRef{
				ID:    row.OwnerID,
				Name:  row.OwnerName,
				Email: row.OwnerEmail,
			},
			JobPosting:   jobPostingRef(row.JobPostingID, row.JobPostingTitle),
			Score:        row.Score,
			HasReport:    row.HasReport,
			HasRecording: row.HasRecording,
		})
	}
	return out, total, nil
}

// Get returns the operational detail of one interview. An unknown ID is ErrInterviewNotFound.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.InterviewDetail, error) {
	row, err := r.queries.GetAdminInterview(ctx, id)
	if err != nil {
		return domain.InterviewDetail{}, persistenceError(err)
	}
	return domain.InterviewDetail{
		InterviewRow: domain.InterviewRow{
			ID:   row.ID,
			Type: row.Type,
			Owner: domain.OwnerRef{
				ID:    row.OwnerID,
				Name:  row.OwnerName,
				Email: row.OwnerEmail,
			},
			JobPosting:   jobPostingRef(row.JobPostingID, row.JobPostingTitle),
			Score:        row.Score,
			HasReport:    row.HasReport,
			HasRecording: row.HasRecording,
		},
		RoleProfile:   domain.RoleProfileRef{ID: row.RoleProfileID, Title: row.RoleProfileTitle},
		ApplicationID: row.ApplicationID,
		Avatar:        namedRef(row.AvatarID, row.AvatarName),
		Voice:         namedRef(row.VoiceID, row.VoiceName),
		QuestionCount: row.QuestionCount,
		TurnCount:     row.TurnCount,
	}, nil
}

// namedRef is nil when the interview has no avatar or voice.
func namedRef(id *uuid.UUID, name *string) *domain.AssetRef {
	if id == nil || name == nil {
		return nil
	}
	return &domain.AssetRef{ID: *id, Name: *name}
}

// typeArg turns an empty filter into a NULL argument, which disables the filter in SQL.
func typeArg(interviewType string) *string {
	if interviewType == "" {
		return nil
	}
	return &interviewType
}

// jobPostingRef is nil for a practice interview, which has no Application and so no Job Posting.
func jobPostingRef(id *uuid.UUID, title *string) *domain.JobPostingRef {
	if id == nil || title == nil {
		return nil
	}
	return &domain.JobPostingRef{ID: *id, Title: *title}
}

func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInterviewNotFound
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "admin interview persistence failed", err)
}
