//go:build integration && llm_smoke

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	qbprovider "github.com/cog-forge/rolecue/api/internal/features/questionbank/provider"
	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// End-to-end: real HTTP handlers + real service + real Gemini + real PostgreSQL. Only the login
// session is stubbed. Run: set -a; . ../.env; set +a; go test -tags "integration llm_smoke" ./internal/handler -run E2E
// It makes one real Gemini call (only the posting with confirmed requirements reaches the model), with synthetic requirements.

type e2eSeed struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
	tag  string
	undo []string
	args [][]any
}

func (s *e2eSeed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed: %v", err)
	}
}
func (s *e2eSeed) later(sql string, args ...any) {
	s.undo = append(s.undo, sql)
	s.args = append(s.args, args)
}
func (s *e2eSeed) cleanup() {
	for i := len(s.undo) - 1; i >= 0; i-- {
		if _, err := s.pool.Exec(context.Background(), s.undo[i], s.args[i]...); err != nil {
			s.t.Errorf("cleanup: %v", err)
		}
	}
}

// jdWithPosting seeds a JD with confirmed requirements and a pending posting owned by the recruiter.
func (s *e2eSeed) jdWithPosting(levelID, recruiterID uuid.UUID, withRequirements bool) (jdID, postingID uuid.UUID) {
	jdID, postingID = uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, refinement_note, content_path, level_id) VALUES ($1, 'Do not ask about Spring Boot.', $2, $3)`,
		jdID, "e2e/"+s.tag, levelID)
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	s.exec(`INSERT INTO public.job_postings (id, jd_id, recruiter_id, title) VALUES ($1, $2, $3, 'e2e posting')`, postingID, jdID, recruiterID)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, postingID)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jdID)
	if withRequirements {
		s.exec(`INSERT INTO public.requirements (jd_id, name, target, type) VALUES
			($1,'Go','backend services','required'), ($1,'PostgreSQL','data storage','required'),
			($1,'Kafka','event streaming','required'), ($1,'API design','REST APIs','required'),
			($1,'Automated testing','quality','required'), ($1,'Docker','deployment','preferred')`, jdID)
		s.later(`DELETE FROM public.requirements WHERE jd_id = $1`, jdID)
	}
	return jdID, postingID
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
	if cfg.Database.URL == "" || cfg.LLM.APIKey == "" {
		t.Skip("DATABASE_URL and LLM_API_KEY must be set (load ../.env)")
	}
	size := cfg.QuestionBank.Size
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
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

	s := &e2eSeed{t: t, pool: pool, ctx: ctx, tag: uuid.NewString()[:8]}
	defer s.cleanup()
	levelID := uuid.New()
	s.exec(`INSERT INTO public.levels (id, name) VALUES ($1, $2)`, levelID, "e2e-"+s.tag)
	s.later(`DELETE FROM public.levels WHERE id = $1`, levelID)
	mkUser := func(role string) uuid.UUID {
		id := uuid.New()
		s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, $4, true)`,
			id, "e2e "+role, fmt.Sprintf("e2e-%s-%s@example.test", s.tag, id), role)
		s.later(`DELETE FROM public.users WHERE id = $1`, id)
		return id
	}
	recruiterID, otherRecruiterID, candidateID := mkUser("recruiter"), mkUser("recruiter"), mkUser("candidate")
	jd1, posting1 := s.jdWithPosting(levelID, recruiterID, true)
	_, postingNoReq := s.jdWithPosting(levelID, recruiterID, false)

	asUser := func(id uuid.UUID, role string) *gin.Engine {
		return questionBankRouter(authdomain.User{ID: id, Role: role, EmailVerified: true}, svc)
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
