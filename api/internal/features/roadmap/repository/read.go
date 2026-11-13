package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/dbnum"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LoadReadModel reads, in one repeatable-read snapshot, everything the Roadmap read model is computed from for
// a profile the candidate owns: the Roadmap with its topics in display order and every result row of the
// profile that maps to one of its topics. A foreign or missing profile is JD_NOT_FOUND, the same answer as the
// Role Profile endpoints. Nothing is cached and nothing is written.
func (r *Repository) LoadReadModel(ctx context.Context, candidate, profile uuid.UUID) (domain.RoadmapRecord, []domain.ResultRow, error) {
	var rec domain.RoadmapRecord
	var rows []domain.ResultRow
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		head, err := q.GetOwnedRoadmapRead(ctx, GetOwnedRoadmapReadParams{RoleProfileID: profile, CandidateID: candidate})
		if err != nil {
			return err
		}
		rec = domain.RoadmapRecord{
			ID: head.ID, RoleProfileID: head.RoleProfileID,
			GenerationStatus: domain.GenerationStatus(head.GenerationStatus), PracticeStarted: head.PracticeStarted,
		}
		items, err := q.ListRoadmapItems(ctx, head.ID)
		if err != nil {
			return err
		}
		rec.Items = make([]domain.ItemRecord, 0, len(items))
		for _, it := range items {
			rec.Items = append(rec.Items, domain.ItemRecord{ID: it.ID, Title: it.Title, Description: it.Description, Position: it.Position})
		}
		results, err := q.ListRoadmapResultRows(ctx, ListRoadmapResultRowsParams{RoadmapID: head.ID, RoleProfileID: profile})
		if err != nil {
			return err
		}
		rows = make([]domain.ResultRow, 0, len(results))
		for _, row := range results {
			rows = append(rows, toResultRow(row))
		}
		return nil
	})
	if err != nil {
		return domain.RoadmapRecord{}, nil, persistence(err)
	}
	return rec, rows, nil
}

func toResultRow(row ListRoadmapResultRowsRow) domain.ResultRow {
	out := domain.ResultRow{
		ItemID: row.ItemID, CoreQuestionID: row.CoreQuestionID, InterviewID: row.InterviewID, Position: row.Position,
		InterviewType: row.InterviewType, EvaluationStatus: row.EvaluationStatus, InjectionFlagged: row.InjectionFlagged,
		Delivered: row.Delivered, Answered: row.Answered, QuestionScore: dbnum.Float(row.Score), Linked: row.Linked,
	}
	if row.CompletedAt.Valid {
		completed := row.CompletedAt.Time.UTC()
		out.CompletedAt = &completed
	}
	if row.QuestionText != "" {
		text := row.QuestionText
		out.QuestionText = &text
	}
	return out
}
