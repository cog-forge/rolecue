package service

import (
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
)

// maxFollowUpQuestionRunes caps a follow-up question that goes back into the scorer's input: the interview
// service limits one to 300 characters when it is asked.
const maxFollowUpQuestionRunes = 300

// scorerQuestion returns the question text the scorer sees. A core question comes from the bank. A follow-up
// was written by a model from earlier candidate answers, so it is untrusted: cleaned of invisible
// characters and capped (KAN-279). The stored question is unchanged.
func scorerQuestion(turn domain.TurnRecord) string {
	if turn.Kind != domain.TurnFollowUp {
		return turn.Question
	}
	return strings.TrimSpace(textsafe.Truncate(textsafe.Clean(turn.Question), maxFollowUpQuestionRunes))
}

// moderateText cleans model-written text before it is stored: markup, links and invisible characters go.
// Text that echoes an instruction-like pattern is not stored at all and flags the evaluation; the length
// rules stay with the caller.
func moderateText(text string) (clean string, echoed bool) {
	clean = textsafe.SanitizeOutput(text)
	if textsafe.LooksLikeInjection(clean) {
		return "", true
	}
	return clean, false
}
