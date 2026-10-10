package service_test

import (
	"context"
	"errors"
	"testing"

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

func (m *MockRepo) CreateWallet(ctx context.Context, userID uuid.UUID) (domain.Wallet, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.Wallet), args.Error(1)
}

func (m *MockRepo) GetWalletForUpdate(ctx context.Context, q *repository.Queries, userID uuid.UUID) (domain.Wallet, error) {
	args := m.Called(ctx, q, userID)
	return args.Get(0).(domain.Wallet), args.Error(1)
}

func (m *MockRepo) UpdateWalletBalance(ctx context.Context, q *repository.Queries, walletID uuid.UUID, amount int32) (domain.Wallet, error) {
	args := m.Called(ctx, q, walletID, amount)
	return args.Get(0).(domain.Wallet), args.Error(1)
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
	mockRepo.On("CreateWallet", mock.Anything, userID).Return(expected, nil)

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

func TestCreateDepositCheckout_PayOSError(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	packageID := uuid.New()
	txID := uuid.New()

	pkg := domain.CoinPackage{
		ID: packageID, Name: "Pack", PriceVND: 20000, CoinAmount: 50, Currency: "VND",
	}
	createdTx := domain.Transaction{ID: txID, To: &userID, Amount: 50, Status: domain.StatusPending}

	mockRepo.On("GetCoinPackageByID", mock.Anything, packageID).Return(pkg, nil)
	mockRepo.On("InsertTransaction", mock.Anything, (*repository.Queries)(nil), mock.Anything).Return(createdTx, nil)
	mockPayOS.On("CreatePaymentLink", mock.Anything, mock.Anything).Return(payos.CreateLinkResponse{}, errors.New("gateway timeout"))

	_, err := svc.CreateDepositCheckout(context.Background(), userID, packageID, "", "")
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodePayOSError))
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
	walletID := uuid.New()
	orderCode := int64(123456789012)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456789012,"amount":50000}}`)

	webhookPayload := &payos.WebhookData{
		Code:    "00",
		Success: true,
	}
	webhookPayload.Data.OrderCode = orderCode
	webhookPayload.Data.Amount = 50000

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)

	pendingTx := domain.Transaction{
		ID:             txID,
		To:             &userID,
		Amount:         100,
		Status:         domain.StatusPending,
		PayOSOrderCode: &orderCode,
	}

	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(pendingTx, nil)
	mockRepo.On("GetWalletForUpdate", mock.Anything, (*repository.Queries)(nil), userID).Return(domain.Wallet{ID: walletID, UserID: userID, Balance: 50}, nil)
	mockRepo.On("UpdateWalletBalance", mock.Anything, (*repository.Queries)(nil), walletID, int32(150)).Return(domain.Wallet{ID: walletID, UserID: userID, Balance: 150}, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// Success via data-level code (webhook envelope code is non-00 but inner data is 00).
func TestHandlePayOSWebhook_Success_ViaDataCode(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	walletID := uuid.New()
	orderCode := int64(200000000001)
	body := []byte(`{"code":"99","success":false,"data":{"orderCode":200000000001,"code":"00"}}`)

	webhookPayload := &payos.WebhookData{Code: "99", Success: false}
	webhookPayload.Data.OrderCode = orderCode
	webhookPayload.Data.Code = "00"

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)
	pendingTx := domain.Transaction{ID: txID, To: &userID, Amount: 200, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(pendingTx, nil)
	mockRepo.On("GetWalletForUpdate", mock.Anything, (*repository.Queries)(nil), userID).Return(domain.Wallet{ID: walletID, Balance: 0}, nil)
	mockRepo.On("UpdateWalletBalance", mock.Anything, (*repository.Queries)(nil), walletID, int32(200)).Return(domain.Wallet{ID: walletID, Balance: 200}, nil)

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
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(
		domain.Transaction{ID: txID, Status: domain.StatusFailed, PayOSOrderCode: &orderCode}, nil,
	)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "UpdateTransactionStatus")
	mockRepo.AssertExpectations(t)
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
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).
		Return(domain.Transaction{}, apperror.New(apperror.CodeTransactionNotFound, "not found"))

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodeTransactionNotFound))
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — failed payment (non-success code)
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

	pendingTx := domain.Transaction{ID: txID, Amount: 100, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusFailed).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	// Wallet must NOT be credited on failure.
	mockRepo.AssertNotCalled(t, "GetWalletForUpdate")
	mockRepo.AssertNotCalled(t, "UpdateWalletBalance")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — cancelled payment (PayOS code "01")
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

	pendingTx := domain.Transaction{ID: txID, Amount: 100, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusCancelled).Return(pendingTx, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	// Wallet must NOT be credited on cancellation.
	mockRepo.AssertNotCalled(t, "GetWalletForUpdate")
	mockRepo.AssertNotCalled(t, "UpdateWalletBalance")
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// HandlePayOSWebhook — pending payment stays pending (no-op until finalized)
// ---------------------------------------------------------------------------

func TestHandlePayOSWebhook_WalletAutoCreated_OnSuccess(t *testing.T) {
	// Wallet does not yet exist for the user; should be auto-created then credited.
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	newWalletID := uuid.New()
	orderCode := int64(333333333333)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":333333333333}}`)

	webhookPayload := &payos.WebhookData{Code: "00", Success: true}
	webhookPayload.Data.OrderCode = orderCode

	mockPayOS.On("VerifyWebhookData", body).Return(webhookPayload, nil)

	pendingTx := domain.Transaction{ID: txID, To: &userID, Amount: 50, Status: domain.StatusPending, PayOSOrderCode: &orderCode}
	mockRepo.On("GetTransactionByPayOSOrderCodeForUpdate", mock.Anything, (*repository.Queries)(nil), orderCode).Return(pendingTx, nil)
	mockRepo.On("UpdateTransactionStatus", mock.Anything, (*repository.Queries)(nil), txID, domain.StatusSuccess).Return(pendingTx, nil)
	mockRepo.On("GetWalletForUpdate", mock.Anything, (*repository.Queries)(nil), userID).
		Return(domain.Wallet{}, apperror.New(apperror.CodeWalletNotFound, "not found"))
	newWallet := domain.Wallet{ID: newWalletID, UserID: userID, Balance: 0}
	mockRepo.On("CreateWallet", mock.Anything, userID).Return(newWallet, nil)
	mockRepo.On("UpdateWalletBalance", mock.Anything, (*repository.Queries)(nil), newWalletID, int32(50)).
		Return(domain.Wallet{ID: newWalletID, Balance: 50}, nil)

	err := svc.HandlePayOSWebhook(context.Background(), body)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Internal coin spending — does NOT call PayOS
// ---------------------------------------------------------------------------

func TestChargePracticeInterview_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	walletID := uuid.New()
	existingWallet := domain.Wallet{ID: walletID, UserID: userID, Balance: 100}

	mockRepo.On("GetWalletForUpdate", mock.Anything, (*repository.Queries)(nil), userID).Return(existingWallet, nil)
	mockRepo.On("UpdateWalletBalance", mock.Anything, (*repository.Queries)(nil), walletID, int32(50)).Return(domain.Wallet{ID: walletID, UserID: userID, Balance: 50}, nil)
	mockRepo.On("InsertTransaction", mock.Anything, (*repository.Queries)(nil), mock.MatchedBy(func(tx domain.Transaction) bool {
		return tx.From != nil && *tx.From == userID && tx.Amount == 50 && tx.Status == domain.StatusSuccess && tx.PayOSOrderCode == nil
	})).Return(domain.Transaction{ID: uuid.New()}, nil)

	err := svc.ChargePracticeInterview(context.Background(), userID, 50)
	assert.NoError(t, err)
	// PayOS must never be invoked for internal charges.
	mockPayOS.AssertNotCalled(t, "CreatePaymentLink")
	mockPayOS.AssertNotCalled(t, "VerifyWebhookData")
	mockRepo.AssertExpectations(t)
}

func TestChargePracticeInterview_InsufficientBalance(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	existingWallet := domain.Wallet{ID: uuid.New(), UserID: userID, Balance: 20}

	mockRepo.On("GetWalletForUpdate", mock.Anything, (*repository.Queries)(nil), userID).Return(existingWallet, nil)

	err := svc.ChargePracticeInterview(context.Background(), userID, 50)
	assert.Error(t, err)
	assert.True(t, apperror.IsCode(err, apperror.CodeInsufficientBalance))
	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Internal refund — does NOT call PayOS
// ---------------------------------------------------------------------------

func TestRefundRecruitmentCapacity_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	recruiterID := uuid.New()
	walletID := uuid.New()
	wallet := domain.Wallet{ID: walletID, UserID: recruiterID, Balance: 100}

	mockRepo.On("GetWalletForUpdate", mock.Anything, (*repository.Queries)(nil), recruiterID).Return(wallet, nil)
	mockRepo.On("UpdateWalletBalance", mock.Anything, (*repository.Queries)(nil), walletID, int32(150)).Return(domain.Wallet{ID: walletID, Balance: 150}, nil)
	mockRepo.On("InsertTransaction", mock.Anything, (*repository.Queries)(nil), mock.MatchedBy(func(tx domain.Transaction) bool {
		return tx.To != nil && *tx.To == recruiterID && tx.Amount == 50 && tx.Status == domain.StatusSuccess && tx.PayOSOrderCode == nil
	})).Return(domain.Transaction{ID: uuid.New()}, nil)

	err := svc.RefundRecruitmentCapacity(context.Background(), recruiterID, 50)
	assert.NoError(t, err)
	mockPayOS.AssertNotCalled(t, "CreatePaymentLink")
	mockRepo.AssertExpectations(t)
}
