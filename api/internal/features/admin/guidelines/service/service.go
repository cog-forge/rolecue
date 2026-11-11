// Package service is the Admin side of the Interview Behavior Guidelines (KAN-276): read and save the
// bounded guideline values. The interviewer consumes the saved version for the sessions that start after it.
package service

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/domain"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const adminRole = "admin"

// Repository is the persistence of the guideline versions.
type Repository interface {
	Current(ctx context.Context) (domain.Stored, bool, error)
	Save(ctx context.Context, adminID uuid.UUID, expectedVersion int32, values interviewdomain.BehaviorGuideline) (domain.Stored, error)
}

type Service struct {
	repository Repository
	ceiling    int32
}

// New takes the configured interview.follow_up_limit, the ceiling follow_up_max can only lower.
func New(repository Repository, followUpCeiling int32) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("interview guidelines repository is required")
	}
	if followUpCeiling < 0 {
		return nil, fmt.Errorf("interview guidelines follow-up ceiling must not be negative")
	}
	return &Service{repository: repository, ceiling: followUpCeiling}, nil
}

func authorize(role string) error {
	if role != adminRole {
		return apperror.New(apperror.CodeForbidden, "only administrators can manage interview behavior guidelines")
	}
	return nil
}

// Get returns the current guidelines, or the defaults at version 0 when nothing was saved. The role is
// checked before anything is read.
func (s *Service) Get(ctx context.Context, role string) (domain.Settings, error) {
	if err := authorize(role); err != nil {
		return domain.Settings{}, err
	}
	stored, found, err := s.repository.Current(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	return s.settings(stored, found), nil
}

// Save validates the values and appends them as the next version. expectedVersion is the version the Admin
// loaded (0 before the first save); any other current version is GUIDELINE_VERSION_CONFLICT and writes
// nothing. Validation and the role are checked before the repository is touched.
func (s *Service) Save(ctx context.Context, adminID uuid.UUID, role string, expectedVersion int32, values interviewdomain.BehaviorGuideline) (domain.Settings, error) {
	if err := authorize(role); err != nil {
		return domain.Settings{}, err
	}
	if expectedVersion < 0 {
		return domain.Settings{}, apperror.New(apperror.CodeValidation, "expected_version must not be negative")
	}
	if err := values.Validate(); err != nil {
		return domain.Settings{}, err
	}
	values = values.Canonical()
	previous, found, err := s.repository.Current(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	saved, err := s.repository.Save(ctx, adminID, expectedVersion, values)
	if err != nil {
		return domain.Settings{}, err
	}
	before := interviewdomain.DefaultBehaviorGuideline()
	if found {
		before = previous.Values
	}
	// Field names only: the values are a closed vocabulary, but the log needs no more than what changed.
	logger.FromContext(ctx).Info("interview guidelines saved",
		logger.Op("admin.interview_guidelines_save"),
		logger.String("admin_id", adminID.String()),
		logger.Int("version", int(saved.Version)),
		logger.String("changed", fmt.Sprint(changedFields(before, saved.Values))),
	)
	return s.settings(saved, true), nil
}

func (s *Service) settings(stored domain.Stored, found bool) domain.Settings {
	settings := domain.Settings{
		Defaults: interviewdomain.DefaultBehaviorGuideline(), Allowed: domain.AllowedValues(), EffectiveFollowUpCeiling: s.ceiling,
		Values: interviewdomain.DefaultBehaviorGuideline(),
	}
	if !found {
		return settings
	}
	settings.Version = stored.Version
	settings.Values = stored.Values
	updated := stored.UpdatedAt
	settings.UpdatedAt = &updated
	settings.UpdatedByName = stored.UpdatedByName
	return settings
}

func changedFields(before, after interviewdomain.BehaviorGuideline) []string {
	changed := []string{}
	if before.Tone != after.Tone {
		changed = append(changed, "tone")
	}
	if before.QuestionLength != after.QuestionLength {
		changed = append(changed, "question_length")
	}
	if !reflect.DeepEqual(before.FollowUpMax, after.FollowUpMax) {
		changed = append(changed, "follow_up_max")
	}
	if before.FollowUpFocus != after.FollowUpFocus {
		changed = append(changed, "follow_up_focus")
	}
	if !reflect.DeepEqual(before.AvoidTopics, after.AvoidTopics) {
		changed = append(changed, "avoid_topics")
	}
	return changed
}
