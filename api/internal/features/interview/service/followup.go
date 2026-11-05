package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FollowUpDecider decides whether the interviewer asks a follow-up after an answer and, if so, words it
// (Decision 14). It is a seam: no decision vendor is selected, and the default adapter is one call to the
// configured LLM. Implementations must honour ctx; the service bounds the call either way and validates
// the proposal before anything is asked. The context carries the Candidate's answers, which are
// untrusted data and must never be followed as instructions.
type FollowUpDecider interface {
	Decide(ctx context.Context, in domain.FollowUpContext) (domain.FollowUpProposal, error)
}

const (
	// maxFollowUpRunes bounds a follow-up question: one short spoken question.
	maxFollowUpRunes = 300
	// maxFollowUpReasonRunes bounds the decider's internal reason.
	maxFollowUpReasonRunes = 300
)

// Outcomes of a follow-up decision, and the failure classes of a skipped one. Only these are logged.
const (
	followUpAsked         = "asked"
	followUpDeclined      = "declined"
	followUpSkipped       = "skipped"
	followUpTimeout       = "timeout"
	followUpProviderError = "provider_error"
	followUpInvalidOutput = "invalid_output"
)

// SetFollowUpDecider sets the follow-up decider. Nil turns follow-ups off whatever FollowUpLimit says. Set
// it before the service serves requests.
func (s *Service) SetFollowUpDecider(decider FollowUpDecider) {
	s.decider = decider
}

// SetLLMBudget sets the per-user daily LLM budget (KAN-278) charged for each follow-up decision. Nil
// leaves follow-ups unbudgeted. A spent budget never fails the interview: the follow-up is skipped and the
// interview moves on to the next core question. Set it before the service serves requests.
func (s *Service) SetLLMBudget(guard llmbudget.Guard) {
	s.llmBudget = guard
}

// followUpLimit is the limit the turn rules apply: zero when follow-ups are off.
func (s *Service) followUpLimit() int32 {
	if s.decider == nil {
		return 0
	}
	return s.settings.FollowUpLimit
}

// FollowUp decides, right after a new answer was stored, whether the interviewer asks a follow-up next
// (Decision 14). It returns the follow-up as the Delivery's Question, or a Delivery without a question when
// none is asked: follow-ups are off, the limit for this core question is reached, or the decider declined
// or failed. The follow-up is not stored: it is stored with its answer (AppendFollowUp), and one that is
// never answered is dropped, so a reconnect goes on with the next core question.
func (s *Service) FollowUp(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Delivery, error) {
	if role != candidateRole {
		return domain.Delivery{}, apperror.New(apperror.CodeForbidden, "only candidates can take interviews")
	}
	limit := s.followUpLimit()
	if limit == 0 {
		return domain.Delivery{}, nil
	}
	delivery, pending, err := s.repository.FollowUpContext(ctx, candidateID, id, s.settings.DefaultProviderVoiceID, limit)
	if err != nil || pending == nil {
		return delivery, err
	}
	if s.llmBudget != nil {
		if err := s.llmBudget.Consume(ctx, candidateID); err != nil {
			// Counts only: no answer, prompt or user text is logged.
			logger.FromContext(ctx).Warn("interview follow-up skipped",
				logger.Op("interview.follow_up"), logger.String("interview_id", id.String()),
				logger.String("outcome", followUpSkipped), logger.String("reason", "llm_budget"))
			return delivery, nil
		}
	}
	text, cues, ok := s.decideFollowUp(ctx, id, *pending)
	if !ok {
		return delivery, nil
	}
	delivery.Question = &domain.AskedQuestion{
		Position: pending.Position, QuestionNumber: pending.QuestionNumber, Kind: domain.TurnFollowUp,
		Text: text, AskedAt: time.Now().UTC(), Cues: cues,
	}
	return delivery, nil
}

// AppendFollowUp stores the Candidate's answer to a follow-up the live interviewer asked, together with
// the follow-up itself. The follow-up comes from the engine that asked it, never from the client.
func (s *Service) AppendFollowUp(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID, followUp domain.AskedQuestion, answer string) (domain.Session, error) {
	if role != candidateRole {
		return domain.Session{}, apperror.New(apperror.CodeForbidden, "only candidates can answer interviews")
	}
	if followUp.Kind != domain.TurnFollowUp || followUp.Position < 1 || strings.TrimSpace(followUp.Text) == "" {
		return domain.Session{}, apperror.New(apperror.CodeInternal, "the follow-up to answer is invalid")
	}
	normalized, err := normalizeAnswer(answer)
	if err != nil {
		return domain.Session{}, err
	}
	session, err := s.repository.AppendFollowUp(ctx, candidateID, id, followUp, normalized, s.settings.FollowUpLimit)
	if err != nil {
		return domain.Session{}, err
	}
	logger.FromContext(ctx).Info("interview answer recorded",
		logger.Op("interview.answer"),
		logger.String("interview_id", id.String()),
		logger.String("user_id", candidateID.String()),
		logger.String("kind", string(domain.TurnFollowUp)),
		logger.Int("question_index", int(followUp.Position)),
		logger.Int("question_number", int(followUp.QuestionNumber)),
		logger.Int("bytes", len(normalized)),
	)
	return presentSession(session), nil
}

// decideFollowUp asks the decider about one slot and returns the follow-up to ask, if any, with its
// validated performance cues (nil when none were proposed or the list was invalid). It never fails: a
// timeout, a provider error, a panic or an invalid proposal is skipped, and the interview moves on to the
// next core question. Only the classified outcome and the duration are logged, never the answer, the
// prompt, the proposed text or the decider's reason.
func (s *Service) decideFollowUp(ctx context.Context, interviewID uuid.UUID, in domain.FollowUpContext) (string, []domain.PerformanceCue, bool) {
	started := time.Now()
	proposal, err := s.proposeFollowUp(ctx, in)
	if err == nil {
		err = validateProposal(&proposal, in)
	}
	outcome, reason := followUpDeclined, ""
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome, reason = followUpSkipped, followUpTimeout
	case errors.Is(err, ErrInvalidFollowUp):
		outcome, reason = followUpSkipped, followUpInvalidOutput
	case err != nil:
		outcome, reason = followUpSkipped, followUpProviderError
	case proposal.FollowUp:
		outcome = followUpAsked
	}
	fields := []zap.Field{
		logger.Op("interview.follow_up"),
		logger.String("interview_id", interviewID.String()),
		logger.Int("question_number", int(in.QuestionNumber)),
		logger.Int("ordinal", int(in.Ordinal)),
		logger.String("outcome", outcome),
		logger.Duration("duration", time.Since(started)),
	}
	if outcome != followUpSkipped {
		fields = append(fields, logger.Int("cues", len(proposal.Cues)))
		logger.FromContext(ctx).Info("interview follow-up decided", fields...)
		return proposal.Question, proposal.Cues, outcome == followUpAsked
	}
	// The cause of an invalid proposal names the rule only; a provider error carries no prompt.
	fields = append(fields, logger.String("reason", reason))
	if reason != followUpTimeout {
		fields = append(fields, logger.Cause(err))
	}
	logger.FromContext(ctx).Warn("interview follow-up skipped", fields...)
	return "", nil, false
}

// proposeFollowUp runs the decider within FollowUpTimeout. The call runs in its own goroutine, so a
// decider that ignores ctx still cannot hold the interview past the timeout, and a panic in it (or its
// LLM client) becomes an error.
func (s *Service) proposeFollowUp(ctx context.Context, in domain.FollowUpContext) (domain.FollowUpProposal, error) {
	ctx, cancel := context.WithTimeout(ctx, s.settings.FollowUpTimeout)
	defer cancel()
	type result struct {
		proposal domain.FollowUpProposal
		err      error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				// The panic value is not used: it could carry the answer the decider was given.
				done <- result{err: fmt.Errorf("follow-up decider panicked (%T)", recovered)}
			}
		}()
		proposal, err := s.decider.Decide(ctx, in)
		done <- result{proposal: proposal, err: err}
	}()
	select {
	case r := <-done:
		if r.err != nil && ctx.Err() != nil {
			return domain.FollowUpProposal{}, context.DeadlineExceeded
		}
		return r.proposal, r.err
	case <-ctx.Done():
		return domain.FollowUpProposal{}, context.DeadlineExceeded
	}
}

// ErrInvalidFollowUp marks a proposal that breaks the output contract. A decider wraps it around a reply
// it cannot decode; the service wraps it around a proposal it refuses. Either way the slot is skipped as
// invalid_output.
var ErrInvalidFollowUp = errors.New("invalid follow-up proposal")

func invalidProposal(rule string) error {
	return fmt.Errorf("%w: %s", ErrInvalidFollowUp, rule)
}

// validateProposal checks an untrusted proposal and normalizes it in place. A follow-up is one bounded
// English question on one line, different from what was already asked; a declined proposal carries no
// question. Both carry a short reason. The error names the rule only, never the text.
//
// Performance cues never make a proposal invalid: the question is what matters. An unusable cue list
// (outside the vocabulary, too long, a word index past the question or a duplicate) is dropped as a
// whole, and the live engine falls back to its rules for the line (KAN-250).
func validateProposal(p *domain.FollowUpProposal, in domain.FollowUpContext) error {
	p.Reason = strings.TrimSpace(p.Reason)
	if p.Reason == "" || !singleLine(p.Reason) || utf8.RuneCountInString(p.Reason) > maxFollowUpReasonRunes {
		return invalidProposal("reason must be one line of 1 to 300 characters")
	}
	p.Question = strings.TrimSpace(p.Question)
	if !p.FollowUp {
		p.Cues = nil
		if p.Question != "" {
			return invalidProposal("a declined follow-up has no question")
		}
		return nil
	}
	if len(p.Cues) == 0 || !domain.ValidCues(p.Cues, len(strings.Fields(p.Question))) {
		p.Cues = nil
	}
	switch {
	case p.Question == "" || utf8.RuneCountInString(p.Question) > maxFollowUpRunes:
		return invalidProposal("question must have 1 to 300 characters")
	case !singleLine(p.Question):
		return invalidProposal("question must be one line")
	case !englishText(p.Question):
		return invalidProposal("question must be English")
	case !strings.HasSuffix(p.Question, "?") || strings.Count(p.Question, "?") != 1:
		return invalidProposal("question must be exactly one question")
	}
	if strings.EqualFold(p.Question, strings.TrimSpace(in.CoreQuestion)) {
		return invalidProposal("question repeats the core question")
	}
	for _, exchange := range in.Exchanges {
		if strings.EqualFold(p.Question, strings.TrimSpace(exchange.Question)) {
			return invalidProposal("question repeats an earlier question")
		}
	}
	return nil
}

// singleLine reports valid UTF-8 without control characters (newlines included).
func singleLine(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// englishText accepts ASCII and typographic punctuation only. The interview is conducted in English
// (frontend/docs/i18n.md), and a letter outside ASCII means the model switched language or script.
func englishText(text string) bool {
	for _, r := range text {
		if r < unicode.MaxASCII {
			continue
		}
		switch r {
		case '‘', '’', '“', '”', '–', '—', '…':
			continue
		}
		return false
	}
	return true
}
