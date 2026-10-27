package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	candidateRole = "candidate"
	// defaultQuestionLimit is used when the configured question limit is zero.
	defaultQuestionLimit = 8
	// evaluationTimeout bounds one background evaluation, which runs after the request has returned.
	evaluationTimeout = 2 * time.Minute
)

// Repository is the persistence contract of interview sessions. It runs each write in one transaction and
// takes the locks the rules need; the service decides what may be asked and what the Candidate sees.
type Repository interface {
	Create(ctx context.Context, params domain.CreateParams) (domain.Session, bool, error)
	Get(ctx context.Context, candidateID, id uuid.UUID) (domain.Session, error)
	AppendTurn(ctx context.Context, candidateID, id uuid.UUID, position int32, answer string) (domain.Session, error)
	Complete(ctx context.Context, candidateID, id uuid.UUID) (domain.Session, bool, error)
}

// Evaluator scores a completed session. It writes the session's evaluation status itself; the service
// only starts it, once per session, after the first completion.
type Evaluator interface {
	Evaluate(ctx context.Context, interviewID uuid.UUID) error
}

// Settings are the configured interview rules.
// QuestionLimit is the most questions one session asks; zero uses the default.
type Settings struct {
	QuestionLimit int32
}

type Service struct {
	repository Repository
	settings   Settings
	evaluator  Evaluator
}

func New(repository Repository, settings Settings) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("interview repository is required")
	}
	if settings.QuestionLimit <= 0 {
		settings.QuestionLimit = defaultQuestionLimit
	}
	return &Service{repository: repository, settings: settings}, nil
}

// SetEvaluator sets the optional evaluation hook. Nil means no evaluation is started. Set it before the
// service serves requests.
func (s *Service) SetEvaluator(evaluator Evaluator) {
	s.evaluator = evaluator
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
	return presentSession(session), nil
}

// Complete marks the session completed. It is idempotent. The evaluation starts once, after the first
// completion, in a goroutine that outlives the request.
func (s *Service) Complete(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error) {
	if role != candidateRole {
		return domain.Session{}, apperror.New(apperror.CodeForbidden, "only candidates can complete interviews")
	}
	session, startEvaluation, err := s.repository.Complete(ctx, candidateID, id)
	if err != nil {
		return domain.Session{}, err
	}
	// The repository claims the evaluation for one caller: the first completion, a failed evaluation
	// retried by a repeat, or a pending one whose run died. A live or finished one is never run twice.
	if startEvaluation {
		s.startEvaluation(ctx, session.ID)
	}
	return presentSession(session), nil
}

// startEvaluation runs the evaluator in the background. The request context is detached, so the
// evaluation is not cancelled when the response is sent, and bounded by its own timeout.
func (s *Service) startEvaluation(ctx context.Context, interviewID uuid.UUID) {
	evaluator := s.evaluator
	if evaluator == nil {
		return
	}
	go func() {
		defer func() {
			// A panic in the scorer or its client must not take the API down. The claim goes stale and a
			// repeated Complete takes the evaluation over.
			if recovered := recover(); recovered != nil {
				slog.Error("interview evaluation panicked", "interview_id", interviewID, "panic", recovered)
			}
		}()
		evalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), evaluationTimeout)
		defer cancel()
		if err := evaluator.Evaluate(evalCtx, interviewID); err != nil {
			slog.ErrorContext(evalCtx, "interview evaluation failed", "interview_id", interviewID, "error", err)
		}
	}()
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
