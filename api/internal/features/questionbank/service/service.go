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
	SaveForPractice(ctx context.Context, candidateID, jdID uuid.UUID, contents []string) ([]domain.CoreQuestion, error)
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
	contents := make([]string, 0, len(questions))
	for _, q := range questions {
		contents = append(contents, q.Content)
	}
	return s.repository.SaveForPractice(ctx, candidateID, jdID, contents)
}

// CurrentBank is the stable read contract for the interview runtime (KAN-56). It is not scoped by
// owner: callers are trusted server code that already authorized the session.
func (s *Service) CurrentBank(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	return s.repository.List(ctx, jdID)
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

// draft checks the cheap preconditions before spending an LLM call, then asks the model. A Recruiter's
// draft (recruiterID set) also carries the rubric catalog and asks for grading data in the same single call;
// a Candidate's practice draft keeps the question-only contract.
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
	if recruiterID != nil {
		input.Grading = true
		if input.Rubrics, err = s.repository.RubricCatalog(ctx, *recruiterID); err != nil {
			return nil, err
		}
	}
	done := logger.Start(ctx, "questionbank.generate", logger.String("jd_id", jdID.String()),
		logger.Int("requested", s.size), logger.Int("requirements", len(input.Requirements)), logger.Int("rubrics", len(input.Rubrics)))
	questions, err := s.generateWithRetry(ctx, input)
	done(err, logger.Int("count", len(questions)))
	return questions, err
}
