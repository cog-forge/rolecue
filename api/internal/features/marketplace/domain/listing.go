// Package domain holds the Avatar Marketplace listing model, its state machine and payload validation
// (KAN-363, KAN-365, KAN-367). Everything here is pure; storage and authorisation live elsewhere.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Limits mirror the CHECK constraints of migration 000036; validation fails first so a client never sees a
// raw constraint error.
const (
	MaxTitle       = 100
	MaxText        = 2000
	MinPriceCoins  = 1
	MaxPriceCoins  = 1_000_000
	MaxSearchQuery = 100
	MaxPageSize    = 50
	DefaultPage    = 20
	MaxOffset      = 10_000
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusRejected  Status = "rejected"
	StatusDelisted  Status = "delisted"
	StatusSuspended Status = "suspended"
)

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPending, StatusActive, StatusRejected, StatusDelisted, StatusSuspended:
		return true
	}
	return false
}

// The state machine:
//
//	draft ----submit----> pending ----approve----> active ----delist (seller)----> delisted
//	rejected --submit-->  pending ----reject-----> rejected   active --suspend (admin)--> suspended
//
// delisted and suspended are terminal in the MVP (no relist, no unsuspend). Only draft and rejected are
// editable; the payload of pending and active listings never changes.
func (s Status) CanEdit() bool    { return s == StatusDraft || s == StatusRejected }
func (s Status) CanSubmit() bool  { return s == StatusDraft || s == StatusRejected }
func (s Status) CanDelist() bool  { return s == StatusActive }
func (s Status) CanApprove() bool { return s == StatusPending }
func (s Status) CanReject() bool  { return s == StatusPending }
func (s Status) CanSuspend() bool { return s == StatusActive }

// Purchasable is the only status the checkout lane may sell.
func (s Status) Purchasable() bool { return s == StatusActive }

// Payload is what a seller writes and an Admin reviews.
type Payload struct {
	Title              string
	Description        *string
	DefaultPersonality string
	RightsNote         string
	PriceCoins         int32
}

func invalid(msg string) error { return apperror.New(apperror.CodeValidation, msg) }

func requiredText(name, value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", invalid(name + " is required")
	}
	if utf8.RuneCountInString(value) > max {
		return "", invalid(name + " is too long")
	}
	return value, nil
}

// Normalize trims the text and checks every bound.
func (p Payload) Normalize() (Payload, error) {
	var err error
	if p.Title, err = requiredText("title", p.Title, MaxTitle); err != nil {
		return Payload{}, err
	}
	if p.DefaultPersonality, err = requiredText("default_personality", p.DefaultPersonality, MaxText); err != nil {
		return Payload{}, err
	}
	if p.RightsNote, err = requiredText("rights_note", p.RightsNote, MaxText); err != nil {
		return Payload{}, err
	}
	if p.Description != nil {
		d := strings.TrimSpace(*p.Description)
		if utf8.RuneCountInString(d) > MaxText {
			return Payload{}, invalid("description is too long")
		}
		p.Description = &d
		if d == "" {
			p.Description = nil
		}
	}
	if p.PriceCoins < MinPriceCoins || p.PriceCoins > MaxPriceCoins {
		return Payload{}, invalid("price_coins must be between 1 and 1000000")
	}
	return p, nil
}

// NormalizeReason checks the reason of a reject or suspend (required, at most 2000 characters).
func NormalizeReason(reason string) (string, error) { return requiredText("reason", reason, MaxText) }

// Listing is a seller's or Admin's view of one listing. It never carries the model key.
type Listing struct {
	ID          uuid.UUID
	SellerID    uuid.UUID
	SellerName  string
	AssetID     uuid.UUID
	AssetStatus string
	Payload
	Status     Status
	ReviewedBy *uuid.UUID
	ReviewedAt *time.Time
	ReviewNote *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CatalogItem is the buyer's view of an active listing.
type CatalogItem struct {
	ID                 uuid.UUID
	AssetID            uuid.UUID
	Title              string
	Description        *string
	DefaultPersonality string
	PriceCoins         int32
	SellerName         string
	AlreadyOwned       bool
	IsOwnListing       bool
	CreatedAt          time.Time
}

// EligibleAvatar is an avatar the seller may start a listing from.
type EligibleAvatar struct {
	AvatarID uuid.UUID
	AssetID  uuid.UUID
	Name     string
}

// PreviewKey is the storage key by convention for a listing's preview image; it never points at the VRM.
func PreviewKey(asset uuid.UUID) string { return "previews/" + asset.String() + ".webp" }

type CatalogSort string

const (
	SortNewest    CatalogSort = "newest"
	SortPriceAsc  CatalogSort = "price_asc"
	SortPriceDesc CatalogSort = "price_desc"
)

type ListFilter struct {
	Status *Status
	Query  string
	Sort   CatalogSort
	Limit  int32
	Offset int32
}
