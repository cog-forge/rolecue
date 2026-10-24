package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const InitialCapacity int32 = 3
const PricePerSlot int32 = 1

type Capacity struct {
	Capacity  int32 `json:"capacity"`
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
	IsFull    bool  `json:"is_full"`
}

type Avatar struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Personality string    `json:"personality"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateAvatarInput struct {
	Name        string `json:"name"`
	Personality string `json:"personality"`
}

func (m *CreateAvatarInput) Validate() error {
	m.Name = strings.TrimSpace(m.Name)
	m.Personality = strings.TrimSpace(m.Personality)
	if !utf8.ValidString(m.Name) || m.Name == "" || utf8.RuneCountInString(m.Name) > 100 || strings.ContainsRune(m.Name, '\x00') ||
		!utf8.ValidString(m.Personality) || m.Personality == "" || utf8.RuneCountInString(m.Personality) > 2000 || strings.ContainsRune(m.Personality, '\x00') {
		return apperror.New(apperror.CodeValidation, "invalid avatar metadata")
	}
	return nil
}

// StoredAvatar keeps the private object key out of the public DTO.
type StoredAvatar struct {
	Avatar
	ModelPath string `json:"-"`
}

type ModelAccess struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type AvatarDetail struct {
	Avatar
	ModelAccess ModelAccess `json:"model_access"`
}

type Wallet struct {
	ID      uuid.UUID
	Balance int32
}

type SlotPurchaseRequest struct {
	Amount int32 `json:"amount"`
}
