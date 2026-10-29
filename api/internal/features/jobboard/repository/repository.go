package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository reads open Job Postings for the Candidate job board. It never writes.
type Repository struct{ queries *Queries }

func NewRepository(db DBTX) *Repository {
	return &Repository{queries: New(db)}
}

func (r *Repository) ListOpen(ctx context.Context, filter domain.BoardFilter) ([]domain.BoardPosting, int64, error) {
	rows, err := r.queries.ListOpenPostings(ctx, ListOpenPostingsParams{
		PublishedFrom: timeArg(filter.PublishedFrom), PublishedTo: timeArg(filter.PublishedTo), Search: filter.Search, Location: filter.Location, EmploymentType: filter.EmploymentType,
		PageLimit: filter.Limit, PageOffset: filter.Offset,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountOpenPostings(ctx, CountOpenPostingsParams{
		PublishedFrom: timeArg(filter.PublishedFrom), PublishedTo: timeArg(filter.PublishedTo), Search: filter.Search, Location: filter.Location, EmploymentType: filter.EmploymentType,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.BoardPosting, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.BoardPosting{
			ID: row.ID, Title: row.Title, CompanyName: row.CompanyName, Location: row.Location,
			EmploymentType: row.EmploymentType, SalaryMin: row.SalaryMin, SalaryMax: row.SalaryMax,
			SeniorityLevel: row.LevelName, CreatedAt: row.CreatedAt.Time, PublishedAt: timeValue(row.PublishedAt),
		})
	}
	return out, total, nil
}

func (r *Repository) GetOpen(ctx context.Context, candidateID, id uuid.UUID) (domain.BoardRecord, error) {
	row, err := r.queries.GetOpenPosting(ctx, GetOpenPostingParams{ID: id, CandidateID: candidateID})
	if err != nil {
		return domain.BoardRecord{}, persistenceError(err)
	}
	requirements, err := r.queries.ListPostingRequirements(ctx, row.JdID)
	if err != nil {
		return domain.BoardRecord{}, persistenceError(err)
	}
	record := domain.BoardRecord{StoredText: row.RawText, BoardPostingDetail: domain.BoardPostingDetail{
		BoardPosting: domain.BoardPosting{
			ID: row.ID, Title: row.Title, CompanyName: row.CompanyName, Location: row.Location,
			EmploymentType: row.EmploymentType, SalaryMin: row.SalaryMin, SalaryMax: row.SalaryMax,
			SeniorityLevel: row.LevelName, CreatedAt: row.CreatedAt.Time, PublishedAt: timeValue(row.PublishedAt),
		},
		CompanyWebsite: row.CompanyWebsite, InterviewDuration: row.InterviewDuration,
		AlreadyApplied: row.AlreadyApplied, Requirements: make([]domain.BoardRequirement, 0, len(requirements)),
	}}
	if row.ContentPath != nil {
		record.ContentPath = *row.ContentPath
	}
	for _, req := range requirements {
		record.Requirements = append(record.Requirements, domain.BoardRequirement{Name: req.Name, Target: req.Target, Type: req.Type})
	}
	return record, nil
}

// persistenceError reports a missing or non-open posting as not found, with the same message either
// way, so the response never tells whether the posting exists.
func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "job board persistence failed", err)
}

func timeArg(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}
func timeValue(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
