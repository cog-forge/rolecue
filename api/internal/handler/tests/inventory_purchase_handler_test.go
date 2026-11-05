package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// purchaseInventory implements only the purchase of handler.InventoryService; the other methods are
// never called by these tests.
type purchaseInventory struct {
	handler.InventoryService
	calls  int
	amount int32
	key    string
	result domain.SlotPurchase
	err    error
}

func (s *purchaseInventory) PurchaseSlot(_ context.Context, _ authdomain.User, amount int32, key string) (domain.SlotPurchase, error) {
	s.key = key
	s.calls++
	s.amount = amount
	return s.result, s.err
}

func postPurchase(t *testing.T, svc *purchaseInventory, body string) *httptest.ResponseRecorder {
	return postPurchaseWithKey(t, svc, body, "")
}

func postPurchaseWithKey(t *testing.T, svc *purchaseInventory, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		setUser(c, authdomain.User{ID: uuid.New(), Role: "candidate", EmailVerified: true})
	})
	router.POST("/inventory/purchase", handler.NewInventoryHandler(svc).PurchaseSlot)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/inventory/purchase", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	router.ServeHTTP(w, req)
	return w
}

// TestInventoryPurchaseReturnsTheSummaryAndBalance pins the response contract: 200 with the GET
// /inventory summary fields plus balance in the success envelope.
func TestInventoryPurchaseReturnsTheSummaryAndBalance(t *testing.T) {
	svc := &purchaseInventory{result: domain.SlotPurchase{
		Capacity: domain.Capacity{Capacity: 5, Used: 5, Available: 0, IsFull: true}, Balance: 12,
	}}
	w := postPurchase(t, svc, `{"amount":2}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool                       `json:"success"`
		Data    map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"capacity": "5", "used": "5", "available": "0", "is_full": "true", "balance": "12"}
	if !body.Success || len(body.Data) != len(want) {
		t.Fatalf("body = %s, want exactly the fields %v", w.Body.String(), want)
	}
	for key, value := range want {
		if string(body.Data[key]) != value {
			t.Fatalf("data.%s = %s, want %s (body %s)", key, body.Data[key], value, w.Body.String())
		}
	}
	if svc.calls != 1 || svc.amount != 2 {
		t.Fatalf("service called %d times with amount %d, want once with 2", svc.calls, svc.amount)
	}
}

func TestInventoryPurchaseRejectsAmountsOutsideOneToTen(t *testing.T) {
	for _, body := range []string{`{"amount":0}`, `{"amount":-1}`, `{"amount":11}`, `{"amount":2147483647}`, `{}`, `{"amount":"2"}`} {
		svc := &purchaseInventory{}
		w := postPurchase(t, svc, body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), string(apperror.CodeValidation)) {
			t.Fatalf("body %s: status = %d, response %s, want 400 VALIDATION_ERROR", body, w.Code, w.Body.String())
		}
		if svc.calls != 0 {
			t.Fatalf("body %s reached the service", body)
		}
	}
	for _, amount := range []string{"1", "10"} {
		svc := &purchaseInventory{}
		if w := postPurchase(t, svc, `{"amount":`+amount+`}`); w.Code != http.StatusOK || svc.calls != 1 {
			t.Fatalf("amount %s: status = %d, calls %d, want 200 and one call", amount, w.Code, svc.calls)
		}
	}
}

func TestInventoryPurchaseForwardsTheIdempotencyKey(t *testing.T) {
	svc := &purchaseInventory{}
	if w := postPurchaseWithKey(t, svc, `{"amount":2}`, "  key-123 "); w.Code != http.StatusOK || svc.key != "key-123" {
		t.Fatalf("status = %d, key = %q, want 200 and the trimmed key", w.Code, svc.key)
	}
	svc = &purchaseInventory{}
	if w := postPurchase(t, svc, `{"amount":2}`); w.Code != http.StatusOK || svc.key != "" {
		t.Fatalf("status = %d, key = %q, want 200 and no key", w.Code, svc.key)
	}
}

func TestInventoryPurchaseRejectsBadIdempotencyKeys(t *testing.T) {
	for name, key := range map[string]string{
		"too long":     strings.Repeat("k", 129),
		"control byte": "key\x01",
		"non ascii":    "kéy",
	} {
		svc := &purchaseInventory{}
		w := postPurchaseWithKey(t, svc, `{"amount":2}`, key)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), string(apperror.CodeValidation)) || svc.calls != 0 {
			t.Fatalf("%s: status = %d, calls %d, body %s, want 400 VALIDATION_ERROR without a service call", name, w.Code, svc.calls, w.Body.String())
		}
	}
}

func TestInventoryPurchaseInsufficientBalanceIs422(t *testing.T) {
	svc := &purchaseInventory{err: apperror.New(apperror.CodeInsufficientBalance, "insufficient coin balance")}
	w := postPurchase(t, svc, `{"amount":3}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), string(apperror.CodeInsufficientBalance)) {
		t.Fatalf("status = %d, body %s, want 422 INSUFFICIENT_BALANCE", w.Code, w.Body.String())
	}
}
