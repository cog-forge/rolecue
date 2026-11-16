// Package service holds the Avatar Marketplace seller, admin and catalog use cases. Every write is one guarded
// UPDATE in the repository; when it matches nothing this package reads the row again to say why.
package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/repository"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type Repository interface {
	EnsureUser(ctx context.Context, id uuid.UUID) error
	OwnedAvatarAsset(ctx context.Context, avatar, owner uuid.UUID) (repository.OwnedAvatarAsset, bool, error)
	Create(ctx context.Context, seller, avatar uuid.UUID, p domain.Payload) (uuid.UUID, bool, error)
	EligibleAvatars(ctx context.Context, seller uuid.UUID) ([]domain.EligibleAvatar, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Listing, bool, error)
	ListSeller(ctx context.Context, seller uuid.UUID, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error)
	ListAdmin(ctx context.Context, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error)
	Update(ctx context.Context, id, seller uuid.UUID, expected time.Time, p domain.Payload) (bool, error)
	Submit(ctx context.Context, id, seller uuid.UUID, expected time.Time) (bool, error)
	Delist(ctx context.Context, id, seller uuid.UUID, expected time.Time) (bool, error)
	Approve(ctx context.Context, id, admin uuid.UUID, expected time.Time) (bool, error)
	Reject(ctx context.Context, id, admin uuid.UUID, expected time.Time, reason string) (bool, error)
	Suspend(ctx context.Context, id, admin uuid.UUID, reason string) (bool, error)
	Catalog(ctx context.Context, caller uuid.UUID, pattern *string, sort domain.CatalogSort, limit, offset int32) ([]domain.CatalogItem, int64, error)
	CatalogItem(ctx context.Context, caller, id uuid.UUID) (domain.CatalogItem, bool, error)
}

// Signer signs a short-lived GET for one storage key (storage.Store satisfies it).
type Signer interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (storage.SignedRequest, error)
}

type Service struct {
	repo   Repository
	signer Signer
}

func New(repo Repository, signer Signer) *Service { return &Service{repo: repo, signer: signer} }

// PreviewTTL is how long a preview link stays valid. Previews are public by design; the key is always
// previews/<asset_id>.webp, built here from a listing the caller may see, never taken from the client.
const PreviewTTL = 5 * time.Minute

func (s *Service) signPreview(ctx context.Context, asset uuid.UUID) (string, error) {
	grant, err := s.signer.PresignGet(ctx, domain.PreviewKey(asset), PreviewTTL)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeInternal, "unable to sign the preview", err)
	}
	return grant.URL, nil
}

// CatalogPreview signs the preview of an active listing of a ready asset (anything else is not found).
func (s *Service) CatalogPreview(ctx context.Context, caller, id uuid.UUID) (string, error) {
	item, err := s.CatalogItem(ctx, caller, id)
	if err != nil {
		return "", err
	}
	return s.signPreview(ctx, item.AssetID)
}

// MyListingPreview signs the preview of one of the seller's own listings, whatever its status.
func (s *Service) MyListingPreview(ctx context.Context, seller, id uuid.UUID) (string, error) {
	l, err := s.MyListing(ctx, seller, id)
	if err != nil {
		return "", err
	}
	return s.signPreview(ctx, l.AssetID)
}

// AdminPreview signs the preview of any listing for an Admin.
func (s *Service) AdminPreview(ctx context.Context, role string, id uuid.UUID) (string, error) {
	l, err := s.AdminGet(ctx, role, id)
	if err != nil {
		return "", err
	}
	return s.signPreview(ctx, l.AssetID)
}

func notFound() error {
	return apperror.New(apperror.CodeMarketplaceListingNotFound, "listing not found")
}
func stale() error {
	return apperror.New(apperror.CodeMarketplaceListingStale, "the listing changed since it was loaded; reload and try again")
}
func illegal(msg string) error { return apperror.New(apperror.CodeMarketplaceInvalidTransition, msg) }

func (s *Service) requireAdmin(role string) error {
	if role != "admin" {
		return apperror.New(apperror.CodeForbidden, "you do not have access to this resource")
	}
	return nil
}

// ---- seller (KAN-363) ----

// own loads a listing the seller owns. Someone else's listing is reported as not found.
func (s *Service) own(ctx context.Context, seller, id uuid.UUID) (domain.Listing, error) {
	l, ok, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok || l.SellerID != seller {
		return domain.Listing{}, notFound()
	}
	return l, nil
}

// Create starts a draft from one of the seller's avatars. seller_id always comes from the session; the asset
// is resolved from the avatar so the client never names a seller or an asset.
func (s *Service) Create(ctx context.Context, seller, avatar uuid.UUID, p domain.Payload) (domain.Listing, error) {
	p, err := p.Normalize()
	if err != nil {
		return domain.Listing{}, err
	}
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return domain.Listing{}, err
	}
	id, ok, err := s.repo.Create(ctx, seller, avatar, p)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok {
		return domain.Listing{}, s.whyNotEligible(ctx, seller, avatar)
	}
	return s.own(ctx, seller, id)
}

func (s *Service) whyNotEligible(ctx context.Context, seller, avatar uuid.UUID) error {
	asset, found, err := s.repo.OwnedAvatarAsset(ctx, avatar, seller)
	if err != nil {
		return err
	}
	if found && asset.CreatedBy != nil && *asset.CreatedBy == seller && asset.Status != "ready" {
		return apperror.New(apperror.CodeMarketplaceAssetNotReady, "the avatar's model is not ready to be sold")
	}
	return apperror.New(apperror.CodeMarketplaceAssetNotEligible, "only an avatar you created and still own can be listed")
}

func (s *Service) EligibleAvatars(ctx context.Context, seller uuid.UUID) ([]domain.EligibleAvatar, error) {
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return nil, err
	}
	return s.repo.EligibleAvatars(ctx, seller)
}

func page(limit, offset int32) error {
	if limit < 1 || limit > domain.MaxPageSize || offset < 0 || offset > domain.MaxOffset {
		return apperror.New(apperror.CodeValidation, "invalid pagination parameters")
	}
	return nil
}

func (s *Service) MyListings(ctx context.Context, seller uuid.UUID, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error) {
	if err := page(limit, offset); err != nil {
		return nil, 0, err
	}
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return nil, 0, err
	}
	return s.repo.ListSeller(ctx, seller, status, limit, offset)
}

func (s *Service) MyListing(ctx context.Context, seller, id uuid.UUID) (domain.Listing, error) {
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return domain.Listing{}, err
	}
	return s.own(ctx, seller, id)
}

// Edit rewrites the payload of a draft or rejected listing the caller last saw at expected.
func (s *Service) Edit(ctx context.Context, seller, id uuid.UUID, expected time.Time, p domain.Payload) (domain.Listing, error) {
	p, err := p.Normalize()
	if err != nil {
		return domain.Listing{}, err
	}
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return domain.Listing{}, err
	}
	ok, err := s.repo.Update(ctx, id, seller, expected, p)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok {
		return domain.Listing{}, s.diagnoseSeller(ctx, seller, id, expected, domain.Status.CanEdit, "only a draft or rejected listing can be edited")
	}
	return s.own(ctx, seller, id)
}

func (s *Service) Submit(ctx context.Context, seller, id uuid.UUID, expected time.Time) (domain.Listing, error) {
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return domain.Listing{}, err
	}
	ok, err := s.repo.Submit(ctx, id, seller, expected)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok {
		return domain.Listing{}, s.diagnoseSeller(ctx, seller, id, expected, domain.Status.CanSubmit, "only a draft or rejected listing can be submitted")
	}
	return s.own(ctx, seller, id)
}

// Delist stops new sales of an active listing. Delisting a delisted listing succeeds without a write.
func (s *Service) Delist(ctx context.Context, seller, id uuid.UUID, expected time.Time) (domain.Listing, error) {
	if err := s.repo.EnsureUser(ctx, seller); err != nil {
		return domain.Listing{}, err
	}
	ok, err := s.repo.Delist(ctx, id, seller, expected)
	if err != nil {
		return domain.Listing{}, err
	}
	if ok {
		return s.own(ctx, seller, id)
	}
	l, err := s.own(ctx, seller, id)
	if err != nil {
		return domain.Listing{}, err
	}
	if l.Status == domain.StatusDelisted {
		return l, nil
	}
	return domain.Listing{}, s.diagnoseSeller(ctx, seller, id, expected, domain.Status.CanDelist, "only an active listing can be delisted")
}

// diagnoseSeller explains a guarded seller write that matched no row, in the order: not mine, wrong state,
// stale, then the conditions of a submit (edit required after a rejection, asset still sold by its creator).
func (s *Service) diagnoseSeller(ctx context.Context, seller, id uuid.UUID, expected time.Time, allowed func(domain.Status) bool, wrongState string) error {
	l, err := s.own(ctx, seller, id)
	if err != nil {
		return err
	}
	switch {
	case !allowed(l.Status):
		return illegal(wrongState)
	case !l.UpdatedAt.Equal(expected):
		return stale()
	case l.Status == domain.StatusRejected && l.ReviewedAt != nil && !l.UpdatedAt.After(*l.ReviewedAt):
		return illegal("edit the rejected listing before submitting it again")
	case l.AssetStatus != "ready":
		return apperror.New(apperror.CodeMarketplaceAssetNotReady, "the avatar's model is not ready to be sold")
	}
	return apperror.New(apperror.CodeMarketplaceAssetNotEligible, "only an avatar you created and still own can be listed")
}

// ---- admin (KAN-365) ----

func (s *Service) AdminList(ctx context.Context, role string, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error) {
	if err := s.requireAdmin(role); err != nil {
		return nil, 0, err
	}
	if err := page(limit, offset); err != nil {
		return nil, 0, err
	}
	return s.repo.ListAdmin(ctx, status, limit, offset)
}

func (s *Service) AdminGet(ctx context.Context, role string, id uuid.UUID) (domain.Listing, error) {
	if err := s.requireAdmin(role); err != nil {
		return domain.Listing{}, err
	}
	l, ok, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok {
		return domain.Listing{}, notFound()
	}
	return l, nil
}

// Approve publishes the pending listing whose updated_at the Admin loaded. Anything the seller changed since
// (reject, edit, resubmit) makes the request stale and nothing is approved.
func (s *Service) Approve(ctx context.Context, admin uuid.UUID, role string, id uuid.UUID, expected time.Time) (domain.Listing, error) {
	if err := s.requireAdmin(role); err != nil {
		return domain.Listing{}, err
	}
	ok, err := s.repo.Approve(ctx, id, admin, expected)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok {
		return domain.Listing{}, s.diagnoseReview(ctx, id, expected, domain.Status.CanApprove, "only a pending listing can be approved")
	}
	return s.AdminGet(ctx, role, id)
}

func (s *Service) Reject(ctx context.Context, admin uuid.UUID, role string, id uuid.UUID, expected time.Time, reason string) (domain.Listing, error) {
	if err := s.requireAdmin(role); err != nil {
		return domain.Listing{}, err
	}
	reason, err := domain.NormalizeReason(reason)
	if err != nil {
		return domain.Listing{}, err
	}
	ok, err := s.repo.Reject(ctx, id, admin, expected, reason)
	if err != nil {
		return domain.Listing{}, err
	}
	if !ok {
		return domain.Listing{}, s.diagnoseReview(ctx, id, expected, domain.Status.CanReject, "only a pending listing can be rejected")
	}
	return s.AdminGet(ctx, role, id)
}

// Suspend blocks new purchases of an active listing. Suspending a suspended listing succeeds without a write
// (the first reason is kept). Buyers' avatars, the asset and its files are untouched.
func (s *Service) Suspend(ctx context.Context, admin uuid.UUID, role string, id uuid.UUID, reason string) (domain.Listing, error) {
	if err := s.requireAdmin(role); err != nil {
		return domain.Listing{}, err
	}
	reason, err := domain.NormalizeReason(reason)
	if err != nil {
		return domain.Listing{}, err
	}
	ok, err := s.repo.Suspend(ctx, id, admin, reason)
	if err != nil {
		return domain.Listing{}, err
	}
	l, err := s.AdminGet(ctx, role, id)
	if err != nil {
		return domain.Listing{}, err
	}
	if ok || l.Status == domain.StatusSuspended {
		return l, nil
	}
	return domain.Listing{}, illegal("only an active listing can be suspended")
}

func (s *Service) diagnoseReview(ctx context.Context, id uuid.UUID, expected time.Time, allowed func(domain.Status) bool, wrongState string) error {
	l, ok, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	switch {
	case !ok:
		return notFound()
	case !allowed(l.Status):
		return illegal(wrongState)
	case !l.UpdatedAt.Equal(expected):
		return stale()
	case l.AssetStatus != "ready":
		return apperror.New(apperror.CodeMarketplaceAssetNotReady, "the listing's model is not ready to be sold")
	}
	return stale()
}

// ---- catalog (KAN-367) ----

// likePattern turns a user query into an ILIKE contains-pattern with LIKE wildcards escaped.
func likePattern(q string) *string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	p := "%" + q + "%"
	return &p
}

func (s *Service) Catalog(ctx context.Context, caller uuid.UUID, f domain.ListFilter) ([]domain.CatalogItem, int64, error) {
	if err := page(f.Limit, f.Offset); err != nil {
		return nil, 0, err
	}
	if utf8.RuneCountInString(strings.TrimSpace(f.Query)) > domain.MaxSearchQuery {
		return nil, 0, apperror.New(apperror.CodeValidation, "search query is too long")
	}
	switch f.Sort {
	case "", domain.SortNewest, domain.SortPriceAsc, domain.SortPriceDesc:
	default:
		return nil, 0, apperror.New(apperror.CodeValidation, "invalid sort")
	}
	if err := s.repo.EnsureUser(ctx, caller); err != nil {
		return nil, 0, err
	}
	return s.repo.Catalog(ctx, caller, likePattern(f.Query), f.Sort, f.Limit, f.Offset)
}

func (s *Service) CatalogItem(ctx context.Context, caller, id uuid.UUID) (domain.CatalogItem, error) {
	if err := s.repo.EnsureUser(ctx, caller); err != nil {
		return domain.CatalogItem{}, err
	}
	item, ok, err := s.repo.CatalogItem(ctx, caller, id)
	if err != nil {
		return domain.CatalogItem{}, err
	}
	if !ok {
		return domain.CatalogItem{}, notFound()
	}
	return item, nil
}
