package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type parsedData struct {
	Skills          []domain.ExtractedSkill `json:"skills"`
	Technologies    []string                `json:"technologies"`
	DomainKnowledge []string                `json:"domainKnowledge"`
}

type Repository struct{ queries *Queries }

func NewRepository(db DBTX) *Repository { return &Repository{queries: New(db)} }

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, input domain.CreateInput) (domain.JD, error) {
	data, err := encodeReviewed(input.StructuredJD)
	if err != nil {
		return domain.JD{}, err
	}
	row, err := r.queries.CreateCustomizedJD(ctx, CreateCustomizedJDParams{UserID: userID, Title: input.StructuredJD.Title, SeniorityLevel: SeniorityLevel(*input.StructuredJD.SeniorityLevel), RawText: input.RawText, ParsedData: data})
	return decodeJD(row, err)
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]domain.ListItem, int64, error) {
	rows, err := r.queries.ListJDs(ctx, ListJDsParams{UserID: userID, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountJDs(ctx, userID)
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	result := make([]domain.ListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.ListItem{ID: row.ID, Title: row.Title, SeniorityLevel: domain.Seniority(row.SeniorityLevel), Status: domain.Status(row.Status), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time})
	}
	return result, total, nil
}

func (r *Repository) Get(ctx context.Context, userID, id uuid.UUID) (domain.JD, error) {
	row, err := r.queries.GetJD(ctx, GetJDParams{ID: id, UserID: userID})
	return decodeJD(row, err)
}

func (r *Repository) Update(ctx context.Context, userID, id uuid.UUID, input domain.UpdateInput) (domain.JD, error) {
	data, err := encodeReviewed(input.StructuredJD)
	if err != nil {
		return domain.JD{}, err
	}
	row, err := r.queries.UpdateJD(ctx, UpdateJDParams{ID: id, UserID: userID, Title: input.StructuredJD.Title, SeniorityLevel: SeniorityLevel(*input.StructuredJD.SeniorityLevel), ParsedData: data})
	return decodeJD(row, err)
}

func (r *Repository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	_, err := r.queries.DeleteJD(ctx, DeleteJDParams{ID: id, UserID: userID})
	var pgErr *pgconn.PgError
	// PostgreSQL can report either foreign_key_violation or restrict_violation.
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		return apperror.Wrap(apperror.CodeJDInUse, "job description is in use", err)
	}
	return persistenceError(err)
}

func encodeReviewed(jd domain.StructuredJD) ([]byte, error) {
	if strings.TrimSpace(jd.Title) == "" {
		return nil, apperror.New(apperror.CodeValidation, "reviewed JD requires title")
	}
	if jd.SeniorityLevel == nil {
		return nil, apperror.New(apperror.CodeValidation, "reviewed JD requires seniority")
	}
	switch *jd.SeniorityLevel {
	case domain.Intern, domain.Junior, domain.Mid, domain.Senior, domain.Lead:
	default:
		return nil, apperror.New(apperror.CodeValidation, "reviewed JD has invalid seniority")
	}
	if jd.Skills == nil || jd.Technologies == nil || jd.DomainKnowledge == nil {
		return nil, apperror.New(apperror.CodeValidation, "reviewed JD requires competency arrays")
	}
	data, err := json.Marshal(parsedData{Skills: jd.Skills, Technologies: jd.Technologies, DomainKnowledge: jd.DomainKnowledge})
	if err != nil {
		return nil, persistenceError(err)
	}
	return data, nil
}

func decodeJD(row JobDescription, err error) (domain.JD, error) {
	if err != nil {
		return domain.JD{}, persistenceError(err)
	}
	var data parsedData
	// The baseline permits NULL parsed_data for older, unparsed JDs.
	if len(row.ParsedData) != 0 {
		if err := json.Unmarshal(row.ParsedData, &data); err != nil {
			return domain.JD{}, persistenceError(err)
		}
	}
	seniority := domain.Seniority(row.SeniorityLevel)
	return domain.JD{ID: row.ID, UserID: row.UserID, RawText: row.RawText, Status: domain.Status(row.Status),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		StructuredJD: domain.StructuredJD{Title: row.Title, SeniorityLevel: &seniority, Skills: data.Skills, Technologies: data.Technologies, DomainKnowledge: data.DomainKnowledge}}, nil
}

func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJDNotFound, "job description not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "job description persistence failed", err)
}
