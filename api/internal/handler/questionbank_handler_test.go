package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const trustedOrigin = "http://localhost:3000"

// sessionStub satisfies the auth service the real RequireAuth middleware calls.
type sessionStub struct{ user authdomain.User }

func (s sessionStub) Authenticate(context.Context, *http.Request) (authdomain.User, []string, error) {
	return s.user, nil, nil
}

func withUser(user authdomain.User) gin.HandlerFunc {
	return middleware.RequireAuth(sessionStub{user}, config.CORSConfig{AllowOrigins: []string{trustedOrigin}})
}

type questionBankStub struct {
	gotUser     uuid.UUID
	gotRole     string
	gotPosting  uuid.UUID
	gotQuestion uuid.UUID
	gotContent  string
	calls       int
	list        []domain.CoreQuestion
	question    domain.CoreQuestion
	err         error
}

func (s *questionBankStub) record(u uuid.UUID, role string, posting uuid.UUID) {
	s.calls++
	s.gotUser, s.gotRole, s.gotPosting = u, role, posting
}
func (s *questionBankStub) List(_ context.Context, u uuid.UUID, role string, posting uuid.UUID) ([]domain.CoreQuestion, error) {
	s.record(u, role, posting)
	return s.list, s.err
}
func (s *questionBankStub) Generate(_ context.Context, u uuid.UUID, role string, posting uuid.UUID) ([]domain.CoreQuestion, error) {
	s.record(u, role, posting)
	return s.list, s.err
}
func (s *questionBankStub) Add(_ context.Context, u uuid.UUID, role string, posting uuid.UUID, content string) (domain.CoreQuestion, error) {
	s.record(u, role, posting)
	s.gotContent = content
	return s.question, s.err
}
func (s *questionBankStub) Update(_ context.Context, u uuid.UUID, role string, posting, question uuid.UUID, content string) (domain.CoreQuestion, error) {
	s.record(u, role, posting)
	s.gotQuestion, s.gotContent = question, content
	return s.question, s.err
}
func (s *questionBankStub) Delete(_ context.Context, u uuid.UUID, role string, posting, question uuid.UUID) error {
	s.record(u, role, posting)
	s.gotQuestion = question
	return s.err
}

func questionBankRouter(user authdomain.User, svc QuestionBankService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewQuestionBankHandler(svc)
	r := gin.New()
	g := r.Group("/job-postings", withUser(user))
	g.GET("/:id/questions", h.List)
	g.POST("/:id/questions/generate", h.Generate)
	g.POST("/:id/questions", h.Add)
	g.PATCH("/:id/questions/:questionId", h.Update)
	g.DELETE("/:id/questions/:questionId", h.Delete)
	return r
}

func do(r *gin.Engine, method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Origin", trustedOrigin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func envelope(t *testing.T, w *httptest.ResponseRecorder) response.Envelope {
	t.Helper()
	var env response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON %q: %v", w.Body.String(), err)
	}
	return env
}

func recruiter() authdomain.User {
	return authdomain.User{ID: uuid.New(), Role: "recruiter", EmailVerified: true}
}

func TestQuestionBankReadAndGenerate(t *testing.T) {
	posting := uuid.New()
	for _, tc := range []struct {
		name, method, path string
		status             int
	}{
		{"list", http.MethodGet, "/job-postings/" + posting.String() + "/questions", http.StatusOK},
		{"generate", http.MethodPost, "/job-postings/" + posting.String() + "/questions/generate", http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := recruiter()
			svc := &questionBankStub{list: []domain.CoreQuestion{{ID: uuid.New(), Content: "What is a goroutine?"}}}
			w := do(questionBankRouter(user, svc), tc.method, tc.path, "", "")
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, tc.status, w.Body.String())
			}
			if svc.gotUser != user.ID || svc.gotRole != "recruiter" || svc.gotPosting != posting {
				t.Fatal("identity or posting ID did not come from the session and the path")
			}
			var body struct {
				Success bool `json:"success"`
				Data    struct {
					Questions []map[string]any `json:"questions"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !body.Success || len(body.Data.Questions) != 1 {
				t.Fatalf("unexpected body: %s", w.Body.String())
			}
			if len(body.Data.Questions[0]) != 2 || body.Data.Questions[0]["id"] == nil || body.Data.Questions[0]["content"] == nil {
				t.Fatalf("a question must expose exactly id and content: %s", w.Body.String())
			}
		})
	}
}

func TestQuestionBankEmptyListIsAnArray(t *testing.T) {
	svc := &questionBankStub{list: nil}
	w := do(questionBankRouter(recruiter(), svc), http.MethodGet, "/job-postings/"+uuid.NewString()+"/questions", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"questions":[]`) {
		t.Fatalf("empty bank must be [] not null: %d %s", w.Code, w.Body.String())
	}
}

func TestQuestionBankAddAndUpdate(t *testing.T) {
	posting, question := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name, method, path string
		status             int
	}{
		{"add", http.MethodPost, "/job-postings/" + posting.String() + "/questions", http.StatusCreated},
		{"update", http.MethodPatch, "/job-postings/" + posting.String() + "/questions/" + question.String(), http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := recruiter()
			svc := &questionBankStub{question: domain.CoreQuestion{ID: question, Content: "Explain channels."}}
			w := do(questionBankRouter(user, svc), tc.method, tc.path, "application/json", `{"content":"Explain channels."}`)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, tc.status, w.Body.String())
			}
			if svc.gotUser != user.ID || svc.gotContent != "Explain channels." || svc.gotPosting != posting {
				t.Fatalf("service got user=%v content=%q posting=%v", svc.gotUser, svc.gotContent, svc.gotPosting)
			}
			if tc.name == "update" && svc.gotQuestion != question {
				t.Fatal("question ID did not come from the path")
			}
			if !envelope(t, w).Success {
				t.Fatal("expected success envelope")
			}
		})
	}
}

func TestQuestionBankDeleteReturnsNoContent(t *testing.T) {
	svc := &questionBankStub{}
	user := recruiter()
	w := do(questionBankRouter(user, svc), http.MethodDelete, "/job-postings/"+uuid.NewString()+"/questions/"+uuid.NewString(), "", "")
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("status = %d body = %q", w.Code, w.Body.String())
	}
	if svc.gotUser != user.ID {
		t.Fatal("owner did not come from the session")
	}
}

func TestQuestionBankStrictRequestDecoding(t *testing.T) {
	posting := uuid.NewString()
	question := uuid.NewString()
	longBody := `{"content":"` + strings.Repeat("a", 17<<10) + `"}`
	for _, tc := range []struct {
		name, method, path, contentType, body string
	}{
		{"unknown field", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", `{"content":"q","jd_id":"` + uuid.NewString() + `"}`},
		{"user ID in body is rejected", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", `{"content":"q","recruiter_id":"` + uuid.NewString() + `"}`},
		{"trailing data", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", `{"content":"q"} {}`},
		{"missing content", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", `{}`},
		{"null content", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", `{"content":null}`},
		{"wrong type", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", `{"content":5}`},
		{"empty body", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", ``},
		{"wrong content type", http.MethodPost, "/job-postings/" + posting + "/questions", "text/plain", `{"content":"q"}`},
		{"no content type", http.MethodPost, "/job-postings/" + posting + "/questions", "", `{"content":"q"}`},
		{"oversized body", http.MethodPost, "/job-postings/" + posting + "/questions", "application/json", longBody},
		{"patch unknown field", http.MethodPatch, "/job-postings/" + posting + "/questions/" + question, "application/json", `{"content":"q","id":"x"}`},
		{"patch missing content", http.MethodPatch, "/job-postings/" + posting + "/questions/" + question, "application/json", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &questionBankStub{}
			w := do(questionBankRouter(recruiter(), svc), tc.method, tc.path, tc.contentType, tc.body)
			env := envelope(t, w)
			if w.Code != http.StatusBadRequest || env.Success || env.Error == nil || env.Error.Code != apperror.CodeValidation {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			if svc.calls != 0 {
				t.Fatal("a rejected request reached the service")
			}
		})
	}
}

func TestQuestionBankRejectsBadIDs(t *testing.T) {
	good := uuid.NewString()
	for _, tc := range []struct{ name, method, path, body string }{
		{"list posting", http.MethodGet, "/job-postings/not-a-uuid/questions", ""},
		{"generate posting", http.MethodPost, "/job-postings/123/questions/generate", ""},
		{"add posting", http.MethodPost, "/job-postings/x/questions", `{"content":"q"}`},
		{"update question", http.MethodPatch, "/job-postings/" + good + "/questions/nope", `{"content":"q"}`},
		{"delete question", http.MethodDelete, "/job-postings/" + good + "/questions/nope", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &questionBankStub{}
			ct := ""
			if tc.body != "" {
				ct = "application/json"
			}
			w := do(questionBankRouter(recruiter(), svc), tc.method, tc.path, ct, tc.body)
			if w.Code != http.StatusBadRequest || envelope(t, w).Error.Code != apperror.CodeValidation || svc.calls != 0 {
				t.Fatalf("status = %d calls = %d body = %s", w.Code, svc.calls, w.Body.String())
			}
		})
	}
}

func TestQuestionBankServiceErrorsMapToStatuses(t *testing.T) {
	for _, tc := range []struct {
		code   apperror.Code
		status int
	}{
		{apperror.CodeForbidden, http.StatusForbidden},
		{apperror.CodeJobPostingNotFound, http.StatusNotFound},
		{apperror.CodeCoreQuestionNotFound, http.StatusNotFound},
		{apperror.CodeQuestionBankExists, http.StatusConflict},
		{apperror.CodeQuestionBankLocked, http.StatusConflict},
		{apperror.CodeCoreQuestionInUse, http.StatusConflict},
		{apperror.CodeRequirementsNotConfirmed, http.StatusConflict},
		{apperror.CodeQuestionGenerationFailed, http.StatusBadGateway},
		{apperror.CodeInvalidQuestionGenerationOutput, http.StatusBadGateway},
		{apperror.CodeQuestionBankUnavailable, http.StatusServiceUnavailable},
	} {
		t.Run(string(tc.code), func(t *testing.T) {
			svc := &questionBankStub{err: apperror.Wrap(tc.code, "public message", context.DeadlineExceeded)}
			w := do(questionBankRouter(recruiter(), svc), http.MethodPost, "/job-postings/"+uuid.NewString()+"/questions/generate", "", "")
			env := envelope(t, w)
			if w.Code != tc.status || env.Success || env.Error == nil || env.Error.Code != tc.code {
				t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "deadline") {
				t.Fatalf("internal cause leaked: %s", w.Body.String())
			}
		})
	}
}

func TestQuestionBankWritesNeedTrustedOrigin(t *testing.T) {
	svc := &questionBankStub{}
	r := questionBankRouter(recruiter(), svc)
	req := httptest.NewRequest(http.MethodPost, "/job-postings/"+uuid.NewString()+"/questions", strings.NewReader(`{"content":"q"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || svc.calls != 0 {
		t.Fatalf("status = %d calls = %d", w.Code, svc.calls)
	}
}

func TestQuestionBankRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &questionBankStub{}
	h := NewQuestionBankHandler(svc)
	r := gin.New()
	r.GET("/job-postings/:id/questions", h.List) // no RequireAuth: no user in context
	w := do(r, http.MethodGet, "/job-postings/"+uuid.NewString()+"/questions", "", "")
	if w.Code != http.StatusUnauthorized || svc.calls != 0 {
		t.Fatalf("status = %d calls = %d", w.Code, svc.calls)
	}
}
