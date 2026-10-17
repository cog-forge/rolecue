package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) (domain.List, error) {
	role := ""
	if filter.Filter != "all" && filter.Filter != "locked" {
		role = filter.Filter
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(filter.Search) + "%"
	row, err := r.queries.ListManagedAccounts(ctx, ListManagedAccountsParams{
		Search: filter.Search, Pattern: pattern, RoleFilter: role, LockedOnly: filter.Filter == "locked",
		Now: pgtype.Timestamptz{Time: time.Now(), Valid: true}, PageLimit: filter.Limit, PageOffset: filter.Offset,
	})
	if err != nil {
		return domain.List{}, persistenceError(err)
	}
	result := domain.List{Total: row.Total}
	if err := json.Unmarshal(row.Users, &result.Users); err != nil {
		return domain.List{}, persistenceError(err)
	}
	return result, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return getAccount(ctx, r.queries, id)
}

// The transaction coordinates requests; Better Auth owns the account write and session revocation.
func (r *Repository) WithAccountLock(ctx context.Context, id uuid.UUID, operation func(domain.Account) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return persistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '10s'"); err != nil {
		return persistenceError(err)
	}
	queries := New(tx)
	if err := queries.AcquireAccountRequestLock(ctx, "rolecue-account-lock:"+id.String()); err != nil {
		return persistenceError(err)
	}
	account, err := getAccount(ctx, queries, id)
	if err != nil {
		return err
	}
	if err := operation(account); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return persistenceError(err)
	}
	return nil
}

func getAccount(ctx context.Context, queries *Queries, id uuid.UUID) (domain.Account, error) {
	data, err := queries.GetManagedAccount(ctx, id)
	if err != nil {
		return domain.Account{}, persistenceError(err)
	}
	var account domain.Account
	if err := json.Unmarshal(data, &account); err != nil {
		return domain.Account{}, persistenceError(err)
	}
	return account, nil
}

func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeAccountNotFound, "account not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "account persistence failed", err)
}
