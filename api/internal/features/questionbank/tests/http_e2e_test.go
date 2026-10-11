//go:build integration && llm_smoke

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	qbprovider "github.com/cog-forge/rolecue/api/internal/features/questionbank/provider"
	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// End-to-end: real HTTP handlers + real service + real Gemini + real PostgreSQL. Only the login
// session is stubbed. It makes one real Gemini call (only the posting with confirmed requirements
// reaches the model), with synthetic requirements. See doc.go for the run command.

const trustedOrigin = "http://localhost:3000"

// sessionStub satisfies the auth service that the real RequireAuth middleware calls.
type sessionStub struct{ user authdomain.User }

func (s sessionStub) Authenticate(context.Context, *http.Request) (authdomain.User, []string, error) {
	return s.user, nil, nil
}

// newRouter mounts the real question-bank handlers behind the real RequireAuth middleware.
func newRouter(user authdomain.User, svc handler.QuestionBankService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewQuestionBankHandler(svc)
	auth := middleware.RequireAuth(sessionStub{user}, config.CORSConfig{AllowOrigins: []string{trustedOrigin}})
	r := gin.New()
	g := r.Group("/job-postings", auth)
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

type bankBody struct {
	Success bool `json:"success"`
	Data    struct {
		Questions []struct {
			ID      uuid.UUID `json:"id"`
			Content string    `json:"content"`
		} `json:"questions"`
	} `json:"data"`
}

func TestQuestionBankE2E(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal("invalid configuration")
	}
	if cfg.LLM.APIKey == "" {
		t.Skip("LLM_API_KEY must be set (load ../.env)")
	}
	size := cfg.QuestionBank.Size
	ctx, pool := openPool(t, 3*time.Minute)
	chatModel, err := ai.NewChatModel(ctx, cfg.LLM)
	if err != nil {
		t.Fatal("invalid Gemini configuration")
	}
	generator, err := qbprovider.New(chatModel)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := qbservice.New(qbrepository.NewRepository(pool), generator, cfg.LLM.MaxRetries, size)
	if err != nil {
		t.Fatal(err)
	}

	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	levelID := s.level("Senior", "e2e "+s.tag)
	recruiterID, otherRecruiterID, candidateID := s.user("recruiter"), s.user("recruiter"), s.user("candidate")
	jd1, posting1 := s.jdWithPosting(levelID, recruiterID, true)
	_, postingNoReq := s.jdWithPosting(levelID, recruiterID, false)

	asUser := func(id uuid.UUID, role string) *gin.Engine {
		return newRouter(authdomain.User{ID: id, Role: role, EmailVerified: true}, svc)
	}
	recruiterAPI := asUser(recruiterID, "recruiter")
	base := func(posting uuid.UUID) string { return "/job-postings/" + posting.String() + "/questions" }

	var bank bankBody
	t.Run("candidate is forbidden", func(t *testing.T) {
		w := do(asUser(candidateID, "candidate"), http.MethodPost, base(posting1)+"/generate", "", "")
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("another recruiter cannot see or generate", func(t *testing.T) {
		other := asUser(otherRecruiterID, "recruiter")
		if w := do(other, http.MethodGet, base(posting1), "", ""); w.Code != http.StatusNotFound {
			t.Fatalf("list: status = %d", w.Code)
		}
		if w := do(other, http.MethodPost, base(posting1)+"/generate", "", ""); w.Code != http.StatusNotFound {
			t.Fatalf("generate: status = %d", w.Code)
		}
	})
	t.Run("generation needs confirmed requirements", func(t *testing.T) {
		w := do(recruiterAPI, http.MethodPost, base(postingNoReq)+"/generate", "", "")
		if w.Code != http.StatusConflict || envelope(t, w).Error.Code != apperror.CodeRequirementsNotConfirmed {
			t.Fatalf("status = %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("empty bank lists as []", func(t *testing.T) {
		w := do(recruiterAPI, http.MethodGet, base(posting1), "", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"questions":[]`) {
			t.Fatalf("status = %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("generate with real Gemini", func(t *testing.T) {
		start := time.Now()
		w := do(recruiterAPI, http.MethodPost, base(posting1)+"/generate", "", "")
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d: %s", w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &bank); err != nil || len(bank.Data.Questions) != size {
			t.Fatalf("got %d questions, want %d (%v)", len(bank.Data.Questions), size, err)
		}
		t.Logf("generated %d questions in %s through the HTTP API", size, time.Since(start).Round(time.Millisecond))
		t.Logf("first question: %s", bank.Data.Questions[0].Content)
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.core_questions WHERE jd_id = $1`, jd1).Scan(&n); err != nil || n != size {
			t.Fatalf("database holds %d questions, want %d (%v)", n, size, err)
		}
	})
	t.Run("second generate is refused", func(t *testing.T) {
		w := do(recruiterAPI, http.MethodPost, base(posting1)+"/generate", "", "")
		if w.Code != http.StatusConflict || envelope(t, w).Error.Code != apperror.CodeQuestionBankExists {
			t.Fatalf("status = %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("list returns the stored bank", func(t *testing.T) {
		var listed bankBody
		w := do(recruiterAPI, http.MethodGet, base(posting1), "", "")
		if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed.Data.Questions) != size {
			t.Fatalf("status = %d, %d questions (%v)", w.Code, len(listed.Data.Questions), err)
		}
	})
	t.Run("add, edit and delete", func(t *testing.T) {
		w := do(recruiterAPI, http.MethodPost, base(posting1), "application/json", `{"content":"  What is your favourite Go proverb?  "}`)
		var added struct {
			Data struct {
				ID      uuid.UUID `json:"id"`
				Content string    `json:"content"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &added); err != nil || w.Code != http.StatusCreated || added.Data.Content != "What is your favourite Go proverb?" {
			t.Fatalf("add: status = %d: %s", w.Code, w.Body.String())
		}
		path := base(posting1) + "/" + added.Data.ID.String()
		if w := do(recruiterAPI, http.MethodPatch, path, "application/json", `{"content":"Edited question?"}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Edited question?") {
			t.Fatalf("edit: status = %d: %s", w.Code, w.Body.String())
		}
		if w := do(recruiterAPI, http.MethodDelete, path, "", ""); w.Code != http.StatusNoContent {
			t.Fatalf("delete: status = %d", w.Code)
		}
		if w := do(recruiterAPI, http.MethodDelete, path, "", ""); w.Code != http.StatusNotFound {
			t.Fatalf("second delete: status = %d", w.Code)
		}
	})
	t.Run("approved posting locks the bank", func(t *testing.T) {
		s.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, posting1)
		qid := bank.Data.Questions[0].ID.String()
		for name, w := range map[string]*struct {
			method, path, body string
		}{
			"add":      {http.MethodPost, base(posting1), `{"content":"Too late?"}`},
			"edit":     {http.MethodPatch, base(posting1) + "/" + qid, `{"content":"Too late?"}`},
			"delete":   {http.MethodDelete, base(posting1) + "/" + qid, ""},
			"generate": {http.MethodPost, base(posting1) + "/generate", ""},
		} {
			ct := ""
			if w.body != "" {
				ct = "application/json"
			}
			rec := do(recruiterAPI, w.method, w.path, ct, w.body)
			if rec.Code != http.StatusConflict || envelope(t, rec).Error.Code != apperror.CodeQuestionBankLocked {
				t.Fatalf("%s: status = %d: %s", name, rec.Code, rec.Body.String())
			}
		}
		if w := do(recruiterAPI, http.MethodGet, base(posting1), "", ""); w.Code != http.StatusOK {
			t.Fatalf("a locked bank must stay readable: status = %d", w.Code)
		}
	})
}
