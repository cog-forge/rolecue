package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	// gradingConcurrency bounds the parallel grading calls of one evaluation. With the 30 s route budget and
	// the default 8 questions a session fits in two waves, well inside the interview service's 2 minute bound.
	gradingConcurrency = 4
	// maxSummaryRunes bounds the session summary, which joins the one-sentence summaries of the questions.
	maxSummaryRunes = 3500
	// failureWriteTimeout bounds the write that marks an evaluation failed. That write runs even when the
	// evaluation's own context is done, so a timed out evaluation is not left pending forever.
	failureWriteTimeout = 10 * time.Second
)

// InterviewRepository is the persistence contract of evaluation and reports. SaveInterviewEvaluation runs
// in one transaction and locks the interview row itself.
type InterviewRepository interface {
	InterviewRecord(ctx context.Context, interviewID uuid.UUID) (domain.InterviewRecord, error)
	InterviewTurns(ctx context.Context, interviewID uuid.UUID) ([]domain.TurnRecord, error)
	GradingQuestions(ctx context.Context, interviewID uuid.UUID) ([]domain.GradingQuestion, error)
	InterviewResults(ctx context.Context, interviewID uuid.UUID) (domain.StoredResults, error)
	SaveInterviewEvaluation(ctx context.Context, interviewID uuid.UUID, evaluation domain.Evaluation) error
	MarkInterviewEvaluationFailed(ctx context.Context, interviewID uuid.UUID) error
	ListMetrics(ctx context.Context) ([]domain.Metric, error)
	ListWeights(ctx context.Context, postingID uuid.UUID) ([]domain.Weight, error)
}

// Scoring evaluates completed interviews and builds their reports. It is separate from Service, which
// manages the posting weights, so the weights contract stays as it is.
type Scoring struct {
	repository InterviewRepository
	grader     Grader
	policy     domain.ScoringPolicy
}

// NewScoring builds the evaluation service around an interview repository and a rubric grader.
func NewScoring(repository InterviewRepository, grader Grader) (*Scoring, error) {
	if repository == nil {
		return nil, fmt.Errorf("interview evaluation repository is required")
	}
	if grader == nil {
		return nil, fmt.Errorf("interview grader is required")
	}
	return &Scoring{repository: repository, grader: grader, policy: domain.DefaultScoringPolicy()}, nil
}

// Evaluate grades a completed interview with its rubrics (KAN-334). It is the interview service's Evaluator
// hook, called once after the first completion and again by its claim-and-retry path. Each answered core
// question that has a rubric and an expected answer is graded in its own model call, together with its
// follow-ups; the pure calculator then turns the criterion points into question, metric and overall scores,
// and everything is saved in one transaction.
//
//   - A completed evaluation is a no-op: no second model call, no rewrite.
//   - A result a retry could not change (nothing answered, nothing gradable, a metric no question covered)
//     completes with a withheld overall and a stored reason; it never counts as a failure.
//   - A grader, calculator or save error marks the evaluation failed and writes nothing else, so the claim and
//     retry mechanism can run it again.
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
	questions, err := s.repository.GradingQuestions(ctx, interviewID)
	if err != nil {
		return err
	}
	metrics, err := s.weights(ctx, record)
	if err != nil {
		return s.fail(ctx, interviewID, err)
	}
	plan, err := planGrading(turns, questions)
	if err != nil {
		return s.fail(ctx, interviewID, err)
	}
	var evaluation domain.Evaluation
	switch {
	case plan.answered == 0:
		evaluation = withheldEvaluation(domain.WithheldNoAnswers, metrics)
	case len(plan.inputs) == 0:
		evaluation = withheldEvaluation(domain.WithheldNoGradableQuestions, metrics)
	default:
		done := logger.Start(ctx, "evaluation.grade",
			logger.String("interview_id", interviewID.String()),
			logger.Int("assigned_count", int(record.QuestionCount)),
			logger.Int("answered_count", plan.answered),
			logger.Int("graded_count", len(plan.inputs)),
			logger.Int("metric_count", len(metrics)),
		)
		grades, gradeErr := s.gradeAll(ctx, plan.inputs)
		done(gradeErr)
		if gradeErr != nil {
			return s.fail(ctx, interviewID, gradeErr)
		}
		evaluation, err = s.buildEvaluation(record, plan, grades, metrics)
		if err != nil {
			return s.fail(ctx, interviewID, err)
		}
	}
	if err := s.repository.SaveInterviewEvaluation(ctx, interviewID, evaluation); err != nil {
		// A failed save is marked failed too, so the report is honest and a repeated Complete can start the
		// evaluation again (see interview service Complete); left pending it would look in flight forever.
		return s.fail(ctx, interviewID, err)
	}
	overall := ""
	if evaluation.Overall != nil {
		overall = strconv.FormatFloat(*evaluation.Overall, 'f', 2, 64)
	}
	logger.FromContext(ctx).Info("interview evaluation completed",
		logger.Op("evaluation.complete"),
		logger.String("interview_id", interviewID.String()),
		logger.String("outcome", string(evaluation.Outcome)),
		logger.String("withheld_reason", string(evaluation.Reason)),
		logger.String("score", overall),
		logger.Int("injection_flagged", boolInt(evaluation.InjectionFlagged)),
		logger.Int("graded_count", len(evaluation.Questions)),
		logger.Int("metric_count", len(evaluation.Metrics)),
	)
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

// weights returns the metrics the overall is weighted over: the posting's saved weights, or every catalog
// metric with an equal weight (the defaults, which are never stored) for practice and for a posting without
// saved weights.
func (s *Scoring) weights(ctx context.Context, record domain.InterviewRecord) ([]domain.ScoredMetric, error) {
	var saved []domain.Weight
	if record.PostingID != nil {
		var err error
		if saved, err = s.repository.ListWeights(ctx, *record.PostingID); err != nil {
			return nil, err
		}
	}
	catalog, err := s.repository.ListMetrics(ctx)
	if err != nil {
		return nil, err
	}
	return scoredMetrics(saved, catalog)
}

// scoredMetrics returns the saved posting weights when there are any. Otherwise every catalog metric counts
// equally. Weights must be finite, not negative, and add up to more than zero.
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

// gradingPlan is what a session gives the grader: one input per gradable answered core question, and the
// core turn of each question for its feedback.
type gradingPlan struct {
	inputs   []domain.QuestionGradeInput
	coreTurn map[int32]int32
	// answered counts the assigned questions with at least one non-empty answer (graded or not).
	answered int
}

// planGrading groups the turns by the core question they belong to and builds one grading input per answered
// question that has a rubric with criteria and an expected answer. A question without them (a legacy or
// hand-added question) is not graded: it counts as an unscored question of a partial session, and the
// missing-coverage policy decides whether an overall can still be given. A turn that maps to no assigned
// question means the conversation and the assignment disagree; that is an error, never a guess.
func planGrading(turns []domain.TurnRecord, questions []domain.GradingQuestion) (gradingPlan, error) {
	byPosition := make(map[int32]domain.GradingQuestion, len(questions))
	for _, q := range questions {
		byPosition[q.Position] = q
	}
	plan := gradingPlan{coreTurn: map[int32]int32{}}
	answers := make(map[int32][]domain.GradeTurn)
	for _, turn := range turns {
		if _, ok := byPosition[turn.QuestionNumber]; !ok {
			return gradingPlan{}, fmt.Errorf("turn %d belongs to core question %d, which was not assigned", turn.Position, turn.QuestionNumber)
		}
		if turn.Kind == domain.TurnCore {
			if _, seen := plan.coreTurn[turn.QuestionNumber]; !seen {
				plan.coreTurn[turn.QuestionNumber] = turn.Position
			}
		}
		if turn.Answer == nil || strings.TrimSpace(*turn.Answer) == "" {
			continue
		}
		answers[turn.QuestionNumber] = append(answers[turn.QuestionNumber], domain.GradeTurn{
			Position: turn.Position, Kind: turn.Kind, Question: scorerQuestion(turn), Answer: *turn.Answer,
		})
	}
	for _, q := range questions {
		answered := answers[q.Position]
		if len(answered) == 0 {
			continue
		}
		plan.answered++
		if q.RubricID == nil || len(q.Criteria) == 0 || strings.TrimSpace(q.ExpectedAnswer) == "" {
			continue
		}
		criteria := make([]domain.GradeCriterion, 0, len(q.Criteria))
		for i, c := range q.Criteria {
			criteria = append(criteria, domain.GradeCriterion{
				ID: c.ID, Key: "C" + strconv.Itoa(i+1), MetricID: c.MetricID, MetricName: c.MetricName,
				Name: c.Name, MaxPoints: c.MaxPoints, ScoringGuide: c.ScoringGuide,
			})
		}
		plan.inputs = append(plan.inputs, domain.QuestionGradeInput{
			QuestionPosition: q.Position, RubricName: q.RubricName, RubricDescription: q.RubricDescription,
			ExpectedAnswer: q.ExpectedAnswer, Criteria: criteria, Turns: answered,
		})
	}
	return plan, nil
}

// gradeAll grades every question with at most gradingConcurrency calls in flight. Each call sees one question
// only, so an injected answer can move the points of its own question and never those of another (KAN-279).
// The first error cancels the calls still running; nothing is kept from a partly graded session.
func (s *Scoring) gradeAll(ctx context.Context, inputs []domain.QuestionGradeInput) ([]domain.QuestionGrade, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	grades := make([]domain.QuestionGrade, len(inputs))
	errs := make([]error, len(inputs))
	slots := make(chan struct{}, gradingConcurrency)
	var wg sync.WaitGroup
	for i, input := range inputs {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			defer func() {
				// The panic value is not kept: it can carry the answer text the grader was given.
				if recovered := recover(); recovered != nil {
					errs[i] = fmt.Errorf("grader panicked (%T)", recovered)
					cancel()
				}
			}()
			if err := ctx.Err(); err != nil {
				errs[i] = err
				return
			}
			grades[i], errs[i] = s.grader.GradeQuestion(ctx, input)
			if errs[i] != nil {
				cancel()
			}
		}()
	}
	wg.Wait()
	// Report the first real failure, not the cancellations it caused.
	var first error
	for _, err := range errs {
		if err == nil {
			continue
		}
		if first == nil || (errors.Is(first, context.Canceled) && !errors.Is(err, context.Canceled)) {
			first = err
		}
	}
	if first != nil {
		return nil, first
	}
	for i, grade := range grades {
		if grade.QuestionPosition != inputs[i].QuestionPosition || len(grade.Criteria) != len(inputs[i].Criteria) {
			return nil, fmt.Errorf("grade of question %d does not match its input", inputs[i].QuestionPosition)
		}
	}
	return grades, nil
}

// buildEvaluation applies the calculator to the validated grades and assembles what is stored.
func (s *Scoring) buildEvaluation(record domain.InterviewRecord, plan gradingPlan, grades []domain.QuestionGrade, metrics []domain.ScoredMetric) (domain.Evaluation, error) {
	input := domain.RubricScoreInput{Weights: metrics, AssignedQuestions: record.QuestionCount}
	byPosition := make(map[int32]domain.QuestionGrade, len(grades))
	for _, grade := range grades {
		points := domain.QuestionPoints{Position: grade.QuestionPosition}
		for _, c := range grade.Criteria {
			points.Criteria = append(points.Criteria, domain.CriterionPoints{
				CriterionID: c.CriterionID, MetricID: c.MetricID, MaxPoints: c.MaxPoints, Points: c.Points,
			})
		}
		input.Questions = append(input.Questions, points)
		byPosition[grade.QuestionPosition] = grade
	}
	score, err := CalculateRubricScore(input, s.policy)
	if err != nil {
		return domain.Evaluation{}, err
	}
	evaluation := domain.Evaluation{
		Outcome: score.Outcome, Reason: score.Reason, Overall: score.Overall,
		Metrics: metricResults(score.Metrics, metrics),
	}
	var summaries []string
	for _, q := range score.Questions {
		grade := byPosition[q.Position]
		evaluation.Questions = append(evaluation.Questions, domain.QuestionResult{Position: q.Position, Total: q.Total, Criteria: grade.Criteria})
		evaluation.InjectionFlagged = evaluation.InjectionFlagged || grade.InjectionFlagged
		if position, ok := plan.coreTurn[q.Position]; ok && grade.Feedback != "" {
			evaluation.TurnFeedback = append(evaluation.TurnFeedback, domain.TurnFeedback{Position: position, Feedback: grade.Feedback})
		}
		if grade.Summary != "" {
			summaries = append(summaries, grade.Summary)
		}
	}
	if summary := textsafe.Truncate(strings.Join(summaries, " "), maxSummaryRunes); summary != "" {
		evaluation.Summary = &summary
	}
	return evaluation, nil
}

// withheldEvaluation is the stored result of a session nothing could be graded in. It still records every
// weighted metric with its weight, so the report can say what would have been measured.
func withheldEvaluation(reason domain.RubricWithheldReason, metrics []domain.ScoredMetric) domain.Evaluation {
	return domain.Evaluation{Outcome: domain.RubricWithheld, Reason: reason, Metrics: metricResults(nil, metrics)}
}

// metricResults lists every weighted metric with its weight (a nil score when no graded question assessed
// it) and every assessed metric the weights do not cover (weight 0), in a stable order.
func metricResults(assessed []domain.RubricMetricScore, weights []domain.ScoredMetric) []domain.MetricResult {
	results := make([]domain.MetricResult, 0, len(weights)+len(assessed))
	seen := make(map[uuid.UUID]bool, len(weights))
	scores := make(map[uuid.UUID]domain.RubricMetricScore, len(assessed))
	for _, m := range assessed {
		scores[m.MetricID] = m
	}
	for _, w := range weights {
		result := domain.MetricResult{MetricID: w.MetricID, Weight: w.Weight}
		if m, ok := scores[w.MetricID]; ok {
			score := m.Score
			result.Score, result.Questions = &score, m.Questions
		}
		results = append(results, result)
		seen[w.MetricID] = true
	}
	for _, m := range assessed {
		if !seen[m.MetricID] {
			score := m.Score
			results = append(results, domain.MetricResult{MetricID: m.MetricID, Score: &score, Questions: m.Questions})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return compareIDs(results[i].MetricID, results[j].MetricID) < 0 })
	return results
}
