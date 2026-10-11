package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type onboardingSession struct{ user authdomain.User }

func (s onboardingSession) Authenticate(context.Context, *http.Request) (authdomain.User, []string, error) {
	return s.user, nil, nil
}

type roleSelectionSpy struct {
	calls int
	id    uuid.UUID
	role  string
	err   error
}

func (s *roleSelectionSpy) SelectRole(_ context.Context, id uuid.UUID, role string) (authdomain.User, error) {
	s.calls++
	s.id, s.role = id, role
	return authdomain.User{ID: id, Role: role, OnboardingRoleSelected: true}, s.err
}

func TestSelectRoleStrictJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, contentType, body string }{
		{"unknown security field", "application/json", `{"role":"recruiter","is_locked":false}`},
		{"client user ID", "application/json", `{"role":"candidate","user_id":"another-user"}`},
		{"trailing JSON", "application/json", `{"role":"recruiter"}{"role":"candidate"}`},
		{"trailing garbage", "application/json", `{"role":"candidate"}junk`},
		{"missing content type", "", `{"role":"candidate"}`},
		{"wrong content type", "text/plain", `{"role":"candidate"}`},
		{"malformed JSON", "application/json", `{"role":`},
		{"null body", "application/json", `null`},
		{"array body", "application/json", `[]`},
		{"missing role", "application/json", `{}`},
		{"null role", "application/json", `{"role":null}`},
		{"wrong role type", "application/json", `{"role":1}`},
		{"oversized body", "application/json", `{"role":"candidate"}` + strings.Repeat(" ", 16<<10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &roleSelectionSpy{}
			user := authdomain.User{ID: uuid.New(), Role: "candidate"}
			r := roleSelectionRouter(user, service)
			w := selectRoleRequest(r, tc.contentType, tc.body)
			var body response.Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusBadRequest || body.Error == nil || body.Error.Code != apperror.CodeValidation || service.calls != 0 {
				t.Fatalf("status=%d body=%s service calls=%d; want 400 VALIDATION and no service call", w.Code, w.Body.String(), service.calls)
			}
		})
	}
}

func TestSelectRoleUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"candidate", "recruiter"} {
		t.Run(role, func(t *testing.T) {
			service := &roleSelectionSpy{}
			user := authdomain.User{ID: uuid.New(), Role: "candidate"}
			w := selectRoleRequest(roleSelectionRouter(user, service), "application/json; charset=utf-8", `{"role":"`+role+`"} `)
			if w.Code != http.StatusOK || service.calls != 1 || service.id != user.ID || service.role != role {
				t.Fatalf("status=%d body=%s service=%+v", w.Code, w.Body.String(), service)
			}
		})
	}
}

func TestSelectRoleRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	service := &roleSelectionSpy{}
	r.POST("/onboarding/role", NewOnboardingHandler(service).SelectRole)
	w := selectRoleRequest(r, "application/json", `{"role":"candidate"}`)
	if w.Code != http.StatusUnauthorized || service.calls != 0 {
		t.Fatalf("status=%d service calls=%d", w.Code, service.calls)
	}
}

func TestSelectRolePropagatesRefusal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &roleSelectionSpy{err: apperror.New(apperror.CodeForbidden, "role cannot be changed")}
	user := authdomain.User{ID: uuid.New(), Role: "recruiter"}
	w := selectRoleRequest(roleSelectionRouter(user, service), "application/json", `{"role":"candidate"}`)
	var body response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusForbidden || body.Error == nil || body.Error.Code != apperror.CodeForbidden || service.calls != 1 {
		t.Fatalf("status=%d body=%s service calls=%d", w.Code, w.Body.String(), service.calls)
	}
}

func roleSelectionRouter(user authdomain.User, service OnboardingService) *gin.Engine {
	r := gin.New()
	auth := middleware.RequireAuth(onboardingSession{user}, config.CORSConfig{AllowOrigins: []string{"http://localhost:3000"}})
	r.POST("/onboarding/role", auth, NewOnboardingHandler(service).SelectRole)
	return r
}

func selectRoleRequest(r *gin.Engine, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/onboarding/role", strings.NewReader(body))
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
