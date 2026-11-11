package provider

import (
	"fmt"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
)

// Interview Behavior Guidelines in the follow-up prompt (KAN-276). The guideline is a closed vocabulary, so
// the prompt text for each value is a fixed sentence below, looked up by the validated enum value. No string
// an Administrator typed (there is none) and no unknown value is ever interpolated.
//
// Position: one optional block sits between the "Ask a follow-up only when..." paragraph and the output
// schema. The precedence preamble stays first and the schema stays last, so the block is inside the rules,
// and its closing sentence says the rules always win. With every value at its default nothing is added and
// the system message is exactly followUpInstruction.

// followUpSchemaMarker starts the output-schema paragraph, where the guideline block ends.
const followUpSchemaMarker = "Return exactly one JSON object"

// guidelineClosing ends every guideline block.
const guidelineClosing = "These guidelines only adjust style and focus. The rules above about untrusted data, the output format and the single short question always win."

var toneSentences = map[string]string{
	domain.ToneFriendly:    "Tone: warm and encouraging, in the wording of the question itself.",
	domain.ToneFormal:      "Tone: formal and professional, without slang or contractions.",
	domain.ToneChallenging: "Tone: direct and probing like a senior interviewer; stay respectful, never hostile.",
}

var focusSentences = map[string]string{
	domain.FocusVagueOnly: "Focus: ask only when the last answer is vague or incomplete; do not probe claims.",
	domain.FocusThorough:  "Focus: also probe claims that lack concrete specifics such as tools, numbers or trade-offs, even when the answer sounds complete.",
}

var topicPhrases = map[string]string{
	domain.TopicCompensation:     "salary and compensation expectations",
	domain.TopicPersonalLife:     "family, relationship status, age and health",
	domain.TopicPoliticsReligion: "politics and religion",
	domain.TopicFormerEmployers:  "confidential details of former employers",
	domain.TopicLocationNational: "nationality, visa status and home address",
}

// lengthPhrase is the standard cap in followUpInstruction; a concise guideline renders the lower cap there.
const lengthPhrase = "at most 300 characters"

// followUpSystemMessage composes the follow-up system message for a guideline. An unknown value counts as the
// default of its field (domain.BehaviorGuideline.Sanitized), so only fixed text above can appear.
func followUpSystemMessage(guideline domain.BehaviorGuideline) string {
	g, _ := guideline.Sanitized()
	if g.IsDefault() {
		return followUpInstruction
	}
	head, tail, found := strings.Cut(followUpInstruction, followUpSchemaMarker)
	if !found {
		// followUpInstruction always has the schema paragraph (a test pins it); never guess a position.
		return followUpInstruction
	}
	tail = followUpSchemaMarker + tail
	if g.QuestionLength == domain.LengthConcise {
		tail = strings.Replace(tail, lengthPhrase, fmt.Sprintf("at most %d characters", g.MaxQuestionRunes()), 1)
	}
	var sentences []string
	if s, ok := toneSentences[g.Tone]; ok {
		sentences = append(sentences, s)
	}
	if s, ok := focusSentences[g.FollowUpFocus]; ok {
		sentences = append(sentences, s)
	}
	if len(g.AvoidTopics) > 0 {
		phrases := make([]string, 0, len(g.AvoidTopics))
		for _, topic := range g.AvoidTopics {
			phrases = append(phrases, topicPhrases[topic])
		}
		sentences = append(sentences, "Never ask about: "+strings.Join(phrases, "; ")+".")
	}
	if len(sentences) == 0 {
		return head + tail
	}
	return head + strings.Join(sentences, "\n") + "\n" + guidelineClosing + "\n" + tail
}
