package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct{ queries *Queries }

func NewRepository(db DBTX) *Repository { return &Repository{queries: New(db)} }

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.PostingSummary, int64, error) {
	status := statusArg(filter.Status)
	from, to := timeArg(filter.From), timeArg(filter.To)
	rows, err := r.queries.ListPostings(ctx, ListPostingsParams{
		Status: status, RecruiterID: filter.RecruiterID, CreatedFrom: from, CreatedTo: to,
		PageLimit: filter.Limit, PageOffset: filter.Offset,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountPostings(ctx, CountPostingsParams{
		Status: status, RecruiterID: filter.RecruiterID, CreatedFrom: from, CreatedTo: to,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.PostingSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PostingSummary{
			ID: row.ID, Title: row.Title, Status: domain.PostingStatus(row.Status), CreatedAt: row.CreatedAt.Time,
			Recruiter: domain.Recruiter{ID: row.RecruiterID, Name: row.RecruiterName, Email: row.RecruiterEmail},
		})
	}
	return out, total, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.ReviewPosting, error) {
	row, err := r.queries.GetPosting(ctx, id)
	if err != nil {
		return domain.ReviewPosting{}, persistenceError(err)
	}
	requirements, err := r.queries.ListReviewRequirements(ctx, row.JdID)
	if err != nil {
		return domain.ReviewPosting{}, persistenceError(err)
	}
	confirmed := make([]domain.Requirement, 0, len(requirements))
	for _, requirement := range requirements {
		confirmed = append(confirmed, domain.Requirement{Name: requirement.Name, Target: requirement.Target, Type: requirement.Type})
	}
	return domain.ReviewPosting{
		CompanyName: row.CompanyName, CompanyWebsite: row.CompanyWebsite,
		SeniorityLevel: row.SeniorityLevel, RefinementNote: row.RefinementNote, Requirements: confirmed,
		ID: row.ID, Title: row.Title, Status: domain.PostingStatus(row.Status), RejectReason: row.RejectReason,
		Recruiter: domain.Recruiter{ID: row.RecruiterID, Name: row.RecruiterName, Email: row.RecruiterEmail},
		Location:  row.Location, EmploymentType: row.EmploymentType, SalaryMin: row.SalaryMin, SalaryMax: row.SalaryMax,
		InterviewSlot: row.InterviewSlot, InterviewDuration: row.InterviewDuration,
		RequiredAvatar:    namedRef(row.RequiredAvatarID, row.AvatarName),
		RequiredVoice:     namedRef(row.RequiredVoiceID, row.VoiceName),
		CreatedAt:         row.CreatedAt.Time,
		ContentPath:       row.ContentPath,
		StoredText:        row.RawText,
		CoreQuestionCount: row.CoreQuestionCount,
		ReviewSubmitted:   row.ReviewSubmitted,
		WeightsConfigured: row.WeightsConfigured,
	}, nil
}

// Decide returns domain.ErrNotPending when no row matched the guards (pending and submitted, and a
// non-empty bank for approval); the service then reads the posting to tell the cases apart.
func (r *Repository) Decide(ctx context.Context, id uuid.UUID, status domain.PostingStatus, rejectReason *string) error {
	n, err := r.queries.DecidePosting(ctx, DecidePostingParams{ID: id, NewStatus: string(status), RejectReason: rejectReason})
	if err != nil {
		return persistenceError(err)
	}
	if n == 0 {
		return domain.ErrNotPending
	}
	return nil
}

func statusArg(status *domain.PostingStatus) *string {
	if status == nil {
		return nil
	}
	s := string(*status)
	return &s
}

func timeArg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func namedRef(id *uuid.UUID, name *string) *domain.NamedRef {
	if id == nil {
		return nil
	}
	return &domain.NamedRef{ID: *id, Name: name}
}

func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "job posting moderation persistence failed", err)
}
