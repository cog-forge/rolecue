package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ipRouter(limiter *IPRateLimiter, trusted []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	_ = router.SetTrustedProxies(trusted)
	router.POST("/x", limiter.Middleware(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func postFrom(router *gin.Engine, remote, forwarded string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = remote + ":1234"
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestIPRateLimiterLimitsPerAddressAndResets(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	limiter := NewIPRateLimiter(PerMinute(6), 2) // 2 at once, then one every 10s
	limiter.now = func() time.Time { return now }
	router := ipRouter(limiter, nil)

	for i := 0; i < 2; i++ {
		if got := postFrom(router, "203.0.113.5", "").Code; got != http.StatusNoContent {
			t.Fatalf("request %d = %d, want 204", i+1, got)
		}
	}
	limited := postFrom(router, "203.0.113.5", "")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("third request = %d, want 429", limited.Code)
	}
	if got := limited.Header().Get("Retry-After"); got != "10" {
		t.Fatalf("Retry-After = %q, want 10", got)
	}
	if !strings.Contains(limited.Body.String(), `"RATE_LIMITED"`) {
		t.Fatalf("body = %s, want RATE_LIMITED", limited.Body.String())
	}
	if got := postFrom(router, "203.0.113.6", "").Code; got != http.StatusNoContent {
		t.Fatalf("another address = %d, want 204", got)
	}

	now = now.Add(11 * time.Second)
	if got := postFrom(router, "203.0.113.5", "").Code; got != http.StatusNoContent {
		t.Fatalf("after the refill = %d, want 204", got)
	}
}

func TestIPRateLimiterIgnoresForgedForwardedFor(t *testing.T) {
	limiter := NewIPRateLimiter(time.Hour, 1)
	router := ipRouter(limiter, nil) // no trusted proxy: X-Forwarded-For is never believed

	if got := postFrom(router, "203.0.113.5", "10.0.0.1").Code; got != http.StatusNoContent {
		t.Fatalf("first = %d", got)
	}
	if got := postFrom(router, "203.0.113.5", "10.0.0.2").Code; got != http.StatusTooManyRequests {
		t.Fatalf("a forged X-Forwarded-For got a fresh bucket: %d, want 429", got)
	}
}

func TestIPRateLimiterUsesForwardedForBehindATrustedProxy(t *testing.T) {
	limiter := NewIPRateLimiter(time.Hour, 1)
	router := ipRouter(limiter, []string{"127.0.0.1"})

	if got := postFrom(router, "127.0.0.1", "198.51.100.1").Code; got != http.StatusNoContent {
		t.Fatalf("first client = %d", got)
	}
	if got := postFrom(router, "127.0.0.1", "198.51.100.2").Code; got != http.StatusNoContent {
		t.Fatalf("second client behind the proxy shared the proxy bucket: %d", got)
	}
	if got := postFrom(router, "127.0.0.1", "198.51.100.1").Code; got != http.StatusTooManyRequests {
		t.Fatalf("first client again = %d, want 429", got)
	}
}

func TestPerMinute(t *testing.T) {
	if got := PerMinute(60); got != time.Second {
		t.Fatalf("PerMinute(60) = %s", got)
	}
	if got := PerMinute(0); got != time.Minute {
		t.Fatalf("PerMinute(0) = %s", got)
	}
}

type budgetStub struct {
	calls int
	err   error
}

func (b *budgetStub) Consume(context.Context, uuid.UUID) error {
	b.calls++
	return b.err
}

func budgetRouter(budget LLMBudget, user *uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/x", func(c *gin.Context) {
		if user != nil {
			c.Set(currentUserIDKey, *user)
		}
	}, LLMBudgetMiddleware(budget), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func TestLLMBudgetMiddlewareAnswers429WhenSpent(t *testing.T) {
	user := uuid.New()
	stub := &budgetStub{}
	router := budgetRouter(stub, &user)
	if got := post2(router).Code; got != http.StatusNoContent || stub.calls != 1 {
		t.Fatalf("status = %d calls = %d, want 204 and 1 charge", got, stub.calls)
	}
	stub.err = apperror.New(apperror.CodeLLMBudgetExceeded, "spent")
	rec := post2(router)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"LLM_BUDGET_EXCEEDED"`) {
		t.Fatalf("status = %d body = %s, want 429 LLM_BUDGET_EXCEEDED", rec.Code, rec.Body.String())
	}
}

func TestLLMBudgetMiddlewareNilBudgetAndMissingUser(t *testing.T) {
	user := uuid.New()
	if got := post2(budgetRouter(nil, &user)).Code; got != http.StatusNoContent {
		t.Fatalf("nil budget = %d, want 204", got)
	}
	if got := post2(budgetRouter(&budgetStub{}, nil)).Code; got != http.StatusUnauthorized {
		t.Fatalf("no user = %d, want 401", got)
	}
}

func post2(router *gin.Engine) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	return rec
}
