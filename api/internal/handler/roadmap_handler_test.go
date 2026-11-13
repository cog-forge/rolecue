package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type roadmapStub struct {
	view    domain.RoadmapView
	entries []domain.HistoryEntry
	total   int64
	err     error
	limit   int32
	offset  int32
	role    string
}

func (s *roadmapStub) Get(_ context.Context, _ uuid.UUID, role string, _ uuid.UUID) (domain.RoadmapView, error) {
	s.role = role
	return s.view, s.err
}

func (s *roadmapStub) ItemHistory(_ context.Context, _ uuid.UUID, role string, _, _ uuid.UUID, limit, offset int32) ([]domain.HistoryEntry, int64, error) {
	s.role, s.limit, s.offset = role, limit, offset
	return s.entries, s.total, s.err
}

func roadmapRouter(stub *roadmapStub, authenticated bool, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewRoadmapHandler(stub)
	r := gin.New()
	if authenticated {
		r.Use(func(c *gin.Context) {
			c.Set("auth.middleware.currentUser", authdomain.User{ID: uuid.New(), Role: role})
		})
	}
	r.GET("/role-profiles/:id/roadmap", h.Get)
	r.GET("/role-profiles/:id/roadmap/items/:item_id/history", h.History)
	return r
}

func roadmapServe(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func roadmapErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return body.Error.Code
}

func TestRoadmapRoutesRequireAuthentication(t *testing.T) {
	r := roadmapRouter(&roadmapStub{}, false, "")
	id := uuid.NewString()
	for _, path := range []string{"/role-profiles/" + id + "/roadmap", "/role-profiles/" + id + "/roadmap/items/" + id + "/history"} {
		if w := roadmapServe(r, path); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s = %d, want 401", path, w.Code)
		}
	}
}

func TestRoadmapRoutesRejectMalformedIDs(t *testing.T) {
	r := roadmapRouter(&roadmapStub{}, true, "candidate")
	id := uuid.NewString()
	for _, path := range []string{"/role-profiles/nope/roadmap", "/role-profiles/nope/roadmap/items/" + id + "/history", "/role-profiles/" + id + "/roadmap/items/nope/history"} {
		if w := roadmapServe(r, path); w.Code != http.StatusBadRequest || roadmapErrorCode(t, w) != string(apperror.CodeValidation) {
			t.Fatalf("%s = %d %s, want 400 VALIDATION_ERROR", path, w.Code, w.Body.String())
		}
	}
}

func TestRoadmapGetReturnsSnakeCaseEnvelopeWithoutCaching(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	score := 72.5
	stub := &roadmapStub{view: domain.RoadmapView{
		ID: uuid.New(), RoleProfileID: uuid.New(), GenerationStatus: domain.StatusReady, LearningStatus: domain.LearningInProgress,
		CompletedItems: 0, TotalItems: 1, UpdateState: domain.UpdateUpToDate, WindowSize: 5, AchievedThreshold: 80,
		Items: []domain.RoadmapItemView{{ID: uuid.New(), Title: "SQL", Position: 1, Score: &score, EvidenceQuestionCount: 3,
			LearningStatus: domain.TopicInProgress, LastEvaluatedAt: &now}},
	}}
	w := roadmapServe(roadmapRouter(stub, true, "candidate"), "/role-profiles/"+uuid.NewString()+"/roadmap")
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d, cache-control %q", w.Code, w.Header().Get("Cache-Control"))
	}
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "role_profile_id", "generation_status", "learning_status", "progress_percent", "completed_items",
		"total_items", "practice_started", "update_state", "pending_result_count", "window_size", "achieved_threshold", "items"} {
		if _, ok := body.Data[key]; !ok {
			t.Errorf("response has no %q field: %s", key, w.Body.String())
		}
	}
	if string(body.Data["progress_percent"]) != "null" {
		t.Errorf("progress_percent = %s, want null", body.Data["progress_percent"])
	}
	for _, leak := range []string{"expected_answer", "rubric", "feedback", "core_question"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("response mentions %q", leak)
		}
	}
	if stub.role != "candidate" {
		t.Errorf("service role = %q", stub.role)
	}
}

func TestRoadmapErrorsMapToStatuses(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   apperror.Code
	}{
		{apperror.New(apperror.CodeJDNotFound, "Role Profile not found."), http.StatusNotFound, apperror.CodeJDNotFound},
		{apperror.New(apperror.CodeRoadmapItemNotFound, "x"), http.StatusNotFound, apperror.CodeRoadmapItemNotFound},
		{apperror.New(apperror.CodeForbidden, "x"), http.StatusForbidden, apperror.CodeForbidden},
	}
	for _, tc := range cases {
		r := roadmapRouter(&roadmapStub{err: tc.err}, true, "candidate")
		id := uuid.NewString()
		for _, path := range []string{"/role-profiles/" + id + "/roadmap", "/role-profiles/" + id + "/roadmap/items/" + id + "/history"} {
			if w := roadmapServe(r, path); w.Code != tc.status || roadmapErrorCode(t, w) != string(tc.code) {
				t.Errorf("%s with %s = %d %s", path, tc.code, w.Code, w.Body.String())
			}
		}
	}
}

func TestRoadmapHistoryPagination(t *testing.T) {
	id := uuid.NewString()
	base := "/role-profiles/" + id + "/roadmap/items/" + id + "/history"
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=x", "?offset=-1", "?limit=5&limit=6", "?offset=1&offset=2", "?offset=99999999999"} {
		w := roadmapServe(roadmapRouter(&roadmapStub{}, true, "candidate"), base+query)
		if w.Code != http.StatusBadRequest || roadmapErrorCode(t, w) != string(apperror.CodeValidation) {
			t.Errorf("%s = %d %s, want 400 VALIDATION_ERROR", query, w.Code, w.Body.String())
		}
	}
	stub := &roadmapStub{entries: []domain.HistoryEntry{}, total: 7}
	w := roadmapServe(roadmapRouter(stub, true, "candidate"), base)
	if w.Code != http.StatusOK || stub.limit != 20 || stub.offset != 0 {
		t.Fatalf("defaults = %d limit %d offset %d, want 200 20 0", w.Code, stub.limit, stub.offset)
	}
	var body struct {
		Data struct {
			Items      []json.RawMessage `json:"items"`
			Pagination struct {
				Limit  int32 `json:"limit"`
				Offset int32 `json:"offset"`
				Total  int64 `json:"total"`
			} `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Items == nil || body.Data.Pagination.Total != 7 || body.Data.Pagination.Limit != 20 {
		t.Fatalf("body = %s", w.Body.String())
	}
	roadmapServe(roadmapRouter(stub, true, "candidate"), base+"?limit=100&offset=40")
	if stub.limit != 100 || stub.offset != 40 {
		t.Fatalf("limit/offset = %d/%d, want 100/40", stub.limit, stub.offset)
	}
}
