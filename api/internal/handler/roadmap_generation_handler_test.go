package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	roadmapdomain "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeRoadmapGeneration struct {
	calls   []string
	role    string
	profile uuid.UUID
	roadmap roadmapdomain.Roadmap
	err     error
}

func (f *fakeRoadmapGeneration) Retry(_ context.Context, _ uuid.UUID, role string, profile uuid.UUID) (roadmapdomain.Roadmap, error) {
	f.calls, f.role, f.profile = append(f.calls, "retry"), role, profile
	return f.roadmap, f.err
}

func (f *fakeRoadmapGeneration) Regenerate(_ context.Context, _ uuid.UUID, role string, profile uuid.UUID) (roadmapdomain.Roadmap, error) {
	f.calls, f.role, f.profile = append(f.calls, "regenerate"), role, profile
	return f.roadmap, f.err
}

func roadmapRequest(t *testing.T, h *RoleProfileHandler, operation string, id string) (*deadlineWriter, time.Time, time.Time) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth.middleware.currentUser", authdomain.User{ID: uuid.New(), Role: "candidate"})
	})
	handle := h.RetryRoadmap
	if operation == "regenerate" {
		handle = h.RegenerateRoadmap
	}
	r.POST("/:id", handle)
	w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	start := time.Now()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/"+id, nil))
	return w, start, time.Now()
}

func TestRoadmapRoutesCallTheServiceAndReturnTheStatusInSnakeCase(t *testing.T) {
	for _, operation := range []string{"retry", "regenerate"} {
		t.Run(operation, func(t *testing.T) {
			profile := uuid.New()
			fake := &fakeRoadmapGeneration{roadmap: roadmapdomain.Roadmap{ID: uuid.New(), RoleProfileID: profile, GenerationStatus: roadmapdomain.StatusReady}}
			h := NewRoleProfileHandler(&deadlineService{}, nil).WithRoadmap(fake)
			w, start, end := roadmapRequest(t, h, operation, profile.String())
			if w.Code != http.StatusOK || len(fake.calls) != 1 || fake.calls[0] != operation || fake.profile != profile || fake.role != "candidate" {
				t.Fatalf("status=%d calls=%v profile=%s role=%s", w.Code, fake.calls, fake.profile, fake.role)
			}
			if w.deadline.Before(start.Add(roadmapWriteDeadline)) || w.deadline.After(end.Add(roadmapWriteDeadline)) {
				t.Fatalf("deadline %v: a generation needs the two-model-step window", w.deadline)
			}
			var body struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Data["generation_status"] != "ready" || body.Data["roadmap_id"] != fake.roadmap.ID.String() || body.Data["role_profile_id"] != profile.String() {
				t.Fatalf("data = %v", body.Data)
			}
			if _, leaked := body.Data["questions"]; leaked || len(body.Data) != 3 {
				t.Fatalf("the response must carry the status only: %v", body.Data)
			}
		})
	}
}

func TestRoadmapRoutesMapDomainErrorsToTheirStatuses(t *testing.T) {
	cases := map[apperror.Code]int{
		apperror.CodeRoadmapGenerating:           http.StatusConflict,
		apperror.CodeRoadmapNotRetryable:         http.StatusConflict,
		apperror.CodeRoadmapRegenerateNotAllowed: http.StatusConflict,
		apperror.CodeRoadmapGenerationFailed:     http.StatusBadGateway,
		apperror.CodeJDNotFound:                  http.StatusNotFound,
		apperror.CodeForbidden:                   http.StatusForbidden,
	}
	for code, status := range cases {
		fake := &fakeRoadmapGeneration{err: apperror.New(code, "x")}
		h := NewRoleProfileHandler(&deadlineService{}, nil).WithRoadmap(fake)
		if w, _, _ := roadmapRequest(t, h, "retry", uuid.NewString()); w.Code != status {
			t.Errorf("%s -> %d, want %d", code, w.Code, status)
		}
	}
}

func TestRoadmapRoutesRejectABadIDAndAMissingServiceBeforeAnyWork(t *testing.T) {
	fake := &fakeRoadmapGeneration{}
	h := NewRoleProfileHandler(&deadlineService{}, nil).WithRoadmap(fake)
	if w, _, _ := roadmapRequest(t, h, "regenerate", "not-a-uuid"); w.Code != http.StatusBadRequest || len(fake.calls) != 0 {
		t.Fatalf("status=%d calls=%v", w.Code, fake.calls)
	}
	bare := NewRoleProfileHandler(&deadlineService{}, nil)
	if w, _, _ := roadmapRequest(t, bare, "retry", uuid.NewString()); w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500 when generation is not wired", w.Code)
	}
}
