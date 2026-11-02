package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	sentinelSignature = "SENTINEL-PAYOS-SIGNATURE-VALUE"
	logOrderCode      = int64(123456789012)
)

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

// renderLogs flattens every message and field into one string, so a test can look for a secret anywhere.
func renderLogs(logs *observer.ObservedLogs) string {
	var b strings.Builder
	for _, e := range logs.All() {
		fmt.Fprintf(&b, "%s %v\n", e.Message, e.ContextMap())
	}
	return b.String()
}

func onlyLog(t *testing.T, logs *observer.ObservedLogs, message string) map[string]any {
	t.Helper()
	found := logs.FilterMessage(message).All()
	if len(found) != 1 {
		t.Fatalf("%q logged %d times, want once; logs:\n%s", message, len(found), renderLogs(logs))
	}
	return found[0].ContextMap()
}

func assertFields(t *testing.T, got map[string]any, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if fmt.Sprint(got[key]) != value {
			t.Errorf("field %s = %v, want %s; all fields: %v", key, got[key], value, got)
		}
	}
}

// paidWebhook mocks a PAID webhook for a pending top-up of 100 coins and returns the transaction ID.
func paidWebhook(mockRepo *MockRepo, mockPayOS *MockPayOS, body []byte, userID uuid.UUID, paid int) uuid.UUID {
	txID := uuid.New()
	webhook := &payos.WebhookData{Code: "00", Success: true}
	webhook.Data.OrderCode = logOrderCode
	webhook.Data.Amount = 50000
	mockPayOS.On("VerifyWebhookData", body).Return(webhook, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, logOrderCode).Return(&payos.PaymentLink{
		OrderCode: logOrderCode, Amount: 50000, AmountPaid: paid, Status: payos.PaymentLinkStatusPaid,
	}, nil)
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), logOrderCode).Return(domain.Transaction{
		ID: txID, To: &userID, Amount: 100, Status: domain.StatusPending, PayOSOrderCode: &[]int64{logOrderCode}[0],
	}, nil)
	return txID
}

func TestPayOSWebhookLogsTheSettledDepositWithoutTheSignature(t *testing.T) {
	ctx, logs := observedContext()
	mockRepo, mockPayOS := new(MockRepo), new(MockPayOS)
	userID := uuid.New()
	body := []byte(`{"signature":"` + sentinelSignature + `"}`)
	txID := paidWebhook(mockRepo, mockPayOS, body, userID, 50000)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(domain.Transaction{
		ID: txID, To: &userID, Amount: 100, Status: domain.StatusSuccess,
	}, nil)
	mockRepo.On("CreditPaidOrder", mock.Anything, (*repository.Queries)(nil), userID, int32(100)).Return(nil)

	if err := service.NewService(mockRepo, mockPayOS).HandlePayOSWebhook(ctx, body); err != nil {
		t.Fatal(err)
	}
	assertFields(t, onlyLog(t, logs, "deposit status changed"), map[string]string{
		"op": "wallet.payos_webhook", "transaction_id": txID.String(), "order_code": fmt.Sprint(logOrderCode),
		"from_status": "pending", "to_status": "success", "user_id": userID.String(), "amount": "50000",
		"currency": "VND", "coin_amount": "100",
	})
	if strings.Contains(renderLogs(logs), sentinelSignature) {
		t.Fatalf("the webhook body reached the log:\n%s", renderLogs(logs))
	}
}

func TestPayOSWebhookRejectedSignatureLogsTheReasonOnly(t *testing.T) {
	ctx, logs := observedContext()
	mockRepo, mockPayOS := new(MockRepo), new(MockPayOS)
	body := []byte(`{"signature":"` + sentinelSignature + `"}`)
	mockPayOS.On("VerifyWebhookData", body).Return(nil, errors.New("invalid webhook signature "+sentinelSignature))

	if err := service.NewService(mockRepo, mockPayOS).HandlePayOSWebhook(ctx, body); err == nil {
		t.Fatal("HandlePayOSWebhook() error = nil, want a rejection")
	}
	assertFields(t, onlyLog(t, logs, "payos webhook rejected"), map[string]string{
		"op": "wallet.payos_webhook", "reason": "signature_or_format_invalid",
	})
	if strings.Contains(renderLogs(logs), sentinelSignature) {
		t.Fatalf("the signature reached the log:\n%s", renderLogs(logs))
	}
}

func TestPayOSWebhookReplayIsLoggedAsIgnored(t *testing.T) {
	ctx, logs := observedContext()
	mockRepo, mockPayOS := new(MockRepo), new(MockPayOS)
	body := []byte(`{}`)
	webhook := &payos.WebhookData{Code: "00", Success: true}
	webhook.Data.OrderCode = logOrderCode
	mockPayOS.On("VerifyWebhookData", body).Return(webhook, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, logOrderCode).Return(&payos.PaymentLink{
		OrderCode: logOrderCode, Amount: 50000, AmountPaid: 50000, Status: payos.PaymentLinkStatusPaid,
	}, nil)
	txID := uuid.New()
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), logOrderCode).Return(domain.Transaction{
		ID: txID, Status: domain.StatusSuccess,
	}, nil)

	if err := service.NewService(mockRepo, mockPayOS).HandlePayOSWebhook(ctx, body); err != nil {
		t.Fatal(err)
	}
	assertFields(t, onlyLog(t, logs, "payos webhook replay ignored"), map[string]string{
		"op": "wallet.payos_webhook", "reason": "already_processed", "transaction_id": txID.String(), "status": "success",
	})
	if n := logs.FilterMessage("deposit status changed").Len(); n != 0 {
		t.Fatalf("a replay logged a status change")
	}
}

func TestPayOSWebhookUnknownOrderIsLogged(t *testing.T) {
	ctx, logs := observedContext()
	mockRepo, mockPayOS := new(MockRepo), new(MockPayOS)
	body := []byte(`{}`)
	webhook := &payos.WebhookData{Code: "00", Success: true}
	webhook.Data.OrderCode = logOrderCode
	mockPayOS.On("VerifyWebhookData", body).Return(webhook, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, logOrderCode).Return(&payos.PaymentLink{
		OrderCode: logOrderCode, Amount: 50000, AmountPaid: 50000, Status: payos.PaymentLinkStatusPaid,
	}, nil)
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), logOrderCode).
		Return(domain.Transaction{}, apperror.New(apperror.CodeTransactionNotFound, "transaction not found"))

	if err := service.NewService(mockRepo, mockPayOS).HandlePayOSWebhook(ctx, body); err == nil {
		t.Fatal("HandlePayOSWebhook() error = nil, want not found")
	}
	assertFields(t, onlyLog(t, logs, "payos webhook rejected"), map[string]string{"reason": "unknown_order", "order_code": fmt.Sprint(logOrderCode)})
}

func TestPayOSWebhookUnderpaidLogsTheAmountsAndDoesNotSettle(t *testing.T) {
	ctx, logs := observedContext()
	mockRepo, mockPayOS := new(MockRepo), new(MockPayOS)
	userID := uuid.New()
	body := []byte(`{}`)
	paidWebhook(mockRepo, mockPayOS, body, userID, 10000)

	if err := service.NewService(mockRepo, mockPayOS).HandlePayOSWebhook(ctx, body); err == nil {
		t.Fatal("HandlePayOSWebhook() error = nil, want an underpaid rejection")
	}
	assertFields(t, onlyLog(t, logs, "payos payment rejected"), map[string]string{
		"op": "wallet.payos_webhook", "reason": "amount_invalid_or_underpaid", "amount": "50000", "amount_paid": "10000",
	})
	if n := logs.FilterMessage("deposit status changed").Len(); n != 0 {
		t.Fatal("an underpaid top-up was logged as settled")
	}
}

func TestCreateDepositCheckoutLogsTheCheckout(t *testing.T) {
	ctx, logs := observedContext()
	mockRepo, mockPayOS := new(MockRepo), new(MockPayOS)
	userID, packageID, txID := uuid.New(), uuid.New(), uuid.New()
	pkg := domain.CoinPackage{ID: packageID, Name: "Starter Pack", PriceVND: 50000, CoinAmount: 100, Currency: "VND", IsActive: true}
	mockRepo.On("GetCoinPackageByID", mock.Anything, packageID).Return(pkg, nil)
	mockRepo.On("InsertTransaction", mock.Anything, (*repository.Queries)(nil), mock.Anything).Return(domain.Transaction{
		ID: txID, To: &userID, Amount: 100, Status: domain.StatusPending,
	}, nil)
	mockPayOS.On("CreatePaymentLink", mock.Anything, mock.Anything).Return(payos.CreateLinkResponse{CheckoutURL: "https://payos.vn/checkout/x"}, nil)

	if _, err := service.NewService(mockRepo, mockPayOS).CreateDepositCheckout(ctx, userID, packageID, "https://app/return", "https://app/cancel"); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "deposit checkout created")
	assertFields(t, got, map[string]string{
		"op": "wallet.checkout", "user_id": userID.String(), "transaction_id": txID.String(),
		"amount": "50000", "currency": "VND", "coin_amount": "100",
	})
	if strings.Contains(renderLogs(logs), "https://payos.vn/checkout/x") {
		t.Fatal("the checkout URL reached the log")
	}
}
