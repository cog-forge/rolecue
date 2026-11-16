// Package repository stores Avatar Marketplace listings. State changes are single guarded UPDATEs
// (compare-and-set on status and updated_at); the service diagnoses a miss.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func persistenceError(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "unable to load or save marketplace listings", err)
}

func stamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func optionalUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}

func optionalTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func statusArg(s *domain.Status) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func listingOf(l AvatarListing, assetStatus, sellerName string) domain.Listing {
	return domain.Listing{
		ID: l.ID, SellerID: l.SellerID, SellerName: sellerName, AssetID: l.AssetID, AssetStatus: assetStatus,
		Payload: domain.Payload{Title: l.Title, Description: l.Description, DefaultPersonality: l.DefaultPersonality,
			RightsNote: l.RightsNote, PriceCoins: l.PriceCoins},
		Status: domain.Status(l.Status), ReviewedBy: optionalUUID(l.ReviewedBy), ReviewedAt: optionalTime(l.ReviewedAt),
		ReviewNote: l.ReviewNote, CreatedAt: l.CreatedAt.Time, UpdatedAt: l.UpdatedAt.Time,
	}
}

// matched turns the RETURNING id of a guarded UPDATE into "did the guard match".
func matched(_ uuid.UUID, err error) (bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, persistenceError(err)
	}
	return true, nil
}

// EnsureUser fails with FORBIDDEN unless the account is a verified, unlocked Candidate or Recruiter.
func (r *Repository) EnsureUser(ctx context.Context, id uuid.UUID) error {
	if _, err := r.queries.LockMarketplaceUser(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperror.New(apperror.CodeForbidden, "account is not permitted to use the Marketplace")
		}
		return persistenceError(err)
	}
	return nil
}

// OwnedAvatarAsset describes the asset behind one of the caller's avatars.
type OwnedAvatarAsset struct {
	AssetID   uuid.UUID
	CreatedBy *uuid.UUID
	Status    string
}

func (r *Repository) OwnedAvatarAsset(ctx context.Context, avatar, owner uuid.UUID) (OwnedAvatarAsset, bool, error) {
	row, err := r.queries.GetOwnedAvatarAsset(ctx, GetOwnedAvatarAssetParams{ID: avatar, UserID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnedAvatarAsset{}, false, nil
	}
	if err != nil {
		return OwnedAvatarAsset{}, false, persistenceError(err)
	}
	return OwnedAvatarAsset{AssetID: row.AssetID, CreatedBy: optionalUUID(row.CreatedBy), Status: row.Status}, true, nil
}

// Create inserts a draft when the avatar is the seller's, its asset was created by the seller and is ready.
// found is false when that guard matched nothing; a second listing for one asset is MARKETPLACE_LISTING_EXISTS.
func (r *Repository) Create(ctx context.Context, seller, avatar uuid.UUID, p domain.Payload) (id uuid.UUID, found bool, err error) {
	id, err = r.queries.CreateListing(ctx, CreateListingParams{SellerID: seller, AvatarID: avatar, Title: p.Title,
		Description: p.Description, DefaultPersonality: p.DefaultPersonality, PriceCoins: p.PriceCoins, RightsNote: p.RightsNote})
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, nil
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return uuid.Nil, false, apperror.New(apperror.CodeMarketplaceListingExists, "this avatar already has a listing")
	case err != nil:
		return uuid.Nil, false, persistenceError(err)
	}
	return id, true, nil
}

func (r *Repository) EligibleAvatars(ctx context.Context, seller uuid.UUID) ([]domain.EligibleAvatar, error) {
	rows, err := r.queries.ListEligibleAvatars(ctx, seller)
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.EligibleAvatar, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.EligibleAvatar{AvatarID: row.AvatarID, AssetID: row.AssetID, Name: row.Name})
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Listing, bool, error) {
	row, err := r.queries.GetListing(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Listing{}, false, nil
	}
	if err != nil {
		return domain.Listing{}, false, persistenceError(err)
	}
	return listingOf(row.AvatarListing, row.AssetStatus, row.SellerName), true, nil
}

func (r *Repository) ListSeller(ctx context.Context, seller uuid.UUID, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error) {
	rows, err := r.queries.ListSellerListings(ctx, ListSellerListingsParams{SellerID: seller, Status: statusArg(status), Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountSellerListings(ctx, CountSellerListingsParams{SellerID: seller, Status: statusArg(status)})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.Listing, 0, len(rows))
	for _, row := range rows {
		out = append(out, listingOf(row.AvatarListing, row.AssetStatus, row.SellerName))
	}
	return out, total, nil
}

func (r *Repository) ListAdmin(ctx context.Context, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error) {
	rows, err := r.queries.ListAdminListings(ctx, ListAdminListingsParams{Status: statusArg(status), Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountAdminListings(ctx, statusArg(status))
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.Listing, 0, len(rows))
	for _, row := range rows {
		out = append(out, listingOf(row.AvatarListing, row.AssetStatus, row.SellerName))
	}
	return out, total, nil
}

// The guarded writes below report whether the guard matched; nothing is written when it did not.

func (r *Repository) Update(ctx context.Context, id, seller uuid.UUID, expected time.Time, p domain.Payload) (bool, error) {
	return matched(r.queries.UpdateListingPayload(ctx, UpdateListingPayloadParams{ID: id, SellerID: seller, ExpectedUpdatedAt: stamp(expected),
		Title: p.Title, Description: p.Description, DefaultPersonality: p.DefaultPersonality, PriceCoins: p.PriceCoins, RightsNote: p.RightsNote}))
}

func (r *Repository) Submit(ctx context.Context, id, seller uuid.UUID, expected time.Time) (bool, error) {
	return matched(r.queries.SubmitListing(ctx, SubmitListingParams{ID: id, SellerID: seller, ExpectedUpdatedAt: stamp(expected)}))
}

func (r *Repository) Delist(ctx context.Context, id, seller uuid.UUID, expected time.Time) (bool, error) {
	return matched(r.queries.DelistListing(ctx, DelistListingParams{ID: id, SellerID: seller, ExpectedUpdatedAt: stamp(expected)}))
}

func (r *Repository) Approve(ctx context.Context, id, admin uuid.UUID, expected time.Time) (bool, error) {
	return matched(r.queries.ApproveListing(ctx, ApproveListingParams{ID: id, AdminID: pgtype.UUID{Bytes: admin, Valid: true}, ExpectedUpdatedAt: stamp(expected)}))
}

func (r *Repository) Reject(ctx context.Context, id, admin uuid.UUID, expected time.Time, reason string) (bool, error) {
	return matched(r.queries.RejectListing(ctx, RejectListingParams{ID: id, AdminID: pgtype.UUID{Bytes: admin, Valid: true}, ExpectedUpdatedAt: stamp(expected), Reason: &reason}))
}

func (r *Repository) Suspend(ctx context.Context, id, admin uuid.UUID, reason string) (bool, error) {
	return matched(r.queries.SuspendListing(ctx, SuspendListingParams{ID: id, AdminID: pgtype.UUID{Bytes: admin, Valid: true}, Reason: &reason}))
}

func catalogItem(id, asset uuid.UUID, title string, desc *string, persona string, price int32, created pgtype.Timestamptz, seller string, own, owned bool) domain.CatalogItem {
	return domain.CatalogItem{ID: id, AssetID: asset, Title: title, Description: desc, DefaultPersonality: persona, PriceCoins: price,
		SellerName: seller, IsOwnListing: own, AlreadyOwned: owned, CreatedAt: created.Time}
}

// Catalog lists active listings of ready assets; pattern is an already escaped ILIKE pattern or nil.
func (r *Repository) Catalog(ctx context.Context, caller uuid.UUID, pattern *string, sort domain.CatalogSort, limit, offset int32) ([]domain.CatalogItem, int64, error) {
	total, err := r.queries.CountCatalog(ctx, pattern)
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	var out []domain.CatalogItem
	switch sort {
	case domain.SortPriceAsc:
		rows, qerr := r.queries.ListCatalogPriceAsc(ctx, ListCatalogPriceAscParams{CallerID: caller, Pattern: pattern, PageLimit: limit, PageOffset: offset})
		err = qerr
		for _, x := range rows {
			out = append(out, catalogItem(x.ID, x.AssetID, x.Title, x.Description, x.DefaultPersonality, x.PriceCoins, x.CreatedAt, x.SellerName, x.IsOwnListing, x.AlreadyOwned))
		}
	case domain.SortPriceDesc:
		rows, qerr := r.queries.ListCatalogPriceDesc(ctx, ListCatalogPriceDescParams{CallerID: caller, Pattern: pattern, PageLimit: limit, PageOffset: offset})
		err = qerr
		for _, x := range rows {
			out = append(out, catalogItem(x.ID, x.AssetID, x.Title, x.Description, x.DefaultPersonality, x.PriceCoins, x.CreatedAt, x.SellerName, x.IsOwnListing, x.AlreadyOwned))
		}
	default:
		rows, qerr := r.queries.ListCatalogNewest(ctx, ListCatalogNewestParams{CallerID: caller, Pattern: pattern, PageLimit: limit, PageOffset: offset})
		err = qerr
		for _, x := range rows {
			out = append(out, catalogItem(x.ID, x.AssetID, x.Title, x.Description, x.DefaultPersonality, x.PriceCoins, x.CreatedAt, x.SellerName, x.IsOwnListing, x.AlreadyOwned))
		}
	}
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	if out == nil {
		out = []domain.CatalogItem{}
	}
	return out, total, nil
}

func (r *Repository) CatalogItem(ctx context.Context, caller, id uuid.UUID) (domain.CatalogItem, bool, error) {
	x, err := r.queries.GetCatalogListing(ctx, GetCatalogListingParams{CallerID: caller, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CatalogItem{}, false, nil
	}
	if err != nil {
		return domain.CatalogItem{}, false, persistenceError(err)
	}
	return catalogItem(x.ID, x.AssetID, x.Title, x.Description, x.DefaultPersonality, x.PriceCoins, x.CreatedAt, x.SellerName, x.IsOwnListing, x.AlreadyOwned), true, nil
}
