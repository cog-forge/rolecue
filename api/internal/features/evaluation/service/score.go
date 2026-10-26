package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	maxScore         = 100
	maxSummaryRunes  = 4000
	maxFeedbackRunes = 2000
	// failureWriteTimeout bounds the write that marks an evaluation failed. That write runs even when the
	// evaluation's own context is done, so a timed out evaluation is not left pending forever.
	failureWriteTimeout = 10 * time.Second
)

// Scorer grades the answers of one interview session. It returns one score per metric; the service
// checks the result before anything is stored.
type Scorer interface {
	Score(ctx context.Context, in domain.ScoreInput) (domain.ScoreOutput, error)
}

// InterviewRepository is the persistence contract of evaluation and reports. SaveInterviewEvaluation runs
// in one transaction and locks the interview row itself.
type InterviewRepository interface {
	InterviewRecord(ctx context.Context, interviewID uuid.UUID) (domain.InterviewRecord, error)
	InterviewTurns(ctx context.Context, interviewID uuid.UUID) ([]domain.TurnRecord, error)
	InterviewScores(ctx context.Context, interviewID uuid.UUID) ([]domain.ReportBreakdown, error)
	SaveInterviewEvaluation(ctx context.Context, interviewID uuid.UUID, evaluation domain.Evaluation) error
	MarkInterviewEvaluationFailed(ctx context.Context, interviewID uuid.UUID) error
	ListMetrics(ctx context.Context) ([]domain.Metric, error)
	ListWeights(ctx context.Context, postingID uuid.UUID) ([]domain.Weight, error)
}

// Scoring evaluates completed interviews and builds their reports. It is separate from Service, which
// manages the posting weights, so the weights contract stays as it is.
type Scoring struct {
	repository InterviewRepository
	scorer     Scorer
}

// NewScoring builds the evaluation service around an interview repository and a scorer.
func NewScoring(repository InterviewRepository, scorer Scorer) (*Scoring, error) {
	if repository == nil {
		return nil, fmt.Errorf("interview evaluation repository is required")
	}
	if scorer == nil {
		return nil, fmt.Errorf("interview scorer is required")
	}
	return &Scoring{repository: repository, scorer: scorer}, nil
}

// Evaluate scores a completed interview. It is the interview service's Evaluator hook, called once after
// the first completion. A completed evaluation is a no-op, so no second LLM call is made. Without answered
// questions, with invalid weights or when the scorer fails, the evaluation is marked failed and nothing
// else is written. A failed save changes nothing and leaves the status pending, so a later run retries.
func (s *Scoring) Evaluate(ctx context.Context, interviewID uuid.UUID) error {
	record, err := s.repository.InterviewRecord(ctx, interviewID)
	if err != nil {
		return err
	}
	if record.EvaluationStatus == domain.EvaluationCompleted {
		return nil
	}
	if record.CompletedAt == nil {
		return apperror.New(apperror.CodeInterviewNotCompleted, "interview is not completed")
	}
	turns, err := s.repository.InterviewTurns(ctx, interviewID)
	if err != nil {
		return err
	}
	var saved []domain.Weight
	if record.PostingID != nil {
		saved, err = s.repository.ListWeights(ctx, *record.PostingID)
		if err != nil {
			return err
		}
	}
	catalog, err := s.repository.ListMetrics(ctx)
	if err != nil {
		return err
	}
	input, err := buildScoreInput(record, turns, saved, catalog)
	if err != nil {
		return s.fail(ctx, interviewID, err)
	}
	output, err := s.score(ctx, input)
	if err != nil {
		return s.fail(ctx, interviewID, err)
	}
	evaluation, err := buildEvaluation(input, output)
	if err != nil {
		return s.fail(ctx, interviewID, err)
	}
	if err := s.repository.SaveInterviewEvaluation(ctx, interviewID, evaluation); err != nil {
		// A failed save is marked failed too, so the report is honest and a repeated Complete can
		// start the evaluation again (see interview service Complete); left pending it would look
		// in flight forever.
		return s.fail(ctx, interviewID, err)
	}
	return nil
}

// fail marks the evaluation failed and returns the cause as an internal error. It is logged by the caller
// and never shown to a client.
func (s *Scoring) fail(ctx context.Context, interviewID uuid.UUID, cause error) error {
	failure := apperror.Wrap(apperror.CodeInternal, "interview evaluation failed", cause)
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureWriteTimeout)
	defer cancel()
	if err := s.repository.MarkInterviewEvaluationFailed(markCtx, interviewID); err != nil {
		return errors.Join(failure, fmt.Errorf("mark evaluation failed: %w", err))
	}
	return failure
}

// buildScoreInput keeps the answered questions and the metrics to score. It fails when the session has no
// answers or the weights cannot score it.
func buildScoreInput(record domain.InterviewRecord, turns []domain.TurnRecord, saved []domain.Weight, catalog []domain.Metric) (domain.ScoreInput, error) {
	answered := make([]domain.ScoreTurn, 0, len(turns))
	for _, turn := range turns {
		if turn.Answer == nil || strings.TrimSpace(*turn.Answer) == "" {
			continue
		}
		answered = append(answered, domain.ScoreTurn{Position: turn.Position, Question: turn.Question, Answer: *turn.Answer})
	}
	if len(answered) == 0 {
		return domain.ScoreInput{}, errors.New("interview has no answered questions to evaluate")
	}
	metrics, err := scoredMetrics(saved, catalog)
	if err != nil {
		return domain.ScoreInput{}, err
	}
	return domain.ScoreInput{InterviewID: record.ID, Type: record.Type, Turns: answered, Metrics: metrics}, nil
}

// scoredMetrics returns the saved posting weights when there are any. Otherwise every catalog metric counts
// equally (the equal split of the defaults, which are never stored). Weights must be finite, not negative,
// and add up to more than zero.
func scoredMetrics(saved []domain.Weight, catalog []domain.Metric) ([]domain.ScoredMetric, error) {
	if len(saved) > 0 {
		metrics := make([]domain.ScoredMetric, 0, len(saved))
		sum := 0.0
		for _, weight := range saved {
			if math.IsNaN(weight.Percentage) || math.IsInf(weight.Percentage, 0) || weight.Percentage < 0 {
				return nil, fmt.Errorf("evaluation weight of %s is invalid", weight.Name)
			}
			sum += weight.Percentage
			metrics = append(metrics, domain.ScoredMetric{MetricID: weight.MetricID, Name: weight.Name, Weight: weight.Percentage})
		}
		if sum <= 0 {
			return nil, errors.New("evaluation weights add up to zero")
		}
		return metrics, nil
	}
	if len(catalog) == 0 {
		return nil, errors.New("evaluation metrics catalog is empty")
	}
	metrics := make([]domain.ScoredMetric, 0, len(catalog))
	for _, metric := range catalog {
		metrics = append(metrics, domain.ScoredMetric{MetricID: metric.ID, Name: metric.Name, Weight: 1})
	}
	return metrics, nil
}

// buildEvaluation checks the Scorer's output against the input and computes the weighted overall score. The
// output must score each metric exactly once, within 0..100, and give feedback only for answered questions.
func buildEvaluation(input domain.ScoreInput, output domain.ScoreOutput) (domain.Evaluation, error) {
	weights := make(map[uuid.UUID]float64, len(input.Metrics))
	totalWeight := 0.0
	for _, metric := range input.Metrics {
		weights[metric.MetricID] = metric.Weight
		totalWeight += metric.Weight
	}
	if totalWeight <= 0 {
		return domain.Evaluation{}, errors.New("evaluation weights add up to zero")
	}
	if len(output.Scores) != len(input.Metrics) {
		return domain.Evaluation{}, fmt.Errorf("scorer returned %d scores for %d metrics", len(output.Scores), len(input.Metrics))
	}
	scored := make(map[uuid.UUID]bool, len(output.Scores))
	weighted := 0.0
	for _, score := range output.Scores {
		weight, ok := weights[score.MetricID]
		if !ok {
			return domain.Evaluation{}, fmt.Errorf("scorer scored unknown metric %s", score.MetricID)
		}
		if scored[score.MetricID] {
			return domain.Evaluation{}, fmt.Errorf("scorer scored metric %s twice", score.MetricID)
		}
		if score.Score < 0 || score.Score > maxScore {
			return domain.Evaluation{}, fmt.Errorf("score %d is out of range", score.Score)
		}
		scored[score.MetricID] = true
		weighted += weight * float64(score.Score)
	}
	summary := strings.TrimSpace(output.Summary)
	if !utf8.ValidString(summary) || utf8.RuneCountInString(summary) > maxSummaryRunes {
		return domain.Evaluation{}, errors.New("scorer summary is invalid or too long")
	}
	questions, err := feedbackFor(input.Turns, output.Questions)
	if err != nil {
		return domain.Evaluation{}, err
	}
	evaluation := domain.Evaluation{
		OverallScore: int32(math.Round(weighted / totalWeight)),
		Scores:       output.Scores,
		Questions:    questions,
	}
	if summary != "" {
		evaluation.Summary = &summary
	}
	return evaluation, nil
}

// feedbackFor keeps the feedback of answered positions only. Empty feedback is skipped.
func feedbackFor(turns []domain.ScoreTurn, feedback []domain.QuestionFeedback) ([]domain.QuestionFeedback, error) {
	answered := make(map[int32]bool, len(turns))
	for _, turn := range turns {
		answered[turn.Position] = true
	}
	seen := make(map[int32]bool, len(feedback))
	kept := make([]domain.QuestionFeedback, 0, len(feedback))
	for _, item := range feedback {
		if !answered[item.Position] {
			return nil, fmt.Errorf("feedback for unanswered position %d", item.Position)
		}
		if seen[item.Position] {
			return nil, fmt.Errorf("feedback for position %d given twice", item.Position)
		}
		seen[item.Position] = true
		text := strings.TrimSpace(item.Feedback)
		if !utf8.ValidString(text) || utf8.RuneCountInString(text) > maxFeedbackRunes {
			return nil, fmt.Errorf("feedback for position %d is invalid or too long", item.Position)
		}
		if text == "" {
			continue
		}
		kept = append(kept, domain.QuestionFeedback{Position: item.Position, Feedback: text})
	}
	return kept, nil
}

// score calls the scorer and turns a panic in it (or its LLM client) into an error, so the evaluation is
// marked failed and the API keeps running.
func (s *Scoring) score(ctx context.Context, input domain.ScoreInput) (output domain.ScoreOutput, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scorer panicked: %v", recovered)
		}
	}()
	return s.scorer.Score(ctx, input)
}
