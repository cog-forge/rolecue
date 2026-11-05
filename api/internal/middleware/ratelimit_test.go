package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func rateLimitedRouter(limiter *UserRateLimiter, userID *uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/complete", func(c *gin.Context) {
		if userID != nil {
			c.Set(currentUserIDKey, *userID)
		}
	}, limiter.Middleware(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func post(router *gin.Engine) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/complete", nil))
	return recorder
}

func TestUserRateLimiterAnswers429AfterTheBurst(t *testing.T) {
	user := uuid.New()
	limiter := NewUserRateLimiter(time.Minute, 2)
	router := rateLimitedRouter(limiter, &user)

	for i := 0; i < 2; i++ {
		if got := post(router).Code; got != http.StatusNoContent {
			t.Fatalf("request %d = %d, want 204", i+1, got)
		}
	}
	limited := post(router)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("third request = %d, want 429", limited.Code)
	}
	if limited.Header().Get("Retry-After") == "" {
		t.Fatal("429 without Retry-After")
	}
	if body := limited.Body.String(); !strings.Contains(body, `"RATE_LIMITED"`) {
		t.Fatalf("body = %s, want the RATE_LIMITED code", body)
	}
}

func TestUserRateLimiterKeepsUsersApartAndRefills(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	limiter := NewUserRateLimiter(10*time.Second, 1)
	limiter.now = func() time.Time { return now }
	alice, bob := uuid.New(), uuid.New()

	if ok, _ := limiter.Allow(alice); !ok {
		t.Fatal("first request of alice refused")
	}
	if ok, wait := limiter.Allow(alice); ok || wait <= 0 {
		t.Fatalf("second request of alice = %v, %v; want refused with a wait", ok, wait)
	}
	if ok, _ := limiter.Allow(bob); !ok {
		t.Fatal("bob was limited by alice's requests")
	}
	now = now.Add(11 * time.Second)
	if ok, _ := limiter.Allow(alice); !ok {
		t.Fatal("alice was not refilled after the interval")
	}
}

func TestUserRateLimiterEvictsIdleBuckets(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	limiter := NewUserRateLimiter(time.Second, 1)
	limiter.now = func() time.Time { return now }
	for i := 0; i < 50; i++ {
		limiter.Allow(uuid.New())
	}
	now = now.Add(limiterIdleTTL + time.Minute)
	limiter.Allow(uuid.New())

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.buckets) != 1 {
		t.Fatalf("buckets = %d, want only the fresh one after eviction", len(limiter.buckets))
	}
}

func TestUserRateLimiterRejectsAnUnauthenticatedRequest(t *testing.T) {
	router := rateLimitedRouter(NewUserRateLimiter(time.Second, 5), nil)
	if got := post(router).Code; got != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a user", got)
	}
}
