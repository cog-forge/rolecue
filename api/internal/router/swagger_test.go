package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

func TestSwaggerUIOnlyOutsideProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		env    string
		status int
	}{
		{"development", http.StatusOK},
		{"staging", http.StatusOK},
		{"production", http.StatusNotFound},
	} {
		t.Run(tc.env, func(t *testing.T) {
			cfg := &config.Config{App: config.AppConfig{Environment: tc.env}}
			engine := NewRouter(cfg, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(routerAuth{}),
				handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
				handler.NewWalletHandler(nil), handler.NewQuestionBankHandler(nil, time.Minute),
				&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}, &handler.AdminInterviewsHandler{}, &handler.AdminOverviewHandler{}, &handler.InterviewHandlers{}).Setup()
			for _, path := range []string{"/swagger/index.html", "/swagger/doc.json"} {
				w := httptest.NewRecorder()
				engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				if w.Code != tc.status {
					t.Fatalf("GET %s in %s = %d, want %d", path, tc.env, w.Code, tc.status)
				}
			}
		})
	}
}
