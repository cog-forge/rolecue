package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/google/uuid"
)

// ListHistory returns one page of the Candidate's sessions, newest first, with the total of the filter.
// The page offset comes from the page and page size, which the service has already validated. Status is
// derived here from the stored completion and evaluation state.
func (r *Repository) ListHistory(ctx context.Context, candidateID uuid.UUID, filter domain.HistoryFilter) ([]domain.HistoryItem, int64, error) {
	var interviewType *string
	if filter.Type != "" {
		value := string(filter.Type)
		interviewType = &value
	}
	rows, err := r.queries.ListCandidateInterviews(ctx, ListCandidateInterviewsParams{
		CandidateID:   candidateID,
		InterviewType: interviewType,
		PageOffset:    (filter.Page - 1) * filter.PageSize,
		PageLimit:     filter.PageSize,
	})
	if err != nil {
		return nil, 0, internal(err)
	}
	total, err := r.queries.CountCandidateInterviews(ctx, CountCandidateInterviewsParams{
		CandidateID:   candidateID,
		InterviewType: interviewType,
	})
	if err != nil {
		return nil, 0, internal(err)
	}
	items := make([]domain.HistoryItem, 0, len(rows))
	for _, row := range rows {
		evaluation := domain.EvaluationStatus(row.EvaluationStatus)
		completedAt := timePtr(row.CompletedAt)
		items = append(items, domain.HistoryItem{
			ID:               row.ID,
			Type:             domain.Type(row.Type),
			StartedAt:        row.StartedAt.Time,
			CompletedAt:      completedAt,
			EvaluationStatus: evaluation,
			Status:           domain.DeriveSessionStatus(completedAt, evaluation),
			Score:            row.Score,
			RoleProfileTitle: row.RoleProfileTitle,
			JobPostingTitle:  row.JobPostingTitle,
			ApplicationID:    row.ApplicationID,
		})
	}
	return items, total, nil
}
