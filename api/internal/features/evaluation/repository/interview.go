package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/dbnum"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// InterviewRecord reads one interview with its owners. A missing interview is domain.ErrInterviewNotFound.
func (r *Repository) InterviewRecord(ctx context.Context, interviewID uuid.UUID) (domain.InterviewRecord, error) {
	row, err := r.queries.GetInterviewRecord(ctx, interviewID)
	if err != nil {
		return domain.InterviewRecord{}, interviewPersistenceError(err)
	}
	return domain.InterviewRecord{
		ID:                  row.ID,
		Type:                row.Type,
		StartedAt:           row.StartedAt.Time,
		CompletedAt:         timePtr(row.CompletedAt),
		EndReason:           row.EndReason,
		QuestionCount:       row.QuestionCount,
		EvaluationStatus:    row.EvaluationStatus,
		Score:               dbnum.Float(row.Score),
		ScoreOutcome:        row.ScoreOutcome,
		ScoreWithheldReason: row.ScoreWithheldReason,
		Feedback:            row.Feedback,
		InjectionFlagged:    row.InjectionFlagged,
		CandidateID:         row.CandidateID,
		PostingID:           uuidPtr(row.JobPostingID),
		RecruiterID:         uuidPtr(row.RecruiterID),
	}, nil
}

// InterviewTurns reads the stored questions of an interview in position order.
func (r *Repository) InterviewTurns(ctx context.Context, interviewID uuid.UUID) ([]domain.TurnRecord, error) {
	rows, err := r.queries.ListEvaluationTurns(ctx, interviewID)
	if err != nil {
		return nil, interviewPersistenceError(err)
	}
	turns := make([]domain.TurnRecord, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, domain.TurnRecord{
			Position: row.Position, QuestionNumber: row.QuestionNumber, Kind: row.Kind,
			Question: row.Question, Answer: row.Answer, Feedback: row.Feedback,
		})
	}
	return turns, nil
}

// GradingQuestions reads the core questions assigned to an interview with their rubric criteria and expected
// answer, in position order.
func (r *Repository) GradingQuestions(ctx context.Context, interviewID uuid.UUID) ([]domain.GradingQuestion, error) {
	rows, err := r.queries.ListGradingQuestions(ctx, interviewID)
	if err != nil {
		return nil, interviewPersistenceError(err)
	}
	questions := make([]domain.GradingQuestion, 0, len(rows))
	var rubricIDs []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for _, row := range rows {
		q := domain.GradingQuestion{Position: row.Position, RubricID: uuidPtr(row.RubricID)}
		if row.ExpectedAnswer != nil {
			q.ExpectedAnswer = *row.ExpectedAnswer
		}
		if row.RubricName != nil {
			q.RubricName = *row.RubricName
		}
		if row.RubricDescription != nil {
			q.RubricDescription = *row.RubricDescription
		}
		if q.RubricID != nil && !seen[*q.RubricID] {
			seen[*q.RubricID] = true
			rubricIDs = append(rubricIDs, *q.RubricID)
		}
		questions = append(questions, q)
	}
	if len(rubricIDs) == 0 {
		return questions, nil
	}
	criteria, err := r.queries.ListGradingCriteria(ctx, rubricIDs)
	if err != nil {
		return nil, interviewPersistenceError(err)
	}
	byRubric := map[uuid.UUID][]domain.GradingCriterion{}
	for _, c := range criteria {
		byRubric[c.RubricID] = append(byRubric[c.RubricID], domain.GradingCriterion{
			ID: c.ID, MetricID: c.MetricID, MetricName: c.MetricName, Name: c.Name, MaxPoints: c.MaxPoints, ScoringGuide: c.ScoringGuide,
		})
	}
	for i := range questions {
		if questions[i].RubricID != nil {
			questions[i].Criteria = byRubric[*questions[i].RubricID]
		}
	}
	return questions, nil
}

// InterviewResults reads what a completed evaluation stored: the session metrics, the assigned questions with
// their totals and the criterion results.
func (r *Repository) InterviewResults(ctx context.Context, interviewID uuid.UUID) (domain.StoredResults, error) {
	metrics, err := r.queries.ListInterviewScores(ctx, interviewID)
	if err != nil {
		return domain.StoredResults{}, interviewPersistenceError(err)
	}
	questions, err := r.queries.ListQuestionScores(ctx, interviewID)
	if err != nil {
		return domain.StoredResults{}, interviewPersistenceError(err)
	}
	criteria, err := r.queries.ListCriterionScores(ctx, interviewID)
	if err != nil {
		return domain.StoredResults{}, interviewPersistenceError(err)
	}
	results := domain.StoredResults{
		Metrics:   make([]domain.StoredMetric, 0, len(metrics)),
		Questions: make([]domain.StoredQuestion, 0, len(questions)),
		Criteria:  make([]domain.StoredCriterion, 0, len(criteria)),
	}
	for _, m := range metrics {
		results.Metrics = append(results.Metrics, domain.StoredMetric{
			MetricID: m.MetricID, Name: m.Name, Score: dbnum.Float(m.Score), Weight: m.Weight, Questions: m.QuestionCount, Evidence: m.Evidence,
		})
	}
	for _, q := range questions {
		results.Questions = append(results.Questions, domain.StoredQuestion{
			Position: q.Position, Score: dbnum.Float(q.Score), HasRubric: q.RubricID.Valid, RubricName: q.RubricName,
			SystemRubric: q.RubricScope != nil && *q.RubricScope == "system",
		})
	}
	for _, c := range criteria {
		results.Criteria = append(results.Criteria, domain.StoredCriterion{
			QuestionPosition: c.QuestionPosition, CriterionID: c.RubricCriterionID, Name: c.Name, MetricID: c.MetricID,
			MetricName: c.MetricName, MaxPoints: c.MaxPoints, Points: c.Points, Evidence: c.Evidence,
			EvidencePosition: c.EvidenceTurnPosition, Feedback: c.Feedback,
		})
	}
	return results, nil
}

// SaveInterviewEvaluation writes the criterion results, the question totals, the session metrics with their
// weights, the per-question feedback, the overall with its outcome and the summary, and sets the evaluation
// status to completed, all in one transaction. The interview row is locked first. An interview already
// completed (by a concurrent run or an earlier one) is left exactly as it is, so a replay never rewrites a
// result. Any error rolls the whole write back: no partial scores are ever visible.
func (r *Repository) SaveInterviewEvaluation(ctx context.Context, interviewID uuid.UUID, evaluation domain.Evaluation) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		status, err := q.LockInterviewForEvaluation(ctx, interviewID)
		if err != nil {
			return interviewPersistenceError(err)
		}
		if status == domain.EvaluationCompleted {
			return nil
		}
		if err := q.DeleteCriterionScores(ctx, interviewID); err != nil {
			return interviewPersistenceError(err)
		}
		if err := q.DeleteScoreDetails(ctx, interviewID); err != nil {
			return interviewPersistenceError(err)
		}
		for _, question := range evaluation.Questions {
			for _, c := range question.Criteria {
				err := q.InsertCriterionScore(ctx, InsertCriterionScoreParams{
					InterviewID: interviewID, QuestionPosition: question.Position, RubricCriterionID: c.CriterionID,
					Points: c.Points, Evidence: textPtr(c.Evidence), EvidenceTurnPosition: c.EvidencePosition, Feedback: textPtr(c.Feedback),
				})
				if err != nil {
					return interviewPersistenceError(err)
				}
			}
			err := q.SetQuestionScore(ctx, SetQuestionScoreParams{InterviewID: interviewID, Position: question.Position, Score: question.Total})
			if err != nil {
				return interviewPersistenceError(err)
			}
		}
		for _, metric := range evaluation.Metrics {
			weight, count := metric.Weight, metric.Questions
			err := q.InsertScoreDetail(ctx, InsertScoreDetailParams{
				InterviewID: interviewID, MetricID: metric.MetricID, Score: metric.Score, Weight: &weight, QuestionCount: &count,
			})
			if err != nil {
				return interviewPersistenceError(err)
			}
		}
		for _, item := range evaluation.TurnFeedback {
			feedback := item.Feedback
			err := q.SetTurnFeedback(ctx, SetTurnFeedbackParams{InterviewID: interviewID, Position: item.Position, Feedback: &feedback})
			if err != nil {
				return interviewPersistenceError(err)
			}
		}
		outcome := string(evaluation.Outcome)
		err = q.CompleteInterviewEvaluation(ctx, CompleteInterviewEvaluationParams{
			ID: interviewID, OverallScore: evaluation.Overall, ScoreOutcome: &outcome,
			ScoreWithheldReason: textPtr(string(evaluation.Reason)), Feedback: evaluation.Summary,
			InjectionFlagged: evaluation.InjectionFlagged,
		})
		return interviewPersistenceError(err)
	})
	return interviewPersistenceError(err)
}

// MarkInterviewEvaluationFailed sets the evaluation status to failed unless it is already completed.
func (r *Repository) MarkInterviewEvaluationFailed(ctx context.Context, interviewID uuid.UUID) error {
	return interviewPersistenceError(r.queries.MarkInterviewEvaluationFailed(ctx, interviewID))
}

// interviewPersistenceError keeps AppErrors, maps a missing interview to not found and everything else to
// an internal error. The cause is kept for logs and never sent to the client.
func interviewPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInterviewNotFound
	}
	return apperror.Wrap(apperror.CodeInternal, "interview evaluation persistence failed", err)
}

func textPtr(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func uuidPtr(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}

// CoreResultFacts reads, in position order, what is stored about each core question assigned to an interview:
// delivered, answered and the Barem question score. It applies no policy and does not check the interview type
// or state; the caller reads InterviewRecord for that.
func (r *Repository) CoreResultFacts(ctx context.Context, interviewID uuid.UUID) ([]domain.CoreResultFacts, error) {
	rows, err := r.queries.ListCoreResultFacts(ctx, interviewID)
	if err != nil {
		return nil, interviewPersistenceError(err)
	}
	facts := make([]domain.CoreResultFacts, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, domain.CoreResultFacts{
			Position: row.Position, CoreQuestionID: row.CoreQuestionID,
			Delivered: row.Delivered, Answered: row.Answered, Score: dbnum.Float(row.Score),
		})
	}
	return facts, nil
}
