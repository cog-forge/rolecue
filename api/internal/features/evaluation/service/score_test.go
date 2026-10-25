package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestEvaluateWeighsEqualMetricsForPractice(t *testing.T) {
	repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: answeredTurns(), metrics: scoringCatalog}
	scorer := &fakeScorer{output: domain.ScoreOutput{
		Scores:  scoresFor(90, 60, 30),
		Summary: "  Solid answers.  ",
		Questions: []domain.QuestionFeedback{
			{Position: 1, Feedback: " Name the index type. "},
			{Position: 2, Feedback: "   "},
		},
	}}

	if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if scorer.calls != 1 || repo.saved == nil {
		t.Fatalf("scorer calls = %d, saved = %v; want one call and a save", scorer.calls, repo.saved)
	}
	if repo.saved.OverallScore != 60 {
		t.Fatalf("OverallScore = %d, want 60 (equal mean of 90, 60, 30)", repo.saved.OverallScore)
	}
	if repo.saved.Summary == nil || *repo.saved.Summary != "Solid answers." {
		t.Fatalf("Summary = %v, want the trimmed summary", repo.saved.Summary)
	}
	if len(repo.saved.Questions) != 1 || repo.saved.Questions[0].Position != 1 || repo.saved.Questions[0].Feedback != "Name the index type." {
		t.Fatalf("Questions = %+v, want only the non-empty feedback of position 1", repo.saved.Questions)
	}
	if repo.marked {
		t.Fatal("evaluation was marked failed on success")
	}
	for _, metric := range scorer.input.Metrics {
		if metric.Weight != 1 {
			t.Fatalf("metric %s weight = %v, want the equal weight 1", metric.Name, metric.Weight)
		}
	}
}

func TestEvaluateUsesPostingWeightsForRecruitment(t *testing.T) {
	repo := &fakeInterviewRepository{
		record: completedRecruitmentRecord(), turns: answeredTurns(), metrics: scoringCatalog,
		weights: []domain.Weight{
			{MetricID: metricAccuracy.ID, Name: metricAccuracy.Name, Percentage: 60},
			{MetricID: metricDepth.ID, Name: metricDepth.Name, Percentage: 40},
		},
	}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: []domain.MetricScore{
		{MetricID: metricAccuracy.ID, Score: 80},
		{MetricID: metricDepth.ID, Score: 50},
	}}}

	if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if repo.saved == nil || repo.saved.OverallScore != 68 {
		t.Fatalf("OverallScore = %v, want 68 = (60*80 + 40*50) / 100", repo.saved)
	}
	if len(scorer.input.Metrics) != 2 || scorer.input.Metrics[0].Weight != 60 || scorer.input.Metrics[1].Weight != 40 {
		t.Fatalf("scored metrics = %+v, want the two posting weights", scorer.input.Metrics)
	}
}

func TestEvaluateRecruitmentWithoutSavedWeightsUsesCatalog(t *testing.T) {
	repo := &fakeInterviewRepository{record: completedRecruitmentRecord(), turns: answeredTurns(), metrics: scoringCatalog}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(90, 60, 30)}}

	if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(scorer.input.Metrics) != 3 || repo.saved == nil || repo.saved.OverallScore != 60 {
		t.Fatalf("metrics = %d, saved = %v; want the three catalog metrics scored equally", len(scorer.input.Metrics), repo.saved)
	}
}

func TestEvaluateRoundsTheOverallScore(t *testing.T) {
	tests := []struct {
		name   string
		scores []int32
		want   int32
	}{
		{name: "rounds down", scores: []int32{90, 60, 61}, want: 70},
		{name: "rounds up", scores: []int32{90, 61, 61}, want: 71},
		{name: "perfect", scores: []int32{100, 100, 100}, want: 100},
		{name: "zero", scores: []int32{0, 0, 0}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: answeredTurns(), metrics: scoringCatalog}
			scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(tt.scores...)}}
			if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if repo.saved.OverallScore != tt.want {
				t.Fatalf("OverallScore = %d, want %d", repo.saved.OverallScore, tt.want)
			}
		})
	}
}

func TestEvaluateEmptySummaryIsNotStored(t *testing.T) {
	repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: answeredTurns(), metrics: scoringCatalog}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(70, 70, 70), Summary: "   "}}

	if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if repo.saved.Summary != nil {
		t.Fatalf("Summary = %q, want nil for an empty summary", *repo.saved.Summary)
	}
}

func TestEvaluateCompletedIsANoOp(t *testing.T) {
	record := completedPracticeRecord()
	record.EvaluationStatus = domain.EvaluationCompleted
	repo := &fakeInterviewRepository{record: record, turns: answeredTurns(), metrics: scoringCatalog}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(90, 90, 90)}}

	if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if scorer.calls != 0 || repo.saveCalls != 0 || repo.marked {
		t.Fatalf("scorer calls = %d, saves = %d, marked = %v; want no work for a completed evaluation", scorer.calls, repo.saveCalls, repo.marked)
	}
}

func TestEvaluateRejectsAnInterviewThatIsNotCompleted(t *testing.T) {
	record := completedPracticeRecord()
	record.CompletedAt = nil
	repo := &fakeInterviewRepository{record: record, turns: answeredTurns(), metrics: scoringCatalog}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(90, 90, 90)}}

	err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID)
	if !apperror.IsCode(err, apperror.CodeInterviewNotCompleted) {
		t.Fatalf("Evaluate() error = %v, want INTERVIEW_NOT_COMPLETED", err)
	}
	if scorer.calls != 0 || repo.marked || repo.saveCalls != 0 {
		t.Fatal("an interview that is not completed must not be scored, marked or saved")
	}
}

func TestEvaluateMissingInterviewIsNotFound(t *testing.T) {
	repo := &fakeInterviewRepository{metrics: scoringCatalog}
	err := newScoringForTest(t, repo, &fakeScorer{}).Evaluate(context.Background(), scoringInterviewID)
	if !apperror.IsCode(err, apperror.CodeInterviewNotFound) {
		t.Fatalf("Evaluate() error = %v, want INTERVIEW_NOT_FOUND", err)
	}
}

func TestEvaluateWithoutAnswersFailsWithoutCallingTheScorer(t *testing.T) {
	unanswered := []domain.TurnRecord{{Position: 1, Question: "How would you index a table?"}}
	repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: unanswered, metrics: scoringCatalog}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(90, 90, 90)}}

	err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID)
	if err == nil {
		t.Fatal("Evaluate() error = nil, want an error for missing evidence")
	}
	if scorer.calls != 0 || repo.saveCalls != 0 || !repo.marked {
		t.Fatalf("scorer calls = %d, saves = %d, marked = %v; want failed with nothing written", scorer.calls, repo.saveCalls, repo.marked)
	}
}

func TestEvaluateInvalidWeightsFail(t *testing.T) {
	tests := []struct {
		name    string
		weights []domain.Weight
	}{
		{name: "all zero", weights: []domain.Weight{{MetricID: metricAccuracy.ID, Name: "Accuracy", Percentage: 0}}},
		{name: "negative", weights: []domain.Weight{{MetricID: metricAccuracy.ID, Name: "Accuracy", Percentage: -5}, {MetricID: metricDepth.ID, Name: "Depth", Percentage: 10}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeInterviewRepository{
				record: completedRecruitmentRecord(), turns: answeredTurns(), metrics: scoringCatalog, weights: tt.weights,
			}
			scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(90, 90)}}
			if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err == nil {
				t.Fatal("Evaluate() error = nil, want an error for invalid weights")
			}
			if scorer.calls != 0 || repo.saveCalls != 0 || !repo.marked {
				t.Fatalf("scorer calls = %d, saves = %d, marked = %v; want failed before scoring", scorer.calls, repo.saveCalls, repo.marked)
			}
		})
	}
}

func TestEvaluateRejectsInvalidScorerOutput(t *testing.T) {
	unknown := uuid.MustParse("00000000-0000-4000-8000-0000000000ff")
	tests := []struct {
		name   string
		output domain.ScoreOutput
	}{
		{name: "missing metric", output: domain.ScoreOutput{Scores: []domain.MetricScore{
			{MetricID: metricAccuracy.ID, Score: 90}, {MetricID: metricDepth.ID, Score: 90},
		}}},
		{name: "score above 100", output: domain.ScoreOutput{Scores: scoresFor(101, 90, 90)}},
		{name: "negative score", output: domain.ScoreOutput{Scores: scoresFor(-1, 90, 90)}},
		{name: "duplicate metric", output: domain.ScoreOutput{Scores: []domain.MetricScore{
			{MetricID: metricAccuracy.ID, Score: 90}, {MetricID: metricAccuracy.ID, Score: 90}, {MetricID: metricDepth.ID, Score: 90},
		}}},
		{name: "unknown metric", output: domain.ScoreOutput{Scores: []domain.MetricScore{
			{MetricID: unknown, Score: 90}, {MetricID: metricDepth.ID, Score: 90}, {MetricID: metricClarity.ID, Score: 90},
		}}},
		{name: "feedback for unanswered position", output: domain.ScoreOutput{
			Scores:    scoresFor(90, 90, 90),
			Questions: []domain.QuestionFeedback{{Position: 9, Feedback: "Not asked."}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: answeredTurns(), metrics: scoringCatalog}
			scorer := &fakeScorer{output: tt.output}
			if err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID); err == nil {
				t.Fatal("Evaluate() error = nil, want an error for invalid scorer output")
			}
			if repo.saveCalls != 0 || !repo.marked {
				t.Fatalf("saves = %d, marked = %v; want nothing saved and the evaluation marked failed", repo.saveCalls, repo.marked)
			}
		})
	}
}

func TestEvaluateScorerErrorMarksFailed(t *testing.T) {
	repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: answeredTurns(), metrics: scoringCatalog}
	scorer := &fakeScorer{err: errors.New("llm down")}

	err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID)
	if !apperror.IsCode(err, apperror.CodeInternal) {
		t.Fatalf("Evaluate() error = %v, want an internal error", err)
	}
	if repo.saveCalls != 0 || !repo.marked {
		t.Fatalf("saves = %d, marked = %v; want nothing saved and marked failed", repo.saveCalls, repo.marked)
	}
}

func TestEvaluateSaveFailureMarksTheEvaluationFailed(t *testing.T) {
	saveErr := errors.New("connection reset")
	repo := &fakeInterviewRepository{record: completedPracticeRecord(), turns: answeredTurns(), metrics: scoringCatalog, saveErr: saveErr}
	scorer := &fakeScorer{output: domain.ScoreOutput{Scores: scoresFor(90, 90, 90)}}

	err := newScoringForTest(t, repo, scorer).Evaluate(context.Background(), scoringInterviewID)
	if !errors.Is(err, saveErr) {
		t.Fatalf("Evaluate() error = %v, want the save error", err)
	}
	if !repo.marked {
		t.Fatal("a failed save must mark the evaluation failed: left pending it would look in flight forever, and a repeated Complete retries a failed one")
	}
}
