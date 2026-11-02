// Package dbtrace logs failed and slow database queries without ever logging SQL parameters or
// result data. A query is identified by its sqlc name ("-- name: GetRoleProfile :one").
package dbtrace

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// SlowQueryThreshold is the duration above which a successful query is logged at warn.
const SlowQueryThreshold = 500 * time.Millisecond

type startKey struct{}

type started struct {
	name string
	at   time.Time
}

// Tracer implements pgx.QueryTracer. It logs through logger.FromContext, so a query run inside a
// request carries that request's ID.
type Tracer struct {
	Slow time.Duration
}

var _ pgx.QueryTracer = (*Tracer)(nil)

func New() *Tracer { return &Tracer{Slow: SlowQueryThreshold} }

func (t *Tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, startKey{}, started{name: QueryName(data.SQL), at: time.Now()})
}

func (t *Tracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	begin, ok := ctx.Value(startKey{}).(started)
	if !ok {
		return
	}
	elapsed := time.Since(begin.at)
	log := logger.FromContext(ctx)
	fields := []zap.Field{
		logger.Op("db.query"),
		zap.String("query", begin.name),
		zap.Duration("duration", elapsed),
	}
	switch err := data.Err; {
	case err == nil:
		if elapsed >= t.Slow {
			log.Warn("slow database query", fields...)
		}
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, context.Canceled):
		// Expected outcomes, not faults.
		log.Debug("database query ended", append(fields, zap.String("outcome", err.Error()))...)
	default:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Code, table and constraint locate the fault; Detail is omitted because it echoes row values.
			fields = append(fields,
				zap.String("pg_code", pgErr.Code),
				zap.String("pg_table", pgErr.TableName),
				zap.String("pg_constraint", pgErr.ConstraintName),
			)
			if isExpectedConflict(pgErr.Code) {
				// The code handles these on purpose (idempotency keys, retried transactions), so they must
				// not page anyone or carry a stack trace; response.Error logs the request if it fails.
				log.Warn("database query conflict", append(fields, logger.Cause(err))...)
				return
			}
		}
		log.Error("database query failed", append(fields, logger.Cause(err))...)
	}
}

// isExpectedConflict reports SQLSTATE classes that callers routinely handle: 23 (integrity constraint
// violations such as unique_violation) and 40 (serialization failure, deadlock).
func isExpectedConflict(sqlState string) bool {
	return strings.HasPrefix(sqlState, "23") || strings.HasPrefix(sqlState, "40")
}

// QueryName extracts the sqlc name from the leading "-- name: X :kind" comment, or "unnamed".
func QueryName(sql string) string {
	const marker = "-- name:"
	line := strings.TrimSpace(sql)
	if !strings.HasPrefix(line, marker) {
		return "unnamed"
	}
	fields := strings.Fields(strings.TrimPrefix(line, marker))
	if len(fields) == 0 {
		return "unnamed"
	}
	return fields[0]
}
