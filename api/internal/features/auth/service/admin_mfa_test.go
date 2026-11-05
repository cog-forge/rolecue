package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func authenticateAs(t *testing.T, user domain.User, opts ...Option) error {
	t.Helper()
	s := NewService(fixedVerifier{id: user.ID}, fixedUsers{user: user}, opts...)
	_, _, err := s.Authenticate(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil))
	return err
}

func TestAdminMFAPolicy(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	admin := func(mfa bool) domain.User {
		return domain.User{ID: uuid.New(), Role: "admin", EmailVerified: true, TwoFactorEnabled: mfa}
	}
	on := AdminMFAPolicy{Required: true, Now: clock}
	for _, tt := range []struct {
		name   string
		user   domain.User
		policy AdminMFAPolicy
		denied bool
	}{
		{"flag off lets an admin without MFA in", admin(false), AdminMFAPolicy{Now: clock}, false},
		{"enrolled admin passes", admin(true), on, false},
		{"admin without MFA is rejected", admin(false), on, true},
		{"grace in the future lets the admin in", admin(false), AdminMFAPolicy{Required: true, GraceUntil: now.Add(time.Hour), Now: clock}, false},
		{"expired grace rejects", admin(false), AdminMFAPolicy{Required: true, GraceUntil: now.Add(-time.Second), Now: clock}, true},
		{"candidate stays optional", domain.User{ID: uuid.New(), Role: "candidate", EmailVerified: true}, on, false},
		{"recruiter stays optional", domain.User{ID: uuid.New(), Role: "recruiter", EmailVerified: true}, on, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := authenticateAs(t, tt.user, WithAdminMFA(tt.policy))
			if tt.denied != apperror.IsCode(err, apperror.CodeAdminMFARequired) {
				t.Fatalf("denied=%t, err=%v", tt.denied, err)
			}
			if !tt.denied && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	if err := authenticateAs(t, admin(false)); err != nil {
		t.Fatalf("default service must not enforce MFA: %v", err)
	}
}

func TestNewAdminMFAPolicyParsesGrace(t *testing.T) {
	p, err := NewAdminMFAPolicy(true, "2026-12-01T00:00:00Z")
	if err != nil || p.GraceUntil.IsZero() || !p.Required {
		t.Fatalf("policy %+v err %v", p, err)
	}
	if p, err = NewAdminMFAPolicy(true, ""); err != nil || !p.GraceUntil.IsZero() {
		t.Fatalf("empty grace must mean none: %+v %v", p, err)
	}
	if _, err = NewAdminMFAPolicy(true, "tomorrow"); err == nil {
		t.Fatal("invalid grace must be rejected")
	}
}
