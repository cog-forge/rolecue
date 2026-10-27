package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// Fixed ids keep the scoring tests readable.
var (
	scoringInterviewID = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	scoringCandidateID = uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	scoringRecruiterID = uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	scoringPostingID   = uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd")

	metricAccuracy = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-0000000000a1"), Name: "Accuracy"}
	metricDepth    = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-0000000000a2"), Name: "Depth"}
	metricClarity  = domain.Metric{ID: uuid.MustParse("00000000-0000-4000-8000-0000000000a3"), Name: "Clarity"}
	scoringCatalog = []domain.Metric{metricAccuracy, metricDepth, metricClarity}
)

// fakeInterviewRepository keeps the rows the scoring tests read and records the writes.
type fakeInterviewRepository struct {
	record  *domain.InterviewRecord // nil means no such interview
	turns   []domain.TurnRecord
	scores  []domain.ReportBreakdown
	metrics []domain.Metric
	weights []domain.Weight

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

func (f *fakeInterviewRepository) InterviewScores(context.Context, uuid.UUID) ([]domain.ReportBreakdown, error) {
	return f.scores, nil
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

// fakeScorer returns a fixed output or error and records how often and with what input it was called.
type fakeScorer struct {
	output domain.ScoreOutput
	err    error
	calls  int
	input  domain.ScoreInput
}

func (f *fakeScorer) Score(_ context.Context, in domain.ScoreInput) (domain.ScoreOutput, error) {
	f.calls++
	f.input = in
	return f.output, f.err
}

// scoresFor is the deterministic output of a fake scorer: the given score for each metric, in order.
func scoresFor(scores ...int32) []domain.MetricScore {
	metrics := []domain.Metric{metricAccuracy, metricDepth, metricClarity}
	out := make([]domain.MetricScore, 0, len(scores))
	for i, score := range scores {
		out = append(out, domain.MetricScore{MetricID: metrics[i].ID, Score: score})
	}
	return out
}

func answeredTurns() []domain.TurnRecord {
	first, second := "I would index it.", "Use a cache."
	return []domain.TurnRecord{
		{Position: 1, Question: "How would you index a table?", Answer: &first},
		{Position: 2, Question: "How do you cache reads?", Answer: &second},
	}
}

// completedPracticeRecord is a practice interview the Candidate finished, waiting for its evaluation.
func completedPracticeRecord() *domain.InterviewRecord {
	completed := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	return &domain.InterviewRecord{
		ID: scoringInterviewID, Type: domain.InterviewTypePractice, StartedAt: completed.Add(-time.Hour),
		CompletedAt: &completed, EvaluationStatus: domain.EvaluationPending, CandidateID: scoringCandidateID,
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

func newScoringForTest(t *testing.T, repository *fakeInterviewRepository, scorer Scorer) *Scoring {
	t.Helper()
	scoring, err := NewScoring(repository, scorer)
	if err != nil {
		t.Fatalf("NewScoring() error = %v", err)
	}
	return scoring
}
