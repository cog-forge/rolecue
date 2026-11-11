// Package domain holds the Admin Interview Behavior Guidelines settings (KAN-276). The guideline values
// themselves live in the interview domain, because the interviewer consumes them.
package domain

import (
	"time"

	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/google/uuid"
)

// Stored is one saved version of the guidelines.
type Stored struct {
	ID            uuid.UUID
	Version       int32
	Values        interviewdomain.BehaviorGuideline
	UpdatedBy     uuid.UUID
	UpdatedByName string
	UpdatedAt     time.Time
}

// Allowed lists what each field accepts, so a client never hard-codes the vocabulary.
type Allowed struct {
	Tone           []string `json:"tone"`
	QuestionLength []string `json:"question_length"`
	FollowUpMax    []int32  `json:"follow_up_max"`
	FollowUpFocus  []string `json:"follow_up_focus"`
	AvoidTopics    []string `json:"avoid_topics"`
}

// Settings is what the Admin API returns. Version 0 means nothing was saved yet and Values are the defaults.
// EffectiveFollowUpCeiling is the configured interview.follow_up_limit: follow_up_max can lower it, never
// raise it.
type Settings struct {
	Version                  int32                             `json:"version"`
	Values                   interviewdomain.BehaviorGuideline `json:"values"`
	Defaults                 interviewdomain.BehaviorGuideline `json:"defaults"`
	Allowed                  Allowed                           `json:"allowed"`
	EffectiveFollowUpCeiling int32                             `json:"effective_follow_up_ceiling"`
	UpdatedAt                *time.Time                        `json:"updated_at"`
	UpdatedByName            string                            `json:"updated_by_name"`
}

// AllowedValues returns the vocabulary of every field.
func AllowedValues() Allowed {
	maxes := make([]int32, 0, interviewdomain.MaxGuidelineFollowUps+1)
	for n := int32(0); n <= interviewdomain.MaxGuidelineFollowUps; n++ {
		maxes = append(maxes, n)
	}
	return Allowed{
		Tone:           append([]string{}, interviewdomain.GuidelineTones...),
		QuestionLength: append([]string{}, interviewdomain.GuidelineLengths...),
		FollowUpMax:    maxes,
		FollowUpFocus:  append([]string{}, interviewdomain.GuidelineFocuses...),
		AvoidTopics:    append([]string{}, interviewdomain.GuidelineTopics...),
	}
}
