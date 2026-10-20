package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type WalletRepository interface {
	ExecTx(ctx context.Context, fn func(q *repository.Queries) error) error
	GetWalletByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error)
	CreateWallet(ctx context.Context, q *repository.Queries, userID uuid.UUID) (domain.Wallet, error)
	GetWalletForUpdate(ctx context.Context, q *repository.Queries, userID uuid.UUID) (domain.Wallet, error)
	UpdateWalletBalance(ctx context.Context, q *repository.Queries, walletID uuid.UUID, amount int32) (domain.Wallet, error)
	InsertTransaction(ctx context.Context, q *repository.Queries, tx domain.Transaction) (domain.Transaction, error)
	GetTransactionByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error)
	GetTransactionByPayOSOrderCode(ctx context.Context, orderCode int64) (domain.Transaction, error)
	GetTransactionByPayOSOrderCodeForUpdate(ctx context.Context, q *repository.Queries, orderCode int64) (domain.Transaction, error)
	UpdateTransactionStatus(ctx context.Context, q *repository.Queries, id uuid.UUID, status domain.TransactionStatus) (domain.Transaction, error)
	ListTransactionsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]domain.Transaction, int64, error)
	ListCoinPackages(ctx context.Context) ([]domain.CoinPackage, error)
	GetCoinPackageByID(ctx context.Context, id uuid.UUID) (domain.CoinPackage, error)
}

type PayOSClient interface {
	CreatePaymentLink(ctx context.Context, req payos.CreateLinkRequest) (payos.CreateLinkResponse, error)
	VerifyWebhookData(body []byte) (*payos.WebhookData, error)
	GetPaymentLink(ctx context.Context, orderCode int64) (*payos.PaymentLink, error)
}

type Service struct {
	repo  WalletRepository
	payos PayOSClient
}

func NewService(repo WalletRepository, payos PayOSClient) *Service {
	return &Service{
		repo:  repo,
		payos: payos,
	}
}

func (s *Service) GetWallet(ctx context.Context, userID uuid.UUID) (domain.Wallet, error) {
	w, err := s.repo.GetWalletByUserID(ctx, userID)
	if err != nil {
		if apperror.IsCode(err, apperror.CodeWalletNotFound) {
			return s.repo.CreateWallet(ctx, nil, userID)
		}
		return domain.Wallet{}, err
	}
	return w, nil
}

func (s *Service) ListTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int32) ([]domain.Transaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if page > 10_000 {
		return []domain.Transaction{}, 0, nil
	}
	offset := (page - 1) * pageSize
	return s.repo.ListTransactionsByUserID(ctx, userID, pageSize, offset)
}

func (s *Service) ListPackages(ctx context.Context) ([]domain.CoinPackage, error) {
	return s.repo.ListCoinPackages(ctx)
}

func (s *Service) CreateDepositCheckout(ctx context.Context, userID uuid.UUID, packageID uuid.UUID, returnURL, cancelURL string) (domain.DepositCheckoutResponse, error) {
	cp, err := s.repo.GetCoinPackageByID(ctx, packageID)
	if err != nil {
		return domain.DepositCheckoutResponse{}, err
	}
	if !cp.IsActive {
		return domain.DepositCheckoutResponse{}, apperror.New(apperror.CodeCoinPackageNotFound, "coin package is inactive")
	}

	orderCode, err := generateOrderCode()
	if err != nil {
		return domain.DepositCheckoutResponse{}, apperror.Wrap(apperror.CodeInternal, "failed to generate order code", err)
	}

	// PayOS description has a 25-character limit.
	payosDesc := cp.Name
	if len(payosDesc) > 25 {
		payosDesc = payosDesc[:25]
	}

	// Internal transaction description stores package name and expected price for verification.
	dbDescription := fmt.Sprintf("Deposit: %s (%d VND)", cp.Name, cp.PriceVND)

	// Create pending transaction record first
	var txRecord domain.Transaction
	err = s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		created, err := s.repo.InsertTransaction(ctx, q, domain.Transaction{
			From:           nil,
			To:             &userID,
			Amount:         cp.CoinAmount,
			Currency:       "VND",
			Description:    dbDescription,
			Status:         domain.StatusPending,
			PayOSOrderCode: &orderCode,
		})
		if err != nil {
			return err
		}
		txRecord = created
		return nil
	})
	if err != nil {
		return domain.DepositCheckoutResponse{}, err
	}

	payosReq := payos.CreateLinkRequest{
		OrderCode:   orderCode,
		Amount:      int(cp.PriceVND),
		Description: payosDesc,
		ReturnURL:   returnURL,
		CancelURL:   cancelURL,
	}

	linkData, err := s.payos.CreatePaymentLink(ctx, payosReq)
	if err != nil {
		// Mark transaction failed to prevent leaving dangling pending transactions
		_ = s.repo.ExecTx(ctx, func(q *repository.Queries) error {
			_, updateErr := s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusFailed)
			return updateErr
		})
		return domain.DepositCheckoutResponse{}, apperror.Wrap(apperror.CodePayOSError, "failed to create PayOS payment link", err)
	}

	return domain.DepositCheckoutResponse{
		CheckoutURL:   linkData.CheckoutURL,
		OrderCode:     orderCode,
		Amount:        cp.PriceVND,
		CoinAmount:    cp.CoinAmount,
		TransactionID: txRecord.ID,
	}, nil
}

func (s *Service) HandlePayOSWebhook(ctx context.Context, body []byte) error {
	webhookData, err := s.payos.VerifyWebhookData(body)
	if err != nil {
		return apperror.Wrap(apperror.CodePayOSError, "invalid webhook signature or format", err)
	}

	payment, err := s.payos.GetPaymentLink(ctx, webhookData.Data.OrderCode)
	if err != nil {
		return apperror.Wrap(apperror.CodePayOSError, "failed to get PayOS payment link", err)
	}

	if payment.OrderCode != webhookData.Data.OrderCode {
		return apperror.New(apperror.CodePayOSError, "payos order code mismatch")
	}

	return s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		txRecord, err := s.repo.GetTransactionByPayOSOrderCodeForUpdate(ctx, q, webhookData.Data.OrderCode)
		if err != nil {
			return err
		}

		if txRecord.Status != domain.StatusPending {
			// Already processed — idempotent, do nothing.
			return nil
		}

		_, err = s.processPaymentStatus(ctx, q, txRecord, payment, true)
		return err
	})
}

// SyncDepositStatus checks authoritative PayOS payment status and reconciles the transaction.
func (s *Service) SyncDepositStatus(ctx context.Context, userID uuid.UUID, orderCode int64) (domain.Transaction, error) {
	if orderCode <= 0 {
		return domain.Transaction{}, apperror.New(apperror.CodeValidation, "invalid order code")
	}

	txRecord, err := s.repo.GetTransactionByPayOSOrderCode(ctx, orderCode)
	if err != nil {
		return domain.Transaction{}, err
	}

	if txRecord.To == nil || *txRecord.To != userID {
		return domain.Transaction{}, apperror.New(apperror.CodeForbidden, "transaction does not belong to user")
	}

	if txRecord.Status != domain.StatusPending {
		// Already in terminal state
		return txRecord, nil
	}

	payment, err := s.payos.GetPaymentLink(ctx, orderCode)
	if err != nil {
		return domain.Transaction{}, apperror.Wrap(apperror.CodePayOSError, "failed to get PayOS payment link", err)
	}

	if payment.OrderCode != orderCode {
		return domain.Transaction{}, apperror.New(apperror.CodePayOSError, "payos order code mismatch")
	}

	var updatedTx domain.Transaction
	err = s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		lockedTx, err := s.repo.GetTransactionByPayOSOrderCodeForUpdate(ctx, q, orderCode)
		if err != nil {
			return err
		}

		if lockedTx.Status != domain.StatusPending {
			updatedTx = lockedTx
			return nil
		}

		res, err := s.processPaymentStatus(ctx, q, lockedTx, payment, false)
		if err != nil {
			return err
		}
		updatedTx = res
		return nil
	})
	if err != nil {
		return domain.Transaction{}, err
	}

	return updatedTx, nil
}

func (s *Service) processPaymentStatus(
	ctx context.Context,
	q *repository.Queries,
	txRecord domain.Transaction,
	payment *payos.PaymentLink,
	isWebhook bool,
) (domain.Transaction, error) {
	switch payment.Status {
	case payos.PaymentLinkStatusPaid:
		if payment.Amount <= 0 || payment.AmountPaid < payment.Amount {
			return domain.Transaction{}, apperror.New(apperror.CodePayOSError, "payment amount is invalid or underpaid")
		}
		if expectedVND, ok := parseExpectedVND(txRecord.Description); ok {
			if payment.AmountPaid < expectedVND {
				return domain.Transaction{}, apperror.New(apperror.CodePayOSError, fmt.Sprintf("amount paid %d VND is less than expected %d VND", payment.AmountPaid, expectedVND))
			}
		}
		// Payment confirmed by PayOS.
		updated, err := s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusSuccess)
		if err != nil {
			return domain.Transaction{}, err
		}
		if txRecord.To != nil {
			wallet, err := s.repo.GetWalletForUpdate(ctx, q, *txRecord.To)
			if err != nil {
				if apperror.IsCode(err, apperror.CodeWalletNotFound) {
					wallet, err = s.repo.CreateWallet(ctx, q, *txRecord.To)
					if err != nil {
						return domain.Transaction{}, err
					}
				} else {
					return domain.Transaction{}, err
				}
			}
			if int64(wallet.Balance)+int64(txRecord.Amount) > math.MaxInt32 {
				return domain.Transaction{}, apperror.New(apperror.CodeValidation, "wallet balance overflow")
			}
			newBalance := wallet.Balance + txRecord.Amount
			_, err = s.repo.UpdateWalletBalance(ctx, q, wallet.ID, newBalance)
			if err != nil {
				return domain.Transaction{}, err
			}
		}
		return updated, nil
	case payos.PaymentLinkStatusCancelled:
		// PayOS signals payment was cancelled by the user.
		return s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusCancelled)
	case payos.PaymentLinkStatusExpired, payos.PaymentLinkStatusFailed:
		// Terminal failure states.
		return s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusFailed)
	case payos.PaymentLinkStatusUnderpaid:
		// Insufficient amount transferred by user.
		return s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusFailed)
	case payos.PaymentLinkStatusPending, payos.PaymentLinkStatusProcessing:
		// Transient in-progress states: for webhook, return error to allow retry; for client sync, return current record.
		if isWebhook {
			return domain.Transaction{}, apperror.New(apperror.CodePayOSError, fmt.Sprintf("payment is in progress (status: %s)", payment.Status))
		}
		return txRecord, nil
	default:
		// Unknown status: do not mark failed destructively.
		return domain.Transaction{}, apperror.New(apperror.CodePayOSError, fmt.Sprintf("unhandled payment link status: %s", payment.Status))
	}
}

func (s *Service) ChargePracticeInterview(ctx context.Context, candidateID uuid.UUID, amount int32) error {
	return s.deductBalance(ctx, candidateID, amount, "Practice interview charge")
}

func (s *Service) ChargeRecruitmentCapacity(ctx context.Context, recruiterID uuid.UUID, amount int32) error {
	return s.deductBalance(ctx, recruiterID, amount, "Recruitment interview capacity charge")
}

func (s *Service) RefundRecruitmentCapacity(ctx context.Context, recruiterID uuid.UUID, amount int32) error {
	if amount <= 0 {
		return apperror.New(apperror.CodeValidation, "refund amount must be positive")
	}
	return s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		wallet, err := s.repo.GetWalletForUpdate(ctx, q, recruiterID)
		if err != nil {
			if apperror.IsCode(err, apperror.CodeWalletNotFound) {
				wallet, err = s.repo.CreateWallet(ctx, q, recruiterID)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if int64(wallet.Balance)+int64(amount) > math.MaxInt32 {
			return apperror.New(apperror.CodeValidation, "wallet balance overflow")
		}

		_, err = s.repo.UpdateWalletBalance(ctx, q, wallet.ID, wallet.Balance+amount)
		if err != nil {
			return err
		}

		_, err = s.repo.InsertTransaction(ctx, q, domain.Transaction{
			From:        nil,
			To:          &recruiterID,
			Amount:      amount,
			Currency:    "VND",
			Description: "Recruitment interview capacity refund",
			Status:      domain.StatusSuccess,
		})
		return err
	})
}

func (s *Service) ChargeAvatarCapacity(ctx context.Context, recruiterID uuid.UUID, amount int32) error {
	return s.deductBalance(ctx, recruiterID, amount, "Avatar capacity purchase charge")
}

func (s *Service) deductBalance(ctx context.Context, userID uuid.UUID, amount int32, description string) error {
	if amount <= 0 {
		return apperror.New(apperror.CodeValidation, "charge amount must be positive")
	}
	return s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		wallet, err := s.repo.GetWalletForUpdate(ctx, q, userID)
		if err != nil {
			if apperror.IsCode(err, apperror.CodeWalletNotFound) {
				return apperror.New(apperror.CodeInsufficientBalance, "insufficient balance: wallet has 0 coins")
			}
			return err
		}

		if wallet.Balance < amount {
			return apperror.New(apperror.CodeInsufficientBalance, "insufficient wallet balance")
		}

		_, err = s.repo.UpdateWalletBalance(ctx, q, wallet.ID, wallet.Balance-amount)
		if err != nil {
			return err
		}

		_, err = s.repo.InsertTransaction(ctx, q, domain.Transaction{
			From:        &userID,
			To:          nil,
			Amount:      amount,
			Currency:    "VND",
			Description: description,
			Status:      domain.StatusSuccess,
		})
		return err
	})
}

// generateOrderCode produces a cryptographically random positive int64
// suitable for use as a PayOS orderCode. The range is [100_000_000_000,
// 999_999_999_999] (12 digits), giving ~900 billion distinct values and
// negligible collision probability.
func generateOrderCode() (int64, error) {
	const (
		min    int64 = 100_000_000_000
		range_ int64 = 899_999_999_999
	)
	nBig, err := rand.Int(rand.Reader, big.NewInt(range_))
	if err != nil {
		return 0, err
	}
	return min + nBig.Int64(), nil
}

func parseExpectedVND(desc string) (int, bool) {
	idx := strings.LastIndex(desc, " VND)")
	if idx == -1 {
		return 0, false
	}
	start := strings.LastIndex(desc[:idx], "(")
	if start == -1 {
		return 0, false
	}
	numStr := strings.TrimSpace(desc[start+1 : idx])
	val, err := strconv.Atoi(numStr)
	if err != nil || val <= 0 {
		return 0, false
	}
	return val, true
}
