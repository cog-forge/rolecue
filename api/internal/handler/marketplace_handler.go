package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	invdomain "github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MarketplaceService is the seller, admin and catalog use case set of the Avatar Marketplace.
type MarketplaceService interface {
	Create(ctx context.Context, seller, avatar uuid.UUID, p domain.Payload) (domain.Listing, error)
	EligibleAvatars(ctx context.Context, seller uuid.UUID) ([]domain.EligibleAvatar, error)
	MyListings(ctx context.Context, seller uuid.UUID, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error)
	MyListing(ctx context.Context, seller, id uuid.UUID) (domain.Listing, error)
	Edit(ctx context.Context, seller, id uuid.UUID, expected time.Time, p domain.Payload) (domain.Listing, error)
	Submit(ctx context.Context, seller, id uuid.UUID, expected time.Time) (domain.Listing, error)
	Delist(ctx context.Context, seller, id uuid.UUID, expected time.Time) (domain.Listing, error)
	AdminList(ctx context.Context, role string, status *domain.Status, limit, offset int32) ([]domain.Listing, int64, error)
	AdminGet(ctx context.Context, role string, id uuid.UUID) (domain.Listing, error)
	Approve(ctx context.Context, admin uuid.UUID, role string, id uuid.UUID, expected time.Time) (domain.Listing, error)
	Reject(ctx context.Context, admin uuid.UUID, role string, id uuid.UUID, expected time.Time, reason string) (domain.Listing, error)
	Suspend(ctx context.Context, admin uuid.UUID, role string, id uuid.UUID, reason string) (domain.Listing, error)
	Catalog(ctx context.Context, caller uuid.UUID, f domain.ListFilter) ([]domain.CatalogItem, int64, error)
	CatalogItem(ctx context.Context, caller, id uuid.UUID) (domain.CatalogItem, error)
	CatalogPreview(ctx context.Context, caller, id uuid.UUID) (string, error)
	MyListingPreview(ctx context.Context, seller, id uuid.UUID) (string, error)
	AdminPreview(ctx context.Context, role string, id uuid.UUID) (string, error)
	Purchase(ctx context.Context, buyer, listingID uuid.UUID, confirmedPrice int32, key string) (domain.PurchaseResult, error)
}

type MarketplaceHandler struct{ service MarketplaceService }

func NewMarketplaceHandler(service MarketplaceService) *MarketplaceHandler {
	return &MarketplaceHandler{service: service}
}

// SellerListingView is the seller's own listing. It never carries a model key or URL.
type SellerListingView struct {
	ID                 uuid.UUID  `json:"id"`
	AssetID            uuid.UUID  `json:"asset_id"`
	Status             string     `json:"status" enums:"draft,pending,active,rejected,delisted,suspended"`
	Title              string     `json:"title"`
	Description        *string    `json:"description"`
	DefaultPersonality string     `json:"default_personality"`
	PriceCoins         int32      `json:"price_coins"`
	RightsNote         string     `json:"rights_note"`
	ReviewNote         *string    `json:"review_note"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	PreviewURL         string     `json:"preview_url"`
	CanEdit            bool       `json:"can_edit"`
	CanSubmit          bool       `json:"can_submit"`
	CanDelist          bool       `json:"can_delist"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// reasonVisible reports whether review_note is shown to the seller: only the reason of a rejection or suspension.
func reasonVisible(s domain.Status) bool {
	return s == domain.StatusRejected || s == domain.StatusSuspended
}

func sellerListingView(l domain.Listing) SellerListingView {
	v := SellerListingView{
		ID: l.ID, AssetID: l.AssetID, Status: string(l.Status), Title: l.Title, Description: l.Description,
		DefaultPersonality: l.DefaultPersonality, PriceCoins: l.PriceCoins, RightsNote: l.RightsNote,
		PreviewURL: "/marketplace/my-listings/" + l.ID.String() + "/preview", CanEdit: l.Status.CanEdit(), CanSubmit: l.Status.CanSubmit(),
		CanDelist: l.Status.CanDelist(), CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
	if reasonVisible(l.Status) {
		v.ReviewNote, v.ReviewedAt = l.ReviewNote, l.ReviewedAt
	}
	return v
}

// SellerPublicView is the only seller information a buyer sees.
type SellerPublicView struct {
	DisplayName string `json:"display_name"`
}

// AdminListingView is the exact payload under review plus the review trail. No model key, no email.
type AdminListingView struct {
	ID                 uuid.UUID        `json:"id"`
	AssetID            uuid.UUID        `json:"asset_id"`
	AssetReady         bool             `json:"asset_ready"`
	Seller             SellerPublicView `json:"seller"`
	SellerID           uuid.UUID        `json:"seller_id"`
	Status             string           `json:"status" enums:"draft,pending,active,rejected,delisted,suspended"`
	Title              string           `json:"title"`
	Description        *string          `json:"description"`
	DefaultPersonality string           `json:"default_personality"`
	PriceCoins         int32            `json:"price_coins"`
	RightsNote         string           `json:"rights_note"`
	ReviewedBy         *uuid.UUID       `json:"reviewed_by"`
	ReviewedAt         *time.Time       `json:"reviewed_at"`
	ReviewNote         *string          `json:"review_note"`
	PreviewURL         string           `json:"preview_url"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

func adminListingView(l domain.Listing) AdminListingView {
	return AdminListingView{
		ID: l.ID, AssetID: l.AssetID, AssetReady: l.AssetStatus == "ready", Seller: SellerPublicView{DisplayName: l.SellerName},
		SellerID: l.SellerID, Status: string(l.Status), Title: l.Title, Description: l.Description,
		DefaultPersonality: l.DefaultPersonality, PriceCoins: l.PriceCoins, RightsNote: l.RightsNote, ReviewedBy: l.ReviewedBy,
		ReviewedAt: l.ReviewedAt, ReviewNote: l.ReviewNote, PreviewURL: "/admin/marketplace/listings/" + l.ID.String() + "/preview", CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

// CatalogItemView is what a buyer sees of an active listing: no rights_note, review data, model key or email.
type CatalogItemView struct {
	ID                 uuid.UUID        `json:"id"`
	Title              string           `json:"title"`
	Description        *string          `json:"description"`
	DefaultPersonality string           `json:"default_personality"`
	PriceCoins         int32            `json:"price_coins"`
	Seller             SellerPublicView `json:"seller"`
	AlreadyOwned       bool             `json:"already_owned"`
	IsOwnListing       bool             `json:"is_own_listing"`
	PreviewURL         string           `json:"preview_url"`
	CreatedAt          time.Time        `json:"created_at"`
}

func catalogItemView(i domain.CatalogItem) CatalogItemView {
	return CatalogItemView{ID: i.ID, Title: i.Title, Description: i.Description, DefaultPersonality: i.DefaultPersonality,
		PriceCoins: i.PriceCoins, Seller: SellerPublicView{DisplayName: i.SellerName}, AlreadyOwned: i.AlreadyOwned,
		IsOwnListing: i.IsOwnListing, PreviewURL: "/marketplace/listings/" + i.ID.String() + "/preview", CreatedAt: i.CreatedAt}
}

// EligibleAvatarView is an avatar the seller may list.
type EligibleAvatarView struct {
	AvatarID uuid.UUID `json:"avatar_id"`
	Name     string    `json:"name"`
}

// ListingPayloadRequest carries the fields a seller writes. seller and asset are not accepted.
type ListingPayloadRequest struct {
	Title              *string `json:"title" binding:"required"`
	Description        *string `json:"description"`
	DefaultPersonality *string `json:"default_personality" binding:"required"`
	PriceCoins         *int32  `json:"price_coins" binding:"required"`
	RightsNote         *string `json:"rights_note" binding:"required"`
}

func (r ListingPayloadRequest) payload() domain.Payload {
	return domain.Payload{Title: *r.Title, Description: r.Description, DefaultPersonality: *r.DefaultPersonality,
		PriceCoins: *r.PriceCoins, RightsNote: *r.RightsNote}
}

// CreateListingRequest starts a draft from one of the caller's own avatars.
type CreateListingRequest struct {
	AvatarID *uuid.UUID `json:"avatar_id" binding:"required"`
	ListingPayloadRequest
}

// EditListingRequest replaces the payload of a draft or rejected listing last seen at expected_updated_at.
type EditListingRequest struct {
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at" binding:"required"`
	ListingPayloadRequest
}

// ExpectedRequest carries the version (updated_at) the caller last saw.
type ExpectedRequest struct {
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at" binding:"required"`
}

// RejectListingRequest rejects the pending payload last seen at expected_updated_at.
type RejectListingRequest struct {
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at" binding:"required"`
	Reason            *string    `json:"reason" binding:"required"`
}

// SuspendListingRequest suspends an active listing.
type SuspendListingRequest struct {
	Reason *string `json:"reason" binding:"required"`
}

func marketplaceUser(c *gin.Context) (uuid.UUID, string, bool) { return moderationUser(c) }

func marketplaceID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid listing ID"))
		return uuid.Nil, false
	}
	return id, true
}

// marketplaceBody binds a strict JSON body; a missing required field is a validation error.
func marketplaceBody(c *gin.Context, target any, validate func() bool) bool {
	if !moderationJSON(c, target) {
		return false
	}
	if !validate() {
		response.Error(c, apperror.New(apperror.CodeValidation, "a required field is missing"))
		return false
	}
	return true
}

func (r ListingPayloadRequest) complete() bool {
	return r.Title != nil && r.DefaultPersonality != nil && r.PriceCoins != nil && r.RightsNote != nil
}

// marketplacePaging reads limit and offset (each at most once) with the Marketplace bounds.
func marketplacePaging(c *gin.Context) (int32, int32, bool) {
	limit, offset := int32(domain.DefaultPage), int32(0)
	for _, p := range []struct {
		name     string
		dest     *int32
		min, max int64
	}{{"limit", &limit, 1, domain.MaxPageSize}, {"offset", &offset, 0, domain.MaxOffset}} {
		raw, exists := c.GetQueryArray(p.name)
		if !exists {
			continue
		}
		v, err := strconv.ParseInt(raw[0], 10, 32)
		if len(raw) != 1 || err != nil || v < p.min || v > p.max {
			response.Error(c, apperror.New(apperror.CodeValidation, "invalid pagination parameters"))
			return 0, 0, false
		}
		*p.dest = int32(v)
	}
	return limit, offset, true
}

// marketplaceStatus reads an optional status filter; "all" or absent means no filter unless def is set.
func marketplaceStatus(c *gin.Context, def *domain.Status) (*domain.Status, bool) {
	raw, exists := c.GetQueryArray("status")
	if !exists {
		return def, true
	}
	if len(raw) != 1 {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid status filter"))
		return nil, false
	}
	if raw[0] == "all" {
		return nil, true
	}
	s := domain.Status(raw[0])
	if !s.Valid() {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid status filter"))
		return nil, false
	}
	return &s, true
}

func listingPage[V any, T any](items []T, total int64, limit, offset int32, view func(T) V) response.Page[V] {
	out := make([]V, 0, len(items))
	for _, i := range items {
		out = append(out, view(i))
	}
	return response.Page[V]{Items: out, Pagination: response.Pagination{Limit: limit, Offset: offset, Total: total}}
}

// ---- seller ----

// EligibleAvatars godoc
// @Summary List avatars the caller may sell
// @Description Candidate and Recruiter. Avatars in the caller's own inventory whose model the caller created, that are ready and have no listing yet.
// @Tags marketplace-seller
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=[]EligibleAvatarView}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /marketplace/eligible-avatars [get]
func (h *MarketplaceHandler) EligibleAvatars(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	items, err := h.service.EligibleAvatars(c.Request.Context(), user)
	if err != nil {
		response.Error(c, err)
		return
	}
	out := make([]EligibleAvatarView, 0, len(items))
	for _, i := range items {
		out = append(out, EligibleAvatarView{AvatarID: i.AvatarID, Name: i.Name})
	}
	response.OK(c, out)
}

// Create godoc
// @Summary Create a draft listing
// @Description Candidate and Recruiter. Trusted Origin required. Starts a draft from one of the caller's own avatars; seller_id is the session user and the asset is resolved from the avatar. The avatar's model must have been created by the caller and be ready (a bought copy can never be listed). One listing per asset: 409 MARKETPLACE_LISTING_EXISTS. 422 MARKETPLACE_ASSET_NOT_ELIGIBLE otherwise. Body limit 16 KiB.
// @Tags marketplace-seller
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body CreateListingRequest true "Avatar and listing payload"
// @Success 201 {object} response.Envelope{data=SellerListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 422 {object} response.Envelope
// @Router /marketplace/my-listings [post]
func (h *MarketplaceHandler) Create(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	var req CreateListingRequest
	if !marketplaceBody(c, &req, func() bool { return req.AvatarID != nil && req.complete() }) {
		return
	}
	l, err := h.service.Create(c.Request.Context(), user, *req.AvatarID, req.payload())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, sellerListingView(l))
}

// MyListings godoc
// @Summary List my listings
// @Description Candidate and Recruiter. The caller's own listings, most recently changed first. status filters one status; all or absent means every status. limit 1..50 (default 20), offset 0..10000.
// @Tags marketplace-seller
// @Produce json
// @Security SessionCookie
// @Param status query string false "draft, pending, active, rejected, delisted, suspended or all"
// @Param limit query int false "Page size" default(20) minimum(1) maximum(50)
// @Param offset query int false "Items to skip" default(0) minimum(0) maximum(10000)
// @Success 200 {object} response.Envelope{data=response.Page[SellerListingView]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /marketplace/my-listings [get]
func (h *MarketplaceHandler) MyListings(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	limit, offset, ok := marketplacePaging(c)
	if !ok {
		return
	}
	status, ok := marketplaceStatus(c, nil)
	if !ok {
		return
	}
	items, total, err := h.service.MyListings(c.Request.Context(), user, status, limit, offset)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, listingPage(items, total, limit, offset, sellerListingView))
}

// MyListing godoc
// @Summary Get my listing
// @Description Candidate and Recruiter. Another seller's listing is 404.
// @Tags marketplace-seller
// @Produce json
// @Security SessionCookie
// @Param id path string true "Listing ID" format(uuid)
// @Success 200 {object} response.Envelope{data=SellerListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /marketplace/my-listings/{id} [get]
func (h *MarketplaceHandler) MyListing(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	l, err := h.service.MyListing(c.Request.Context(), user, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sellerListingView(l))
}

// Edit godoc
// @Summary Edit a draft or rejected listing
// @Description Candidate and Recruiter. Trusted Origin required. Only draft and rejected listings are editable (409 MARKETPLACE_INVALID_TRANSITION otherwise); a rejected listing stays rejected until it is submitted again. expected_updated_at is the updated_at the caller loaded; any other current value is 409 MARKETPLACE_LISTING_STALE and nothing is written. Seller and asset cannot change.
// @Tags marketplace-seller
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body EditListingRequest true "New payload and the version it replaces"
// @Success 200 {object} response.Envelope{data=SellerListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /marketplace/my-listings/{id} [put]
func (h *MarketplaceHandler) Edit(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var req EditListingRequest
	if !marketplaceBody(c, &req, func() bool { return req.ExpectedUpdatedAt != nil && req.complete() }) {
		return
	}
	l, err := h.service.Edit(c.Request.Context(), user, id, *req.ExpectedUpdatedAt, req.payload())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sellerListingView(l))
}

// Submit godoc
// @Summary Submit or resubmit a listing for review
// @Description Candidate and Recruiter. Trusted Origin required. draft to pending, or rejected to pending after an edit made since the rejection. The payload is validated again and the avatar's model must still be ready and owned by the seller. Stale expected_updated_at is 409 MARKETPLACE_LISTING_STALE.
// @Tags marketplace-seller
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body ExpectedRequest true "The version being submitted"
// @Success 200 {object} response.Envelope{data=SellerListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 422 {object} response.Envelope
// @Router /marketplace/my-listings/{id}/submit [post]
func (h *MarketplaceHandler) Submit(c *gin.Context) {
	h.sellerAction(c, h.service.Submit)
}

// Delist godoc
// @Summary Delist an active listing
// @Description Candidate and Recruiter, owner only. Trusted Origin required. active to delisted stops new sales; buyers' avatars, the asset and files are untouched. Delisting a delisted listing succeeds without a change. There is no relist.
// @Tags marketplace-seller
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body ExpectedRequest true "The version being delisted"
// @Success 200 {object} response.Envelope{data=SellerListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /marketplace/my-listings/{id}/delist [post]
func (h *MarketplaceHandler) Delist(c *gin.Context) {
	h.sellerAction(c, h.service.Delist)
}

func (h *MarketplaceHandler) sellerAction(c *gin.Context, act func(context.Context, uuid.UUID, uuid.UUID, time.Time) (domain.Listing, error)) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var req ExpectedRequest
	if !marketplaceBody(c, &req, func() bool { return req.ExpectedUpdatedAt != nil }) {
		return
	}
	l, err := act(c.Request.Context(), user, id, *req.ExpectedUpdatedAt)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, sellerListingView(l))
}

// ---- admin ----

// AdminList godoc
// @Summary List Marketplace listings for review
// @Description Admin only. Oldest change first. Without status the pending queue; status=all removes the filter. limit 1..50 (default 20), offset 0..10000.
// @Tags admin-marketplace
// @Produce json
// @Security SessionCookie
// @Param status query string false "pending (default), draft, active, rejected, delisted, suspended or all"
// @Param limit query int false "Page size" default(20) minimum(1) maximum(50)
// @Param offset query int false "Items to skip" default(0) minimum(0) maximum(10000)
// @Success 200 {object} response.Envelope{data=response.Page[AdminListingView]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /admin/marketplace/listings [get]
func (h *MarketplaceHandler) AdminList(c *gin.Context) {
	_, role, ok := marketplaceUser(c)
	if !ok {
		return
	}
	limit, offset, ok := marketplacePaging(c)
	if !ok {
		return
	}
	pending := domain.StatusPending
	status, ok := marketplaceStatus(c, &pending)
	if !ok {
		return
	}
	items, total, err := h.service.AdminList(c.Request.Context(), role, status, limit, offset)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, listingPage(items, total, limit, offset, adminListingView))
}

// AdminGet godoc
// @Summary Get a listing under review
// @Description Admin only. The exact payload (including rights_note), seller display name, asset readiness and review trail. updated_at is the version to send back to approve or reject.
// @Tags admin-marketplace
// @Produce json
// @Security SessionCookie
// @Param id path string true "Listing ID" format(uuid)
// @Success 200 {object} response.Envelope{data=AdminListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /admin/marketplace/listings/{id} [get]
func (h *MarketplaceHandler) AdminGet(c *gin.Context) {
	_, role, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	l, err := h.service.AdminGet(c.Request.Context(), role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, adminListingView(l))
}

// Approve godoc
// @Summary Approve a pending listing
// @Description Admin only. Trusted Origin required. pending to active, recording reviewer and time. Review is by exact payload: expected_updated_at must equal the listing's current updated_at, else 409 MARKETPLACE_LISTING_STALE and nothing changes. A non-pending listing is 409 MARKETPLACE_INVALID_TRANSITION (a repeat of an approve is therefore a conflict), a model that is not ready is 409 MARKETPLACE_ASSET_NOT_READY.
// @Tags admin-marketplace
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body ExpectedRequest true "The reviewed version"
// @Success 200 {object} response.Envelope{data=AdminListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /admin/marketplace/listings/{id}/approve [post]
func (h *MarketplaceHandler) Approve(c *gin.Context) {
	admin, role, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var req ExpectedRequest
	if !marketplaceBody(c, &req, func() bool { return req.ExpectedUpdatedAt != nil }) {
		return
	}
	l, err := h.service.Approve(c.Request.Context(), admin, role, id, *req.ExpectedUpdatedAt)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, adminListingView(l))
}

// Reject godoc
// @Summary Reject a pending listing
// @Description Admin only. Trusted Origin required. pending to rejected with a required reason (1..2000 characters), recording reviewer and time. Same exact-payload rule as approve. The seller may edit and submit again.
// @Tags admin-marketplace
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body RejectListingRequest true "The reviewed version and the reason"
// @Success 200 {object} response.Envelope{data=AdminListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /admin/marketplace/listings/{id}/reject [post]
func (h *MarketplaceHandler) Reject(c *gin.Context) {
	admin, role, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var req RejectListingRequest
	if !marketplaceBody(c, &req, func() bool { return req.ExpectedUpdatedAt != nil && req.Reason != nil }) {
		return
	}
	l, err := h.service.Reject(c.Request.Context(), admin, role, id, *req.ExpectedUpdatedAt, *req.Reason)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, adminListingView(l))
}

// Suspend godoc
// @Summary Suspend an active listing
// @Description Admin only. Trusted Origin required. active to suspended with a required reason; blocks new purchases only. Buyers' avatars, the asset and files are untouched and the Admin receives nothing. Suspending a suspended listing succeeds without a change (the first reason stays). No individual revoke or refund exists. There is no unsuspend in this release.
// @Tags admin-marketplace
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body SuspendListingRequest true "The reason"
// @Success 200 {object} response.Envelope{data=AdminListingView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /admin/marketplace/listings/{id}/suspend [post]
func (h *MarketplaceHandler) Suspend(c *gin.Context) {
	admin, role, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	var req SuspendListingRequest
	if !marketplaceBody(c, &req, func() bool { return req.Reason != nil }) {
		return
	}
	l, err := h.service.Suspend(c.Request.Context(), admin, role, id, *req.Reason)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, adminListingView(l))
}

// ---- catalog ----

// Catalog godoc
// @Summary Browse the Marketplace
// @Description Candidate and Recruiter. Active listings whose model is ready, nothing else. q searches the title (at most 100 characters, case-insensitive contains); sort is newest (default), price_asc or price_desc; limit 1..50 (default 20), offset 0..10000. already_owned is true when the caller has an avatar of the same model in their inventory; is_own_listing when the caller is the seller. preview_key is previews/<asset_id>.webp by convention and never a model URL.
// @Tags marketplace-catalog
// @Produce json
// @Security SessionCookie
// @Param q query string false "Title search" maxLength(100)
// @Param sort query string false "newest, price_asc or price_desc" default(newest)
// @Param limit query int false "Page size" default(20) minimum(1) maximum(50)
// @Param offset query int false "Items to skip" default(0) minimum(0) maximum(10000)
// @Success 200 {object} response.Envelope{data=response.Page[CatalogItemView]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Router /marketplace/listings [get]
func (h *MarketplaceHandler) Catalog(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	limit, offset, ok := marketplacePaging(c)
	if !ok {
		return
	}
	f := domain.ListFilter{Query: c.Query("q"), Sort: domain.CatalogSort(c.DefaultQuery("sort", string(domain.SortNewest))), Limit: limit, Offset: offset}
	items, total, err := h.service.Catalog(c.Request.Context(), user, f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, listingPage(items, total, limit, offset, catalogItemView))
}

// CatalogItem godoc
// @Summary Get an active listing
// @Description Candidate and Recruiter. Any listing that is not active (or whose model is not ready) is 404, whoever asks.
// @Tags marketplace-catalog
// @Produce json
// @Security SessionCookie
// @Param id path string true "Listing ID" format(uuid)
// @Success 200 {object} response.Envelope{data=CatalogItemView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /marketplace/listings/{id} [get]
func (h *MarketplaceHandler) CatalogItem(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	item, err := h.service.CatalogItem(c.Request.Context(), user, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, catalogItemView(item))
}

// previewRedirect answers a preview request with a 302 to a short-lived signed GET of previews/<asset_id>.webp.
// The browser follows it from an <img> tag. It does not check that the object exists: the UI shows its default
// image when loading fails. The model file is never signed here.
func previewRedirect(c *gin.Context, url string, err error) {
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=60")
	c.Redirect(302, url)
}

// CatalogPreview godoc
// @Summary Preview image of an active listing
// @Description Candidate and Recruiter. 302 to a signed GET (valid 5 minutes) of the listing's preview image, usable directly as an img src. Only active listings of a ready asset; anything else is 404. The object may not exist yet, so the UI must fall back to its default image on load failure. The model file is never signed.
// @Tags marketplace-catalog
// @Param id path string true "Listing ID" format(uuid)
// @Success 302
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /marketplace/listings/{id}/preview [get]
func (h *MarketplaceHandler) CatalogPreview(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	url, err := h.service.CatalogPreview(c.Request.Context(), user, id)
	previewRedirect(c, url, err)
}

// MyListingPreview godoc
// @Summary Preview image of my listing
// @Description Candidate and Recruiter, owner only. Same 302 as the catalog preview, for any status of the caller's own listing.
// @Tags marketplace-seller
// @Param id path string true "Listing ID" format(uuid)
// @Success 302
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /marketplace/my-listings/{id}/preview [get]
func (h *MarketplaceHandler) MyListingPreview(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	url, err := h.service.MyListingPreview(c.Request.Context(), user, id)
	previewRedirect(c, url, err)
}

// AdminPreview godoc
// @Summary Preview image of a listing under review
// @Description Admin only. Same 302 as the catalog preview, for a listing in any status.
// @Tags admin-marketplace
// @Param id path string true "Listing ID" format(uuid)
// @Success 302
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /admin/marketplace/listings/{id}/preview [get]
func (h *MarketplaceHandler) AdminPreview(c *gin.Context) {
	_, role, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	url, err := h.service.AdminPreview(c.Request.Context(), role, id)
	previewRedirect(c, url, err)
}

// PurchaseRequest confirms the price the buyer saw. The listing comes from the path, the buyer from the session,
// the idempotency key from the header; none of them is accepted in the body.
type PurchaseRequest struct {
	PriceCoins *int32 `json:"price_coins" binding:"required"`
}

// PurchaseTransactionView is the receipt of the coin transfer. On a replay it is the row of the first success.
type PurchaseTransactionView struct {
	ID        uuid.UUID `json:"id"`
	Amount    int32     `json:"amount"`
	Currency  string    `json:"currency" enums:"COIN"`
	CreatedAt time.Time `json:"created_at"`
}

// PurchaseView is the checkout response. avatar uses the inventory avatar DTO (no model key, no URL) and is null
// only on a replay after the buyer deleted the avatar. balance is the buyer's coin balance after the call.
type PurchaseView struct {
	Avatar      *invdomain.Avatar       `json:"avatar"`
	Transaction PurchaseTransactionView `json:"transaction"`
	Balance     int32                   `json:"balance"`
	Replayed    bool                    `json:"replayed"`
}

func purchaseView(r domain.PurchaseResult) PurchaseView {
	v := PurchaseView{
		Transaction: PurchaseTransactionView{ID: r.Receipt.TransactionID, Amount: r.Receipt.Amount, Currency: r.Receipt.Currency, CreatedAt: r.Receipt.CreatedAt},
		Balance:     r.Balance, Replayed: r.Replayed,
	}
	if r.Avatar != nil {
		v.Avatar = &invdomain.Avatar{ID: r.Avatar.ID, Name: r.Avatar.Name, Personality: r.Avatar.Personality, CreatedAt: r.Avatar.CreatedAt, UpdatedAt: r.Avatar.UpdatedAt}
	}
	return v
}

// Purchase godoc
// @Summary Buy an avatar from the Marketplace
// @Description Candidate and Recruiter. Trusted Origin required. Buys one active listing with coins (full price to the seller, no fee) and puts an owned copy of the avatar in the caller's inventory, in one transaction. The Idempotency-Key header is required and is `mkt1:<listing id>:<price>:<nonce>`: the listing id and price must equal the path and the body price_coins, the nonce (16..64 of A-Z a-z 0-9 _ -) is generated once per purchase intent by the client and reused for retries of that intent. A repeat with the same key returns the original receipt with replayed=true and never charges or grants again, even after a suspend, a delist or the buyer deleting the avatar (the avatar is then null and is not restored). To buy again after deleting, use a new nonce. See api/docs/marketplace-checkout-api.md.
// @Tags marketplace-catalog
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param Idempotency-Key header string true "mkt1:<listing id>:<price>:<nonce>"
// @Param id path string true "Listing ID" format(uuid)
// @Param request body PurchaseRequest true "The price the buyer saw"
// @Success 200 {object} response.Envelope{data=PurchaseView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 422 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /marketplace/listings/{id}/purchase [post]
func (h *MarketplaceHandler) Purchase(c *gin.Context) {
	user, _, ok := marketplaceUser(c)
	if !ok {
		return
	}
	id, ok := marketplaceID(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	var req PurchaseRequest
	if !marketplaceBody(c, &req, func() bool { return req.PriceCoins != nil }) {
		return
	}
	result, err := h.service.Purchase(c.Request.Context(), user, id, *req.PriceCoins, key)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, purchaseView(result))
}
