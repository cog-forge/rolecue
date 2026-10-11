package service

import (
	"context"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const recruiterRole = "recruiter"

// Repository is the persistence contract consumed by question-bank operations.
type Repository interface {
	Posting(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.Posting, error)
	CandidateOwnsJD(ctx context.Context, candidateID, jdID uuid.UUID) error
	List(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error)
	GenerationInput(ctx context.Context, jdID uuid.UUID) (domain.GenerationInput, error)
	Add(ctx context.Context, recruiterID, postingID uuid.UUID, content string) (domain.CoreQuestion, error)
	Update(ctx context.Context, recruiterID, postingID, questionID uuid.UUID, content string) (domain.CoreQuestion, error)
	Delete(ctx context.Context, recruiterID, postingID, questionID uuid.UUID) error
	SaveForPosting(ctx context.Context, recruiterID, postingID uuid.UUID, contents []string) ([]domain.CoreQuestion, error)
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

// List returns the bank of a posting owned by the Recruiter.
func (s *Service) List(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) ([]domain.CoreQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return nil, err
	}
	posting, err := s.repository.Posting(ctx, recruiterID, postingID)
	if err != nil {
		return nil, err
	}
	return s.repository.List(ctx, posting.JDID)
}

// Generate writes the bank of an owned, pending posting once.
func (s *Service) Generate(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) ([]domain.CoreQuestion, error) {
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
	if posting.Status != domain.PostingPending {
		return nil, apperror.New(apperror.CodeQuestionBankLocked, "the question bank can only be changed while the job posting is pending")
	}
	contents, err := s.draft(ctx, posting.JDID)
	if err != nil {
		return nil, err
	}
	return s.repository.SaveForPosting(ctx, recruiterID, postingID, contents)
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
	contents, err := s.draft(ctx, jdID)
	if err != nil {
		return nil, err
	}
	return s.repository.SaveForPractice(ctx, candidateID, jdID, contents)
}

// CurrentBank is the stable read contract for the interview runtime (KAN-56). It is not scoped by
// owner: callers are trusted server code that already authorized the session.
func (s *Service) CurrentBank(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	return s.repository.List(ctx, jdID)
}

func (s *Service) Add(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, content string) (domain.CoreQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.CoreQuestion{}, err
	}
	content, err := normalizeContent(content)
	if err != nil {
		return domain.CoreQuestion{}, err
	}
	return s.repository.Add(ctx, recruiterID, postingID, content)
}

func (s *Service) Update(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID, content string) (domain.CoreQuestion, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.CoreQuestion{}, err
	}
	content, err := normalizeContent(content)
	if err != nil {
		return domain.CoreQuestion{}, err
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

// draft checks the cheap preconditions before spending an LLM call, then asks the model.
func (s *Service) draft(ctx context.Context, jdID uuid.UUID) ([]string, error) {
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
	return s.generateWithRetry(ctx, input)
}
