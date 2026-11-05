package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type countingBudget struct {
	left  int
	calls int
}

func (b *countingBudget) Consume(context.Context, uuid.UUID) error {
	b.calls++
	if b.left <= 0 {
		return apperror.New(apperror.CodeLLMBudgetExceeded, "spent")
	}
	b.left--
	return nil
}

func limitedEngine(chain []gin.HandlerFunc, user uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// RequireAuth stores the user under this key; the real middleware needs a live auth service.
	engine.POST("/x", with(append([]gin.HandlerFunc{func(c *gin.Context) { c.Set("auth.middleware.currentUserID", user) }}, chain...),
		func(c *gin.Context) { c.Status(http.StatusNoContent) })...)
	return engine
}

func hit(engine *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func realLimiters() (func(int, int) gin.HandlerFunc, func(int, int) gin.HandlerFunc) {
	ip := func(perMinute, burst int) gin.HandlerFunc {
		return middleware.NewIPRateLimiter(middleware.PerMinute(perMinute), burst).Middleware()
	}
	user := func(perMinute, burst int) gin.HandlerFunc {
		return middleware.NewUserRateLimiter(middleware.PerMinute(perMinute), burst).Middleware()
	}
	return ip, user
}

func TestLLMChainRateLimitsBeforeSpendingBudget(t *testing.T) {
	ip, user := realLimiters()
	budget := &countingBudget{left: 100}
	limits := buildRequestLimits(config.RateLimitConfig{Enabled: true, LLMPerMinute: 1, LLMBurst: 2, IPPerMinute: 1000, IPBurst: 1000}, ip, user, budget)
	engine := limitedEngine(limits.llm, uuid.New())

	for i := 0; i < 2; i++ {
		if got := hit(engine).Code; got != http.StatusNoContent {
			t.Fatalf("request %d = %d, want 204", i+1, got)
		}
	}
	rec := hit(engine)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), `"RATE_LIMITED"`) {
		t.Fatalf("third request = %d %s retry=%q, want 429 RATE_LIMITED with Retry-After", rec.Code, rec.Body.String(), rec.Header().Get("Retry-After"))
	}
	if budget.calls != 2 {
		t.Fatalf("budget charged %d times, want 2 (a rate-limited request must not burn budget)", budget.calls)
	}
}

func TestLLMChainAnswersBudgetExceeded(t *testing.T) {
	ip, user := realLimiters()
	limits := buildRequestLimits(config.RateLimitConfig{Enabled: true, LLMPerMinute: 1000, LLMBurst: 1000, IPPerMinute: 1000, IPBurst: 1000}, ip, user, &countingBudget{left: 1})
	engine := limitedEngine(limits.llm, uuid.New())

	if got := hit(engine).Code; got != http.StatusNoContent {
		t.Fatalf("first = %d, want 204", got)
	}
	rec := hit(engine)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"LLM_BUDGET_EXCEEDED"`) {
		t.Fatalf("second = %d %s, want 429 LLM_BUDGET_EXCEEDED", rec.Code, rec.Body.String())
	}
}

func TestDisabledLimitsAddNothingAndNilBudgetIsNotCharged(t *testing.T) {
	ip, user := realLimiters()
	limits := buildRequestLimits(config.RateLimitConfig{}, ip, user, nil)
	if len(limits.llm)+len(limits.llmOwnLimiter)+len(limits.expensive) != 0 {
		t.Fatalf("zero config produced chains: %d %d %d", len(limits.llm), len(limits.llmOwnLimiter), len(limits.expensive))
	}
	// A zero per-minute turns one limiter off while the others stay.
	limits = buildRequestLimits(config.RateLimitConfig{Enabled: true, ExpensivePerMinute: 5, ExpensiveBurst: 1}, ip, user, nil)
	if len(limits.expensive) != 1 || len(limits.llm) != 0 {
		t.Fatalf("expensive=%d llm=%d, want 1 and 0", len(limits.expensive), len(limits.llm))
	}
}

func TestExpensiveChainLimitsPerIPAcrossUsersAndResets(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	ipLimiter := middleware.NewIPRateLimiter(middleware.PerMinute(6), 2)
	ipLimiter.SetClock(func() time.Time { return now })
	chain := []gin.HandlerFunc{ipLimiter.Middleware()}

	// Two different accounts from one address share the address bucket.
	a, b := limitedEngine(chain, uuid.New()), limitedEngine(chain, uuid.New())
	if hit(a).Code != http.StatusNoContent || hit(b).Code != http.StatusNoContent {
		t.Fatal("first two requests from the address were refused")
	}
	if got := hit(a).Code; got != http.StatusTooManyRequests {
		t.Fatalf("third request from the address = %d, want 429", got)
	}
	now = now.Add(11 * time.Second)
	if got := hit(b).Code; got != http.StatusNoContent {
		t.Fatalf("after the refill = %d, want 204", got)
	}
}
