package service

import (
	"context"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
)

// SessionInfo is what the assessment writer needs to know about one interview: who owns it, the facts that
// decide whether its results may feed a Roadmap, and the facts of each assigned core question. It carries no
// text, answer, expected answer or rubric.
type SessionInfo struct {
	CandidateID uuid.UUID
	Session     domain.SessionFacts
	Cores       []domain.CoreFacts
}

// SessionSource reads SessionInfo from the interview and evaluation data. The provider adapts the evaluation
// repository to it, because features do not import each other.
type SessionSource interface {
	LoadSession(ctx context.Context, interviewID uuid.UUID) (SessionInfo, error)
}

// AssessmentStore writes the links of one interview in one statement, so they all happen or none does.
type AssessmentStore interface {
	// LinkCores links the listed assignment positions of the interview to the topic of their core question.
	// It writes nothing for a position unless the interview is a completed practice session of that
	// candidate, the core question has a topic, and the topic belongs to the Roadmap of the interview's Role
	// Profile. A position that is already linked is left as it is (the UNIQUE key and ON CONFLICT DO NOTHING),
	// so a retry or a concurrent worker never adds a second row.
	LinkCores(ctx context.Context, candidateID, interviewID uuid.UUID, positions []int32) (LinkOutcome, error)
}

// LinkOutcome is what one LinkCores call did (defined in domain so the repository can return it).
type LinkOutcome = domain.LinkOutcome

// LinkResult reports one run of the writer.
type LinkResult struct {
	// Eligible is false when the session may not feed a Roadmap (recruitment, unfinished, evaluation not
	// successful or not trusted). Nothing is written then.
	Eligible bool
	// Counted is the number of core questions that count as Roadmap evidence (scored or skipped).
	Counted int
	// Linked is the number of links written by this run; a repeated run reports 0.
	Linked int
	// Unlinked is the number of counted cores that have a topic but no link even after the write: the link rules
	// refused them (for example a profile mismatch). It is 0 when everything mappable is linked, by this run or
	// another one, and cores without a topic never count here.
	Unlinked int
}

// AssessmentService links the evaluated practice results of an interview to Roadmap topics (KAN-350, RM-09).
// Assessments hold no score, feedback, transcript or expected answer: they only say which topic receives which
// answer. The score is read from interview_questions on every Roadmap read, so there is nothing to copy.
type AssessmentService struct {
	source SessionSource
	store  AssessmentStore
}

func NewAssessmentService(source SessionSource, store AssessmentStore) *AssessmentService {
	return &AssessmentService{source: source, store: store}
}

// LinkPracticeResults links every countable core question of the interview in one atomic write. It decides
// eligibility with domain.SessionFacts.Eligible and each core with domain.ClassifyCore, the same two rules the
// read model and the catch-up job use, so a result counted anywhere is counted everywhere. A skipped core
// (shown and not answered) is linked too: its Roadmap score is 0, rebuilt from the stored facts on read.
//
// It is idempotent and safe to call from any number of workers. It never calls an AI model.
func (s *AssessmentService) LinkPracticeResults(ctx context.Context, interviewID uuid.UUID) (LinkResult, error) {
	info, err := s.source.LoadSession(ctx, interviewID)
	if err != nil {
		return LinkResult{}, fmt.Errorf("load session for the roadmap: %w", err)
	}
	if !info.Session.Eligible() {
		return LinkResult{}, nil
	}
	result := LinkResult{Eligible: true}
	positions := make([]int32, 0, len(info.Cores))
	for _, core := range info.Cores {
		if domain.ClassifyCore(core).Counted() {
			positions = append(positions, core.Position)
		}
	}
	result.Counted = len(positions)
	if len(positions) == 0 {
		return result, nil
	}
	outcome, err := s.store.LinkCores(ctx, info.CandidateID, interviewID, positions)
	if err != nil {
		return result, fmt.Errorf("link results to roadmap topics: %w", err)
	}
	result.Linked = int(outcome.Written)
	if result.Unlinked = int(outcome.Mappable) - int(outcome.Present); result.Unlinked < 0 {
		result.Unlinked = 0
	}
	return result, nil
}

// Evaluator is the interview service's evaluation hook.
type Evaluator interface {
	Evaluate(ctx context.Context, interviewID uuid.UUID) error
}

// Linker links the results of an evaluated interview.
type Linker interface {
	LinkPracticeResults(ctx context.Context, interviewID uuid.UUID) (LinkResult, error)
}

// EvaluateThenLink runs the evaluation and then links its results to the Roadmap. The Roadmap never decides
// whether the interview result stands: the evaluation is finished and saved before the link is tried, and any
// error or panic of the link is logged and dropped. The catch-up job (RM-10) links what this missed.
type EvaluateThenLink struct {
	evaluator Evaluator
	linker    Linker
}

func NewEvaluateThenLink(evaluator Evaluator, linker Linker) *EvaluateThenLink {
	return &EvaluateThenLink{evaluator: evaluator, linker: linker}
}

func (e *EvaluateThenLink) Evaluate(ctx context.Context, interviewID uuid.UUID) error {
	if err := e.evaluator.Evaluate(ctx, interviewID); err != nil {
		return err
	}
	e.link(ctx, interviewID)
	return nil
}

func (e *EvaluateThenLink) link(ctx context.Context, interviewID uuid.UUID) {
	defer func() {
		// The panic value is not logged: it can carry stored answer text.
		if recovered := recover(); recovered != nil {
			logger.FromContext(ctx).Error("roadmap link panicked; the catch-up job will retry",
				logger.Op("roadmap.link"), logger.String("interview_id", interviewID.String()),
				logger.String("panic_type", fmt.Sprintf("%T", recovered)))
		}
	}()
	if _, err := e.linker.LinkPracticeResults(ctx, interviewID); err != nil {
		logger.FromContext(ctx).Warn("roadmap link failed; the catch-up job will retry",
			logger.Op("roadmap.link"), logger.String("interview_id", interviewID.String()), logger.Cause(err))
	}
}
