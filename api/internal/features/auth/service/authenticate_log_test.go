package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type fixedVerifier struct{ id uuid.UUID }

func (f fixedVerifier) Verify(context.Context, *http.Request) (VerifiedSession, error) {
	return VerifiedSession{UserID: f.id}, nil
}

type fixedUsers struct {
	UserRepository
	user domain.User
}

func (f fixedUsers) Get(context.Context, uuid.UUID) (domain.User, error) { return f.user, nil }

func TestAuthenticateExplainsForbiddenAccounts(t *testing.T) {
	id := uuid.New()
	for _, tt := range []struct {
		name string
		user domain.User
		want string
	}{
		{"locked", domain.User{ID: id, Role: "candidate", EmailVerified: true, IsLocked: true}, "locked=true email_verified=true"},
		{"unverified", domain.User{ID: id, Role: "candidate"}, "locked=false email_verified=false"},
		{"unknown role", domain.User{ID: id, Role: "guest", EmailVerified: true}, `role "guest" is not permitted`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewService(fixedVerifier{id: id}, fixedUsers{user: tt.user})
			_, _, err := s.Authenticate(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil))
			if !apperror.IsCode(err, apperror.CodeForbidden) {
				t.Fatalf("code lost: %v", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("reason %q missing from %q", tt.want, err.Error())
			}
		})
	}
}
