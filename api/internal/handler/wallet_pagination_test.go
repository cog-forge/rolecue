package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type pagingWallet struct {
	WalletService
	calls          int
	page, pageSize int32
}

func (w *pagingWallet) ListTransactions(_ context.Context, _ uuid.UUID, page, pageSize int32) ([]walletdomain.Transaction, int64, error) {
	w.calls++
	w.page, w.pageSize = page, pageSize
	return nil, 0, nil
}

func TestWalletTransactionsPaginationRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := authdomain.User{ID: uuid.New(), Role: "candidate", OnboardingCompleted: true}
	for _, tc := range []struct {
		name, query string
		status      int
		calls       int
	}{
		{"defaults", "", http.StatusOK, 1},
		{"explicit", "?page=3&page_size=50", http.StatusOK, 1},
		{"page wraps int32", "?page=4294967297", http.StatusBadRequest, 0},
		{"page_size wraps int32", "?page_size=4294967316", http.StatusBadRequest, 0},
		{"page above int32", "?page=2147483648", http.StatusBadRequest, 0},
		{"not a number", "?page=abc", http.StatusBadRequest, 0},
		{"empty value", "?page=", http.StatusBadRequest, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &pagingWallet{}
			r := gin.New()
			r.GET("/wallet/transactions", middleware.RequireAuth(onboardingSession{user}, config.CORSConfig{}), NewWalletHandler(svc).ListTransactions)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wallet/transactions"+tc.query, nil))
			if w.Code != tc.status || svc.calls != tc.calls {
				t.Fatalf("status %d (want %d), service calls %d (want %d), body %s", w.Code, tc.status, svc.calls, tc.calls, w.Body)
			}
			if tc.status == http.StatusBadRequest && !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
				t.Fatalf("body = %s, want VALIDATION_ERROR", w.Body)
			}
		})
	}
}
