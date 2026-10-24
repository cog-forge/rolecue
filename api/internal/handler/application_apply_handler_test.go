package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	applicationdomain "github.com/cog-forge/rolecue/api/internal/features/application/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// candidateSpy records the Candidate calls and answers with the configured values. It also serves
// the screening methods through screeningSpy, so it can stand where the handler needs a service.
type candidateSpy struct {
	*screeningSpy

	submission  applicationdomain.Submission
	page        applicationdomain.CandidatePage
	application applicationdomain.CandidateApplication
	err         error

	applyCalls   int
	applyUser    uuid.UUID
	applyRole    string
	applyPosting uuid.UUID
	applyData    []byte
	listFilter   applicationdomain.CandidateListFilter
	getCalls     int
	getID        uuid.UUID
}

func (s *candidateSpy) Apply(_ context.Context, userID uuid.UUID, role string, postingID uuid.UUID, data []byte) (applicationdomain.Submission, error) {
	s.applyCalls++
	s.applyUser, s.applyRole, s.applyPosting, s.applyData = userID, role, postingID, data
	return s.submission, s.err
}

func (s *candidateSpy) ListMine(_ context.Context, _ uuid.UUID, _ string, filter applicationdomain.CandidateListFilter) (applicationdomain.CandidatePage, error) {
	s.listFilter = filter
	return s.page, s.err
}

func (s *candidateSpy) GetMine(_ context.Context, _ uuid.UUID, _ string, applicationID uuid.UUID) (applicationdomain.CandidateApplication, error) {
	s.getCalls++
	s.getID = applicationID
	return s.application, s.err
}

func candidateRouter(user authdomain.User, service ApplicationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	auth := middleware.RequireAuth(onboardingSession{user}, config.CORSConfig{AllowOrigins: []string{"http://localhost:3000"}})
	h := NewApplicationHandler(service)
	r.POST("/job-board/:id/applications", auth, h.Apply)
	r.GET("/applications", auth, h.ListMine)
	r.GET("/applications/:id", auth, h.DetailMine)
	return r
}

func candidateRequest(r *gin.Engine, method, path, contentType string, body []byte) (*httptest.ResponseRecorder, response.Envelope) {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Origin", "http://localhost:3000")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var envelope response.Envelope
	_ = json.Unmarshal(w.Body.Bytes(), &envelope)
	return w, envelope
}

// cvMultipart builds a multipart body with the CV under the given field name.
func cvMultipart(t *testing.T, field string, data []byte) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, "cv.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType(), buf.Bytes()
}

func TestApplyPassesCandidateAndCVToService(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	postingID := uuid.New()
	spy := &candidateSpy{screeningSpy: &screeningSpy{}, submission: applicationdomain.Submission{
		ID: uuid.New(), JobPostingID: postingID, Status: applicationdomain.StatusPending, CreatedAt: time.Now(),
	}}
	pdf := []byte("%PDF-1.7 candidate cv")
	contentType, body := cvMultipart(t, "file", pdf)

	w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/"+postingID.String()+"/applications", contentType, body)
	if w.Code != http.StatusCreated || !envelope.Success {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if spy.applyCalls != 1 || spy.applyUser != user.ID || spy.applyRole != "candidate" || spy.applyPosting != postingID {
		t.Fatalf("service call = %+v", spy)
	}
	if !bytes.Equal(spy.applyData, pdf) {
		t.Fatal("the service did not receive the CV bytes")
	}
	data, _ := envelope.Data.(map[string]any)
	if data["status"] != "pending" || data["id"] == nil {
		t.Fatalf("response data = %v", envelope.Data)
	}
}

func TestApplyRejectsNonCandidatesBeforeReadingTheBody(t *testing.T) {
	postingID := uuid.New()
	contentType, body := cvMultipart(t, "file", []byte("%PDF-1.7"))
	for _, role := range []string{"recruiter", "admin"} {
		spy := &candidateSpy{screeningSpy: &screeningSpy{}}
		user := authdomain.User{ID: uuid.New(), Role: role}
		w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/"+postingID.String()+"/applications", contentType, body)
		if w.Code != http.StatusForbidden || envelope.Error == nil || envelope.Error.Code != apperror.CodeForbidden {
			t.Fatalf("role %s: status=%d body=%s", role, w.Code, w.Body.String())
		}
		if spy.applyCalls != 0 {
			t.Fatalf("role %s reached the service", role)
		}
	}
}

func TestApplyRejectsBadBodiesAsValidation(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	postingID := uuid.New()
	wrongField, wrongFieldBody := cvMultipart(t, "document", []byte("%PDF-1.7"))
	cases := []struct {
		name        string
		contentType string
		body        []byte
	}{
		{"not multipart", "application/json", []byte(`{"file":"x"}`)},
		{"wrong field name", wrongField, wrongFieldBody},
		{"no body", "", nil},
	}
	for _, tc := range cases {
		spy := &candidateSpy{screeningSpy: &screeningSpy{}}
		w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/"+postingID.String()+"/applications", tc.contentType, tc.body)
		if w.Code != http.StatusBadRequest || envelope.Error == nil || envelope.Error.Code != apperror.CodeValidation {
			t.Fatalf("%s: status=%d body=%s", tc.name, w.Code, w.Body.String())
		}
		if spy.applyCalls != 0 {
			t.Fatalf("%s reached the service", tc.name)
		}
	}
}

func TestApplyRejectsInvalidPostingID(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	contentType, body := cvMultipart(t, "file", []byte("%PDF-1.7"))
	spy := &candidateSpy{screeningSpy: &screeningSpy{}}
	w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/not-a-uuid/applications", contentType, body)
	if w.Code != http.StatusBadRequest || envelope.Error == nil || envelope.Error.Code != apperror.CodeValidation {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestApplyMapsServiceErrors(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	postingID := uuid.New()
	contentType, body := cvMultipart(t, "file", []byte("%PDF-1.7"))
	spy := &candidateSpy{screeningSpy: &screeningSpy{}, err: applicationdomain.ErrIntakeNotOpen}
	w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/"+postingID.String()+"/applications", contentType, body)
	if w.Code != http.StatusConflict || envelope.Error == nil || envelope.Error.Code != apperror.CodeIntakeNotOpen {
		t.Fatalf("INTAKE_NOT_OPEN: status=%d body=%s", w.Code, w.Body.String())
	}

	// ALREADY_APPLIED has no HTTP status in the shared table yet (plan step S0). The envelope code is
	// what clients match on, so only that is checked here.
	spy.err = applicationdomain.ErrAlreadyApplied
	_, envelope = candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/"+postingID.String()+"/applications", contentType, body)
	if envelope.Error == nil || envelope.Error.Code != "ALREADY_APPLIED" {
		t.Fatalf("ALREADY_APPLIED: body=%+v", envelope)
	}

	spy.err = errors.New("database unavailable")
	w, _ = candidateRequest(candidateRouter(user, spy), http.MethodPost, "/job-board/"+postingID.String()+"/applications", contentType, body)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected error: status=%d", w.Code)
	}
}

func TestListMineReadsPagingAndReturnsPage(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	spy := &candidateSpy{screeningSpy: &screeningSpy{}, page: applicationdomain.CandidatePage{
		Items: []applicationdomain.CandidateApplication{}, Total: 0, Page: 2, PageSize: 5,
	}}
	w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodGet, "/applications?page=2&page_size=5", "", nil)
	if w.Code != http.StatusOK || !envelope.Success {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if spy.listFilter.Page != 2 || spy.listFilter.PageSize != 5 {
		t.Fatalf("filter = %+v", spy.listFilter)
	}
	data, _ := envelope.Data.(map[string]any)
	if _, ok := data["items"].([]any); !ok || data["page"] != float64(2) || data["page_size"] != float64(5) || data["total"] != float64(0) {
		t.Fatalf("page body = %v", envelope.Data)
	}
}

func TestListMineDefaultsAndRejectsBadPaging(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	spy := &candidateSpy{screeningSpy: &screeningSpy{}}
	candidateRequest(candidateRouter(user, spy), http.MethodGet, "/applications", "", nil)
	if spy.listFilter.Page != 1 || spy.listFilter.PageSize != 20 {
		t.Fatalf("defaults = %+v", spy.listFilter)
	}
	for _, query := range []string{"?page=abc", "?page_size=x", "?page=1&page=2"} {
		spy := &candidateSpy{screeningSpy: &screeningSpy{}}
		w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodGet, "/applications"+query, "", nil)
		if w.Code != http.StatusBadRequest || envelope.Error == nil || envelope.Error.Code != apperror.CodeValidation {
			t.Fatalf("%s: status=%d body=%s", query, w.Code, w.Body.String())
		}
	}
}

func TestDetailMineReadsIDAndMapsNotFound(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate"}
	applicationID := uuid.New()
	spy := &candidateSpy{screeningSpy: &screeningSpy{}, application: applicationdomain.CandidateApplication{
		ID: applicationID, Status: applicationdomain.StatusPending, CreatedAt: time.Now(),
	}}
	w, envelope := candidateRequest(candidateRouter(user, spy), http.MethodGet, "/applications/"+applicationID.String(), "", nil)
	if w.Code != http.StatusOK || !envelope.Success || spy.getID != applicationID {
		t.Fatalf("status=%d getID=%s body=%s", w.Code, spy.getID, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "cv_path") || strings.Contains(w.Body.String(), "transcript") {
		t.Fatal("candidate detail leaked evidence fields")
	}

	w, envelope = candidateRequest(candidateRouter(user, spy), http.MethodGet, "/applications/not-a-uuid", "", nil)
	if w.Code != http.StatusBadRequest || envelope.Error == nil || envelope.Error.Code != apperror.CodeValidation {
		t.Fatalf("bad id: status=%d body=%s", w.Code, w.Body.String())
	}

	spy.err = applicationdomain.ErrApplicationNotFound
	w, _ = candidateRequest(candidateRouter(user, spy), http.MethodGet, "/applications/"+applicationID.String(), "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("not found: status=%d", w.Code)
	}
}
