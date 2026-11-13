package service

import (
	"context"
	"fmt"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
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
	// Coverage is the shared metric-coverage verdict of a posting's bank under its weights (KAN-377).
	Coverage(ctx context.Context, postingID uuid.UUID) (evaluationdomain.CoverageReport, error)
	// ActiveMetrics are the metrics a generated bank must cover: the posting's, or the catalog (nil posting).
	ActiveMetrics(ctx context.Context, postingID *uuid.UUID) ([]uuid.UUID, error)
}

// Generator performs one attempt and must not retry internally. The input is untrusted data.
type Generator interface {
	Generate(ctx context.Context, in domain.GenerationInput, count int) (domain.GenerationCandidate, error)
}

type Service struct {
	repository    Repository
	generator     Generator
	maxRetries    int
	size          int
	questionLimit int
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
	return &Service{repository: repository, generator: generator, maxRetries: maxRetries, size: size,
		questionLimit: evaluationdomain.DefaultQuestionLimit}, nil
}

// SetQuestionLimit sets how many core questions one interview session asks, the limit a generated bank is
// checked against (KAN-377, KAN-378). It must be the value the interview start uses; a value below 1 keeps
// the default.
func (s *Service) SetQuestionLimit(limit int) {
	s.questionLimit = evaluationdomain.ResolveQuestionLimit(limit)
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
	coverage, err := s.repository.Coverage(ctx, postingID)
	if err != nil {
		return domain.Bank{}, err
	}
	return domain.Bank{
		Coverage:                &coverage,
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
//
// The model is asked again (within the retry budget) when its bank cannot cover every active metric; if the
// attempts run out the best bank is still saved, and the coverage in the result is the warning (KAN-377).
func (s *Service) Generate(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.BankMutation, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.BankMutation{}, err
	}
	if err := s.requireEnabled(); err != nil {
		return domain.BankMutation{}, err
	}
	posting, err := s.repository.Posting(ctx, recruiterID, postingID)
	if err != nil {
		return domain.BankMutation{}, err
	}
	if !posting.Editable() {
		return domain.BankMutation{}, apperror.New(apperror.CodeQuestionBankLocked, "the question bank can only be changed while the job posting is pending or rejected")
	}
	questions, err := s.draft(ctx, posting.JDID, &recruiterID, &postingID, nil, true)
	if err != nil {
		return domain.BankMutation{}, err
	}
	saved, err := s.repository.SaveForPosting(ctx, recruiterID, postingID, questions)
	if err != nil {
		return domain.BankMutation{}, err
	}
	return domain.BankMutation{Questions: saved, Coverage: s.coverageAfterWrite(ctx, postingID)}, nil
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
	questions, err := s.draft(ctx, jdID, nil, nil, nil, true)
	if err != nil {
		return nil, err
	}
	return s.repository.SaveForPractice(ctx, candidateID, jdID, questions)
}

// DraftPracticeBank generates, without storing, the practice bank of a Candidate's own profile mapped to the
// given Roadmap topics (KAN-344): every question names one topic and every topic has at least
// MinQuestionsPerTopic distinct questions, or the draft is invalid output. The caller stores the result in the
// same transaction that publishes the topics; an existing bank is that caller's to replace, so it is not an
// error here.
func (s *Service) DraftPracticeBank(ctx context.Context, candidateID, jdID uuid.UUID, topics []domain.RoadmapTopic) ([]domain.GeneratedQuestion, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if len(topics) == 0 {
		return nil, invalid("a roadmap bank needs at least one topic")
	}
	if err := s.repository.CandidateOwnsJD(ctx, candidateID, jdID); err != nil {
		return nil, err
	}
	return s.draft(ctx, jdID, nil, nil, topics, false)
}

// DraftTopicQuestions generates, without storing, count more practice questions for one Roadmap topic of a
// Candidate's own profile (KAN-347, the top-up before a focused start). The questions already in the bank are
// given to the model as data and dropped when repeated, so the new ones are different from every stored one.
// count is raised to MinQuestionsPerTopic, the smallest batch the topic rule accepts. It is one call with the
// usual retry budget; the caller stores the result and decides what a failure means.
func (s *Service) DraftTopicQuestions(ctx context.Context, candidateID, jdID uuid.UUID, topic domain.RoadmapTopic, count int) ([]domain.GeneratedQuestion, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if count < 1 {
		return nil, invalid("a top-up needs at least one question")
	}
	if err := s.repository.CandidateOwnsJD(ctx, candidateID, jdID); err != nil {
		return nil, err
	}
	existing, err := s.repository.List(ctx, jdID)
	if err != nil {
		return nil, err
	}
	contents := make([]string, 0, len(existing))
	for _, question := range existing {
		contents = append(contents, question.Content)
	}
	return s.draft(ctx, jdID, nil, nil, []domain.RoadmapTopic{topic}, false, func(in *domain.GenerationInput) {
		in.Size = max(count, domain.MinQuestionsPerTopic)
		in.Existing = contents
	})
}

// CurrentBank is the stable read contract for the interview runtime (KAN-56). It is not scoped by
// owner: callers are trusted server code that already authorized the session.
func (s *Service) CurrentBank(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	return s.repository.List(ctx, jdID)
}

// WriteQuestion creates or edits an owned bank question. Legacy content-only requests remain valid.
func (s *Service) WriteQuestion(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, questionID *uuid.UUID, patch domain.QuestionPatch) (domain.QuestionMutation, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.QuestionMutation{}, err
	}
	patch, err := normalizeQuestionPatch(patch)
	if err != nil {
		return domain.QuestionMutation{}, err
	}
	if questionID == nil && patch.Content == nil {
		return domain.QuestionMutation{}, invalid("content is required")
	}
	question, err := s.repository.WriteQuestion(ctx, recruiterID, postingID, questionID, patch)
	if err != nil {
		return domain.QuestionMutation{}, err
	}
	return domain.QuestionMutation{BankQuestion: question, Coverage: s.coverageAfterWrite(ctx, postingID)}, nil
}

func (s *Service) ApplyRubric(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, request domain.BulkRubricRequest) (domain.BankMutation, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.BankMutation{}, err
	}
	if len(request.QuestionIDs) == 0 || len(request.QuestionIDs) > domain.MaxBulkQuestions || request.RubricID == uuid.Nil {
		return domain.BankMutation{}, invalid("select 1 to 100 questions and a rubric")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range request.QuestionIDs {
		if id == uuid.Nil || seen[id] || len(request.PreviousRevisions[id]) != 64 {
			return domain.BankMutation{}, invalid("unique questions and their previous revisions are required")
		}
		seen[id] = true
	}
	questions, err := s.repository.ApplyRubric(ctx, recruiterID, postingID, request)
	if err != nil {
		return domain.BankMutation{}, err
	}
	return domain.BankMutation{Questions: questions, Coverage: s.coverageAfterWrite(ctx, postingID)}, nil
}

// coverageAfterWrite is the coverage of the bank right after a write that already committed. A failed read
// leaves it null rather than failing a write that succeeded: the bank view reads the coverage on its own.
func (s *Service) coverageAfterWrite(ctx context.Context, postingID uuid.UUID) *evaluationdomain.CoverageReport {
	report, err := s.repository.Coverage(ctx, postingID)
	if err != nil {
		logger.FromContext(ctx).Warn("bank coverage unavailable after a write", logger.Op("questionbank.coverage"),
			logger.String("posting_id", postingID.String()), logger.Cause(err))
		return nil
	}
	return &report
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
func (s *Service) draft(ctx context.Context, jdID uuid.UUID, recruiterID, postingID *uuid.UUID, topics []domain.RoadmapTopic, requireEmpty bool, options ...func(*domain.GenerationInput)) ([]domain.GeneratedQuestion, error) {
	if requireEmpty {
		existing, err := s.repository.List(ctx, jdID)
		if err != nil {
			return nil, err
		}
		if len(existing) > 0 {
			return nil, apperror.New(apperror.CodeQuestionBankExists, "this job description already has a question bank")
		}
	}
	input, err := s.repository.GenerationInput(ctx, jdID)
	if err != nil {
		return nil, err
	}
	if len(input.Requirements) == 0 {
		return nil, apperror.New(apperror.CodeRequirementsNotConfirmed, "confirm the job description requirements before generating questions")
	}
	input.Grading = true
	input.Topics = topics
	for _, option := range options {
		option(&input)
	}
	if input.Rubrics, err = s.repository.RubricCatalog(ctx, rubricOwner(recruiterID)); err != nil {
		return nil, err
	}
	// The metrics the bank must be able to cover, read before the model call like the catalog is: a posting's
	// active metrics, or the whole catalog for a practice bank (nil posting).
	if input.ActiveMetrics, err = s.repository.ActiveMetrics(ctx, postingID); err != nil {
		return nil, err
	}
	done := logger.Start(ctx, "questionbank.generate", logger.String("jd_id", jdID.String()),
		logger.Int("requested", s.size), logger.Int("topics", len(topics)), logger.Int("requirements", len(input.Requirements)), logger.Int("rubrics", len(input.Rubrics)))
	questions, err := s.generateWithRetry(ctx, input, recruiterID != nil)
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
