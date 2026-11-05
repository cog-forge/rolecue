package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type scriptedAuth struct {
	user authdomain.User
	err  error
	last *http.Request
}

func (a *scriptedAuth) Authenticate(_ context.Context, req *http.Request) (authdomain.User, []string, error) {
	a.last = req
	return a.user, nil, a.err
}

func TestLiveVerifierMapsAuthOutcomes(t *testing.T) {
	owner := uuid.New()
	outage := errors.New("auth service down")
	for _, tc := range []struct {
		name    string
		user    authdomain.User
		err     error
		invalid bool
		other   error
	}{
		{"valid session", authdomain.User{ID: owner}, nil, false, nil},
		{"logged out or revoked", authdomain.User{}, authdomain.ErrInvalidSession, true, nil},
		{"locked or role removed", authdomain.User{}, apperror.New(apperror.CodeForbidden, "locked"), true, nil},
		{"session now belongs to someone else", authdomain.User{ID: uuid.New()}, nil, true, nil},
		{"auth outage keeps the socket", authdomain.User{}, outage, false, outage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &scriptedAuth{user: tc.user, err: tc.err}
			h := NewInterviewLiveHandler(nil, nil, nil).WithAuthenticator(auth)
			req := httptest.NewRequest(http.MethodGet, "/interviews/x/live", nil)
			req.Header.Set("Cookie", "rolecue.session_token=abc")
			verify := h.verifier(req, owner)
			err := verify(context.Background())
			switch {
			case tc.invalid && !errors.Is(err, runtime.ErrSessionInvalid):
				t.Fatalf("verify = %v, want ErrSessionInvalid", err)
			case !tc.invalid && errors.Is(err, runtime.ErrSessionInvalid):
				t.Fatalf("verify = %v, must not revoke", err)
			case !tc.invalid && !errors.Is(err, tc.other):
				t.Fatalf("verify = %v, want %v", err, tc.other)
			}
			if auth.last == nil || auth.last.Header.Get("Cookie") != "rolecue.session_token=abc" {
				t.Fatal("the re-check must authenticate the upgrade request's cookies")
			}
		})
	}
}

func TestLiveVerifierIsOffWithoutAnAuthenticator(t *testing.T) {
	h := NewInterviewLiveHandler(nil, nil, nil)
	if h.verifier(httptest.NewRequest(http.MethodGet, "/", nil), uuid.New()) != nil {
		t.Fatal("no authenticator must mean no re-check")
	}
}
