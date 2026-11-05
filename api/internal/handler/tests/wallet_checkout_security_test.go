package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// postWallet sends a request with the given content type and Origin, as RequireAuth would have
// admitted it, so the handler's own checks run.
func postWallet(t *testing.T, path, contentType, origin string, body []byte, user *authdomain.User, h *handler.WalletHandler) *httptest.ResponseRecorder {
	t.Helper()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if user != nil {
			c.Set("auth.middleware.currentUser", *user)
		}
	})
	engine.POST("/wallet/deposit/checkout", h.CreateDepositCheckout)
	engine.POST("/wallet/deposit/sync", h.SyncDepositStatus)
	engine.POST("/wallet/webhook/payos", h.HandlePayOSWebhook)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// KAN-292 / KAN-262: a return or cancel URL that is not on the request's trusted origin never reaches PayOS.
func TestCreateDepositCheckout_RejectsUntrustedReturnURLs(t *testing.T) {
	packageID := uuid.New()
	cases := map[string]string{
		"other host":            `https://evil.example/return`,
		"other scheme":          `http://app/return`,
		"javascript scheme":     `javascript:alert(1)`,
		"data scheme":           `data:text/html,hi`,
		"protocol relative":     `//evil.example/return`,
		"relative path":         `/billing`,
		"userinfo trick":        `https://app@evil.example/return`,
		"userinfo on origin":    `https://app:pw@app/return`,
		"look-alike suffix":     `https://app.evil.example/return`,
		"look-alike prefix":     `https://evilapp/return`,
		"different port":        `https://app:8443/return`,
		"backslash host":        `https:\\evil.example\return`,
		"control character":     "https://app/re\x00turn",
		"oversized":             `https://app/` + strings.Repeat("a", 2100),
		"empty host":            `https:///return`,
		"mixed case other host": `https://EVIL.example/return`,
	}
	for name, raw := range cases {
		for _, field := range []string{"return_url", "cancel_url"} {
			t.Run(name+"/"+field, func(t *testing.T) {
				svc := new(MockWalletService)
				body := mustJSON(t, map[string]any{"package_id": packageID, field: raw})
				w := postWallet(t, "/wallet/deposit/checkout", "application/json", "https://app", body, candidateUser(), handler.NewWalletHandler(svc))
				assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				assert.Contains(t, w.Body.String(), "VALIDATION_ERROR")
				svc.AssertNotCalled(t, "CreateDepositCheckout")
			})
		}
	}
}

func TestCreateDepositCheckout_RejectsReturnURLsWithoutATrustedOrigin(t *testing.T) {
	svc := new(MockWalletService)
	body := mustJSON(t, map[string]any{"package_id": uuid.New(), "return_url": "https://app/return"})
	w := postWallet(t, "/wallet/deposit/checkout", "application/json", "", body, candidateUser(), handler.NewWalletHandler(svc))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "CreateDepositCheckout")
}

func TestCreateDepositCheckout_AcceptsSameOriginURLs(t *testing.T) {
	for _, origin := range []string{"https://app", "http://localhost:3000", "https://App.Example.com"} {
		svc := new(MockWalletService)
		user := candidateUser()
		packageID := uuid.New()
		ret, cancel := origin+"/billing?status=success", origin+"/billing?cancel=true"
		svc.On("CreateDepositCheckout", mock.Anything, user.ID, packageID, ret, cancel).
			Return(walletdomain.DepositCheckoutResponse{CheckoutURL: "https://pay.payos.vn/web/x"}, nil)
		body := mustJSON(t, map[string]any{"package_id": packageID, "return_url": ret, "cancel_url": cancel})
		w := postWallet(t, "/wallet/deposit/checkout", "application/json", strings.ToLower(origin), body, user, handler.NewWalletHandler(svc))
		assert.Equal(t, http.StatusOK, w.Code, "%s: %s", origin, w.Body.String())
		svc.AssertExpectations(t)
	}
}

func TestCreateDepositCheckout_StrictBody(t *testing.T) {
	packageID := uuid.New().String()
	cases := map[string]struct {
		contentType string
		body        string
	}{
		"unknown field":      {"application/json", `{"package_id":"` + packageID + `","price":1}`},
		"trailing object":    {"application/json", `{"package_id":"` + packageID + `"}{"package_id":"` + packageID + `"}`},
		"trailing garbage":   {"application/json", `{"package_id":"` + packageID + `"} x`},
		"wrong content type": {"text/plain", `{"package_id":"` + packageID + `"}`},
		"no content type":    {"", `{"package_id":"` + packageID + `"}`},
		"not an object":      {"application/json", `[]`},
		"nil package":        {"application/json", `{"package_id":"00000000-0000-0000-0000-000000000000"}`},
		"oversized body":     {"application/json", `{"package_id":"` + packageID + `","return_url":"https://app/` + strings.Repeat("a", 5000) + `"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc := new(MockWalletService)
			w := postWallet(t, "/wallet/deposit/checkout", tc.contentType, "https://app", []byte(tc.body), candidateUser(), handler.NewWalletHandler(svc))
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			svc.AssertNotCalled(t, "CreateDepositCheckout")
		})
	}
}

func TestSyncDeposit_StrictBody(t *testing.T) {
	cases := map[string]struct {
		contentType string
		body        string
	}{
		"unknown field":    {"application/json", `{"order_code":5,"user_id":"x"}`},
		"trailing data":    {"application/json", `{"order_code":5}{}`},
		"wrong type":       {"text/plain", `{"order_code":5}`},
		"zero order":       {"application/json", `{"order_code":0}`},
		"negative order":   {"application/json", `{"order_code":-4}`},
		"string order":     {"application/json", `{"order_code":"5"}`},
		"oversized padded": {"application/json", `{"order_code":5,"pad":"` + strings.Repeat("a", 2000) + `"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc := new(MockWalletService)
			w := postWallet(t, "/wallet/deposit/sync", tc.contentType, "https://app", []byte(tc.body), candidateUser(), handler.NewWalletHandler(svc))
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			svc.AssertNotCalled(t, "SyncDepositStatus")
		})
	}
}

func TestPayOSWebhook_RejectsAnOversizedBody(t *testing.T) {
	svc := new(MockWalletService)
	body := []byte(`{"data":"` + strings.Repeat("a", 70<<10) + `"}`)
	w := postWallet(t, "/wallet/webhook/payos", "application/json", "", body, nil, handler.NewWalletHandler(svc))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "HandlePayOSWebhook")
}
