package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
)

// Repository is the persistence contract consumed by JD application operations.
type Repository interface {
	Create(context.Context, uuid.UUID, domain.CreateInput) (domain.JD, error)
	List(context.Context, uuid.UUID, int32, int32) ([]domain.ListItem, int64, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.JD, error)
	Update(context.Context, uuid.UUID, uuid.UUID, domain.UpdateInput) (domain.JD, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

// Application combines the existing extraction pipeline with reviewed persistence.
type Application struct {
	extraction *Service
	repository Repository
}

func NewApplication(extraction *Service, repository Repository) (*Application, error) {
	if extraction == nil || repository == nil {
		return nil, fmt.Errorf("JD extraction and repository are required")
	}
	return &Application{extraction: extraction, repository: repository}, nil
}

func (a *Application) Analyze(ctx context.Context, raw string) (domain.StructuredJD, error) {
	return a.extraction.Extract(ctx, raw)
}

func (a *Application) Create(ctx context.Context, user uuid.UUID, input domain.CreateInput) (domain.JD, error) {
	if _, err := NormalizeInput(input.RawText); err != nil {
		return domain.JD{}, err
	}
	reviewed, err := ValidateReviewed(input.StructuredJD)
	if err != nil {
		return domain.JD{}, err
	}
	input.StructuredJD = reviewed
	return a.repository.Create(ctx, user, input)
}

func (a *Application) List(ctx context.Context, user uuid.UUID, limit, offset int32) ([]domain.ListItem, int64, error) {
	return a.repository.List(ctx, user, limit, offset)
}

func (a *Application) Get(ctx context.Context, user, id uuid.UUID) (domain.JD, error) {
	return a.repository.Get(ctx, user, id)
}

func (a *Application) Update(ctx context.Context, user, id uuid.UUID, input domain.UpdateInput) (domain.JD, error) {
	reviewed, err := ValidateReviewed(input.StructuredJD)
	if err != nil {
		return domain.JD{}, err
	}
	input.StructuredJD = reviewed
	return a.repository.Update(ctx, user, id, input)
}

func (a *Application) Delete(ctx context.Context, user, id uuid.UUID) error {
	return a.repository.Delete(ctx, user, id)
}
