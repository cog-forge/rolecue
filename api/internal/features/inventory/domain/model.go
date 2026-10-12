package domain

import (
	"time"

	"github.com/google/uuid"
)

const InitialCapacity int32 = 3

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
