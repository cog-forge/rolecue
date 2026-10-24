package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeAdminInterviewsService struct {
	filter domain.ListFilter
	role   string
	calls  int
	page   domain.InterviewPage
	err    error
}

func (f *fakeAdminInterviewsService) List(_ context.Context, role string, filter domain.ListFilter) (domain.InterviewPage, error) {
	f.calls++
	f.role = role
	f.filter = filter
	// The real service owns the role check; the fake mirrors it so the 403 mapping is exercised.
	if role != "admin" {
		return domain.InterviewPage{}, apperror.New(apperror.CodeForbidden, "only administrators can view interview sessions")
	}
	return f.page, f.err
}

func newAdminInterviewsRouter(svc AdminInterviewsService, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("auth.middleware.currentUser", authdomain.User{ID: uuid.New(), Role: role})
		}
		c.Next()
	})
	engine.GET("/admin/interviews", NewAdminInterviewsHandler(svc).List)
	return engine
}

func getInterviews(t *testing.T, engine *gin.Engine, target string) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (%s)", err, recorder.Body.String())
	}
	return recorder.Code, body
}

func errorCode(body map[string]any) string {
	errBody, _ := body["error"].(map[string]any)
	code, _ := errBody["code"].(string)
	return code
}

func TestAdminInterviewsListSuccessEnvelope(t *testing.T) {
	svc := &fakeAdminInterviewsService{page: domain.InterviewPage{Items: []domain.InterviewRow{}, Total: 0, Page: 1, PageSize: 20}}
	status, body := getInterviews(t, newAdminInterviewsRouter(svc, "admin"), "/admin/interviews")
	if status != http.StatusOK || body["success"] != true {
		t.Fatalf("status %d body %v", status, body)
	}
	data, _ := body["data"].(map[string]any)
	if items, ok := data["items"].([]any); !ok || items == nil {
		t.Fatalf("items must be an empty array, got %v", data["items"])
	}
	if svc.filter.Page != 1 || svc.filter.PageSize != 20 || svc.filter.Type != "" {
		t.Fatalf("defaults not applied: %+v", svc.filter)
	}
}

func TestAdminInterviewsListParsesFilters(t *testing.T) {
	candidate := uuid.New()
	posting := uuid.New()
	svc := &fakeAdminInterviewsService{}
	target := "/admin/interviews?type=recruitment&candidate_id=" + candidate.String() +
		"&job_posting_id=" + posting.String() + "&page=2&page_size=5"
	status, _ := getInterviews(t, newAdminInterviewsRouter(svc, "admin"), target)
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	f := svc.filter
	if f.Type != "recruitment" || f.CandidateID == nil || *f.CandidateID != candidate ||
		f.JobPostingID == nil || *f.JobPostingID != posting || f.Page != 2 || f.PageSize != 5 {
		t.Fatalf("filters not parsed: %+v", f)
	}
}

func TestAdminInterviewsListRejectsMalformedQueries(t *testing.T) {
	targets := []string{
		"/admin/interviews?candidate_id=not-a-uuid",
		"/admin/interviews?job_posting_id=",
		"/admin/interviews?page=abc",
		"/admin/interviews?page_size=1.5",
		"/admin/interviews?type=practice&type=recruitment",
		"/admin/interviews?page=1&page=2",
	}
	for _, target := range targets {
		svc := &fakeAdminInterviewsService{}
		status, body := getInterviews(t, newAdminInterviewsRouter(svc, "admin"), target)
		if status != http.StatusBadRequest || errorCode(body) != "VALIDATION_ERROR" {
			t.Fatalf("%s: status %d code %q", target, status, errorCode(body))
		}
		if svc.calls != 0 {
			t.Fatalf("%s reached the service", target)
		}
	}
}

func TestAdminInterviewsListForbiddenMapsTo403(t *testing.T) {
	svc := &fakeAdminInterviewsService{}
	status, body := getInterviews(t, newAdminInterviewsRouter(svc, "candidate"), "/admin/interviews")
	if status != http.StatusForbidden || errorCode(body) != "FORBIDDEN" {
		t.Fatalf("status %d code %q", status, errorCode(body))
	}
	if svc.role != "candidate" {
		t.Fatalf("role not passed to the service: %q", svc.role)
	}
}

func TestAdminInterviewsListWithoutUserIsUnauthorized(t *testing.T) {
	svc := &fakeAdminInterviewsService{}
	status, body := getInterviews(t, newAdminInterviewsRouter(svc, ""), "/admin/interviews")
	if status != http.StatusUnauthorized || errorCode(body) != "INVALID_TOKEN" {
		t.Fatalf("status %d code %q", status, errorCode(body))
	}
	if svc.calls != 0 {
		t.Fatal("unauthenticated request reached the service")
	}
}
