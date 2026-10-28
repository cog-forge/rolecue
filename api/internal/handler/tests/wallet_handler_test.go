package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Mock wallet service
// ---------------------------------------------------------------------------

type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) GetWallet(ctx context.Context, userID uuid.UUID) (walletdomain.Wallet, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(walletdomain.Wallet), args.Error(1)
}

func (m *MockWalletService) ListTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int32) ([]walletdomain.Transaction, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	return args.Get(0).([]walletdomain.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockWalletService) ListPackages(ctx context.Context) ([]walletdomain.CoinPackage, error) {
	args := m.Called(ctx)
	return args.Get(0).([]walletdomain.CoinPackage), args.Error(1)
}

func (m *MockWalletService) CreateDepositCheckout(ctx context.Context, userID uuid.UUID, packageID uuid.UUID, returnURL, cancelURL string) (walletdomain.DepositCheckoutResponse, error) {
	args := m.Called(ctx, userID, packageID, returnURL, cancelURL)
	return args.Get(0).(walletdomain.DepositCheckoutResponse), args.Error(1)
}

func (m *MockWalletService) SyncDepositStatus(ctx context.Context, userID uuid.UUID, orderCode int64) (walletdomain.Transaction, error) {
	args := m.Called(ctx, userID, orderCode)
	return args.Get(0).(walletdomain.Transaction), args.Error(1)
}

func (m *MockWalletService) HandlePayOSWebhook(ctx context.Context, body []byte) error {
	args := m.Called(ctx, body)
	return args.Error(0)
}

func (m *MockWalletService) GetBillingSummary(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.BillingSummary, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(walletdomain.BillingSummary), args.Error(1)
}

func (m *MockWalletService) ListBillingTransactions(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.AdminTransactionPage, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(walletdomain.AdminTransactionPage), args.Error(1)
}

func (m *MockWalletService) GetRevenueReport(ctx context.Context, req walletdomain.RevenueReportRequest) (walletdomain.RevenueReport, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(walletdomain.RevenueReport), args.Error(1)
}

func (m *MockWalletService) GetBillingTransaction(ctx context.Context, id uuid.UUID) (walletdomain.AdminTransactionItem, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(walletdomain.AdminTransactionItem), args.Error(1)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func init() {
	gin.SetMode(gin.TestMode)
}

// setUser injects a user into the gin context, mimicking middleware.RequireAuth.
func setUser(c *gin.Context, user authdomain.User) {
	c.Set("auth.middleware.currentUser", user)
}

func newRouter(h *handler.WalletHandler) *gin.Engine {
	r := gin.New()
	r.GET("/wallet", func(c *gin.Context) {
		// Inject user before handler runs — simulates auth middleware.
		if u, ok := c.Get("auth.middleware.currentUser"); ok {
			c.Set("auth.middleware.currentUser", u)
		}
		h.GetWallet(c)
	})
	r.GET("/wallet/transactions", func(c *gin.Context) {
		h.ListTransactions(c)
	})
	r.GET("/wallet/packages", func(c *gin.Context) {
		h.ListPackages(c)
	})
	r.POST("/wallet/deposit/checkout", func(c *gin.Context) {
		h.CreateDepositCheckout(c)
	})
	r.POST("/wallet/deposit/sync", func(c *gin.Context) {
		h.SyncDepositStatus(c)
	})
	r.POST("/wallet/webhook/payos", func(c *gin.Context) {
		h.HandlePayOSWebhook(c)
	})
	r.GET("/admin/billing/summary", func(c *gin.Context) {
		h.AdminGetBillingSummary(c)
	})
	r.GET("/admin/billing/transactions", func(c *gin.Context) {
		h.AdminListBillingTransactions(c)
	})
	r.GET("/admin/billing/transactions/:id", func(c *gin.Context) {
		h.AdminGetBillingTransaction(c)
	})
	r.GET("/admin/billing/export", func(c *gin.Context) {
		h.AdminExportBillingCSV(c)
	})
	r.GET("/admin/billing/revenue-report", func(c *gin.Context) {
		h.AdminGetRevenueReport(c)
	})
	return r
}

// performRequest builds and fires a test request, injecting a user context key if provided.
func performRequest(r *gin.Engine, method, path string, body []byte, user *authdomain.User) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Inject user via a wrapper that sets the context key before the handler.
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if user != nil {
			c.Set("auth.middleware.currentUser", *user)
		}
		c.Next()
	})
	for _, route := range r.Routes() {
		engine.Handle(route.Method, route.Path, route.HandlerFunc)
	}
	engine.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Ownership: admin is rejected with 403 on all wallet-owning endpoints
// ---------------------------------------------------------------------------

func adminUser() *authdomain.User {
	id := uuid.New()
	return &authdomain.User{ID: id, Email: "admin@rolecue.vn", Role: "admin"}
}

func candidateUser() *authdomain.User {
	id := uuid.New()
	return &authdomain.User{ID: id, Email: "candidate@rolecue.vn", Role: "candidate"}
}

func recruiterUser() *authdomain.User {
	id := uuid.New()
	return &authdomain.User{ID: id, Email: "recruiter@rolecue.vn", Role: "recruiter"}
}

func TestGetWallet_AdminForbidden(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	w := performRequest(r, http.MethodGet, "/wallet", nil, adminUser())

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "GetWallet")
}

func TestGetWallet_Unauthenticated(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	// No user in context.
	w := performRequest(r, http.MethodGet, "/wallet", nil, nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "GetWallet")
}

func TestGetWallet_Candidate_Success(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	expected := walletdomain.Wallet{ID: uuid.New(), UserID: user.ID, Balance: 100}
	svc.On("GetWallet", mock.Anything, user.ID).Return(expected, nil)

	w := performRequest(r, http.MethodGet, "/wallet", nil, user)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestGetWallet_Recruiter_Success(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := recruiterUser()
	expected := walletdomain.Wallet{ID: uuid.New(), UserID: user.ID, Balance: 250}
	svc.On("GetWallet", mock.Anything, user.ID).Return(expected, nil)

	w := performRequest(r, http.MethodGet, "/wallet", nil, user)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Ownership: ListTransactions
// ---------------------------------------------------------------------------

func TestListTransactions_AdminForbidden(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	w := performRequest(r, http.MethodGet, "/wallet/transactions", nil, adminUser())

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "ListTransactions")
}

func TestListTransactions_Candidate_Success(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	svc.On("ListTransactions", mock.Anything, user.ID, int32(1), int32(20)).
		Return([]walletdomain.Transaction{}, int64(0), nil)

	w := performRequest(r, http.MethodGet, "/wallet/transactions", nil, user)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

// Cross-user: the handler always uses the authenticated user's own ID;
// there is no way to request another user's transactions via query param.
func TestListTransactions_AlwaysOwnUser(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	// Regardless of any query param the client might try, the handler
	// calls ListTransactions with user.ID only.
	svc.On("ListTransactions", mock.Anything, user.ID, mock.Anything, mock.Anything).
		Return([]walletdomain.Transaction{}, int64(0), nil)

	// Try injecting a different UUID as a query param — handler ignores it.
	w := performRequest(r, http.MethodGet, "/wallet/transactions?user_id="+uuid.New().String(), nil, user)

	assert.Equal(t, http.StatusOK, w.Code)
	// The service must have been called exactly once with the authenticated user.
	svc.AssertCalled(t, "ListTransactions", mock.Anything, user.ID, mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// Ownership: CreateDepositCheckout
// ---------------------------------------------------------------------------

func TestCreateDepositCheckout_AdminForbidden(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	body, _ := json.Marshal(map[string]any{"package_id": uuid.New()})
	w := performRequest(r, http.MethodPost, "/wallet/deposit/checkout", body, adminUser())

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "CreateDepositCheckout")
}

func TestCreateDepositCheckout_Unauthenticated(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	body, _ := json.Marshal(map[string]any{"package_id": uuid.New()})
	w := performRequest(r, http.MethodPost, "/wallet/deposit/checkout", body, nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "CreateDepositCheckout")
}

func TestCreateDepositCheckout_BadRequest_MissingPackageID(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	// Missing required package_id field.
	body, _ := json.Marshal(map[string]any{"return_url": "https://app/return"})
	w := performRequest(r, http.MethodPost, "/wallet/deposit/checkout", body, candidateUser())

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "CreateDepositCheckout")
}

func TestCreateDepositCheckout_Candidate_Success(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	packageID := uuid.New()
	resp := walletdomain.DepositCheckoutResponse{
		CheckoutURL:   "https://payos.vn/checkout/abc",
		OrderCode:     123456789,
		Amount:        50000,
		CoinAmount:    100,
		TransactionID: uuid.New(),
	}
	svc.On("CreateDepositCheckout", mock.Anything, user.ID, packageID, "https://app/return", "https://app/cancel").
		Return(resp, nil)

	body, _ := json.Marshal(map[string]any{
		"package_id": packageID,
		"return_url": "https://app/return",
		"cancel_url": "https://app/cancel",
	})
	w := performRequest(r, http.MethodPost, "/wallet/deposit/checkout", body, user)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateDepositCheckout_ServiceError_Propagates(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	packageID := uuid.New()
	svc.On("CreateDepositCheckout", mock.Anything, user.ID, packageID, "", "").
		Return(walletdomain.DepositCheckoutResponse{},
			apperror.New(apperror.CodeCoinPackageNotFound, "not found"))

	body, _ := json.Marshal(map[string]any{"package_id": packageID})
	w := performRequest(r, http.MethodPost, "/wallet/deposit/checkout", body, user)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListPackages — no auth guard (public catalogue)
// ---------------------------------------------------------------------------

func TestListPackages_NoAuthRequired(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	packages := []walletdomain.CoinPackage{
		{ID: uuid.New(), Name: "Starter", PriceVND: 20000, CoinAmount: 50},
	}
	svc.On("ListPackages", mock.Anything).Return(packages, nil)

	// No user context — packages are readable without auth.
	w := performRequest(r, http.MethodGet, "/wallet/packages", nil, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestListPackages_ServiceError_Propagates(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	svc.On("ListPackages", mock.Anything).
		Return([]walletdomain.CoinPackage(nil), errors.New("db: timeout"))

	w := performRequest(r, http.MethodGet, "/wallet/packages", nil, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — no auth required (called by PayOS server)
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_Success_Returns200(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	body := []byte(`{"code":"00","success":true}`)
	svc.On("HandlePayOSWebhook", mock.Anything, body).Return(nil)

	w := performRequest(r, http.MethodPost, "/wallet/webhook/payos", body, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestHandlePayOSWebhook_InvalidSignature_Returns502(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	body := []byte(`{}`)
	svc.On("HandlePayOSWebhook", mock.Anything, body).
		Return(apperror.New(apperror.CodePayOSError, "invalid signature"))

	w := performRequest(r, http.MethodPost, "/wallet/webhook/payos", body, nil)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	svc.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// GetWallet: service error propagates to correct HTTP status
// ---------------------------------------------------------------------------

func TestGetWallet_InternalError(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	svc.On("GetWallet", mock.Anything, user.ID).
		Return(walletdomain.Wallet{}, errors.New("db: connection lost"))

	w := performRequest(r, http.MethodGet, "/wallet", nil, user)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

// Verify that CurrentUser helper is used (not CurrentUserID) — the handler
// needs the Role field to enforce admin guard.
func TestGetWallet_ContextKeyConsistency(t *testing.T) {
	// CurrentUser must be set via the same key the middleware uses.
	user := candidateUser()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("auth.middleware.currentUser", *user)

	extracted, ok := middleware.CurrentUser(c)
	assert.True(t, ok)
	assert.Equal(t, user.ID, extracted.ID)
	assert.Equal(t, "candidate", extracted.Role)
}

func TestSyncDepositStatus_Success(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	txID := uuid.New()
	orderCode := int64(123456789)
	body := []byte(`{"order_code":123456789}`)

	svc.On("SyncDepositStatus", mock.Anything, user.ID, orderCode).
		Return(walletdomain.Transaction{
			ID:             txID,
			Amount:         100,
			Status:         walletdomain.StatusCancelled,
			PayOSOrderCode: &orderCode,
		}, nil)

	w := performRequest(r, http.MethodPost, "/wallet/deposit/sync", body, user)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSyncDepositStatus_AdminForbidden(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	admin := adminUser()
	body := []byte(`{"order_code":123456789}`)

	w := performRequest(r, http.MethodPost, "/wallet/deposit/sync", body, admin)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "SyncDepositStatus")
}

func TestSyncDepositStatus_InvalidJSON(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)

	user := candidateUser()
	body := []byte(`{"order_code":"not-a-number"}`)

	w := performRequest(r, http.MethodPost, "/wallet/deposit/sync", body, user)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "SyncDepositStatus")
}

// ---------------------------------------------------------------------------
// Admin Billing & Revenue Read-Only Endpoints (KAN-191, KAN-192, KAN-241)
// ---------------------------------------------------------------------------

func TestAdminBillingEndpoints_RoleAuthorization(t *testing.T) {
	txID := uuid.New()
	endpoints := []string{
		"/admin/billing/summary",
		"/admin/billing/transactions",
		"/admin/billing/transactions/" + txID.String(),
		"/admin/billing/export",
	}

	for _, ep := range endpoints {
		t.Run("Unauthenticated_"+ep, func(t *testing.T) {
			svc := new(MockWalletService)
			h := handler.NewWalletHandler(svc)
			r := newRouter(h)

			w := performRequest(r, http.MethodGet, ep, nil, nil)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("CandidateForbidden_"+ep, func(t *testing.T) {
			svc := new(MockWalletService)
			h := handler.NewWalletHandler(svc)
			r := newRouter(h)

			w := performRequest(r, http.MethodGet, ep, nil, candidateUser())
			assert.Equal(t, http.StatusForbidden, w.Code)
		})

		t.Run("RecruiterForbidden_"+ep, func(t *testing.T) {
			svc := new(MockWalletService)
			h := handler.NewWalletHandler(svc)
			r := newRouter(h)

			w := performRequest(r, http.MethodGet, ep, nil, recruiterUser())
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

func TestAdminBillingEndpoints_StrictValidationReturns400(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)
	admin := adminUser()

	badQueries := []string{
		"/admin/billing/summary?status=unknown_status",
		"/admin/billing/summary?type=unknown_type",
		"/admin/billing/summary?status=success&status=pending",
		"/admin/billing/transactions?page=0",
		"/admin/billing/transactions?page=-2",
		"/admin/billing/transactions?page=10001",
		"/admin/billing/transactions?page_size=0",
		"/admin/billing/transactions?page_size=101",
		"/admin/billing/transactions?amount_min=-5",
		"/admin/billing/transactions?amount_min=100000&amount_max=50000",
		"/admin/billing/transactions?sort_by=drop_table",
		"/admin/billing/transactions?sort_order=sideways",
		"/admin/billing/transactions/not-a-uuid",
		"/admin/billing/transactions?date_from=2026-10-01",
		"/admin/billing/transactions?date_to=yesterday",
		"/admin/billing/summary?date_from=2026-10-01T00:00:00",
		"/admin/billing/summary?date_from=2026-10-01T00:00:00Z&date_from=2026-10-02T00:00:00Z",
		"/admin/billing/summary?date_from=2026-11-01T00:00:00%2B07:00&date_to=2026-10-01T00:00:00%2B07:00",
		"/admin/billing/export?date_from=2026-10-01T00:00:00Z&date_to=2026-10-01T00:00:00Z",
	}

	for _, q := range badQueries {
		t.Run(q, func(t *testing.T) {
			w := performRequest(r, http.MethodGet, q, nil, admin)
			assert.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for query %s", q)
		})
	}

	svc.AssertNotCalled(t, "GetBillingSummary")
	svc.AssertNotCalled(t, "ListBillingTransactions")
	svc.AssertNotCalled(t, "GetBillingTransaction")
}

func TestAdminGetBillingSummary_Success(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)
	admin := adminUser()

	expectedFilter := walletdomain.AdminTransactionFilter{
		Page:        1,
		PageSize:    20,
		Status:      "success",
		Type:        "topup",
		PackageName: "Starter Pack",
		SortBy:      "created_at",
		SortOrder:   "desc",
	}
	summary := walletdomain.BillingSummary{
		TotalRevenue:       250000,
		SuccessfulDeposits: 3,
		SuccessCount:       3,
		TotalTransactions:  3,
		AverageDeposit:     83333,
		UniquePayers:       2,
		CoinsSold:          500,
	}
	svc.On("GetBillingSummary", mock.Anything, expectedFilter).Return(summary, nil)

	w := performRequest(
		r,
		http.MethodGet,
		"/admin/billing/summary?status=success&type=topup&package=Starter+Pack",
		nil,
		admin,
	)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total_revenue":250000`)
	assert.Contains(t, w.Body.String(), `"successful_deposits":3`)
	svc.AssertExpectations(t)
}

func TestAdminExportBillingCSV_SanitizesFormulaInjectionAndAllowsOver20Rows(t *testing.T) {
	svc := new(MockWalletService)
	h := handler.NewWalletHandler(svc)
	r := newRouter(h)
	admin := adminUser()

	// Build 25 rows (> 20) including malicious formula prefixes (=, +, -, @) in user name / description
	items := make([]walletdomain.AdminTransactionItem, 0, 25)
	for i := 0; i < 25; i++ {
		orderCode := int64(100000 + i)
		name := "Normal User"
		desc := "Deposit: Starter (50000 VND)"
		switch i {
		case 0:
			name = "=CMD|' /C calc'!A0"
			desc = "@SUM(1+1)"
		case 1:
			name = "+123456"
			desc = "-9999"
		}
		items = append(items, walletdomain.AdminTransactionItem{
			ID:               uuid.New(),
			UserID:           uuid.New().String(),
			UserName:         name,
			UserEmail:        "user@example.com",
			Type:             "topup",
			PackageOrFeature: "Starter",
			PaidVND:          50000,
			CoinAmount:       100,
			Amount:           100,
			Currency:         "VND",
			Description:      desc,
			Status:           walletdomain.StatusSuccess,
			PayOSOrderCode:   &orderCode,
		})
	}

	expectedFilter := walletdomain.AdminTransactionFilter{
		Page:      1,
		PageSize:  5000,
		SortBy:    "created_at",
		SortOrder: "desc",
	}
	svc.On("ListBillingTransactions", mock.Anything, expectedFilter).Return(walletdomain.AdminTransactionPage{
		Items:    items,
		Total:    25,
		Page:     1,
		PageSize: 5000,
	}, nil)

	w := performRequest(r, http.MethodGet, "/admin/billing/export", nil, admin)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Equal(t, "25", w.Header().Get("X-Total-Count"))
	assert.Equal(t, "false", w.Header().Get("X-Export-Truncated"))
	csvBody := w.Body.String()
	lines := strings.Split(strings.TrimSpace(csvBody), "\n")
	// 1 header row + 25 data rows = 26 lines
	assert.Len(t, lines, 26)
	assert.Contains(t, csvBody, "'=CMD|' /C calc'!A0")
	assert.Contains(t, csvBody, "'@SUM(1+1)")
	assert.Contains(t, csvBody, "'+123456")
	assert.Contains(t, csvBody, "'-9999")
	svc.AssertExpectations(t)
}

// October 2026 in Vietnam time (UTC+07:00), as the admin screens send it.
var (
	octFromVN = time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	octToVN   = time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)
)

const octWindowQuery = "date_from=2026-10-01T00:00:00%2B07:00&date_to=2026-11-01T00:00:00%2B07:00"

func hasOctoberWindow(f walletdomain.AdminTransactionFilter) bool {
	return f.DateFrom != nil && f.DateFrom.Equal(octFromVN) &&
		f.DateTo != nil && f.DateTo.Equal(octToVN) &&
		f.SortBy == "created_at" && f.SortOrder == "desc"
}

func TestAdminBillingEndpoints_PassTheDateWindowToTheService(t *testing.T) {
	svc := new(MockWalletService)
	r := newRouter(handler.NewWalletHandler(svc))
	admin := adminUser()

	svc.On("GetBillingSummary", mock.Anything, mock.MatchedBy(hasOctoberWindow)).
		Return(walletdomain.BillingSummary{}, nil).Once()
	svc.On("ListBillingTransactions", mock.Anything, mock.MatchedBy(func(f walletdomain.AdminTransactionFilter) bool {
		return hasOctoberWindow(f) && f.PageSize == 20
	})).Return(walletdomain.AdminTransactionPage{}, nil).Once()
	svc.On("ListBillingTransactions", mock.Anything, mock.MatchedBy(func(f walletdomain.AdminTransactionFilter) bool {
		return hasOctoberWindow(f) && f.PageSize == 5000
	})).Return(walletdomain.AdminTransactionPage{}, nil).Once()

	for _, path := range []string{"/admin/billing/summary", "/admin/billing/transactions", "/admin/billing/export"} {
		w := performRequest(r, http.MethodGet, path+"?"+octWindowQuery, nil, admin)
		assert.Equal(t, http.StatusOK, w.Code, path)
	}
	svc.AssertExpectations(t)
}

func TestAdminListBillingTransactions_DateWindowEdgeCases(t *testing.T) {
	admin := adminUser()

	t.Run("UTC and offset forms name the same instant", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		svc.On("ListBillingTransactions", mock.Anything, mock.MatchedBy(hasOctoberWindow)).
			Return(walletdomain.AdminTransactionPage{}, nil).Once()

		w := performRequest(r, http.MethodGet,
			"/admin/billing/transactions?date_from=2026-09-30T17:00:00Z&date_to=2026-10-31T17:00:00.000Z", nil, admin)
		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("empty values and one open end are allowed", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		svc.On("ListBillingTransactions", mock.Anything, mock.MatchedBy(func(f walletdomain.AdminTransactionFilter) bool {
			return f.DateFrom != nil && f.DateFrom.Equal(octFromVN) && f.DateTo == nil
		})).Return(walletdomain.AdminTransactionPage{}, nil).Once()

		w := performRequest(r, http.MethodGet,
			"/admin/billing/transactions?date_from=2026-10-01T00:00:00%2B07:00&date_to=", nil, admin)
		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("a bad date names the parameter and the format", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))

		w := performRequest(r, http.MethodGet, "/admin/billing/transactions?date_from=2026-10-01", nil, admin)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "VALIDATION_ERROR")
		assert.Contains(t, w.Body.String(), "invalid date_from")
		assert.Contains(t, w.Body.String(), "RFC 3339")
		svc.AssertNotCalled(t, "ListBillingTransactions")
	})

	t.Run("a reversed window is rejected", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))

		w := performRequest(r, http.MethodGet,
			"/admin/billing/transactions?date_from=2026-11-01T00:00:00Z&date_to=2026-10-01T00:00:00Z", nil, admin)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "date_from must be before date_to")
		svc.AssertNotCalled(t, "ListBillingTransactions")
	})

	t.Run("sort_by=created_at is accepted", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		svc.On("ListBillingTransactions", mock.Anything, mock.MatchedBy(func(f walletdomain.AdminTransactionFilter) bool {
			return f.SortBy == "created_at" && f.SortOrder == "asc"
		})).Return(walletdomain.AdminTransactionPage{}, nil).Once()

		w := performRequest(r, http.MethodGet, "/admin/billing/transactions?sort_by=created_at&sort_order=asc", nil, admin)
		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestAdminBillingTransactions_ReturnCreatedAt(t *testing.T) {
	svc := new(MockWalletService)
	r := newRouter(handler.NewWalletHandler(svc))
	item := walletdomain.AdminTransactionItem{
		ID:        uuid.New(),
		Status:    walletdomain.StatusSuccess,
		CreatedAt: time.Date(2026, 10, 27, 6, 16, 40, 0, time.UTC),
	}
	svc.On("ListBillingTransactions", mock.Anything, mock.Anything).
		Return(walletdomain.AdminTransactionPage{Items: []walletdomain.AdminTransactionItem{item}, Total: 1}, nil)
	svc.On("GetBillingTransaction", mock.Anything, item.ID).Return(item, nil)

	list := performRequest(r, http.MethodGet, "/admin/billing/transactions", nil, adminUser())
	detail := performRequest(r, http.MethodGet, "/admin/billing/transactions/"+item.ID.String(), nil, adminUser())

	assert.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, list.Body.String(), `"created_at":"2026-10-27T06:16:40Z"`)
	assert.Equal(t, http.StatusOK, detail.Code)
	assert.Contains(t, detail.Body.String(), `"created_at":"2026-10-27T06:16:40Z"`)
}

func TestAdminExportBillingCSV_HasCreatedAtInVietnamTime(t *testing.T) {
	svc := new(MockWalletService)
	r := newRouter(handler.NewWalletHandler(svc))
	id := uuid.New()
	svc.On("ListBillingTransactions", mock.Anything, mock.Anything).Return(walletdomain.AdminTransactionPage{
		Items: []walletdomain.AdminTransactionItem{{
			ID:        id,
			Status:    walletdomain.StatusSuccess,
			CreatedAt: time.Date(2026, 10, 27, 6, 16, 40, 129_000_000, time.UTC),
		}},
		Total: 1,
	}, nil)

	w := performRequest(r, http.MethodGet, "/admin/billing/export", nil, adminUser())

	assert.Equal(t, http.StatusOK, w.Code)
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	assert.True(t, strings.HasPrefix(lines[0], "Transaction ID,Created At (UTC+07:00),"), lines[0])
	assert.True(t, strings.HasPrefix(lines[1], id.String()+",2026-10-27T13:16:40+07:00,"), lines[1])
}

func TestListTransactions_ReturnsCreatedAt(t *testing.T) {
	svc := new(MockWalletService)
	r := newRouter(handler.NewWalletHandler(svc))
	user := candidateUser()
	svc.On("ListTransactions", mock.Anything, user.ID, int32(1), int32(20)).Return([]walletdomain.Transaction{{
		ID:        uuid.New(),
		Status:    walletdomain.StatusSuccess,
		CreatedAt: time.Date(2026, 10, 27, 6, 16, 40, 0, time.UTC),
	}}, int64(1), nil)

	w := performRequest(r, http.MethodGet, "/wallet/transactions", nil, user)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"created_at":"2026-10-27T06:16:40Z"`)
}

func TestAdminGetRevenueReport(t *testing.T) {
	path := "/admin/billing/revenue-report?" + octWindowQuery

	t.Run("admins only", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		assert.Equal(t, http.StatusUnauthorized, performRequest(r, http.MethodGet, path, nil, nil).Code)
		assert.Equal(t, http.StatusForbidden, performRequest(r, http.MethodGet, path, nil, candidateUser()).Code)
		assert.Equal(t, http.StatusForbidden, performRequest(r, http.MethodGet, path, nil, recruiterUser()).Code)
		svc.AssertNotCalled(t, "GetRevenueReport")
	})

	t.Run("passes the window and the comparison", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		svc.On("GetRevenueReport", mock.Anything, mock.MatchedBy(func(req walletdomain.RevenueReportRequest) bool {
			return req.From.Equal(octFromVN) && req.To.Equal(octToVN) && req.Compare == "previous_period"
		})).Return(walletdomain.RevenueReport{Compare: "previous_period", Granularity: "day"}, nil).Once()

		w := performRequest(r, http.MethodGet, path+"&compare=previous_period", nil, adminUser())

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"granularity":"day"`)
		svc.AssertExpectations(t)
	})

	t.Run("rejects bad or duplicate dates and comparisons before the service", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		for _, q := range []string{
			"date_from=2026-10-01&date_to=2026-11-01T00:00:00Z",
			"date_from=2026-10-01T00:00:00Z&date_from=2026-10-02T00:00:00Z&date_to=2026-11-01T00:00:00Z",
			octWindowQuery + "&compare=x&compare=y",
		} {
			w := performRequest(r, http.MethodGet, "/admin/billing/revenue-report?"+q, nil, adminUser())
			assert.Equal(t, http.StatusBadRequest, w.Code, q)
			assert.Contains(t, w.Body.String(), "VALIDATION_ERROR", q)
		}
		svc.AssertNotCalled(t, "GetRevenueReport")
	})

	t.Run("service validation errors come back as 400", func(t *testing.T) {
		svc := new(MockWalletService)
		r := newRouter(handler.NewWalletHandler(svc))
		svc.On("GetRevenueReport", mock.Anything, mock.Anything).
			Return(walletdomain.RevenueReport{}, apperror.New(apperror.CodeValidation, "date_from and date_to are required")).Once()

		w := performRequest(r, http.MethodGet, "/admin/billing/revenue-report", nil, adminUser())

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "date_from and date_to are required")
	})
}
