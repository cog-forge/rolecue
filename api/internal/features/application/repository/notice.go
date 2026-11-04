package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/google/uuid"
)

// NoticeContexts reads what the Application emails need for the given Applications: the Candidate, the
// Recruiter who owns the posting, the posting title and the interview deadline (KAN-219). An id that
// matches no Application is left out. An empty list returns nil without a query.
func (r *Repository) NoticeContexts(ctx context.Context, ids []uuid.UUID) ([]domain.NoticeContext, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.queries.ListNoticeContexts(ctx, ids)
	if err != nil {
		return nil, internal(err)
	}
	contexts := make([]domain.NoticeContext, 0, len(rows))
	for _, row := range rows {
		contexts = append(contexts, domain.NoticeContext{
			ApplicationID: row.ApplicationID, JobPostingID: row.JobPostingID, PostingTitle: row.PostingTitle,
			CandidateName: row.CandidateName, CandidateEmail: row.CandidateEmail, RecruiterEmail: row.RecruiterEmail,
			InterviewDeadline: timePtr(row.InterviewDeadline),
		})
	}
	return contexts, nil
}
