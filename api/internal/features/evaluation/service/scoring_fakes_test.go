package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// Fixed ids keep the scoring tests readable. The metrics and the rubric are the spec's worked example
// (Scoring-Rubrics-Spec section 2): R01 has C01 (2 points) and C02 (3 points) on Technical Accuracy and C03
// (5 points) on Problem-Solving.
var (
	scoringInterviewID = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	scoringCandidateID = uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	scoringRecruiterID = uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	scoringPostingID   = uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd")

	metricAccuracy = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-0000000000a1"), Name: "Technical Accuracy"}
	metricSolving  = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-0000000000a2"), Name: "Problem-Solving"}
	metricClarity  = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-0000000000a3"), Name: "Communication Clarity"}
	scoringCatalog = []domain.Metric{metricAccuracy, metricSolving, metricClarity}

	rubricR01 = uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeee1")
	critC01   = uuid.MustParse("00000000-0000-4000-8000-0000000000c1")
	critC02   = uuid.MustParse("00000000-0000-4000-8000-0000000000c2")
	critC03   = uuid.MustParse("00000000-0000-4000-8000-0000000000c3")
)

// fakeInterviewRepository keeps the rows the scoring tests read and records the writes.
type fakeInterviewRepository struct {
	record    *domain.InterviewRecord // nil means no such interview
	turns     []domain.TurnRecord
	questions []domain.GradingQuestion
	results   domain.StoredResults
	metrics   []domain.Metric
	weights   []domain.Weight

	saveErr error
	markErr error

	saved     *domain.Evaluation
	saveCalls int
	marked    bool
}

func (f *fakeInterviewRepository) InterviewRecord(context.Context, uuid.UUID) (domain.InterviewRecord, error) {
	if f.record == nil {
		return domain.InterviewRecord{}, domain.ErrInterviewNotFound
	}
	return *f.record, nil
}

func (f *fakeInterviewRepository) InterviewTurns(context.Context, uuid.UUID) ([]domain.TurnRecord, error) {
	return f.turns, nil
}

func (f *fakeInterviewRepository) GradingQuestions(context.Context, uuid.UUID) ([]domain.GradingQuestion, error) {
	return f.questions, nil
}

func (f *fakeInterviewRepository) InterviewResults(context.Context, uuid.UUID) (domain.StoredResults, error) {
	return f.results, nil
}

func (f *fakeInterviewRepository) SaveInterviewEvaluation(_ context.Context, _ uuid.UUID, evaluation domain.Evaluation) error {
	f.saveCalls++
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = &evaluation
	return nil
}

func (f *fakeInterviewRepository) MarkInterviewEvaluationFailed(context.Context, uuid.UUID) error {
	f.marked = true
	return f.markErr
}

func (f *fakeInterviewRepository) ListMetrics(context.Context) ([]domain.Metric, error) {
	return f.metrics, nil
}

func (f *fakeInterviewRepository) ListWeights(context.Context, uuid.UUID) ([]domain.Weight, error) {
	return f.weights, nil
}

// fakeGrader validates the reply its respond function builds for each question, like the routed adapter
// does, and records the calls and the largest number of calls in flight.
type fakeGrader struct {
	respond func(domain.QuestionGradeInput) (domain.QuestionGradeCandidate, error)
	delay   time.Duration

	mu       sync.Mutex
	inputs   []domain.QuestionGradeInput
	inFlight int
	peak     int
}

func (f *fakeGrader) GradeQuestion(ctx context.Context, in domain.QuestionGradeInput) (domain.QuestionGrade, error) {
	f.mu.Lock()
	f.inputs = append(f.inputs, in)
	f.inFlight++
	f.peak = max(f.peak, f.inFlight)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.mu.Unlock()
	}()
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return domain.QuestionGrade{}, ctx.Err()
		}
	}
	candidate, err := f.respond(in)
	if err != nil {
		return domain.QuestionGrade{}, err
	}
	return ValidateQuestionGrade(in, candidate)
}

func (f *fakeGrader) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.inputs)
}

// pointsGrader gives each question the listed points per criterion (by question position), quoting the
// first answer of the question for every criterion with points.
func pointsGrader(points map[int32][]int32) *fakeGrader {
	return &fakeGrader{respond: func(in domain.QuestionGradeInput) (domain.QuestionGradeCandidate, error) {
		out := domain.QuestionGradeCandidate{Feedback: "Feedback " + in.Turns[0].Answer[:3], Summary: "Summary."}
		for i, c := range in.Criteria {
			p := points[in.QuestionPosition][i]
			quote := ""
			if p > 0 {
				quote = in.Turns[0].Answer
			}
			out.Criteria = append(out.Criteria, domain.CriterionGradeCandidate{CriterionID: c.ID, Points: p, Evidence: quote, Feedback: "Why."})
		}
		return out, nil
	}}
}

func r01Criteria() []domain.GradingCriterion {
	return []domain.GradingCriterion{
		{ID: critC01, MetricID: metricAccuracy.ID, MetricName: metricAccuracy.Name, Name: "Mechanism", MaxPoints: 2, ScoringGuide: "0..2"},
		{ID: critC02, MetricID: metricAccuracy.ID, MetricName: metricAccuracy.Name, Name: "Limits", MaxPoints: 3, ScoringGuide: "0..3"},
		{ID: critC03, MetricID: metricSolving.ID, MetricName: metricSolving.Name, Name: "Recommendation", MaxPoints: 5, ScoringGuide: "0..5"},
	}
}

// gradedQuestions assigns n core questions, all graded with R01 and an expected answer.
func gradedQuestions(n int) []domain.GradingQuestion {
	questions := make([]domain.GradingQuestion, 0, n)
	for i := 1; i <= n; i++ {
		questions = append(questions, domain.GradingQuestion{
			Position: int32(i), RubricID: &rubricR01, RubricName: "R01", ExpectedAnswer: "EXPECTED-SECRET-REFERENCE", Criteria: r01Criteria(),
		})
	}
	return questions
}

func text(s string) *string { return &s }

// answeredTurns is two core questions, the first with a follow-up.
func answeredTurns() []domain.TurnRecord {
	return []domain.TurnRecord{
		{Position: 1, QuestionNumber: 1, Kind: domain.TurnCore, Question: "How would you index a table?", Answer: text("I would add a B-tree index on the filtered column.")},
		{Position: 2, QuestionNumber: 1, Kind: domain.TurnFollowUp, Question: "What does it cost?", Answer: text("Writes get slower.")},
		{Position: 3, QuestionNumber: 2, Kind: domain.TurnCore, Question: "How do you cache reads?", Answer: text("Use a read-through cache with a TTL.")},
	}
}

// completedPracticeRecord is a practice interview the Candidate finished, waiting for its evaluation.
func completedPracticeRecord() *domain.InterviewRecord {
	completed := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	return &domain.InterviewRecord{
		ID: scoringInterviewID, Type: domain.InterviewTypePractice, StartedAt: completed.Add(-time.Hour),
		CompletedAt: &completed, EvaluationStatus: domain.EvaluationPending, CandidateID: scoringCandidateID, QuestionCount: 2,
	}
}

// completedRecruitmentRecord is a recruitment interview of a posting, waiting for its evaluation.
func completedRecruitmentRecord() *domain.InterviewRecord {
	record := completedPracticeRecord()
	record.Type = domain.InterviewTypeRecruitment
	record.PostingID = &scoringPostingID
	record.RecruiterID = &scoringRecruiterID
	return record
}

// workedExampleRepository is the spec's worked example as a recruitment session: weights 60/40 on
// Technical Accuracy and Problem-Solving, two questions graded with R01.
func workedExampleRepository() *fakeInterviewRepository {
	return &fakeInterviewRepository{
		record: completedRecruitmentRecord(), turns: answeredTurns(), questions: gradedQuestions(2), metrics: scoringCatalog,
		weights: []domain.Weight{
			{MetricID: metricAccuracy.ID, Name: metricAccuracy.Name, Percentage: 60},
			{MetricID: metricSolving.ID, Name: metricSolving.Name, Percentage: 40},
		},
	}
}

// workedExamplePoints are the spec's points: Q01 2/2, 2/3, 3/5; Q02 1/2, 2/3, 2/5.
func workedExamplePoints() map[int32][]int32 {
	return map[int32][]int32{1: {2, 2, 3}, 2: {1, 2, 2}}
}

func newScoringForTest(t *testing.T, repository *fakeInterviewRepository, grader Grader) *Scoring {
	t.Helper()
	scoring, err := NewScoring(repository, grader)
	if err != nil {
		t.Fatalf("NewScoring() error = %v", err)
	}
	return scoring
}
