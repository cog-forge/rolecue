package domain

import (
	"fmt"
	"slices"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// BehaviorGuideline is what an Administrator may tune about the live interviewer (KAN-276, Brain Decision
// 16.6). Every field is an enum, a bounded integer or a subset of a closed catalog: no text an Admin types
// ever reaches a prompt, and the zero value of each field is not valid on purpose, so a guideline is always
// built from DefaultBehaviorGuideline or from validated input. Guidelines only change how follow-ups are
// decided and worded and how many are asked; core questions, scoring and stored results are out of reach.
type BehaviorGuideline struct {
	Tone           string   `json:"tone"`
	QuestionLength string   `json:"question_length"`
	FollowUpMax    *int32   `json:"follow_up_max"`
	FollowUpFocus  string   `json:"follow_up_focus"`
	AvoidTopics    []string `json:"avoid_topics"`
}

// Allowed values of each field. The default of an enum is its first value.
const (
	ToneNeutral     = "neutral"
	ToneFriendly    = "friendly"
	ToneFormal      = "formal"
	ToneChallenging = "challenging"

	LengthStandard = "standard"
	LengthConcise  = "concise"

	FocusStandard  = "standard"
	FocusVagueOnly = "vague_only"
	FocusThorough  = "thorough"

	TopicCompensation     = "compensation"
	TopicPersonalLife     = "personal_life"
	TopicPoliticsReligion = "politics_religion"
	TopicFormerEmployers  = "former_employers"
	TopicLocationNational = "location_nationality"
	MaxAvoidTopics        = 5
	MaxGuidelineFollowUps = int32(config.MaxInterviewFollowUpLimit)
	StandardQuestionRunes = 300
	ConciseQuestionRunes  = 150
)

// GuidelineTones, GuidelineLengths, GuidelineFocuses and GuidelineTopics list the allowed values in their
// canonical order (also the order avoid_topics is stored and rendered in).
var (
	GuidelineTones   = []string{ToneNeutral, ToneFriendly, ToneFormal, ToneChallenging}
	GuidelineLengths = []string{LengthStandard, LengthConcise}
	GuidelineFocuses = []string{FocusStandard, FocusVagueOnly, FocusThorough}
	GuidelineTopics  = []string{TopicCompensation, TopicPersonalLife, TopicPoliticsReligion, TopicFormerEmployers, TopicLocationNational}
)

// DefaultBehaviorGuideline is today's behavior: nothing added to the prompt and the configured limits.
func DefaultBehaviorGuideline() BehaviorGuideline {
	return BehaviorGuideline{
		Tone: ToneNeutral, QuestionLength: LengthStandard, FollowUpFocus: FocusStandard, AvoidTopics: []string{},
	}
}

// IsDefault reports whether the guideline changes nothing.
func (g BehaviorGuideline) IsDefault() bool {
	return g.Tone == ToneNeutral && g.QuestionLength == LengthStandard && g.FollowUpMax == nil &&
		g.FollowUpFocus == FocusStandard && len(g.AvoidTopics) == 0
}

// Validate checks Admin input and returns a validation error naming the field, never the value. It does not
// change the guideline; Canonical does.
func (g BehaviorGuideline) Validate() error {
	invalid := func(field, rule string) error {
		return apperror.New(apperror.CodeValidation, fmt.Sprintf("%s %s", field, rule))
	}
	if !slices.Contains(GuidelineTones, g.Tone) {
		return invalid("tone", "must be one of neutral, friendly, formal, challenging")
	}
	if !slices.Contains(GuidelineLengths, g.QuestionLength) {
		return invalid("question_length", "must be one of standard, concise")
	}
	if !slices.Contains(GuidelineFocuses, g.FollowUpFocus) {
		return invalid("follow_up_focus", "must be one of standard, vague_only, thorough")
	}
	if g.FollowUpMax != nil && (*g.FollowUpMax < 0 || *g.FollowUpMax > MaxGuidelineFollowUps) {
		return invalid("follow_up_max", fmt.Sprintf("must be null or between 0 and %d", MaxGuidelineFollowUps))
	}
	if len(g.AvoidTopics) > MaxAvoidTopics {
		return invalid("avoid_topics", fmt.Sprintf("must have at most %d topics", MaxAvoidTopics))
	}
	seen := map[string]bool{}
	for _, topic := range g.AvoidTopics {
		if !slices.Contains(GuidelineTopics, topic) {
			return invalid("avoid_topics", "must only contain topics from the catalog")
		}
		if seen[topic] {
			return invalid("avoid_topics", "must not repeat a topic")
		}
		seen[topic] = true
	}
	return nil
}

// Canonical returns the guideline with the topics in catalog order and a non-nil list, so equal guidelines
// compare and store equal. Call it after Validate.
func (g BehaviorGuideline) Canonical() BehaviorGuideline {
	topics := make([]string, 0, len(g.AvoidTopics))
	for _, topic := range GuidelineTopics {
		if slices.Contains(g.AvoidTopics, topic) {
			topics = append(topics, topic)
		}
	}
	g.AvoidTopics = topics
	return g
}

// Sanitized makes a stored guideline safe to compose with: a value outside the allowed sets (which the
// database constraints should make impossible) falls back to the default of that field, so an unknown value
// is never interpolated anywhere. It reports whether anything had to be replaced.
func (g BehaviorGuideline) Sanitized() (BehaviorGuideline, bool) {
	clean, changed := g, false
	fallback := func(allowed []string, value string) string {
		if slices.Contains(allowed, value) {
			return value
		}
		changed = true
		return allowed[0]
	}
	clean.Tone = fallback(GuidelineTones, g.Tone)
	clean.QuestionLength = fallback(GuidelineLengths, g.QuestionLength)
	clean.FollowUpFocus = fallback(GuidelineFocuses, g.FollowUpFocus)
	if g.FollowUpMax != nil && (*g.FollowUpMax < 0 || *g.FollowUpMax > MaxGuidelineFollowUps) {
		clean.FollowUpMax, changed = nil, true
	}
	topics := make([]string, 0, len(g.AvoidTopics))
	for _, topic := range GuidelineTopics {
		if slices.Contains(g.AvoidTopics, topic) {
			topics = append(topics, topic)
		}
	}
	if len(topics) != len(g.AvoidTopics) {
		changed = true
	}
	clean.AvoidTopics = topics
	return clean, changed
}

// EffectiveFollowUpLimit is the most follow-ups one core question gets: the configured limit, lowered (never
// raised) by follow_up_max. Zero means no follow-ups.
func (g BehaviorGuideline) EffectiveFollowUpLimit(configured int32) int32 {
	if g.FollowUpMax != nil && *g.FollowUpMax < configured {
		return max(*g.FollowUpMax, 0)
	}
	return configured
}

// MaxQuestionRunes is the longest follow-up the validator accepts under this guideline.
func (g BehaviorGuideline) MaxQuestionRunes() int {
	if g.QuestionLength == LengthConcise {
		return ConciseQuestionRunes
	}
	return StandardQuestionRunes
}
