package dbtrace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

func TestQueryName(t *testing.T) {
	for _, tc := range []struct {
		name, sql, want string
	}{
		{"sqlc named query", "-- name: GetRoleProfile :one\nSELECT * FROM role_profiles", "GetRoleProfile"},
		{"sqlc named exec", "-- name: DeleteJD :exec\nDELETE FROM jds", "DeleteJD"},
		{"leading whitespace", "  \n\t-- name: ListUsers :many\nSELECT 1", "ListUsers"},
		{"spaces around name", "-- name:   Spaced   :exec", "Spaced"},
		{"unnamed plain sql", "SELECT 1", "unnamed"},
		{"empty", "", "unnamed"},
		{"marker without name", "-- name:", "unnamed"},
		{"marker not leading", "SELECT 1 -- name: Foo :one", "unnamed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryName(tc.sql); got != tc.want {
				t.Fatalf("QueryName(%q) = %q, want %q", tc.sql, got, tc.want)
			}
		})
	}
}

func newObservedCtx() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	log := &logger.Logger{Logger: zap.New(core, zap.AddCaller())}
	return logger.ToContext(context.Background(), log), logs
}

// run drives the tracer the way pgx does: start, then end with the same context.
func run(tr *Tracer, ctx context.Context, sql string, err error) {
	startCtx := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: sql})
	tr.TraceQueryEnd(startCtx, nil, pgx.TraceQueryEndData{Err: err})
}

func TestTracer_FastSuccessIsSilent(t *testing.T) {
	ctx, logs := newObservedCtx()
	run(New(), ctx, "-- name: GetRoleProfile :one\nSELECT 1", nil)
	if n := logs.Len(); n != 0 {
		t.Fatalf("fast success logged %d entries: %v", n, logs.AllUntimed())
	}
}

func TestTracer_SlowSuccessWarns(t *testing.T) {
	ctx, logs := newObservedCtx()
	tr := &Tracer{Slow: time.Microsecond}

	startCtx := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "-- name: ListJobs :many\nSELECT 1"})
	time.Sleep(time.Millisecond)
	tr.TraceQueryEnd(startCtx, nil, pgx.TraceQueryEndData{})

	entries := logs.FilterMessage("slow database query").All()
	if len(entries) != 1 {
		t.Fatalf("want one slow-query warn, got %v", logs.AllUntimed())
	}
	e := entries[0]
	if e.Level != zapcore.WarnLevel {
		t.Fatalf("level = %v, want warn", e.Level)
	}
	m := e.ContextMap()
	if m["query"] != "ListJobs" || m["op"] != "db.query" {
		t.Fatalf("fields = %v", m)
	}
	if _, ok := m["duration"]; !ok {
		t.Fatalf("duration missing: %v", m)
	}
}

func TestTracer_NoRowsIsDebugOnly(t *testing.T) {
	ctx, logs := newObservedCtx()
	run(New(), ctx, "-- name: GetUser :one\nSELECT 1", pgx.ErrNoRows)

	all := logs.All()
	if len(all) != 1 {
		t.Fatalf("want one entry, got %v", logs.AllUntimed())
	}
	if all[0].Level != zapcore.DebugLevel {
		t.Fatalf("level = %v, want debug for ErrNoRows", all[0].Level)
	}
	if got := all[0].ContextMap()["query"]; got != "GetUser" {
		t.Fatalf("query = %v", got)
	}
}

func TestTracer_CanceledIsDebugOnly(t *testing.T) {
	ctx, logs := newObservedCtx()
	run(New(), ctx, "SELECT 1", context.Canceled)
	for _, e := range logs.All() {
		if e.Level > zapcore.DebugLevel {
			t.Fatalf("canceled query logged at %v", e.Level)
		}
	}
}

func TestTracer_PgErrorOmitsDetailValues(t *testing.T) {
	const email = "a@b.c"
	pgErr := &pgconn.PgError{
		Severity:       "ERROR",
		Code:           "42P01",
		Message:        `relation "users" does not exist`,
		Detail:         "Key (email)=(" + email + ") already exists.",
		TableName:      "users",
		ConstraintName: "users_email_key",
	}
	ctx, logs := newObservedCtx()
	run(New(), ctx, "-- name: CreateUser :one\nINSERT INTO users ...", fmt.Errorf("insert: %w", pgErr))

	entries := logs.FilterMessage("database query failed").All()
	if len(entries) != 1 {
		t.Fatalf("want one failure entry, got %v", logs.AllUntimed())
	}
	e := entries[0]
	if e.Level != zapcore.ErrorLevel {
		t.Fatalf("level = %v, want error", e.Level)
	}
	m := e.ContextMap()
	want := map[string]string{
		"pg_code":       "42P01",
		"pg_table":      "users",
		"pg_constraint": "users_email_key",
		"query":         "CreateUser",
	}
	for k, v := range want {
		if m[k] != v {
			t.Fatalf("%s = %v, want %q", k, m[k], v)
		}
	}

	// The row value in Detail must not reach any rendered field or the message.
	rendered := fmt.Sprintf("%s %v", e.Message, m)
	if strings.Contains(rendered, email) || strings.Contains(rendered, "Key (email)") {
		t.Fatalf("detail leaked into log: %s", rendered)
	}
}

func TestTracer_HandledConflictsLogAtWarnWithoutDetail(t *testing.T) {
	const email = "a@b.c"
	for _, code := range []string{"23505", "23503", "40001", "40P01"} {
		pgErr := &pgconn.PgError{
			Severity: "ERROR",
			Code:     code,
			Message:  "conflict",
			Detail:   "Key (email)=(" + email + ") already exists.",
		}
		ctx, logs := newObservedCtx()
		run(New(), ctx, "-- name: StartInterview :one\nINSERT ...", pgErr)

		if n := len(logs.FilterMessage("database query failed").All()); n != 0 {
			t.Fatalf("%s: handled conflict logged as a failure", code)
		}
		entries := logs.FilterMessage("database query conflict").All()
		if len(entries) != 1 || entries[0].Level != zapcore.WarnLevel {
			t.Fatalf("%s: want one warn conflict entry, got %v", code, logs.AllUntimed())
		}
		if entries[0].ContextMap()["pg_code"] != code {
			t.Fatalf("%s: pg_code missing: %v", code, entries[0].ContextMap())
		}
		if rendered := fmt.Sprintf("%s %v", entries[0].Message, entries[0].ContextMap()); strings.Contains(rendered, email) {
			t.Fatalf("%s: detail leaked: %s", code, rendered)
		}
	}
}

func TestTracer_GenericErrorLogsCauseWithoutPgFields(t *testing.T) {
	ctx, logs := newObservedCtx()
	run(New(), ctx, "-- name: GetJD :one\nSELECT 1", errors.New("conn reset"))

	entries := logs.FilterMessage("database query failed").All()
	if len(entries) != 1 {
		t.Fatalf("want one failure entry, got %v", logs.AllUntimed())
	}
	m := entries[0].ContextMap()
	if _, ok := m["pg_code"]; ok {
		t.Fatalf("non-pg error should not carry pg_code: %v", m)
	}
	if !strings.Contains(fmt.Sprint(m["cause_chain"]), "conn reset") {
		t.Fatalf("cause_chain missing error text: %v", m)
	}
}

func TestTracer_EndWithoutStartIsIgnored(t *testing.T) {
	ctx, logs := newObservedCtx()
	New().TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("x")})
	if n := logs.Len(); n != 0 {
		t.Fatalf("end without start logged %d entries", n)
	}
}
