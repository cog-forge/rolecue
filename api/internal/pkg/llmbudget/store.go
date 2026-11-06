package llmbudget

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PostgresStore keeps the daily counts in the llm_usage table.
type PostgresStore struct {
	queries *repository.Queries
}

// NewPostgresStore accepts a pgx pool or transaction.
func NewPostgresStore(db repository.DBTX) *PostgresStore {
	return &PostgresStore{queries: repository.New(db)}
}

var _ Store = (*PostgresStore)(nil)

// Consume implements Store with one atomic upsert. The statement returns no row when the day's count
// already reached the limit.
func (s *PostgresStore) Consume(ctx context.Context, userID uuid.UUID, day time.Time, limit int) (int, bool, error) {
	calls, err := s.queries.ConsumeLLMCall(ctx, repository.ConsumeLLMCallParams{
		UserID:    userID,
		Day:       pgtype.Date{Time: day, Valid: true},
		CallLimit: int32(limit),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return limit, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return int(calls), true, nil
}

// Release implements Store.
func (s *PostgresStore) Release(ctx context.Context, userID uuid.UUID, day time.Time) error {
	return s.queries.ReleaseLLMCall(ctx, repository.ReleaseLLMCallParams{UserID: userID, Day: pgtype.Date{Time: day, Valid: true}})
}
