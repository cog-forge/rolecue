package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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

func (m *MockWalletService) HandlePayOSWebhook(ctx context.Context, body []byte) error {
	args := m.Called(ctx, body)
	return args.Error(0)
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
	r.POST("/wallet/webhook/payos", func(c *gin.Context) {
		h.HandlePayOSWebhook(c)
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
