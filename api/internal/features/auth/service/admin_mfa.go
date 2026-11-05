package service

import (
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// AdminMFAPolicy decides whether an Admin must have two-factor authentication (KAN-265). The zero value
// enforces nothing. Candidates and Recruiters keep two-factor optional.
type AdminMFAPolicy struct {
	Required bool
	// GraceUntil lets Admins without two-factor in before this instant; the zero time means no grace.
	GraceUntil time.Time
	Now        func() time.Time
}

// NewAdminMFAPolicy builds the policy from configuration; grace is an RFC3339 timestamp or empty.
func NewAdminMFAPolicy(required bool, grace string) (AdminMFAPolicy, error) {
	p := AdminMFAPolicy{Required: required}
	if grace != "" {
		t, err := time.Parse(time.RFC3339, grace)
		if err != nil {
			return AdminMFAPolicy{}, fmt.Errorf("admin MFA grace must be RFC3339: %w", err)
		}
		p.GraceUntil = t
	}
	return p, nil
}

// Check returns ADMIN_MFA_REQUIRED for an Admin without two-factor when the policy is on and grace is over.
func (p AdminMFAPolicy) Check(user domain.User) error {
	if !p.Required || user.Role != "admin" || user.TwoFactorEnabled {
		return nil
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	if !p.GraceUntil.IsZero() && now().Before(p.GraceUntil) {
		return nil
	}
	cause := fmt.Errorf("admin without two-factor authentication user_id=%s", user.ID)
	return apperror.Wrap(apperror.CodeAdminMFARequired, "set up two-factor authentication to use the admin workspace", cause)
}
