package service

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	candidateRole = "candidate"
	// defaultQuestionLimit is used when the configured question limit is zero.
	defaultQuestionLimit = 8
	// defaultFollowUpTimeout bounds one follow-up decision when the setting is zero.
	defaultFollowUpTimeout = 5 * time.Second
	// evaluationTimeout bounds one background evaluation, which runs after the request has returned.
	evaluationTimeout = 2 * time.Minute
	// failureWriteTimeout bounds the write that marks a panicked evaluation failed. It runs on a fresh
	// context, so an evaluation that panicked after its own timeout is still marked.
	failureWriteTimeout = 10 * time.Second
)

// Repository is the persistence contract of interview sessions. It runs each write in one transaction and
// takes the locks the rules need; the service decides what may be asked and what the Candidate sees.
type Repository interface {
	Create(ctx context.Context, params domain.CreateParams) (domain.Session, bool, error)
	Get(ctx context.Context, candidateID, id uuid.UUID) (domain.Session, error)
	AppendTurn(ctx context.Context, candidateID, id uuid.UUID, position int32, answer string) (domain.Session, error)
	End(ctx context.Context, params domain.EndParams) (domain.EndResult, error)
	ListAbandoned(ctx context.Context, now, idleBefore time.Time, limit int32) ([]domain.SessionRef, error)
	MarkEvaluationFailed(ctx context.Context, id uuid.UUID) error
	Deliver(ctx context.Context, candidateID, id uuid.UUID, defaultProviderVoiceID string) (domain.Delivery, error)
	FollowUpContext(ctx context.Context, candidateID, id uuid.UUID, defaultProviderVoiceID string, followUpLimit int32) (domain.Delivery, *domain.FollowUpContext, error)
	AppendFollowUp(ctx context.Context, candidateID, id uuid.UUID, followUp domain.AskedQuestion, answer string, followUpLimit int32) (domain.Session, error)
}

// Evaluator scores a completed session. It writes the session's evaluation status itself; the service
// only starts it, once per session, after the first completion.
type Evaluator interface {
	Evaluate(ctx context.Context, interviewID uuid.UUID) error
}

// EndNotifier hears about every session that ended, whoever ended it, so a live socket of that session
// can report the end and close instead of waiting for its next command. The live engine's hub provides
// it. It must not block.
type EndNotifier interface {
	SessionEnded(interviewID uuid.UUID)
}

// Settings are the configured interview rules.
// QuestionLimit is the most core questions one session asks; zero uses the default. DefaultProviderVoiceID
// is the provider voice a live session speaks with when it has no voice of its own; empty means such a
// session is shown as text only. FollowUpLimit is the most follow-ups the live interviewer asks after one
// core question; zero turns follow-ups off. FollowUpTimeout bounds one follow-up decision; zero uses the
// default. AbandonTimeout is how long an unfinished session may go without activity before the System
// Handler ends it (KAN-180); zero leaves idle sessions alone.
type Settings struct {
	QuestionLimit          int32
	DefaultProviderVoiceID string
	FollowUpLimit          int32
	FollowUpTimeout        time.Duration
	AbandonTimeout         time.Duration
}

type Service struct {
	repository Repository
	settings   Settings
	evaluator  Evaluator
	decider    FollowUpDecider
	notifier   EndNotifier
}

func New(repository Repository, settings Settings) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("interview repository is required")
	}
	if settings.QuestionLimit <= 0 {
		settings.QuestionLimit = defaultQuestionLimit
	}
	if settings.FollowUpLimit < 0 {
		return nil, fmt.Errorf("interview follow-up limit must not be negative")
	}
	if settings.FollowUpTimeout <= 0 {
		settings.FollowUpTimeout = defaultFollowUpTimeout
	}
	if settings.AbandonTimeout < 0 {
		return nil, fmt.Errorf("interview abandon timeout must not be negative")
	}
	return &Service{repository: repository, settings: settings}, nil
}

// SetEvaluator sets the optional evaluation hook. Nil means no evaluation is started. Set it before the
// service serves requests.
func (s *Service) SetEvaluator(evaluator Evaluator) {
	s.evaluator = evaluator
}

// SetEndNotifier sets the optional hook told about every ended session. Set it before the service serves
// requests.
func (s *Service) SetEndNotifier(notifier EndNotifier) {
	s.notifier = notifier
}

// Create starts a session, or returns the existing recruitment session for the Application. The bool is
// true only when a session was created, so the handler can answer 201 or 200.
func (s *Service) Create(ctx context.Context, candidateID uuid.UUID, role string, input domain.CreateInput) (domain.Session, bool, error) {
	if role != candidateRole {
		return domain.Session{}, false, apperror.New(apperror.CodeForbidden, "only candidates can start interviews")
	}
	if err := validateCreate(input); err != nil {
		return domain.Session{}, false, err
	}
	params := domain.CreateParams{
		CandidateID: candidateID, Type: input.Type, RoleProfileID: input.RoleProfileID,
		ApplicationID: input.ApplicationID, AvatarID: input.AvatarID, VoiceID: input.VoiceID,
		QuestionLimit: s.settings.QuestionLimit,
	}
	if input.Type == domain.TypePractice {
		params.PracticeCoins = domain.PracticeInterviewCoins
		if input.IdempotencyKey != "" {
			key := candidateID.String() + ":" + input.IdempotencyKey
			params.StartKey = &key
		}
	}
	session, created, err := s.repository.Create(ctx, params)
	if err != nil {
		return domain.Session{}, false, err
	}
	if created {
		logger.FromContext(ctx).Info("interview started",
			logger.Op("interview.start"),
			logger.String("interview_id", session.ID.String()),
			logger.String("user_id", candidateID.String()),
			logger.String("type", string(session.Type)),
			logger.Int("question_count", int(session.QuestionCount)),
		)
	}
	return presentSession(session), created, nil
}

// Get reads one of the Candidate's sessions.
func (s *Service) Get(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error) {
	if role != candidateRole {
		return domain.Session{}, apperror.New(apperror.CodeForbidden, "only candidates can read interviews")
	}
	session, err := s.repository.Get(ctx, candidateID, id)
	if err != nil {
		return domain.Session{}, err
	}
	return presentSession(session), nil
}

// Resume returns the same session. It has no side effects and never charges; a completed session is
// refused.
func (s *Service) Resume(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error) {
	if role != candidateRole {
		return domain.Session{}, apperror.New(apperror.CodeForbidden, "only candidates can resume interviews")
	}
	session, err := s.repository.Get(ctx, candidateID, id)
	if err != nil {
		return domain.Session{}, err
	}
	if session.CompletedAt != nil {
		return domain.Session{}, domain.ErrInterviewCompleted
	}
	return presentSession(session), nil
}

// AppendTurn stores the Candidate's answer to the next question. The answer is trimmed and bounded here;
// the position and the retry rule are checked by the repository under the session lock.
func (s *Service) AppendTurn(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID, position int32, answer string) (domain.Session, error) {
	if role != candidateRole {
		return domain.Session{}, apperror.New(apperror.CodeForbidden, "only candidates can answer interviews")
	}
	if position < 1 {
		return domain.Session{}, apperror.New(apperror.CodeValidation, "position must be at least 1")
	}
	normalized, err := normalizeAnswer(answer)
	if err != nil {
		return domain.Session{}, err
	}
	session, err := s.repository.AppendTurn(ctx, candidateID, id, position, normalized)
	if err != nil {
		return domain.Session{}, err
	}
	logger.FromContext(ctx).Info("interview answer recorded",
		logger.Op("interview.answer"),
		logger.String("interview_id", id.String()),
		logger.String("user_id", candidateID.String()),
		logger.Int("question_index", int(position)),
		logger.Int("answered_count", int(session.AnsweredCount)),
		logger.Int("question_count", int(session.QuestionCount)),
		logger.Int("bytes", len(normalized)),
	)
	return presentSession(session), nil
}

// Deliver prepares the next core question: the question the interviewer asks now, frozen with the
// session's provider voice so a reconnect repeats the same words. It never charges and never consumes a
// slot. A completed session is INTERVIEW_COMPLETED and a closed recruitment window
// APPLICATION_NOT_ELIGIBLE. It never asks a follow-up, so a reconnect during a follow-up goes on with
// the next core question (team decision 2026-11-03).
func (s *Service) Deliver(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Delivery, error) {
	if role != candidateRole {
		return domain.Delivery{}, apperror.New(apperror.CodeForbidden, "only candidates can take interviews")
	}
	return s.repository.Deliver(ctx, candidateID, id, s.settings.DefaultProviderVoiceID)
}

// Complete ends the session for its Candidate: POST /interviews/{id}/complete and the live socket after
// the last answer both call it. Ending before every core question is answered is leaving: the session
// ends with reason left and is evaluated on what was answered (decision 2026-11-03: no pause, leaving
// ends the interview). It is idempotent; see end.
func (s *Service) Complete(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error) {
	if role != candidateRole {
		return domain.Session{}, apperror.New(apperror.CodeForbidden, "only candidates can complete interviews")
	}
	result, err := s.end(ctx, domain.EndParams{CandidateID: candidateID, ID: id, Cause: domain.EndByCandidate})
	if err != nil {
		return domain.Session{}, err
	}
	return presentSession(result.Session), nil
}

// abandonBatch bounds the sessions one abandonment pass ends; the next pass takes the rest.
const abandonBatch = 100

// AbandonIdle is the System Handler's abandoned-session job (KAN-180): it ends, through the same end path
// as a completion, every unfinished session with no activity for AbandonTimeout before now. A session
// ended this way is evaluated on what was answered, is never charged again or refunded, and cannot be
// resumed. A session that became active meanwhile is left alone. It returns how many sessions it ended;
// a failure on one session is logged and does not stop the others. Running it again is harmless.
func (s *Service) AbandonIdle(ctx context.Context, now time.Time) (int, error) {
	if s.settings.AbandonTimeout <= 0 {
		return 0, nil
	}
	idleBefore := now.Add(-s.settings.AbandonTimeout)
	refs, err := s.repository.ListAbandoned(ctx, now, idleBefore, abandonBatch)
	if err != nil {
		return 0, err
	}
	ended := 0
	var failures []error
	for _, ref := range refs {
		result, err := s.end(ctx, domain.EndParams{
			CandidateID: ref.CandidateID, ID: ref.ID, Cause: domain.EndBySystem, IdleBefore: &idleBefore,
		})
		if err != nil {
			if ctx.Err() != nil {
				return ended, ctx.Err()
			}
			logger.FromContext(ctx).Warn("abandoned interview could not be ended",
				logger.Op("interview.abandon"),
				logger.String("interview_id", ref.ID.String()),
				logger.Cause(err),
			)
			failures = append(failures, err)
			continue
		}
		if result.Ended {
			ended++
		}
	}
	if len(failures) > 0 {
		return ended, fmt.Errorf("%d of %d abandoned interviews could not be ended: %w", len(failures), len(refs), errors.Join(failures...))
	}
	return ended, nil
}

// end runs the one end path. The ending is logged and the live socket told once, by the call that
// ended the session; the evaluation starts once, by the call that claimed it.
func (s *Service) end(ctx context.Context, params domain.EndParams) (domain.EndResult, error) {
	result, err := s.repository.End(ctx, params)
	if err != nil {
		return domain.EndResult{}, err
	}
	session := result.Session
	if result.Ended {
		var duration time.Duration
		if session.CompletedAt != nil {
			duration = session.CompletedAt.Sub(session.StartedAt)
		}
		reason := ""
		if session.EndReason != nil {
			reason = string(*session.EndReason)
		}
		logger.FromContext(ctx).Info("interview ended",
			logger.Op("interview.end"),
			logger.String("interview_id", session.ID.String()),
			logger.String("user_id", params.CandidateID.String()),
			logger.String("type", string(session.Type)),
			logger.String("end_reason", reason),
			logger.Int("answered_count", int(session.AnsweredCount)),
			logger.Int("question_count", int(session.QuestionCount)),
			logger.Duration("duration", duration),
		)
		if s.notifier != nil {
			s.notifier.SessionEnded(session.ID)
		}
	}
	// The repository claims the evaluation for one caller: the first ending, a failed evaluation
	// retried by a repeat, or a pending one whose run died. A live or finished one is never run twice.
	if result.StartEvaluation {
		s.startEvaluation(ctx, session.ID)
	}
	return result, nil
}

// startEvaluation runs the evaluator in the background. The request context is detached, so the
// evaluation is not cancelled when the response is sent, and bounded by its own timeout.
func (s *Service) startEvaluation(ctx context.Context, interviewID uuid.UUID) {
	evaluator := s.evaluator
	if evaluator == nil {
		logger.FromContext(ctx).Warn("interview evaluation not started",
			logger.Op("interview.evaluate"),
			logger.String("interview_id", interviewID.String()),
			logger.String("reason", "no evaluator configured"),
		)
		return
	}
	go func() {
		// A panic anywhere in the evaluation must not take the API down. It is marked failed, so the next
		// Complete claims and retries it instead of waiting for the claim to go stale.
		defer s.recoverEvaluation(ctx, interviewID)
		evalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), evaluationTimeout)
		defer cancel()
		if err := evaluator.Evaluate(evalCtx, interviewID); err != nil {
			logger.FromContext(evalCtx).Error("interview evaluation failed",
				logger.Op("interview.evaluate"),
				logger.String("interview_id", interviewID.String()),
				logger.Cause(err),
			)
		}
	}()
}

// recoverEvaluation is deferred by the evaluation goroutine. It logs a panic with its stack and marks the
// evaluation failed; a failed write is logged too, and the claim then goes stale as before.
func (s *Service) recoverEvaluation(ctx context.Context, interviewID uuid.UUID) {
	recovered := recover()
	if recovered == nil {
		return
	}
	// The panic value is not logged: a scorer panic can carry the answer text it was given.
	log := logger.FromContext(ctx)
	log.Error("interview evaluation panicked",
		logger.Op("interview.evaluate"),
		logger.String("interview_id", interviewID.String()),
		logger.String("panic_type", fmt.Sprintf("%T", recovered)),
		logger.String("stack", string(debug.Stack())),
	)
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureWriteTimeout)
	defer cancel()
	if err := s.repository.MarkEvaluationFailed(markCtx, interviewID); err != nil {
		log.Error("mark panicked interview evaluation failed",
			logger.Op("interview.evaluate"),
			logger.String("interview_id", interviewID.String()),
			logger.Cause(err),
		)
	}
}

// presentSession is the Candidate's view of a session. Recruitment sessions never show turns; the
// current question is the only question they see.
func presentSession(session domain.Session) domain.Session {
	if session.Type != domain.TypePractice {
		session.Turns = nil
	}
	return session
}

// validateCreate checks the fields each session type needs before the repository reads anything.
func validateCreate(input domain.CreateInput) error {
	switch input.Type {
	case domain.TypePractice:
		if input.RoleProfileID == nil {
			return apperror.New(apperror.CodeValidation, "role_profile_id is required for a practice interview")
		}
		if input.ApplicationID != nil {
			return apperror.New(apperror.CodeValidation, "application_id is only for a recruitment interview")
		}
	case domain.TypeRecruitment:
		if input.ApplicationID == nil {
			return apperror.New(apperror.CodeValidation, "application_id is required for a recruitment interview")
		}
		// The posting fixes the interviewer and the voice; the Candidate cannot choose their own.
		if input.AvatarID != nil || input.VoiceID != nil {
			return apperror.New(apperror.CodeValidation, "avatar_id and voice_id are fixed by the job posting for a recruitment interview")
		}
	default:
		return apperror.New(apperror.CodeValidation, "type must be practice or recruitment")
	}
	return nil
}
