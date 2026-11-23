package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// setupService counts what reaches it, so a refused caller can be shown to stop at the route.
type setupService struct{ calls int }

func (s *setupService) Practice(context.Context, uuid.UUID, string, uuid.UUID) (domain.Setup, error) {
	s.calls++
	return domain.Setup{}.WithBlockers(nil), nil
}

func (s *setupService) SavePractice(context.Context, uuid.UUID, string, uuid.UUID, domain.PracticeInput) (domain.Setup, error) {
	s.calls++
	return domain.Setup{}.WithBlockers(nil), nil
}

func (s *setupService) Recruitment(context.Context, uuid.UUID, string, uuid.UUID) (domain.Setup, error) {
	s.calls++
	return domain.Setup{}.WithBlockers(nil), nil
}

func (s *setupService) SaveRecruitment(context.Context, uuid.UUID, string, uuid.UUID, *string) (domain.Setup, error) {
	s.calls++
	return domain.Setup{}.WithBlockers(nil), nil
}

// The setup routes are for a signed-in Candidate who finished onboarding: everyone else is stopped by the
// route, before the service runs, on reads and on writes.
func TestInterviewSetupRoutesAreCandidateOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const origin = "http://localhost:3000"
	id := uuid.NewString()
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/interview-setups/practice/" + id, ""},
		{http.MethodPut, "/interview-setups/practice/" + id, `{"environment":"office-3d"}`},
		{http.MethodGet, "/interview-setups/recruitment/" + id, ""},
		{http.MethodPut, "/interview-setups/recruitment/" + id, `{"environment":"office-3d"}`},
	}
	for _, tc := range []struct {
		name       string
		role       string
		authErr    error
		onboarding bool
		want       int
		reaches    bool
	}{
		{"unauthenticated", "", authdomain.ErrInvalidSession, true, http.StatusUnauthorized, false},
		{"recruiter", "recruiter", nil, true, http.StatusForbidden, false},
		{"admin", "admin", nil, true, http.StatusForbidden, false},
		{"candidate before onboarding", "candidate", nil, false, http.StatusForbidden, false},
		{"candidate", "candidate", nil, true, http.StatusOK, true},
	} {
		for _, route := range routes {
			t.Run(tc.name+" "+route.method+" "+route.path, func(t *testing.T) {
				svc := &setupService{}
				auth := routerAuth{
					user: authdomain.User{ID: uuid.New(), Role: tc.role, EmailVerified: true, OnboardingRoleSelected: true, OnboardingCompleted: tc.onboarding},
					err:  tc.authErr,
				}
				cfg := &config.Config{CORS: config.CORSConfig{AllowOrigins: []string{origin}}}
				r := NewRouter(cfg, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(auth),
					handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
					handler.NewWalletHandler(&routerWallet{}), handler.NewQuestionBankHandler(nil, time.Minute),
					&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}, &handler.AdminInterviewsHandler{}, &handler.AdminOverviewHandler{}, &handler.InterviewHandlers{}).
					WithInterviewSetupHandler(handler.NewInterviewSetupHandler(svc)).Setup()

				req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				req.Header.Set("Origin", origin)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != tc.want {
					t.Fatalf("status = %d, want %d (%s)", w.Code, tc.want, w.Body.String())
				}
				if reached := svc.calls > 0; reached != tc.reaches {
					t.Fatalf("service reached = %v, want %v", reached, tc.reaches)
				}
			})
		}
	}
}
