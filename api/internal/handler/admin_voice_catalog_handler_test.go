package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// syncOnlyService implements VoiceCatalogService for the sync route; the other methods are not used.
type syncOnlyService struct {
	VoiceCatalogService
	result domain.VoiceSyncResult
	err    error
	role   string
	admin  uuid.UUID
	calls  int
}

func (s *syncOnlyService) Sync(_ context.Context, adminID uuid.UUID, role string) (domain.VoiceSyncResult, error) {
	s.calls++
	s.role, s.admin = role, adminID
	if role != "admin" {
		return domain.VoiceSyncResult{}, apperror.New(apperror.CodeForbidden, "only administrators can manage voice profiles")
	}
	return s.result, s.err
}

func postVoiceSync(svc VoiceCatalogService, user *authdomain.User) (*httptest.ResponseRecorder, map[string]any) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if user != nil {
			c.Set("auth.middleware.currentUser", *user)
		}
		c.Next()
	})
	engine.POST("/admin/voices/sync", NewVoiceCatalogHandler(svc).Sync)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/admin/voices/sync", nil))
	var body map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

func TestVoiceSyncReturnsTheSummary(t *testing.T) {
	adminID := uuid.New()
	svc := &syncOnlyService{result: domain.VoiceSyncResult{Provider: "azure", Fetched: 5, Added: 2, Updated: 1, Deactivated: 1, Unchanged: 2}}
	recorder, body := postVoiceSync(svc, &authdomain.User{ID: adminID, Role: "admin"})
	if recorder.Code != http.StatusOK || svc.admin != adminID {
		t.Fatalf("status=%d admin=%s body=%s", recorder.Code, svc.admin, recorder.Body.String())
	}
	data, _ := body["data"].(map[string]any)
	want := map[string]float64{"fetched": 5, "added": 2, "updated": 1, "deactivated": 1, "unchanged": 2}
	for key, value := range want {
		if data[key] != value {
			t.Fatalf("data[%s] = %v, want %v (%s)", key, data[key], value, recorder.Body.String())
		}
	}
	if data["provider"] != "azure" {
		t.Fatalf("provider = %v", data["provider"])
	}
}

func TestVoiceSyncMapsErrors(t *testing.T) {
	admin := &authdomain.User{ID: uuid.New(), Role: "admin"}
	tests := []struct {
		name   string
		user   *authdomain.User
		err    error
		status int
		code   string
	}{
		{"candidate", &authdomain.User{ID: uuid.New(), Role: "candidate"}, nil, http.StatusForbidden, string(apperror.CodeForbidden)},
		{"no session user", nil, nil, http.StatusUnauthorized, string(apperror.CodeInvalidToken)},
		{"provider failure", admin, apperror.Wrap(apperror.CodeVoiceSyncFailed, "the voice provider could not be reached", errors.New("secret internals")), http.StatusBadGateway, string(apperror.CodeVoiceSyncFailed)},
		{"provider not configured", admin, apperror.New(apperror.CodeVoiceProviderUnavailable, "the voice provider is not configured"), http.StatusServiceUnavailable, string(apperror.CodeVoiceProviderUnavailable)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &syncOnlyService{err: tt.err}
			recorder, body := postVoiceSync(svc, tt.user)
			if recorder.Code != tt.status || errorCode(body) != tt.code {
				t.Fatalf("status=%d code=%s, want %d %s (%s)", recorder.Code, errorCode(body), tt.status, tt.code, recorder.Body.String())
			}
			if tt.user == nil && svc.calls != 0 {
				t.Fatal("an unauthenticated call reached the service")
			}
			if recorder.Body.Len() > 0 && json.Valid(recorder.Body.Bytes()) && strings.Contains(recorder.Body.String(), "secret internals") {
				t.Fatalf("provider internals leaked: %s", recorder.Body.String())
			}
		})
	}
}
