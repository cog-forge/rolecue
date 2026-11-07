package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// auditStub serves the access audit through the real service rules (admin only) over an empty audit.
type auditStub struct {
	handler.InterviewRecordingService
	reached bool
}

func (s *auditStub) AccessAudit(ctx context.Context, role string, filter domain.AccessAuditFilter) (domain.AccessAuditPage, error) {
	s.reached = true
	return domain.AccessAuditPage{Items: []domain.AccessAuditEntry{}, Page: 1, PageSize: 20}, nil
}

// GET /admin/access-audit (KAN-296) is Admin-only: other roles and anonymous callers never reach the
// handler, an Admin does.
func TestAdminAccessAuditRouteRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const origin = "http://localhost:3000"
	for _, tc := range []struct {
		name    string
		role    string
		authErr error
		want    int
		reached bool
	}{
		{"unauthenticated", "", authdomain.ErrInvalidSession, http.StatusUnauthorized, false},
		{"candidate", "candidate", nil, http.StatusForbidden, false},
		{"recruiter", "recruiter", nil, http.StatusForbidden, false},
		{"admin", "admin", nil, http.StatusOK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := routerAuth{
				user: authdomain.User{ID: uuid.New(), Role: tc.role, EmailVerified: true, OnboardingRoleSelected: true, OnboardingCompleted: true},
				err:  tc.authErr,
			}
			stub := &auditStub{}
			cfg := &config.Config{CORS: config.CORSConfig{AllowOrigins: []string{origin}}}
			r := NewRouter(cfg, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(auth),
				handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
				handler.NewWalletHandler(&routerWallet{}), handler.NewQuestionBankHandler(nil, time.Minute),
				&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}, &handler.AdminInterviewsHandler{}, &handler.AdminOverviewHandler{},
				&handler.InterviewHandlers{Recording: handler.NewInterviewRecordingHandler(stub)}).Setup()

			req := httptest.NewRequest(http.MethodGet, "/admin/access-audit?page=1", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want || stub.reached != tc.reached {
				t.Fatalf("GET /admin/access-audit as %s: status = %d reached = %v, want %d / %v", tc.name, w.Code, stub.reached, tc.want, tc.reached)
			}
		})
	}
}
