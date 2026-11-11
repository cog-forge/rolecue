package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// guidelinesService counts what reaches it, so a refused caller can be shown to stop earlier.
type guidelinesService struct{ calls int }

func (s *guidelinesService) Get(context.Context, string) (domain.Settings, error) {
	s.calls++
	return domain.Settings{Values: interviewdomain.DefaultBehaviorGuideline()}, nil
}

func (s *guidelinesService) Save(context.Context, uuid.UUID, string, int32, interviewdomain.BehaviorGuideline) (domain.Settings, error) {
	s.calls++
	return domain.Settings{Version: 1, Values: interviewdomain.DefaultBehaviorGuideline()}, nil
}

// The guidelines are Admin only: Candidates, Recruiters and unauthenticated callers are stopped by the
// route's role check, before the handler or the service runs, on reads and on writes.
func TestInterviewGuidelinesRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const origin = "http://localhost:3000"
	body := `{"expected_version":0,"values":{"tone":"formal","question_length":"standard","follow_up_max":null,"follow_up_focus":"standard","avoid_topics":[]}}`
	for _, tc := range []struct {
		name    string
		role    string
		authErr error
		want    int
		reaches bool
	}{
		{"unauthenticated", "", authdomain.ErrInvalidSession, http.StatusUnauthorized, false},
		{"candidate", "candidate", nil, http.StatusForbidden, false},
		{"recruiter", "recruiter", nil, http.StatusForbidden, false},
		{"admin", "admin", nil, http.StatusOK, true},
	} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			t.Run(tc.name+" "+method, func(t *testing.T) {
				svc := &guidelinesService{}
				auth := routerAuth{
					user: authdomain.User{ID: uuid.New(), Role: tc.role, EmailVerified: true, OnboardingRoleSelected: true, OnboardingCompleted: true},
					err:  tc.authErr,
				}
				cfg := &config.Config{CORS: config.CORSConfig{AllowOrigins: []string{origin}}}
				interviews := &handler.InterviewHandlers{Guidelines: handler.NewAdminGuidelinesHandler(svc)}
				r := NewRouter(cfg, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(auth),
					handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
					handler.NewWalletHandler(&routerWallet{}), handler.NewQuestionBankHandler(nil, time.Minute),
					&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}, &handler.AdminInterviewsHandler{}, &handler.AdminOverviewHandler{}, interviews).Setup()

				req := httptest.NewRequest(method, "/admin/interview-guidelines", strings.NewReader(body))
				req.Header.Set("Origin", origin)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != tc.want {
					t.Fatalf("%s as %s: status = %d, want %d (%s)", method, tc.name, w.Code, tc.want, w.Body.String())
				}
				if reached := svc.calls > 0; reached != tc.reaches {
					t.Fatalf("%s as %s: service reached = %v, want %v", method, tc.name, reached, tc.reaches)
				}
			})
		}
	}
}
