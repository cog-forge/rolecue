package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
	"github.com/google/uuid"
)

const (
	// maxCriterionFeedbackRunes bounds the feedback of one criterion.
	maxCriterionFeedbackRunes = 600
	// maxQuestionFeedbackRunes bounds the feedback of one core question, stored on its core turn.
	maxQuestionFeedbackRunes = 2000
	// maxQuestionSummaryRunes bounds the one-sentence summary of one core question.
	maxQuestionSummaryRunes = 600
)

// Grader grades one core question against its rubric (KAN-334). It returns the decoded reply; the service
// validates it with ValidateQuestionGrade. The routed AI adapter runs that validation inside every provider
// attempt, so an invalid reply is retried or falls back like any other unusable output.
type Grader interface {
	GradeQuestion(ctx context.Context, in domain.QuestionGradeInput) (domain.QuestionGrade, error)
}

// errInvalidGrade wraps every refusal of a grading reply. The messages name rules and criterion keys only,
// never answer, rubric or expected-answer text, because they reach the logs.
var errInvalidGrade = errors.New("invalid grading output")

func invalidGrade(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errInvalidGrade, fmt.Sprintf(format, args...))
}

// IsInvalidGrade reports whether err is a refused grading reply.
func IsInvalidGrade(err error) bool { return errors.Is(err, errInvalidGrade) }

// ValidateQuestionGrade checks a decoded reply against the question it grades and returns the trusted
// result. The reply must grade every criterion of the rubric exactly once with a whole number from 0 to its
// maximum; nothing is repaired by guessing. A criterion with points needs a quote that appears in this
// question's own answers (a quote from another question, or from no answer at all, is refused); a criterion
// with 0 points may have no quote. Feedback and the summary are bounded and sanitised: text that echoes an
// instruction is dropped and flags the question, as before.
func ValidateQuestionGrade(in domain.QuestionGradeInput, out domain.QuestionGradeCandidate) (domain.QuestionGrade, error) {
	if err := validateGradeInput(in); err != nil {
		return domain.QuestionGrade{}, err
	}
	byID := make(map[uuid.UUID]domain.GradeCriterion, len(in.Criteria))
	for _, c := range in.Criteria {
		byID[c.ID] = c
	}
	if len(out.Criteria) != len(in.Criteria) {
		return domain.QuestionGrade{}, invalidGrade("graded %d criteria, want %d", len(out.Criteria), len(in.Criteria))
	}
	answers := evidenceSources(in.Turns)
	flagged := out.InjectionSuspected || screenGradeInjection(in.Turns)
	graded := make(map[uuid.UUID]domain.CriterionGrade, len(out.Criteria))
	for _, candidate := range out.Criteria {
		criterion, ok := byID[candidate.CriterionID]
		if !ok {
			return domain.QuestionGrade{}, invalidGrade("graded an unknown criterion")
		}
		if _, dup := graded[candidate.CriterionID]; dup {
			return domain.QuestionGrade{}, invalidGrade("graded criterion %s twice", criterion.Key)
		}
		if candidate.Points < 0 || candidate.Points > criterion.MaxPoints {
			return domain.QuestionGrade{}, invalidGrade("criterion %s has %d points, want 0..%d", criterion.Key, candidate.Points, criterion.MaxPoints)
		}
		result := domain.CriterionGrade{
			CriterionID: criterion.ID, MetricID: criterion.MetricID, MaxPoints: criterion.MaxPoints, Points: candidate.Points,
		}
		quote := strings.TrimSpace(candidate.Evidence)
		switch position, found := locateEvidence(quote, answers); {
		case found:
			result.Evidence, result.EvidencePosition = quote, &position
		case quote == "" && candidate.Points == 0:
		case quote == "":
			return domain.QuestionGrade{}, invalidGrade("criterion %s has points but no evidence", criterion.Key)
		default:
			return domain.QuestionGrade{}, invalidGrade("criterion %s quotes text that is not in this question's answers", criterion.Key)
		}
		feedback, echoed, err := boundedModelText(candidate.Feedback, maxCriterionFeedbackRunes)
		if err != nil {
			return domain.QuestionGrade{}, invalidGrade("feedback of criterion %s: %v", criterion.Key, err)
		}
		flagged = flagged || echoed
		result.Feedback = feedback
		graded[criterion.ID] = result
	}
	feedback, echoed, err := boundedModelText(out.Feedback, maxQuestionFeedbackRunes)
	if err != nil {
		return domain.QuestionGrade{}, invalidGrade("question feedback: %v", err)
	}
	summary, summaryEchoed, err := boundedModelText(out.Summary, maxQuestionSummaryRunes)
	if err != nil {
		return domain.QuestionGrade{}, invalidGrade("question summary: %v", err)
	}
	result := domain.QuestionGrade{
		QuestionPosition: in.QuestionPosition,
		Feedback:         feedback,
		Summary:          summary,
		InjectionFlagged: flagged || echoed || summaryEchoed,
	}
	for _, c := range in.Criteria {
		result.Criteria = append(result.Criteria, graded[c.ID])
	}
	return result, nil
}

// validateGradeInput refuses a question the grader must never be asked about: no criteria, a criterion
// that cannot be scored, a repeated id or key, no expected answer or no answered turn.
func validateGradeInput(in domain.QuestionGradeInput) error {
	if len(in.Criteria) == 0 {
		return invalidGrade("question %d has no criteria", in.QuestionPosition)
	}
	if strings.TrimSpace(in.ExpectedAnswer) == "" {
		return invalidGrade("question %d has no expected answer", in.QuestionPosition)
	}
	if len(in.Turns) == 0 {
		return invalidGrade("question %d has no answered turn", in.QuestionPosition)
	}
	ids := make(map[uuid.UUID]bool, len(in.Criteria))
	keys := make(map[string]bool, len(in.Criteria))
	for _, c := range in.Criteria {
		if c.ID == uuid.Nil || c.MetricID == uuid.Nil || c.Key == "" || c.MaxPoints <= 0 || ids[c.ID] || keys[c.Key] {
			return invalidGrade("question %d has an invalid or repeated criterion", in.QuestionPosition)
		}
		ids[c.ID], keys[c.Key] = true, true
	}
	return nil
}

// boundedModelText trims, length-checks and moderates text the model wrote. echoed is true when the text
// repeated an instruction-like pattern; such text is dropped.
func boundedModelText(text string, maxRunes int) (clean string, echoed bool, err error) {
	text = strings.TrimSpace(text)
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > maxRunes {
		return "", false, errors.New("invalid or too long")
	}
	clean, echoed = moderateText(text)
	return clean, echoed, nil
}

// evidenceSource is one answer a quote may come from, normalised, with the position of its turn.
type evidenceSource struct {
	position int32
	text     string
}

// evidenceSources returns the normalised answers of one question's turns. The grader saw each answer
// cleaned and with identifiers replaced by placeholders (KAN-298), so that form is accepted too.
func evidenceSources(turns []domain.GradeTurn) []evidenceSource {
	sources := make([]evidenceSource, 0, 2*len(turns))
	for _, turn := range turns {
		plain := normalizeEvidenceText(turn.Answer)
		sources = append(sources, evidenceSource{position: turn.Position, text: plain})
		if redacted := normalizeEvidenceText(textsafe.Redact(textsafe.Clean(turn.Answer))); redacted != plain {
			sources = append(sources, evidenceSource{position: turn.Position, text: redacted})
		}
	}
	return sources
}

// locateEvidence finds the turn whose answer contains the quote (see verifyEvidence for the matching rules).
// Models often shorten a long quote by joining verbatim parts of one answer with an ellipsis ("A ... B"); such a
// quote is accepted only when every part is itself a verbatim quote of at least minEvidenceRunes and the parts
// appear in that order in the same answer. Nothing is repaired: the quote is stored as the model wrote it.
func locateEvidence(quote string, sources []evidenceSource) (int32, bool) {
	if quote == "" {
		return 0, false
	}
	for _, source := range sources {
		if verifyEvidence(quote, []string{source.text}) {
			return source.position, true
		}
	}
	parts := ellipsisParts(quote)
	if len(parts) < 2 {
		return 0, false
	}
	for _, source := range sources {
		if partsInOrder(parts, source.text) {
			return source.position, true
		}
	}
	return 0, false
}

// ellipsisParts splits a quote at "...", "…" or "[...]" into its normalised, non-empty parts. It returns nil
// when the quote has no ellipsis, is too long, or has a part too short to verify anything on its own.
func ellipsisParts(quote string) []string {
	if !utf8.ValidString(quote) || utf8.RuneCountInString(normalizeEvidenceText(quote)) > maxEvidenceRunes {
		return nil
	}
	marked := strings.NewReplacer("[...]", "\x00", "[…]", "\x00", "...", "\x00", "…", "\x00").Replace(quote)
	if !strings.Contains(marked, "\x00") {
		return nil
	}
	var parts []string
	for _, part := range strings.Split(marked, "\x00") {
		part = normalizeEvidenceText(part)
		if part == "" {
			continue
		}
		if utf8.RuneCountInString(part) < minEvidenceRunes {
			return nil
		}
		parts = append(parts, part)
	}
	return parts
}

// partsInOrder reports whether every part appears in the answer, each after the previous one.
func partsInOrder(parts []string, answer string) bool {
	rest := answer
	for _, part := range parts {
		i := strings.Index(rest, part)
		if i < 0 {
			return false
		}
		rest = rest[i+len(part):]
	}
	return true
}

// screenGradeInjection reports whether an answer, or a follow-up question generated from answers, matches a
// common prompt-injection pattern. A hit only flags the evaluation.
func screenGradeInjection(turns []domain.GradeTurn) bool {
	for _, turn := range turns {
		if textsafe.LooksLikeInjection(turn.Answer) {
			return true
		}
		if turn.Kind == domain.TurnFollowUp && textsafe.LooksLikeInjection(turn.Question) {
			return true
		}
	}
	return false
}
