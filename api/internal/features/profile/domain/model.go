package domain

import (
	"time"

	"github.com/google/uuid"
)

// Profile contains public account metadata, never credentials or session state.
type Profile struct {
	OnboardingRoleSelected bool      `json:"onboarding_role_selected"`
	OnboardingCompleted    bool      `json:"onboarding_completed"`
	ID                     uuid.UUID `json:"id"`
	FullName               string    `json:"full_name"`
	Email                  string    `json:"email"`
	Image                  *string   `json:"image"`
	Role                   string    `json:"role"`
	EmailVerified          bool      `json:"email_verified"`
	CompanyName            *string   `json:"company_name"`
	CompanyWebsite         *string   `json:"company_website"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
