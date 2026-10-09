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
		return tx.From != nil && *tx.From == userID && tx.Amount == 50 && tx.Status == domain.StatusSuccess
	})).Return(domain.Transaction{ID: uuid.New()}, nil)

	err := svc.ChargePracticeInterview(context.Background(), userID, 50)
	assert.NoError(t, err)
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

func TestHandlePayOSWebhook_Success(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	userID := uuid.New()
	txID := uuid.New()
	walletID := uuid.New()
	orderCode := int64(123456)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456,"amount":50000}}`)

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

func TestHandlePayOSWebhook_AlreadyProcessed(t *testing.T) {
	mockRepo := new(MockRepo)
	mockPayOS := new(MockPayOS)
	svc := service.NewService(mockRepo, mockPayOS)

	txID := uuid.New()
	orderCode := int64(123456)
	body := []byte(`{"code":"00","success":true,"data":{"orderCode":123456,"amount":50000}}`)

	webhookPayload := &payos.WebhookData{
		Code:    "00",
		Success: true,
	}
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
	mockRepo.AssertExpectations(t)
	mockPayOS.AssertExpectations(t)
}

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
