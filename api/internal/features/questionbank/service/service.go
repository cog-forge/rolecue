package service

import (
	"context"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const recruiterRole = "recruiter"

// Repository is the persistence contract consumed by question-bank operations.
type Repository interface {
	WriteQuestion(ctx context.Context, recruiterID, postingID uuid.UUID, questionID *uuid.UUID, patch domain.QuestionPatch) (domain.BankQuestion, error)
	ApplyRubric(ctx context.Context, recruiterID, postingID uuid.UUID, request domain.BulkRubricRequest) ([]domain.BankQuestion, error)
	Posting(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.Posting, error)
	CandidateOwnsJD(ctx context.Context, candidateID, jdID uuid.UUID) error
	List(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error)
	ListBank(ctx context.Context, jdID, recruiterID uuid.UUID) ([]domain.BankQuestion, error)
	// RubricCatalog is the allowed set for generation: the active system rubrics and the Recruiter's own
	// active rubrics, bounded and in a stable order.
	RubricCatalog(ctx context.Context, recruiterID uuid.UUID) ([]domain.RubricOption, error)
	GenerationInput(ctx context.Context, jdID uuid.UUID) (domain.GenerationInput, error)
	Add(ctx context.Context, recruiterID, postingID uuid.UUID, content string) (domain.BankQuestion, error)
	Update(ctx context.Context, recruiterID, postingID, questionID uuid.UUID, content string) (domain.BankQuestion, error)
	Delete(ctx context.Context, recruiterID, postingID, questionID uuid.UUID) error
	SaveForPosting(ctx context.Context, recruiterID, postingID uuid.UUID, questions []domain.GeneratedQuestion) ([]domain.BankQuestion, error)
	SaveForPractice(ctx context.Context, candidateID, jdID uuid.UUID, questions []domain.GeneratedQuestion) ([]domain.CoreQuestion, error)
}

// Generator performs one attempt and must not retry internally. The input is untrusted data.
type Generator interface {
	Generate(ctx context.Context, in domain.GenerationInput, count int) (domain.GenerationCandidate, error)
}

type Service struct {
	repository Repository
	generator  Generator
	maxRetries int
	size       int
}

// New builds the service. size is the number of questions to generate; 0 disables generation
// because the product decision on the bank size is still open.
func New(repository Repository, generator Generator, maxRetries, size int) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("question bank repository is required")
	}
	if generator == nil {
		return nil, fmt.Errorf("question generator is required")
	}
	if maxRetries < 0 || maxRetries > 1 {
		return nil, fmt.Errorf("question generation max retries must be 0 or 1")
	}
	if size < 0 {
		return nil, fmt.Errorf("question bank size must not be negative")
	}
	return &Service{repository: repository, generator: generator, maxRetries: maxRetries, size: size}, nil
}

func requireRecruiter(role string) error {
	if role != recruiterRole {
		return apperror.New(apperror.CodeForbidden, "only recruiters can manage a job posting's question bank")
	}
	return nil
}

// List returns the bank of a posting owned by the Recruiter, with whether it can still change.
func (s *Service) List(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.Bank, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.Bank{}, err
	}
	posting, err := s.repository.Posting(ctx, recruiterID, postingID)
	if err != nil {
		return domain.Bank{}, err
	}
	questions, err := s.repository.ListBank(ctx, posting.JDID, recruiterID)
	if err != nil {
		return domain.Bank{}, err
	}
	if questions == nil {
		questions = []domain.BankQuestion{}
	}
	incomplete := 0
	for _, q := range questions {
		if q.GradingIncomplete {
			incomplete++
		}
	}
	return domain.Bank{
		MaxExpectedAnswerLength: domain.MaxExpectedAnswerRunes, MaxExpectedAnswerWords: domain.MaxExpectedAnswerWords, ExpectedAnswerGuidanceWords: domain.ExpectedAnswerPromptWords, MaxBulkQuestions: domain.MaxBulkQuestions,
		Questions:     questions,
		PostingStatus: posting.Status,
		// An empty bank is generated, not filled by hand.
		Editable:          posting.Editable() && len(questions) > 0,
		MaxQuestionLength: domain.MaxQuestionRunes,
		GenerationSize:    s.size,
		ReviewSubmitted:   posting.ReviewSubmitted,
		// An empty bank is never ready: there is nothing to interview from.
		GradingReady:           len(questions) > 0 && incomplete == 0,
		GradingIncompleteCount: incomplete,
	}, nil
}

// Generate writes the bank of an owned posting being prepared (pending or rejected) once.
func (s *Service) Generate(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) ([]domain.BankQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return nil, err
	}
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	posting, err := s.repository.Posting(ctx, recruiterID, postingID)
	if err != nil {
		return nil, err
	}
	if !posting.Editable() {
		return nil, apperror.New(apperror.CodeQuestionBankLocked, "the question bank can only be changed while the job posting is pending or rejected")
	}
	questions, err := s.draft(ctx, posting.JDID, &recruiterID)
	if err != nil {
		return nil, err
	}
	return s.repository.SaveForPosting(ctx, recruiterID, postingID, questions)
}

// GenerateForPractice writes the bank of a Candidate's own practice JD. There is deliberately no
// route for it: Candidates never see the bank, and the confirm-requirements flow calls this.
func (s *Service) GenerateForPractice(ctx context.Context, candidateID, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if err := s.repository.CandidateOwnsJD(ctx, candidateID, jdID); err != nil {
		return nil, err
	}
	questions, err := s.draft(ctx, jdID, nil)
	if err != nil {
		return nil, err
	}
	return s.repository.SaveForPractice(ctx, candidateID, jdID, questions)
}

// CurrentBank is the stable read contract for the interview runtime (KAN-56). It is not scoped by
// owner: callers are trusted server code that already authorized the session.
func (s *Service) CurrentBank(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	return s.repository.List(ctx, jdID)
}

// WriteQuestion creates or edits an owned bank question. Legacy content-only requests remain valid.
func (s *Service) WriteQuestion(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, questionID *uuid.UUID, patch domain.QuestionPatch) (domain.BankQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.BankQuestion{}, err
	}
	patch, err := normalizeQuestionPatch(patch)
	if err != nil {
		return domain.BankQuestion{}, err
	}
	if questionID == nil && patch.Content == nil {
		return domain.BankQuestion{}, invalid("content is required")
	}
	return s.repository.WriteQuestion(ctx, recruiterID, postingID, questionID, patch)
}

func (s *Service) ApplyRubric(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, request domain.BulkRubricRequest) ([]domain.BankQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return nil, err
	}
	if len(request.QuestionIDs) == 0 || len(request.QuestionIDs) > domain.MaxBulkQuestions || request.RubricID == uuid.Nil {
		return nil, invalid("select 1 to 100 questions and a rubric")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range request.QuestionIDs {
		if id == uuid.Nil || seen[id] || len(request.PreviousRevisions[id]) != 64 {
			return nil, invalid("unique questions and their previous revisions are required")
		}
		seen[id] = true
	}
	return s.repository.ApplyRubric(ctx, recruiterID, postingID, request)
}

func (s *Service) Add(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, content string) (domain.BankQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.BankQuestion{}, err
	}
	content, err := normalizeContent(content)
	if err != nil {
		return domain.BankQuestion{}, err
	}
	return s.repository.Add(ctx, recruiterID, postingID, content)
}

func (s *Service) Update(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID, content string) (domain.BankQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.BankQuestion{}, err
	}
	content, err := normalizeContent(content)
	if err != nil {
		return domain.BankQuestion{}, err
	}
	return s.repository.Update(ctx, recruiterID, postingID, questionID, content)
}

func (s *Service) Delete(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID) error {
	if err := requireRecruiter(role); err != nil {
		return err
	}
	return s.repository.Delete(ctx, recruiterID, postingID, questionID)
}

func (s *Service) requireEnabled() error {
	if s.size == 0 {
		return apperror.New(apperror.CodeQuestionBankUnavailable, "question bank generation is not configured")
	}
	return nil
}

// draft checks the cheap preconditions before spending an LLM call, then asks the model. Every draft carries
// the rubric catalog and asks for a rubric and an expected answer per question in the same single call, so
// every session can be graded (KAN-334). A Recruiter's draft may use the active system templates and the
// Recruiter's own rubrics; a Candidate's practice draft (recruiterID nil) only the active system templates,
// because a Candidate has no rubrics of their own. The rubric and the expected answer stay server-side.
func (s *Service) draft(ctx context.Context, jdID uuid.UUID, recruiterID *uuid.UUID) ([]domain.GeneratedQuestion, error) {
	existing, err := s.repository.List(ctx, jdID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, apperror.New(apperror.CodeQuestionBankExists, "this job description already has a question bank")
	}
	input, err := s.repository.GenerationInput(ctx, jdID)
	if err != nil {
		return nil, err
	}
	if len(input.Requirements) == 0 {
		return nil, apperror.New(apperror.CodeRequirementsNotConfirmed, "confirm the job description requirements before generating questions")
	}
	input.Grading = true
	if input.Rubrics, err = s.repository.RubricCatalog(ctx, rubricOwner(recruiterID)); err != nil {
		return nil, err
	}
	done := logger.Start(ctx, "questionbank.generate", logger.String("jd_id", jdID.String()),
		logger.Int("requested", s.size), logger.Int("requirements", len(input.Requirements)), logger.Int("rubrics", len(input.Rubrics)))
	questions, err := s.generateWithRetry(ctx, input)
	done(err, logger.Int("count", len(questions)))
	return questions, err
}

// systemRubricsOnly is the owner passed to the rubric queries for a practice bank. No rubric is owned by the
// nil id, so the "system or owned" rule of those queries leaves the active system templates only.
var systemRubricsOnly = uuid.Nil

// rubricOwner is the owner whose private rubrics a draft may use: the Recruiter, or nobody for practice.
func rubricOwner(recruiterID *uuid.UUID) uuid.UUID {
	if recruiterID == nil {
		return systemRubricsOnly
	}
	return *recruiterID
}
