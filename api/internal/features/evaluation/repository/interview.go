package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
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
		ID:               row.ID,
		Type:             row.Type,
		StartedAt:        row.StartedAt.Time,
		CompletedAt:      timePtr(row.CompletedAt),
		EvaluationStatus: row.EvaluationStatus,
		Score:            row.Score,
		Feedback:         row.Feedback,
		CandidateID:      row.CandidateID,
		PostingID:        uuidPtr(row.JobPostingID),
		RecruiterID:      uuidPtr(row.RecruiterID),
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
		turns = append(turns, domain.TurnRecord{Position: row.Position, Question: row.Question, Answer: row.Answer, Feedback: row.Feedback})
	}
	return turns, nil
}

// InterviewScores reads the metric scores of an interview.
func (r *Repository) InterviewScores(ctx context.Context, interviewID uuid.UUID) ([]domain.ReportBreakdown, error) {
	rows, err := r.queries.ListInterviewScores(ctx, interviewID)
	if err != nil {
		return nil, interviewPersistenceError(err)
	}
	scores := make([]domain.ReportBreakdown, 0, len(rows))
	for _, row := range rows {
		scores = append(scores, domain.ReportBreakdown{MetricID: row.MetricID, Name: row.Name, Score: row.Score})
	}
	return scores, nil
}

// SaveInterviewEvaluation writes the scores, the per-question feedback, the overall score and the summary,
// and sets the evaluation status to completed, all in one transaction. The interview row is locked first.
// An interview already completed by a concurrent run is left as it is.
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
		for _, score := range evaluation.Scores {
			err := q.UpsertScoreDetail(ctx, UpsertScoreDetailParams{
				InterviewID: interviewID, MetricID: score.MetricID, Score: score.Score,
			})
			if err != nil {
				return interviewPersistenceError(err)
			}
		}
		for _, question := range evaluation.Questions {
			feedback := question.Feedback
			err := q.SetTurnFeedback(ctx, SetTurnFeedbackParams{
				InterviewID: interviewID, Position: question.Position, Feedback: &feedback,
			})
			if err != nil {
				return interviewPersistenceError(err)
			}
		}
		overall := evaluation.OverallScore
		err = q.CompleteInterviewEvaluation(ctx, CompleteInterviewEvaluationParams{
			ID: interviewID, OverallScore: &overall, Feedback: evaluation.Summary,
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
