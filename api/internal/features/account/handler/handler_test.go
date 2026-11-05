package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeService struct {
	confirm string
	calls   int
	err     error
	user    authdomain.User
}

func (f *fakeService) Erase(_ context.Context, u authdomain.User, confirm string) error {
	f.calls++
	f.confirm, f.user = confirm, u
	return f.err
}
func (f *fakeService) Export(_ context.Context, u authdomain.User) (domain.DataExport, error) {
	f.user = u
	return domain.DataExport{Profile: domain.ExportProfile{ID: u.ID}}, f.err
}

func serve(t *testing.T, svc *fakeService, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	engine.Use(func(c *gin.Context) { c.Set("auth.middleware.currentUser", user) })
	h := NewHandler(svc)
	engine.DELETE("/account", h.Erase)
	engine.GET("/account/export", h.Export)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestEraseRequiresStrictJSON(t *testing.T) {
	for name, tc := range map[string]struct{ contentType, body string }{
		"no body":       {"application/json", ""},
		"unknown field": {"application/json", `{"confirm":"DELETE","force":true}`},
		"trailing json": {"application/json", `{"confirm":"DELETE"}{}`},
		"null":          {"application/json", `null`},
		"wrong type":    {"text/plain", `{"confirm":"DELETE"}`},
		"too large":     {"application/json", `{"confirm":"` + strings.Repeat("x", 2048) + `"}`},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeService{}
			w := serve(t, svc, http.MethodDelete, "/account", tc.contentType, tc.body)
			if w.Code != http.StatusBadRequest || svc.calls != 0 {
				t.Fatalf("status %d, service calls %d, body %s", w.Code, svc.calls, w.Body)
			}
		})
	}
}

func TestEraseForwardsConfirmationAndMapsErrors(t *testing.T) {
	svc := &fakeService{}
	w := serve(t, svc, http.MethodDelete, "/account", "application/json", `{"confirm":"DELETE"}`)
	if w.Code != http.StatusOK || svc.confirm != "DELETE" || !strings.Contains(w.Body.String(), `"deleted":true`) {
		t.Fatalf("status %d confirm %q body %s", w.Code, svc.confirm, w.Body)
	}
	svc = &fakeService{err: apperror.New(apperror.CodeAccountHasActiveWork, "busy")}
	w = serve(t, svc, http.MethodDelete, "/account", "application/json", `{"confirm":"DELETE"}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ACCOUNT_HAS_ACTIVE_WORK") {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	svc = &fakeService{err: apperror.New(apperror.CodeAdminCannotSelfDelete, "no")}
	w = serve(t, svc, http.MethodDelete, "/account", "application/json", `{"confirm":"DELETE"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
}

func TestExportUsesTheSessionUserOnly(t *testing.T) {
	svc := &fakeService{}
	w := serve(t, svc, http.MethodGet, "/account/export?user_id="+uuid.NewString(), "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("status %d headers %v", w.Code, w.Header())
	}
	if !strings.Contains(w.Body.String(), svc.user.ID.String()) {
		t.Fatalf("export is not for the session user: %s", w.Body)
	}
}
