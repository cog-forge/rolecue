package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type RoleProfileRepository interface {
	CreateProfile(context.Context, uuid.UUID, string) (domain.RoleProfile, error)
	GetProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	ListProfiles(context.Context, uuid.UUID, int32, int32) ([]domain.RoleProfile, int64, error)
	SaveProfile(context.Context, uuid.UUID, uuid.UUID, domain.SaveRoleProfile) (domain.RoleProfile, error)
	ConfirmProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	ClaimAnalysis(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	ReleaseAnalysis(context.Context, uuid.UUID) error
	CompleteAnalysis(context.Context, uuid.UUID, uuid.UUID, domain.RoleReview) (domain.RoleProfile, error)
	DeleteProfile(context.Context, uuid.UUID, uuid.UUID) error
}
type RoleProfileApplication struct {
	extraction *ExtractionService
	repository RoleProfileRepository
	timeout    time.Duration
}

func NewRoleProfileApplication(extraction *ExtractionService, repo RoleProfileRepository, timeout time.Duration) (*RoleProfileApplication, error) {
	if extraction == nil || repo == nil || timeout <= 0 {
		return nil, fmt.Errorf("invalid Role Profile application configuration")
	}
	return &RoleProfileApplication{extraction: extraction, repository: repo, timeout: timeout}, nil
}
func (a *RoleProfileApplication) Create(ctx context.Context, user uuid.UUID, raw string) (domain.RoleProfile, error) {
	if _, err := NormalizeInput(raw); err != nil {
		return domain.RoleProfile{}, err
	}
	return a.repository.CreateProfile(ctx, user, raw)
}
func (a *RoleProfileApplication) Get(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	return a.repository.GetProfile(ctx, user, id)
}
func (a *RoleProfileApplication) List(ctx context.Context, user uuid.UUID, limit, offset int32) ([]domain.RoleProfile, int64, error) {
	return a.repository.ListProfiles(ctx, user, limit, offset)
}
func (a *RoleProfileApplication) Delete(ctx context.Context, user, id uuid.UUID) error {
	return a.repository.DeleteProfile(ctx, user, id)
}
func (a *RoleProfileApplication) Save(ctx context.Context, user, id uuid.UUID, input domain.SaveRoleProfile) (domain.RoleProfile, error) {
	review, err := ValidateRoleReview(input.RoleReview, false)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	profile, err := a.repository.GetProfile(ctx, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	if err = ValidateEvidence(review, profile.RawText); err != nil {
		return domain.RoleProfile{}, err
	}
	input.RoleReview = review
	return a.repository.SaveProfile(ctx, user, id, input)
}
func (a *RoleProfileApplication) Confirm(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	profile, err := a.repository.GetProfile(ctx, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	if _, err = ValidateRoleReview(profile.RoleReview, true); err != nil {
		return domain.RoleProfile{}, err
	}
	return a.repository.ConfirmProfile(ctx, user, id)
}
func (a *RoleProfileApplication) budget() time.Duration {
	return time.Duration(a.extraction.maxRetries+1)*a.timeout + time.Second
}
func (a *RoleProfileApplication) release(ctx context.Context, profile domain.RoleProfile) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = a.repository.ReleaseAnalysis(cleanup, profile.ID)
}
func (a *RoleProfileApplication) Analyze(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, a.budget())
	defer cancel()
	profile, err := a.repository.ClaimAnalysis(ctx, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	if profile.Status == domain.Review {
		return profile, nil
	}
	jd, err := a.extraction.Extract(ctx, profile.RawText)
	if err == nil {
		review := ReviewFromExtraction(jd)
		review, err = ValidateRoleReview(review, false)
		if err == nil {
			err = ValidateEvidence(review, profile.RawText)
		}
		if err == nil {
			var result domain.RoleProfile
			result, err = a.repository.CompleteAnalysis(ctx, user, id, review)
			if err == nil {
				return result, nil
			}
		}
	}
	a.release(ctx, profile)
	var appErr *apperror.AppError
	if errors.As(err, &appErr) && appErr.Code == apperror.CodeValidation {
		err = apperror.New(apperror.CodeInvalidExtractionOutput, "Analysis returned unusable requirements. Please retry.")
	}
	return domain.RoleProfile{}, err
}
