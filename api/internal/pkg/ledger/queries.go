package ledger

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/pkg/ledger/repository"
	"github.com/jackc/pgx/v5"
)

// queryDB lets the ledger's own DBTX (Exec and QueryRow only, so callers keep passing whatever they
// already pass) satisfy the interface sqlc generates, which also lists Query. The ledger has no
// multi-row query, so Query is never reached and fails loudly if one is ever added.
type queryDB struct{ DBTX }

func (queryDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("ledger: the DBTX adapter has no multi-row queries")
}

// queries binds the generated queries to the caller's transaction.
func queries(db DBTX) *repository.Queries { return repository.New(queryDB{db}) }

// textPtr passes a Go string to a nullable text column. The value is never NULL, empty included.
func textPtr(s string) *string { return &s }
