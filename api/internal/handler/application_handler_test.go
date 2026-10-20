package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	applicationdomain "github.com/cog-forge/rolecue/api/internal/features/application/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type screeningSpy struct {
	approveCalls, rejectCalls int
	userID, postingID, appID  uuid.UUID
	role                      string
	reason                    *string
	err                       error
}

func (s *screeningSpy) ApproveCV(_ context.Context, userID uuid.UUID, role string, postingID, appID uuid.UUID) (applicationdomain.Screening, error) {
	s.approveCalls++
	s.userID, s.role, s.postingID, s.appID = userID, role, postingID, appID
	return applicationdomain.Screening{ID: appID, Status: applicationdomain.StatusInterviewEligible}, s.err
}

func (s *screeningSpy) RejectCV(_ context.Context, userID uuid.UUID, role string, postingID, appID uuid.UUID, reason *string) (applicationdomain.Screening, error) {
	s.rejectCalls++
	s.userID, s.role, s.postingID, s.appID, s.reason = userID, role, postingID, appID, reason
	return applicationdomain.Screening{ID: appID, Status: applicationdomain.StatusRejected, RejectReason: reason}, s.err
}

func screeningRouter(user authdomain.User, service ApplicationService) *gin.Engine {
	r := gin.New()
	auth := middleware.RequireAuth(onboardingSession{user}, config.CORSConfig{AllowOrigins: []string{"http://localhost:3000"}})
	h := NewApplicationHandler(service)
	r.POST("/job-postings/:id/applications/:applicationId/approve-cv", auth, h.ApproveCV)
	r.POST("/job-postings/:id/applications/:applicationId/reject-cv", auth, h.RejectCV)
	return r
}

func screeningRequest(r *gin.Engine, path, contentType, body string) (*httptest.ResponseRecorder, response.Envelope) {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
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

func TestScreeningUsesSessionAndPathIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := authdomain.User{ID: uuid.New(), Role: "recruiter"}
	postingID, appID := uuid.New(), uuid.New()
	base := "/job-postings/" + postingID.String() + "/applications/" + appID.String()

	spy := &screeningSpy{}
	w, _ := screeningRequest(screeningRouter(user, spy), base+"/approve-cv", "", "")
	if w.Code != http.StatusOK || spy.approveCalls != 1 || spy.userID != user.ID || spy.role != "recruiter" || spy.postingID != postingID || spy.appID != appID {
		t.Fatalf("approve: status=%d body=%s spy=%+v", w.Code, w.Body.String(), spy)
	}

	for name, tc := range map[string]struct {
		contentType, body string
		want              *string
	}{
		"no body":         {"", "", nil},
		"empty object":    {"application/json", `{}`, nil},
		"null reason":     {"application/json", `{"reject_reason":null}`, nil},
		"with reason":     {"application/json; charset=utf-8", `{"reject_reason":"Not enough Go"}`, ptrTo("Not enough Go")},
		"trailing spaces": {"application/json", `{"reject_reason":"x"}  `, ptrTo("x")},
	} {
		t.Run(name, func(t *testing.T) {
			spy := &screeningSpy{}
			w, _ := screeningRequest(screeningRouter(user, spy), base+"/reject-cv", tc.contentType, tc.body)
			if w.Code != http.StatusOK || spy.rejectCalls != 1 || spy.userID != user.ID {
				t.Fatalf("reject: status=%d body=%s spy=%+v", w.Code, w.Body.String(), spy)
			}
			if (spy.reason == nil) != (tc.want == nil) || (tc.want != nil && *spy.reason != *tc.want) {
				t.Fatalf("reason = %v, want %v", spy.reason, tc.want)
			}
		})
	}
}

func TestRejectCVStrictJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := authdomain.User{ID: uuid.New(), Role: "recruiter"}
	path := "/job-postings/" + uuid.NewString() + "/applications/" + uuid.NewString() + "/reject-cv"
	for name, tc := range map[string]struct{ contentType, body string }{
		"unknown field":    {"application/json", `{"reject_reason":"x","status":"approved"}`},
		"trailing JSON":    {"application/json", `{"reject_reason":"x"}{}`},
		"wrong type":       {"application/json", `{"reject_reason":1}`},
		"malformed":        {"application/json", `{"reject_reason":`},
		"wrong content":    {"text/plain", `{"reject_reason":"x"}`},
		"missing content":  {"", `{"reject_reason":"x"}`},
		"oversized body":   {"application/json", `{"reject_reason":"x"}` + strings.Repeat(" ", 16<<10)},
		"array body":       {"application/json", `[]`},
		"client deadline":  {"application/json", `{"interview_deadline":"2030-01-01T00:00:00Z"}`},
		"client candidate": {"application/json", `{"candidate_id":"` + uuid.NewString() + `"}`},
	} {
		t.Run(name, func(t *testing.T) {
			spy := &screeningSpy{}
			w, body := screeningRequest(screeningRouter(user, spy), path, tc.contentType, tc.body)
			if w.Code != http.StatusBadRequest || body.Error == nil || body.Error.Code != apperror.CodeValidation || spy.rejectCalls != 0 {
				t.Fatalf("status=%d body=%s calls=%d; want 400 VALIDATION and no service call", w.Code, w.Body.String(), spy.rejectCalls)
			}
		})
	}
}

func TestScreeningRejectsBadIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := authdomain.User{ID: uuid.New(), Role: "recruiter"}
	good := uuid.NewString()
	for _, path := range []string{
		"/job-postings/not-a-uuid/applications/" + good + "/approve-cv",
		"/job-postings/" + good + "/applications/not-a-uuid/approve-cv",
		"/job-postings/not-a-uuid/applications/" + good + "/reject-cv",
		"/job-postings/" + good + "/applications/not-a-uuid/reject-cv",
	} {
		spy := &screeningSpy{}
		w, body := screeningRequest(screeningRouter(user, spy), path, "", "")
		if w.Code != http.StatusBadRequest || body.Error == nil || body.Error.Code != apperror.CodeValidation || spy.approveCalls+spy.rejectCalls != 0 {
			t.Fatalf("%s: status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestScreeningMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := authdomain.User{ID: uuid.New(), Role: "recruiter"}
	path := "/job-postings/" + uuid.NewString() + "/applications/" + uuid.NewString() + "/approve-cv"
	for _, tc := range []struct {
		err    *apperror.AppError
		status int
	}{
		{applicationdomain.ErrCapacityFull, http.StatusConflict},
		{applicationdomain.ErrCapacityNotFunded, http.StatusConflict},
		{applicationdomain.ErrIntakeNotOpen, http.StatusConflict},
		{applicationdomain.ErrApplicationNotPending, http.StatusConflict},
		{applicationdomain.ErrApplicationNotFound, http.StatusNotFound},
		{apperror.New(apperror.CodeForbidden, "only recruiters can screen CVs"), http.StatusForbidden},
	} {
		w, body := screeningRequest(screeningRouter(user, &screeningSpy{err: tc.err}), path, "", "")
		if w.Code != tc.status || body.Error == nil || body.Error.Code != tc.err.Code {
			t.Fatalf("%s: status=%d body=%s", tc.err.Code, w.Code, w.Body.String())
		}
	}
}

// A chunked request has no Content-Length; an empty one is still "no body".
func TestRejectCVChunkedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := authdomain.User{ID: uuid.New(), Role: "recruiter"}
	path := "/job-postings/" + uuid.NewString() + "/applications/" + uuid.NewString() + "/reject-cv"
	for _, tc := range []struct {
		name, body string
		status     int
		calls      int
	}{
		{"empty", "", http.StatusOK, 1},
		{"content without type", `{"reject_reason":"x"}`, http.StatusBadRequest, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spy := &screeningSpy{}
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(tc.body))
			req.ContentLength = -1
			req.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()
			screeningRouter(user, spy).ServeHTTP(w, req)
			if w.Code != tc.status || spy.rejectCalls != tc.calls || spy.reason != nil {
				t.Fatalf("status=%d body=%s calls=%d reason=%v", w.Code, w.Body.String(), spy.rejectCalls, spy.reason)
			}
		})
	}
}

func ptrTo(s string) *string { return &s }
