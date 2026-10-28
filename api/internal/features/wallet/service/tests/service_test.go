package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) ExecTx(ctx context.Context, fn func(q *repository.Queries) error) error {
	return fn(nil)
}

func (m *MockRepo) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.Wallet), args.Error(1)
}

func (m *MockRepo) CreateWallet(ctx context.Context, q *repository.Queries, userID uuid.UUID) (domain.Wallet, error) {
	args := m.Called(ctx, q, userID)
	return args.Get(0).(domain.Wallet), args.Error(1)
}

func (m *MockRepo) CreditPaidOrder(ctx context.Context, q *repository.Queries, userID uuid.UUID, amount int32) error {
	args := m.Called(ctx, q, userID, amount)
	return args.Error(0)
}

func (m *MockRepo) InsertTransaction(ctx context.Context, q *repository.Queries, tx domain.Transaction) (domain.Transaction, error) {
	args := m.Called(ctx, q, tx)
	return args.Get(0).(domain.Transaction), args.Error(1)
}

func (m *MockRepo) GetTransactionByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Transaction), args.Error(1)
}

func (m *MockRepo) GetTransactionByPayOSOrderCode(ctx context.Context, orderCode int64) (domain.Transaction, error) {
	args := m.Called(ctx, orderCode)
	return args.Get(0).(domain.Transaction), args.Error(1)
}

func (m *MockRepo) GetTransactionByPayOSOrderCodeForUpdate(ctx context.Context, q *repository.Queries, orderCode int64) (domain.Transaction, error) {
	args := m.Called(ctx, q, orderCode)
	return args.Get(0).(domain.Transaction), args.Error(1)
}

func (m *MockRepo) UpdateTransactionStatus(ctx context.Context, q *repository.Queries, id uuid.UUID, status domain.TransactionStatus) (domain.Transaction, error) {
	args := m.Called(ctx, q, id, status)
	return args.Get(0).(domain.Transaction), args.Error(1)
}

func (m *MockRepo) ListTransactionsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]domain.Transaction, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]domain.Transaction), args.Get(1).(int64), args.Error(2)
}

func (m *MockRepo) ListCoinPackages(ctx context.Context) ([]domain.CoinPackage, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.CoinPackage), args.Error(1)
}

func (m *MockRepo) GetCoinPackageByID(ctx context.Context, id uuid.UUID) (domain.CoinPackage, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.CoinPackage), args.Error(1)
}

func (m *MockRepo) ListAdminTransactions(ctx context.Context, filter domain.AdminTransactionFilter) (domain.AdminTransactionPage, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(domain.AdminTransactionPage), args.Error(1)
}

func (m *MockRepo) GetAdminTransactionByID(ctx context.Context, id uuid.UUID) (domain.AdminTransactionItem, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.AdminTransactionItem), args.Error(1)
}

func (m *MockRepo) GetBillingSummary(ctx context.Context, filter domain.AdminTransactionFilter) (domain.BillingSummary, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(domain.BillingSummary), args.Error(1)
}

func (m *MockRepo) GetRevenuePeriod(ctx context.Context, from, to time.Time) (domain.RevenuePeriodData, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).(domain.RevenuePeriodData), args.Error(1)
}

func (m *MockRepo) GetCoinTotals(ctx context.Context) (outstanding, allTimeRevenueVND, allTimeCoinsSold int64, err error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

// ---------------------------------------------------------------------------
// Mock PayOS client
// ---------------------------------------------------------------------------

type MockPayOS struct {
	mock.Mock
}

func (m *MockPayOS) CreatePaymentLink(ctx context.Context, req payos.CreateLinkRequest) (payos.CreateLinkResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(payos.CreateLinkResponse), args.Error(1)
}

func (m *MockPayOS) VerifyWebhookData(body []byte) (*payos.WebhookData, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payos.WebhookData), args.Error(1)
}

func (m *MockPayOS) GetPaymentLink(ctx context.Context, orderCode int64) (*payos.PaymentLink, error) {
	args := m.Called(ctx, orderCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payos.PaymentLink), args.Error(1)
}

// ---------------------------------------------------------------------------
// GetWallet
// ---------------------------------------------------------------------------

func TestGetWallet_Existing(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	expected := domain.Wallet{ID: uuid.New(), UserID: userID, Balance: 500}

	mockRepo.On("GetWalletByUserID", mock.Anything, userID).Return(expected, nil)

	w, err := svc.GetWallet(context.Background(), userID)
	assert.NoError(t, err)
	assert.Equal(t, expected, w)
	mockRepo.AssertExpectations(t)
}

func TestGetWallet_NotFound_AutoCreate(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	expected := domain.Wallet{ID: uuid.New(), UserID: userID, Balance: 0}

	mockRepo.On("GetWalletByUserID", mock.Anything, userID).Return(domain.Wallet{}, apperror.New(apperror.CodeWalletNotFound, "wallet not found"))
	mockRepo.On("CreateWallet", mock.Anything, (*repository.Queries)(nil), userID).Return(expected, nil)

	w, err := svc.GetWallet(context.Background(), userID)
	assert.NoError(t, err)
	assert.Equal(t, expected, w)
	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// CreateDepositCheckout — successful purchase
// ---------------------------------------------------------------------------

func TestCreateDepositCheckout_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	packageID := uuid.New()
	txID := uuid.New()

	pkg := domain.CoinPackage{
		ID:         packageID,
		Name:       "Starter Pack",
		PriceVND:   50000,
		CoinAmount: 100,
		Currency:   "VND",
		IsActive:   true,
	}

	createdTx := domain.Transaction{
		ID:     txID,
		To:     &userID,
		Amount: 100,
		Status: domain.StatusPending,
	}

	mockRepo.On("GetCoinPackageByID", mock.Anything, packageID).Return(pkg, nil)
	mockRepo.On("InsertTransaction", mock.Anything, (*repository.Queries)(nil), mock.MatchedBy(func(tx domain.Transaction) bool {
		return tx.To != nil &&
			*tx.To == userID &&
			tx.Amount == pkg.CoinAmount &&
			tx.Status == domain.StatusPending &&
			tx.PayOSOrderCode != nil
	})).Return(createdTx, nil)
	mockPayOS.On("CreatePaymentLink", mock.Anything, mock.MatchedBy(func(req payos.CreateLinkRequest) bool {
		return req.Amount == int(pkg.PriceVND) && req.OrderCode > 0
	})).Return(payos.CreateLinkResponse{
		CheckoutURL: "https://payos.vn/checkout/abc",
		OrderCode:   123456789,
	}, nil)

	resp, err := svc.CreateDepositCheckout(context.Background(), userID, packageID, "https://app/return", "https://app/cancel")

	assert.NoError(t, err)
	assert.Equal(t, "https://payos.vn/checkout/abc", resp.CheckoutURL)
	assert.Equal(t, pkg.PriceVND, resp.Amount)
	assert.Equal(t, pkg.CoinAmount, resp.CoinAmount)
	assert.Equal(t, txID, resp.TransactionID)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

func TestCreateDepositCheckout_PackageNotFound(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	packageID := uuid.New()
	mockRepo.On("GetCoinPackageByID", mock.Anything, packageID).Return(domain.CoinPackage{}, apperror.New(apperror.CodeCoinPackageNotFound, "not found"))

	_, err := svc.CreateDepositCheckout(context.Background(), uuid.New(), packageID, "", "")
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodeCoinPackageNotFound))
	mockRepo.AssertExpectations(t)
}

// PayOS may have created the link even when our call failed (timeout). The order therefore stays
// pending, so a later PAID webhook or sync can still credit it.
func TestCreateDepositCheckout_PayOSError_LeavesOrderPending(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	packageID := uuid.New()
	txID := uuid.New()

	pkg := domain.CoinPackage{
		ID: packageID, Name: "Pack", PriceVND: 20000, CoinAmount: 50, Currency: "VND", IsActive: true,
	}
	createdTx := domain.Transaction{ID: txID, To: &userID, Amount: 50, Status: domain.StatusPending}

	mockRepo.On("GetCoinPackageByID", mock.Anything, packageID).Return(pkg, nil)
	mockRepo.On("InsertTransaction", mock.Anything, (*repository.Queries)(nil), mock.Anything).Return(createdTx, nil)
	mockPayOS.On("CreatePaymentLink", mock.Anything, mock.Anything).Return(payos.CreateLinkResponse{}, errors.New("gateway timeout"))

	_, err := svc.CreateDepositCheckout(context.Background(), userID, packageID, "", "")
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — successful webhook credits wallet
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012,"amount":50000}}`)

	webhookPayload := &payos.WebhookData{
		Code:    "00",
		Success: true,
	}
	webhookPayload.Data.OrderCode = orderCode
	webhookPayload.Data.Amount = 50000

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     50000,
		AmountPaid: 50000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	pendingTx := domain.Transaction{
		ID:             txID,
		To:             &userID,
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}

	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(pendingTx, nil)
	mockRepo.On("CreditPaidOrder", mock.Anything, (*repository.Queries)(nil), userID, int32(100)).Return(nil).Once()

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// Success via PayOS status PAID even if envelope code is non-00.
func TestHandlePayOSWebhook_Success_ViaStatusPaid(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	orderCode := int64(200000000001)
	body := []byte(`{"code":"99","success":false,"data":{"orderCode":200000000001,"code":"00"}}`)

	webhookPayload := &payos.WebhookData{Code: "99", Success: false}
	webhookPayload.Data.OrderCode = orderCode
	webhookPayload.Data.Code = "00"

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     50000,
		AmountPaid: 50000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	pendingTx := domain.Transaction{ID: txID, To: &userID, Amount: 200, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(pendingTx, nil)
	mockRepo.On("CreditPaidOrder", mock.Anything, (*repository.Queries)(nil), userID, int32(200)).Return(nil).Once()

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — duplicate / replayed callback
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_AlreadyProcessed_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	txID := uuid.New()
	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012,"amount":50000}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     50000,
		AmountPaid: 50000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	alreadySuccessTx := domain.Transaction{
		ID:             txID,
		Status:         domain.StatusSuccess,
		PayOSOrderCode: &orderCode,
	}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(alreadySuccessTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	// UpdateTransactionStatus must NOT be called on a replay.
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

func TestHandlePayOSWebhook_AlreadyProcessed_Failed(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	txID := uuid.New()
	orderCode := int64(123456789999)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789999}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode: orderCode,
		Status:    payos.PaymentLinkStatusFailed,
	}, nil)

	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(
		domain.Transaction{ID: txID, Status: domain.StatusFailed, PayOSOrderCode: &orderCode}, nil,
	)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — invalid webhook signature
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_InvalidSignature(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	body := []byte(`{"invalid":"signature"}`)
	mockPayOS.On("VerifyWebhookData", body).Return(nil, errors.New("invalid signature"))

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — wrong / unknown order code
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_WrongOrderCode_NotFound(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(999999999999)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":999999999999}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     50000,
		AmountPaid: 50000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).
		Return(domain.Transaction{}, apperror.New(apperror.CodeTransactionNotFound, "not found"))

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodeTransactionNotFound))
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — failed payment (PayOS status FAILED)
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_Failed(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	txID := uuid.New()
	orderCode := int64(111111111111)
	body := []byte(`{"code":"99","success":false,"data":{"orderCode":111111111111,"code":"99"}}`)

	webhookPayload := &payos.WebhookData{Code: "99", Success: false}
	webhookPayload.Data.OrderCode = orderCode
	webhookPayload.Data.Code = "99"

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode: orderCode,
		Status:    payos.PaymentLinkStatusFailed,
	}, nil)

	pendingTx := domain.Transaction{ID: txID, Amount: 100, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusFailed).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	// Wallet must NOT be credited on failure.
	mockRepo.AssertNotCalled(t, "CreditPaidOrder")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — cancelled payment (PayOS status CANCELLED)
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_Cancelled(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	txID := uuid.New()
	orderCode := int64(222222222222)
	body := []byte(`{"code":"01","success":false,"data":{"orderCode":222222222222,"code":"01"}}`)

	webhookPayload := &payos.WebhookData{Code: "01", Success: false}
	webhookPayload.Data.OrderCode = orderCode
	webhookPayload.Data.Code = "01"

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode: orderCode,
		Status:    payos.PaymentLinkStatusCancelled,
	}, nil)

	pendingTx := domain.Transaction{ID: txID, Amount: 100, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusCancelled).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	// Wallet must NOT be credited on cancellation.
	mockRepo.AssertNotCalled(t, "CreditPaidOrder")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — a failed credit leaves the order retryable
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_CreditFails_ReturnsErrorSoPayOSRetries(t *testing.T) {
	// The wallet is created, locked and overflow-checked by the ledger. Its error must reach the
	// caller, which rolls back the whole transaction so the order stays pending for the retry.
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	orderCode := int64(333333333333)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":333333333333}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     50000,
		AmountPaid: 50000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	pendingTx := domain.Transaction{ID: txID, To: &userID, Amount: 50, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	creditErr := apperror.New(apperror.CodeInternal, "wallet balance would exceed the maximum")
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(pendingTx, nil)
	mockRepo.On("CreditPaidOrder", mock.Anything, (*repository.Queries)(nil), userID, int32(50)).Return(creditErr)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.ErrorIs(t, err, creditErr)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Security & Validation tests
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_OrderCodeMismatch(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012,"amount":50000}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	// Return a payment link with a mismatched order code
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  999999999999,
		Amount:     50000,
		AmountPaid: 50000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "order code mismatch")
	mockPayOS.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "ExecTx")
}

func TestHandlePayOSWebhook_Underpaid(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012,"amount":50000}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	// Amount paid is less than required amount
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     50000,
		AmountPaid: 20000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	pendingTx := domain.Transaction{
		ID:             uuid.New(),
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "invalid or underpaid")
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockPayOS.AssertExpectations(t)
}

func TestHandlePayOSWebhook_ZeroOrNegativeAmount(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     0,
		AmountPaid: 0,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	pendingTx := domain.Transaction{
		ID:             uuid.New(),
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "invalid or underpaid")
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockPayOS.AssertExpectations(t)
}

func TestHandlePayOSWebhook_PendingStatus_ReturnsError(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode: orderCode,
		Status:    payos.PaymentLinkStatusPending,
	}, nil)

	pendingTx := domain.Transaction{
		ID:             uuid.New(),
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "payment is in progress")
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockPayOS.AssertExpectations(t)
}

func TestHandlePayOSWebhook_GetPaymentLinkError(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(nil, errors.New("network error"))

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "failed to get PayOS payment link")
	mockPayOS.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "ExecTx")
}

// ---------------------------------------------------------------------------
// ListTransactions
// ---------------------------------------------------------------------------

func TestListTransactions_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txs := []domain.Transaction{
		{ID: uuid.New(), Amount: 100, Status: domain.StatusSuccess},
		{ID: uuid.New(), Amount: 50, Status: domain.StatusPending},
	}
	// page=1, pageSize=20 → limit=20, offset=0
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userID, int32(20), int32(0)).
		Return(txs, int64(2), nil)

	result, total, err := svc.ListTransactions(context.Background(), userID, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}

func TestListTransactions_PageClampedToOne(t *testing.T) {
	// page < 1 should be treated as page = 1 (offset 0).
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userID, int32(20), int32(0)).
		Return([]domain.Transaction{}, int64(0), nil)

	_, _, err := svc.ListTransactions(context.Background(), userID, 0, 20)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestListTransactions_PageSizeClamped(t *testing.T) {
	// pageSize=0 and pageSize>100 both normalize to 20.
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()

	// pageSize = 0 → defaults to 20
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userID, int32(20), int32(0)).
		Return([]domain.Transaction{}, int64(0), nil).Once()

	_, _, err := svc.ListTransactions(context.Background(), userID, 1, 0)
	assert.NoError(t, err)

	// pageSize = 200 → defaults to 20
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userID, int32(20), int32(0)).
		Return([]domain.Transaction{}, int64(0), nil).Once()

	_, _, err = svc.ListTransactions(context.Background(), userID, 1, 200)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestListTransactions_Pagination_PageTwo(t *testing.T) {
	// page=2, pageSize=10 → limit=10, offset=10
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userID, int32(10), int32(10)).
		Return([]domain.Transaction{}, int64(15), nil)

	_, total, err := svc.ListTransactions(context.Background(), userID, 2, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(15), total)
	mockRepo.AssertExpectations(t)
}

// Scoping: each call receives exactly the authenticated user's ID — cross-user
// access is impossible because the service never accepts a "target user" param.
func TestListTransactions_ScopedToOwnUserID(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userA := uuid.New()
	userB := uuid.New()

	mockRepo.On("ListTransactionsByUserID", mock.Anything, userA, int32(20), int32(0)).
		Return([]domain.Transaction{{ID: uuid.New(), Amount: 100}}, int64(1), nil)
	// userB must get its own separate call — service does not mix IDs.
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userB, int32(20), int32(0)).
		Return([]domain.Transaction{}, int64(0), nil)

	resultA, totalA, _ := svc.ListTransactions(context.Background(), userA, 1, 20)
	resultB, totalB, _ := svc.ListTransactions(context.Background(), userB, 1, 20)

	assert.Len(t, resultA, 1)
	assert.Equal(t, int64(1), totalA)
	assert.Len(t, resultB, 0)
	assert.Equal(t, int64(0), totalB)
	mockRepo.AssertExpectations(t)
}

func TestListTransactions_RepoError(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	dbErr := errors.New("db: timeout")
	mockRepo.On("ListTransactionsByUserID", mock.Anything, userID, int32(20), int32(0)).
		Return([]domain.Transaction(nil), int64(0), dbErr)

	_, _, err := svc.ListTransactions(context.Background(), userID, 1, 20)
	assert.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListPackages
// ---------------------------------------------------------------------------

func TestListPackages_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	packages := []domain.CoinPackage{
		{ID: uuid.New(), Name: "Starter", PriceVND: 20000, CoinAmount: 50, Currency: "VND", IsActive: true},
		{ID: uuid.New(), Name: "Pro", PriceVND: 50000, CoinAmount: 150, Currency: "VND", IsActive: true},
	}
	mockRepo.On("ListCoinPackages", mock.Anything).Return(packages, nil)

	result, err := svc.ListPackages(context.Background())
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}

func TestListPackages_Empty(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	mockRepo.On("ListCoinPackages", mock.Anything).Return([]domain.CoinPackage{}, nil)

	result, err := svc.ListPackages(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestListPackages_RepoError(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	dbErr := errors.New("db: timeout")
	mockRepo.On("ListCoinPackages", mock.Anything).Return([]domain.CoinPackage(nil), dbErr)

	_, err := svc.ListPackages(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	mockRepo.AssertExpectations(t)
}

func TestHandlePayOSWebhook_ProcessingStatus_ReturnsError(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012}}`)
	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode: orderCode,
		Status:    payos.PaymentLinkStatusProcessing,
	}, nil)

	pendingTx := domain.Transaction{
		ID:             uuid.New(),
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "payment is in progress")
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
}

func TestHandlePayOSWebhook_PriceMismatch_AgainstExpectedVND(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012}}`)
	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	// Customer paid 20,000 VND but package was 50,000 VND
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode:  orderCode,
		Amount:     20000,
		AmountPaid: 20000,
		Status:     payos.PaymentLinkStatusPaid,
	}, nil)

	pendingTx := domain.Transaction{
		ID:             uuid.New(),
		Amount:         100,
		Description:    "Deposit: Starter Pack (50000 VND)",
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
	assert.Contains(t, err.Error(), "less than expected 50000 VND")
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
}

func TestCreateDepositCheckout_InactivePackage(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	packageID := uuid.New()
	mockRepo.On("GetCoinPackageByID", mock.Anything, packageID).Return(domain.CoinPackage{
		ID:       packageID,
		Name:     "Disabled Pack",
		IsActive: false,
	}, nil)

	_, err := svc.CreateDepositCheckout(context.Background(), uuid.New(), packageID, "", "")
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodeCoinPackageNotFound))
	assert.Contains(t, err.Error(), "inactive")
}

func TestSyncDepositStatus_Cancelled(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	orderCode := int64(888888888888)

	pendingTx := domain.Transaction{
		ID:             txID,
		To:             &userID,
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}

	mockRepo.On("GetTransactionByPayOSOrderCode", mock.Anything, orderCode).Return(pendingTx, nil)
	mockPayOS.On("GetPaymentLink", mock.Anything, orderCode).Return(&payos.PaymentLink{
		OrderCode: orderCode,
		Status:    payos.PaymentLinkStatusCancelled,
	}, nil)

	cancelledTx := pendingTx
	cancelledTx.Status = domain.StatusCancelled
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusCancelled).Return(cancelledTx, nil)

	res, err := svc.SyncDepositStatus(context.Background(), userID, orderCode)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, res.Status)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

func TestSyncDepositStatus_AlreadyProcessed(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	orderCode := int64(888888888889)

	successTx := domain.Transaction{
		ID:             txID,
		To:             &userID,
		Amount:         100,
		Status:         domain.StatusSuccess,
		PayOSOrderCode: &orderCode,
	}

	mockRepo.On("GetTransactionByPayOSOrderCode", mock.Anything, orderCode).Return(successTx, nil)

	res, err := svc.SyncDepositStatus(context.Background(), userID, orderCode)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusSuccess, res.Status)
	mockPayOS.AssertNotCalled(t, "GetPaymentLink")
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
}

func TestSyncDepositStatus_ForbiddenUser(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	ownerID := uuid.New()
	otherUserID := uuid.New()
	orderCode := int64(888888888890)

	pendingTx := domain.Transaction{
		ID:             uuid.New(),
		To:             &ownerID,
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}

	mockRepo.On("GetTransactionByPayOSOrderCode", mock.Anything, orderCode).Return(pendingTx, nil)

	_, err := svc.SyncDepositStatus(context.Background(), otherUserID, orderCode)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodeForbidden))
}

// ---------------------------------------------------------------------------
// Admin Billing & Revenue Read-Only Service Tests (KAN-191, KAN-192, KAN-241)
// ---------------------------------------------------------------------------

func TestAdminGetBillingSummary_OnlyCountsSuccessPayOSDeposits(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	expectedFilter := domain.AdminTransactionFilter{
		Page:      1,
		PageSize:  20,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	// Sample ledger scenario:
	// - 2 successful PayOS deposits (50,000 VND + 100,000 VND = 150,000 VND, 300 coins sold)
	// - 1 pending PayOS deposit (200,000 VND — NOT counted in revenue)
	// - 1 failed PayOS deposit (50,000 VND — NOT counted in revenue)
	// - 1 cancelled PayOS deposit (50,000 VND — NOT counted in revenue)
	// - 2 internal coin spends (40 coins — NOT subtracted from VND revenue)
	// - 1 internal slot refund (10 coins — NOT subtracted from VND revenue)
	expectedSummary := domain.BillingSummary{
		TotalRevenue:       150000,
		SuccessfulDeposits: 2,
		TotalDeposits:      5, // 2 success + 1 pending + 1 failed + 1 cancelled PayOS top-ups
		SuccessCount:       5, // 2 topups + 2 spends + 1 refund
		PendingCount:       1,
		FailedCount:        1,
		CancelledCount:     1,
		TotalTransactions:  8,
		AverageDeposit:     75000,
		UniquePayers:       2,
		CoinsSold:          300,
		CoinsSpent:         40,
		CoinsRefunded:      10,
		CoinsNet:           270,
		SuccessRate:        40.0, // 2 of 5 PayOS deposit attempts
		PackageBreakdown: []domain.PackageRevenueItem{
			{PackageName: "Pro Pack", RevenueVND: 100000, DepositCount: 1, CoinsSold: 200},
			{PackageName: "Starter Pack", RevenueVND: 50000, DepositCount: 1, CoinsSold: 100},
		},
		FeatureBreakdown: []domain.FeatureCoinItem{
			{FeatureName: "Practice interview charge", CoinsSpent: 40, SpendCount: 2},
		},
	}

	mockRepo.On("GetBillingSummary", mock.Anything, expectedFilter).Return(expectedSummary, nil)

	res, err := svc.GetBillingSummary(context.Background(), domain.AdminTransactionFilter{})
	assert.NoError(t, err)
	assert.Equal(t, int64(150000), res.TotalRevenue)
	assert.Equal(t, int64(2), res.SuccessfulDeposits)
	assert.Equal(t, int64(5), res.TotalDeposits)
	assert.Equal(t, 40.0, res.SuccessRate)
	assert.Equal(t, int64(1), res.PendingCount)
	assert.Equal(t, int64(1), res.FailedCount)
	assert.Equal(t, int64(1), res.CancelledCount)
	assert.Equal(t, res.SuccessCount+res.PendingCount+res.FailedCount+res.CancelledCount, res.TotalTransactions)
	mockRepo.AssertExpectations(t)
}

func TestAdminListBillingTransactions_FiltersAndPagination(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	minAmt := int64(50000)
	maxAmt := int64(200000)
	orderCode := int64(100201)
	txID := uuid.New()

	expectedFilter := domain.AdminTransactionFilter{
		Page:        2,
		PageSize:    50,
		Search:      "alice@example.com",
		Status:      "success",
		Type:        "topup",
		PackageName: "Starter",
		AmountMin:   &minAmt,
		AmountMax:   &maxAmt,
		SortBy:      "amount",
		SortOrder:   "asc",
	}

	expectedPage := domain.AdminTransactionPage{
		Items: []domain.AdminTransactionItem{
			{
				ID:               txID,
				UserID:           uuid.New().String(),
				UserName:         "Alice Nguyen",
				UserEmail:        "alice@example.com",
				Type:             "topup",
				PackageOrFeature: "Starter Pack",
				PaidVND:          50000,
				CoinAmount:       100,
				Amount:           100,
				Currency:         "VND",
				Description:      "Deposit: Starter Pack (50000 VND)",
				Status:           domain.StatusSuccess,
				PayOSOrderCode:   &orderCode,
			},
		},
		Total:    51,
		Page:     2,
		PageSize: 50,
	}

	mockRepo.On("ListAdminTransactions", mock.Anything, expectedFilter).Return(expectedPage, nil)

	res, err := svc.ListBillingTransactions(context.Background(), domain.AdminTransactionFilter{
		Page:        2,
		PageSize:    50,
		Search:      "  alice@example.com  ",
		Status:      "SUCCESS",
		Type:        "TOPUP",
		PackageName: " Starter ",
		AmountMin:   &minAmt,
		AmountMax:   &maxAmt,
		SortBy:      "amount",
		SortOrder:   "ASC",
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(51), res.Total)
	assert.Len(t, res.Items, 1)
	assert.Equal(t, int64(50000), res.Items[0].PaidVND)
	assert.Equal(t, int64(100), res.Items[0].CoinAmount)
	mockRepo.AssertExpectations(t)
}

func TestValidateAdminFilter_RejectsInvalidInputs(t *testing.T) {
	octStart := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	octEnd := time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)
	neg := int64(-10)
	high := int64(500)
	low := int64(100)

	cases := []struct {
		name   string
		filter domain.AdminTransactionFilter
	}{
		{
			name:   "invalid status",
			filter: domain.AdminTransactionFilter{Status: "refunded"},
		},
		{
			name:   "invalid type",
			filter: domain.AdminTransactionFilter{Type: "wire_transfer"},
		},
		{
			name:   "invalid sort_by",
			filter: domain.AdminTransactionFilter{SortBy: "user_email"},
		},
		{
			name:   "invalid sort_order",
			filter: domain.AdminTransactionFilter{SortOrder: "random"},
		},
		{
			name:   "negative amount_min",
			filter: domain.AdminTransactionFilter{AmountMin: &neg},
		},
		{
			name:   "amount_min greater than amount_max",
			filter: domain.AdminTransactionFilter{AmountMin: &high, AmountMax: &low},
		},
		{
			name:   "page below 1",
			filter: domain.AdminTransactionFilter{Page: -1},
		},
		{
			name:   "page exceeds 10000",
			filter: domain.AdminTransactionFilter{Page: 10001},
		},
		{
			name:   "page_size exceeds max",
			filter: domain.AdminTransactionFilter{PageSize: 101},
		},
		{
			name:   "date_from after date_to",
			filter: domain.AdminTransactionFilter{DateFrom: &octEnd, DateTo: &octStart},
		},
		{
			name:   "date_from equal to date_to",
			filter: domain.AdminTransactionFilter{DateFrom: &octStart, DateTo: &octStart},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.filter
			err := service.ValidateAdminFilter(&f, 100)
			assert.Error(t, err)
			assert.True(t, apperror.IsCode(err, apperror.CodeValidation))
		})
	}
}

func TestValidateAdminFilter_DateWindowAndCreatedAtSort(t *testing.T) {
	from := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)

	t.Run("defaults to newest first", func(t *testing.T) {
		f := domain.AdminTransactionFilter{}
		assert.NoError(t, service.ValidateAdminFilter(&f, 100))
		assert.Equal(t, "created_at", f.SortBy)
		assert.Equal(t, "desc", f.SortOrder)
	})

	t.Run("accepts created_at and still accepts id", func(t *testing.T) {
		for _, sortBy := range []string{"created_at", "CREATED_AT", "id"} {
			f := domain.AdminTransactionFilter{SortBy: sortBy}
			assert.NoError(t, service.ValidateAdminFilter(&f, 100), sortBy)
		}
	})

	t.Run("accepts a full window and either open end", func(t *testing.T) {
		for _, f := range []domain.AdminTransactionFilter{
			{DateFrom: &from, DateTo: &to},
			{DateFrom: &from},
			{DateTo: &to},
		} {
			f := f
			assert.NoError(t, service.ValidateAdminFilter(&f, 100))
		}
	})
}

func TestDepositSuccessRate_CountsPayOSDepositsOnly(t *testing.T) {
	// 2 paid out of 5 PayOS attempts. Internal coin rows (always 'success') are not part of either
	// argument, so a busy coin ledger cannot inflate the deposit success rate.
	assert.Equal(t, 40.0, domain.DepositSuccessRate(2, 5))
	assert.Equal(t, 66.7, domain.DepositSuccessRate(2, 3))
	assert.Equal(t, 100.0, domain.DepositSuccessRate(4, 4))
	assert.Equal(t, 0.0, domain.DepositSuccessRate(0, 0), "no deposit attempts must not divide by zero")
}
