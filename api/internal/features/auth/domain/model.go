package domain

import "github.com/google/uuid"

type User struct {
	OnboardingRoleSelected bool      `json:"onboarding_role_selected"`
	OnboardingCompleted    bool      `json:"onboarding_completed"`
	ID                     uuid.UUID `json:"id"`
	Email                  string    `json:"email"`
	FullName               string    `json:"full_name"`
	Role                   string    `json:"role"`
	EmailVerified          bool      `json:"email_verified"`
	IsLocked               bool      `json:"is_locked"`
	Image                  *string   `json:"image"`
}
