package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	questionbankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
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

type PracticeQuestionGenerator interface {
	GenerateForPractice(context.Context, uuid.UUID, uuid.UUID) ([]questionbankdomain.CoreQuestion, error)
}

// RoadmapPreparer generates the Learning Roadmap of a profile whose requirements the candidate has just
// reviewed: its topics and the practice bank (KAN-343). When one is set it replaces the direct question bank
// generation of Confirm.
type RoadmapPreparer interface {
	Prepare(ctx context.Context, candidate, profile, jdID uuid.UUID) error
}

type RoleProfileApplication struct {
	extraction   *ExtractionService
	questionBank PracticeQuestionGenerator
	roadmap      RoadmapPreparer
	repository   RoleProfileRepository
	timeout      time.Duration
}

func NewRoleProfileApplication(extraction *ExtractionService, repo RoleProfileRepository, questionBank PracticeQuestionGenerator, timeout time.Duration) (*RoleProfileApplication, error) {
	if extraction == nil || repo == nil || questionBank == nil || timeout <= 0 {
		return nil, fmt.Errorf("invalid Role Profile application configuration")
	}
	return &RoleProfileApplication{extraction: extraction, questionBank: questionBank, repository: repo, timeout: timeout}, nil
}

// WithRoadmap makes Confirm generate the Roadmap and its bank through preparer instead of generating the
// bank alone. A nil preparer keeps the bank-only behavior.
func (a *RoleProfileApplication) WithRoadmap(preparer RoadmapPreparer) *RoleProfileApplication {
	a.roadmap = preparer
	return a
}
func (a *RoleProfileApplication) Create(ctx context.Context, user uuid.UUID, role string, raw string) (domain.RoleProfile, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoleProfile{}, err
	}
	if _, err := NormalizeInput(raw); err != nil {
		return domain.RoleProfile{}, err
	}
	return a.repository.CreateProfile(ctx, user, raw)
}
func (a *RoleProfileApplication) Get(ctx context.Context, user uuid.UUID, role string, id uuid.UUID) (domain.RoleProfile, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoleProfile{}, err
	}
	return a.repository.GetProfile(ctx, user, id)
}
func (a *RoleProfileApplication) List(ctx context.Context, user uuid.UUID, role string, limit, offset int32) ([]domain.RoleProfile, int64, error) {
	if err := requireCandidate(role); err != nil {
		return nil, 0, err
	}
	return a.repository.ListProfiles(ctx, user, limit, offset)
}
func (a *RoleProfileApplication) Delete(ctx context.Context, user uuid.UUID, role string, id uuid.UUID) error {
	if err := requireCandidate(role); err != nil {
		return err
	}
	return a.repository.DeleteProfile(ctx, user, id)
}
func (a *RoleProfileApplication) Save(ctx context.Context, user uuid.UUID, role string, id uuid.UUID, input domain.SaveRoleProfile) (domain.RoleProfile, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoleProfile{}, err
	}
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
func (a *RoleProfileApplication) Confirm(ctx context.Context, user uuid.UUID, role string, id uuid.UUID) (domain.RoleProfile, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoleProfile{}, err
	}
	profile, err := a.repository.GetProfile(ctx, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	if _, err = ValidateRoleReview(profile.RoleReview, true); err != nil {
		return domain.RoleProfile{}, err
	}
	if a.roadmap != nil {
		// An already confirmed profile is not generated again here: its Roadmap is retried explicitly.
		if profile.Status != domain.Confirmed {
			if err = a.roadmap.Prepare(ctx, user, id, profile.JDID); err != nil {
				return domain.RoleProfile{}, err
			}
		}
		return a.repository.ConfirmProfile(ctx, user, id)
	}
	if _, err = a.questionBank.GenerateForPractice(ctx, user, profile.JDID); err != nil {
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr == nil || appErr.Code != apperror.CodeQuestionBankExists {
			return domain.RoleProfile{}, err
		}
		// A retried confirmation reuses the bank written by the first attempt.
		logger.FromContext(ctx).Info("practice question bank already exists, reusing it",
			logger.String("profile_id", id.String()), logger.String("jd_id", profile.JDID.String()), logger.Op("roleprofile.confirm"))
	}
	return a.repository.ConfirmProfile(ctx, user, id)
}
func (a *RoleProfileApplication) budget() time.Duration {
	return time.Duration(a.extraction.maxRetries+1)*a.timeout + time.Second
}
func (a *RoleProfileApplication) release(ctx context.Context, profile domain.RoleProfile) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := a.repository.ReleaseAnalysis(cleanup, profile.ID); err != nil {
		// Nothing else logs this, and the profile may stay stuck in analyzing.
		logger.FromContext(ctx).Warn("role profile analysis release failed",
			logger.Op("roleprofile.analyze"), logger.String("profile_id", profile.ID.String()), logger.Cause(err))
	}
}
func (a *RoleProfileApplication) Analyze(ctx context.Context, user uuid.UUID, role string, id uuid.UUID) (domain.RoleProfile, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoleProfile{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, a.budget())
	defer cancel()
	profile, err := a.repository.ClaimAnalysis(ctx, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	profileID := logger.String("profile_id", id.String())
	if profile.Status == domain.Review {
		logger.FromContext(ctx).Info("role profile analysis skipped, already in review", logger.Op("roleprofile.analyze"), profileID)
		return profile, nil
	}
	gate := "extract"
	done := logger.Start(ctx, "roleprofile.analyze", profileID, logger.String("from_status", string(profile.Status)))
	jd, err := a.extraction.Extract(ctx, profile.RawText)
	if err == nil {
		gate = "role_review"
		review := ReviewFromExtraction(jd)
		review, err = ValidateRoleReview(review, false)
		if err == nil {
			gate = "evidence"
			err = ValidateEvidence(review, profile.RawText)
		}
		if err == nil {
			gate = "persist"
			var result domain.RoleProfile
			result, err = a.repository.CompleteAnalysis(ctx, user, id, review)
			if err == nil {
				done(nil, logger.Int("requirements", len(review.Requirements)), logger.String("to_status", string(result.Status)))
				return result, nil
			}
		}
	}
	a.release(ctx, profile)
	done(err, logger.String("gate", gate))
	var appErr *apperror.AppError
	if errors.As(err, &appErr) && appErr.Code == apperror.CodeValidation {
		err = apperror.New(apperror.CodeInvalidExtractionOutput, "Analysis returned unusable requirements. Please retry.")
	}
	return domain.RoleProfile{}, err
}

func requireCandidate(role string) error {
	if role != "candidate" {
		return apperror.New(apperror.CodeForbidden, "Role Profiles are available to candidates.")
	}
	return nil
}
