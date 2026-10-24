package router

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
	authservice "github.com/cog-forge/rolecue/api/internal/features/auth/service"
	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type routerAuth struct {
	authservice.AuthService
	user authdomain.User
	err  error
}

func (a routerAuth) Authenticate(context.Context, *http.Request) (authdomain.User, []string, error) {
	return a.user, nil, a.err
}

func TestQuestionBankRoutesRequireOnboarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const origin = "http://localhost:3000"
	routes := []struct{ method, path string }{
		{http.MethodGet, "/job-postings/invalid-id/questions"},
		{http.MethodPost, "/job-postings/invalid-id/questions/generate"},
		{http.MethodPost, "/job-postings/invalid-id/questions"},
		{http.MethodPatch, "/job-postings/invalid-id/questions/invalid-id"},
		{http.MethodDelete, "/job-postings/invalid-id/questions/invalid-id"},
		{http.MethodDelete, "/job-postings/invalid-id/evaluation-weights"},
		{http.MethodGet, "/job-postings/invalid-id/applications"},
		{http.MethodPost, "/job-postings/invalid-id/applications/invalid-id/approve-cv"},
		{http.MethodPost, "/job-postings/invalid-id/applications/invalid-id/reject-cv"},
	}
	for _, tc := range []struct {
		name      string
		completed bool
		selected  bool
		authError error
		status    int
		code      apperror.Code
	}{
		{"unauthenticated", false, false, authdomain.ErrInvalidSession, http.StatusUnauthorized, apperror.CodeInvalidToken},
		{"onboarding pending", false, false, nil, http.StatusForbidden, apperror.CodeForbidden},
		{"role selected", false, true, nil, http.StatusForbidden, apperror.CodeForbidden},
		// An invalid ID proves the completed user reaches handler validation without
		// calling the database or LLM; pending users must be rejected before it.
		{"onboarding complete", true, true, nil, http.StatusBadRequest, apperror.CodeValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := routerAuth{
				user: authdomain.User{Role: "recruiter", EmailVerified: true, OnboardingCompleted: tc.completed, OnboardingRoleSelected: tc.selected},
				err:  tc.authError,
			}
			cfg := &config.Config{CORS: config.CORSConfig{AllowOrigins: []string{origin}}}
			r := NewRouter(cfg, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(auth),
				handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
				handler.NewWalletHandler(nil), handler.NewQuestionBankHandler(nil),
				&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}).Setup()
			for _, route := range routes {
				t.Run(route.method+" "+route.path, func(t *testing.T) {
					req := httptest.NewRequest(route.method, route.path, nil)
					req.Header.Set("Origin", origin)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					var body response.Envelope
					if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if w.Code != tc.status || body.Error == nil || body.Error.Code != tc.code {
						t.Fatalf("status=%d body=%s, want %d %s", w.Code, w.Body.String(), tc.status, tc.code)
					}
				})
			}
			if tc.authError == nil {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/me", nil))
				if w.Code != http.StatusOK {
					t.Fatalf("bootstrap must stay accessible: status=%d body=%s", w.Code, w.Body.String())
				}
			}
		})
	}
}

type routerWallet struct {
	calls int
}

func (s *routerWallet) GetWallet(context.Context, uuid.UUID) (walletdomain.Wallet, error) {
	s.calls++
	return walletdomain.Wallet{}, nil
}

func (s *routerWallet) ListTransactions(context.Context, uuid.UUID, int32, int32) ([]walletdomain.Transaction, int64, error) {
	s.calls++
	return []walletdomain.Transaction{}, 0, nil
}

func (s *routerWallet) ListPackages(context.Context) ([]walletdomain.CoinPackage, error) {
	s.calls++
	return []walletdomain.CoinPackage{}, nil
}

func (s *routerWallet) CreateDepositCheckout(context.Context, uuid.UUID, uuid.UUID, string, string) (walletdomain.DepositCheckoutResponse, error) {
	s.calls++
	return walletdomain.DepositCheckoutResponse{}, nil
}

func (s *routerWallet) SyncDepositStatus(context.Context, uuid.UUID, int64) (walletdomain.Transaction, error) {
	s.calls++
	return walletdomain.Transaction{}, nil
}

func (s *routerWallet) HandlePayOSWebhook(context.Context, []byte) error {
	s.calls++
	return nil
}

func (s *routerWallet) AdminListTransactions(context.Context, walletdomain.AdminTransactionFilter) (walletdomain.AdminTransactionPage, error) {
	s.calls++
	return walletdomain.AdminTransactionPage{}, nil
}

func (s *routerWallet) GetRevenueReport(context.Context, *time.Time, *time.Time) (walletdomain.RevenueReport, error) {
	s.calls++
	return walletdomain.RevenueReport{}, nil
}

func (s *routerWallet) GetBillingSummary(context.Context, walletdomain.AdminTransactionFilter) (walletdomain.BillingSummary, error) {
	s.calls++
	return walletdomain.BillingSummary{}, nil
}

func (s *routerWallet) GetBillingDailySeries(context.Context, walletdomain.AdminTransactionFilter) ([]walletdomain.BillingDailyItem, error) {
	s.calls++
	return []walletdomain.BillingDailyItem{}, nil
}

func (s *routerWallet) ListBillingTransactions(context.Context, walletdomain.AdminTransactionFilter) (walletdomain.AdminTransactionPage, error) {
	s.calls++
	return walletdomain.AdminTransactionPage{}, nil
}

func (s *routerWallet) GetBillingTransaction(context.Context, uuid.UUID) (*walletdomain.AdminTransactionItem, error) {
	s.calls++
	return &walletdomain.AdminTransactionItem{}, nil
}

func (s *routerWallet) SyncPayOSTransaction(context.Context, uuid.UUID, uuid.UUID) (*walletdomain.AdminTransactionItem, error) {
	s.calls++
	return &walletdomain.AdminTransactionItem{}, nil
}

func (s *routerWallet) CancelTransaction(context.Context, uuid.UUID, uuid.UUID, string) (*walletdomain.AdminTransactionItem, error) {
	s.calls++
	return &walletdomain.AdminTransactionItem{}, nil
}

func (s *routerWallet) IssueRefund(context.Context, walletdomain.IssueRefundRequest, uuid.UUID) (*walletdomain.AdminTransactionItem, error) {
	s.calls++
	return &walletdomain.AdminTransactionItem{}, nil
}

func TestWalletRoutesRequireOnboarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const origin = "http://localhost:3000"
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/wallet", ""},
		{http.MethodGet, "/wallet/transactions", ""},
		{http.MethodGet, "/wallet/packages", ""},
		{http.MethodPost, "/wallet/deposit/checkout", `{"package_id":"00000000-0000-0000-0000-000000000001"}`},
		{http.MethodPost, "/wallet/deposit/sync", `{"order_code":100201}`},
	}
	for _, role := range []string{"candidate", "recruiter"} {
		for _, tc := range []struct {
			name                string
			selected, completed bool
			authError           error
			status, calls       int
		}{
			{"unauthenticated", false, false, authdomain.ErrInvalidSession, http.StatusUnauthorized, 0},
			{"onboarding pending", false, false, nil, http.StatusForbidden, 0},
			{"role selected", true, false, nil, http.StatusForbidden, 0},
			{"onboarding complete", true, true, nil, http.StatusOK, 1},
		} {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				auth := routerAuth{
					user: authdomain.User{ID: uuid.New(), Role: role, EmailVerified: true,
						OnboardingRoleSelected: tc.selected, OnboardingCompleted: tc.completed},
					err: tc.authError,
				}
				wallet := &routerWallet{}
				cfg := &config.Config{CORS: config.CORSConfig{AllowOrigins: []string{origin}}}
				r := NewRouter(cfg, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(auth),
					handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
					handler.NewWalletHandler(wallet), handler.NewQuestionBankHandler(nil),
					&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}).Setup()
				for _, route := range routes {
					t.Run(route.method+" "+route.path, func(t *testing.T) {
						wallet.calls = 0
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Origin", origin)
						req.Header.Set("Content-Type", "application/json")
						w := httptest.NewRecorder()
						r.ServeHTTP(w, req)
						if w.Code != tc.status || wallet.calls != tc.calls {
							t.Fatalf("status=%d calls=%d body=%s, want %d and %d calls", w.Code, wallet.calls, w.Body.String(), tc.status, tc.calls)
						}
						if tc.status == http.StatusForbidden {
							var body response.Envelope
							if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
								t.Fatal(err)
							}
							if body.Error == nil || body.Error.Code != apperror.CodeForbidden || body.Error.Message != "complete onboarding before using your workspace" {
								t.Fatalf("unexpected onboarding error: %s", w.Body.String())
							}
						}
					})
				}
			})
		}
	}
}

func TestPayOSWebhookDoesNotRequireSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wallet := &routerWallet{}
	auth := routerAuth{err: authdomain.ErrInvalidSession}
	r := NewRouter(&config.Config{}, &logger.Logger{Logger: zap.NewNop()}, handler.NewAuthHandler(auth),
		handler.NewHealthHandler(), &handler.RoleProfileHandler{}, &handler.ProfileHandler{},
		handler.NewWalletHandler(wallet), handler.NewQuestionBankHandler(nil),
		&handler.ModerationHandler{}, &handler.OnboardingHandler{}, &handler.AccountsHandler{}, &handler.InventoryHandler{}, &handler.AvatarCreateHandler{}, &handler.VoiceCatalogHandler{}, &handler.JobPostingHandler{}, &handler.VoiceHandler{}, &handler.JobBoardHandler{}, &handler.EvaluationHandler{}, &handler.ApplicationHandler{}).Setup()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/wallet/webhook/payos", strings.NewReader(`{}`)))
	if w.Code != http.StatusOK || wallet.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s, want webhook handler to run without a session", w.Code, wallet.calls, w.Body.String())
	}
}
