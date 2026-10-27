package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func persistenceError(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "unable to load or save avatar inventory", err)
}

// Holding a shared user lock until commit serializes role/lock changes with writes.
func (r *Repository) beginOwner(ctx context.Context, owner uuid.UUID) (pgx.Tx, *Queries, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, persistenceError(err)
	}
	queries := New(tx)
	if _, err := queries.LockInventoryOwner(ctx, owner); err != nil {
		_ = tx.Rollback(context.Background())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, apperror.New(apperror.CodeForbidden, "account is no longer permitted to access a personal avatar inventory")
		}
		return nil, nil, persistenceError(err)
	}
	return tx, queries, nil
}

func (r *Repository) Summary(ctx context.Context, owner uuid.UUID) (domain.Capacity, error) {
	tx, queries, err := r.beginOwner(ctx, owner)
	if err != nil {
		return domain.Capacity{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := queries.EnsureInventory(ctx, EnsureInventoryParams{UserID: owner, InitialCapacity: domain.InitialCapacity}); err != nil {
		return domain.Capacity{}, persistenceError(err)
	}
	row, err := queries.GetInventoryCapacity(ctx, owner)
	if err != nil {
		return domain.Capacity{}, persistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Capacity{}, persistenceError(err)
	}
	return capacityOf(row), nil
}

func capacityOf(row GetInventoryCapacityRow) domain.Capacity {
	available := max(int64(row.Slot)-row.Used, 0)
	return domain.Capacity{Capacity: row.Slot, Used: row.Used, Available: available, IsFull: row.Used >= int64(row.Slot)}
}

func (r *Repository) Create(ctx context.Context, owner uuid.UUID, name, personality, modelPath string) (domain.Avatar, error) {
	tx, queries, err := r.beginOwner(ctx, owner)
	if err != nil {
		return domain.Avatar{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := queries.EnsureInventory(ctx, EnsureInventoryParams{UserID: owner, InitialCapacity: domain.InitialCapacity}); err != nil {
		return domain.Avatar{}, persistenceError(err)
	}
	if _, err := queries.LockAvatarInventory(ctx, owner); err != nil {
		return domain.Avatar{}, persistenceError(err)
	}
	row, err := queries.InsertInventoryAvatar(ctx, InsertInventoryAvatarParams{UserID: owner, Name: name, Personality: personality, ModelPath: modelPath})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Avatar{}, apperror.New(apperror.CodeInventoryFull, "avatar inventory is full")
		}
		return domain.Avatar{}, persistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Avatar{}, persistenceError(err)
	}
	return domain.Avatar{ID: row.ID, Name: row.Name, Personality: row.Personality, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, nil
}

func (r *Repository) List(ctx context.Context, owner uuid.UUID, limit, offset int32) ([]domain.Avatar, int64, error) {
	// Initialize at READ COMMITTED: ON CONFLICT waits for another initializer and
	// the next statement can see its row. Do not upsert inside REPEATABLE READ.
	tx, queries, err := r.beginOwner(ctx, owner)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := queries.EnsureInventory(ctx, EnsureInventoryParams{UserID: owner, InitialCapacity: domain.InitialCapacity}); err != nil {
		return nil, 0, persistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, persistenceError(err)
	}

	readTx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	defer func() { _ = readTx.Rollback(context.Background()) }()
	queries = New(readTx)
	total, err := queries.CountInventoryAvatars(ctx, owner)
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	rows, err := queries.ListInventoryAvatars(ctx, ListInventoryAvatarsParams{UserID: owner, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	items := make([]domain.Avatar, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.Avatar{ID: row.ID, Name: row.Name, Personality: row.Personality, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time})
	}
	if err := readTx.Commit(ctx); err != nil {
		return nil, 0, persistenceError(err)
	}
	return items, total, nil
}

func (r *Repository) Get(ctx context.Context, owner, id uuid.UUID) (domain.StoredAvatar, error) {
	tx, queries, err := r.beginOwner(ctx, owner)
	if err != nil {
		return domain.StoredAvatar{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	row, err := queries.GetInventoryAvatar(ctx, GetInventoryAvatarParams{OwnerID: owner, AvatarID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StoredAvatar{}, avatarNotFound()
	}
	if err != nil {
		return domain.StoredAvatar{}, persistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.StoredAvatar{}, persistenceError(err)
	}
	return domain.StoredAvatar{Avatar: domain.Avatar{ID: row.ID, Name: row.Name, Personality: row.Personality, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, ModelPath: row.ModelPath}, nil
}

// PurchaseSlot debits amount slots at PricePerSlot through the shared ledger and adds them to the
// inventory in one transaction. The inventory row is locked before the ledger locks the wallet row, so
// concurrent purchases queue up and each debits the balance it reads under the lock; a top-up is never
// overwritten. A missing wallet or a low balance is INSUFFICIENT_BALANCE and rolls everything back.
func (r *Repository) PurchaseSlot(ctx context.Context, owner uuid.UUID, amount int32) (domain.SlotPurchase, error) {
	tx, queries, err := r.beginOwner(ctx, owner)
	if err != nil {
		return domain.SlotPurchase{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := queries.EnsureInventory(ctx, EnsureInventoryParams{UserID: owner, InitialCapacity: domain.InitialCapacity}); err != nil {
		return domain.SlotPurchase{}, persistenceError(err)
	}
	if _, err := queries.LockAvatarInventory(ctx, owner); err != nil {
		return domain.SlotPurchase{}, persistenceError(err)
	}
	description := fmt.Sprintf("Avatar inventory, %d slots", amount)
	if err := ledger.Debit(ctx, tx, owner, amount*domain.PricePerSlot, description); err != nil {
		return domain.SlotPurchase{}, err
	}
	if err := queries.UpdateInventorySlot(ctx, UpdateInventorySlotParams{UserID: owner, Amount: amount}); err != nil {
		return domain.SlotPurchase{}, persistenceError(err)
	}
	row, err := queries.GetInventoryCapacity(ctx, owner)
	if err != nil {
		return domain.SlotPurchase{}, persistenceError(err)
	}
	balance, err := queries.GetWalletBalance(ctx, owner)
	if err != nil {
		return domain.SlotPurchase{}, persistenceError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SlotPurchase{}, persistenceError(err)
	}
	return domain.SlotPurchase{Capacity: capacityOf(row), Balance: balance}, nil
}

func (r *Repository) Delete(ctx context.Context, owner, id uuid.UUID) error {
	tx, queries, err := r.beginOwner(ctx, owner)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	count, err := queries.DeleteInventoryAvatar(ctx, DeleteInventoryAvatarParams{OwnerID: owner, AvatarID: id})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperror.New(apperror.CodeAvatarInUse, "avatar is used by a job posting or interview")
		}
		return persistenceError(err)
	}
	if count == 0 {
		return avatarNotFound()
	}
	if err := tx.Commit(ctx); err != nil {
		return persistenceError(err)
	}
	return nil
}

func avatarNotFound() error { return apperror.New(apperror.CodeAvatarNotFound, "avatar not found") }
