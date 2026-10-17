package domain

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Image         *string    `json:"image"`
	Role          string     `json:"role"`
	EmailVerified bool       `json:"email_verified"`
	IsLocked      bool       `json:"is_locked"`
	LockReason    *string    `json:"lock_reason"`
	LockExpiresAt *time.Time `json:"lock_expires_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (a Account) Locked(now time.Time) bool {
	return a.IsLocked && (a.LockExpiresAt == nil || a.LockExpiresAt.After(now))
}

type ListFilter struct {
	Search string
	Filter string
	Limit  int32
	Offset int64
}

type List struct {
	Users []Account `json:"users"`
	Total int64     `json:"total"`
}

type LockRequest struct {
	Reason    string `json:"reason"`
	ExpiresIn *int64 `json:"expires_in,omitempty"`
}

type Notification struct {
	Status string `json:"status"`
}

type Transition struct {
	User         Account      `json:"user"`
	Changed      bool         `json:"changed"`
	Notification Notification `json:"notification"`
}
