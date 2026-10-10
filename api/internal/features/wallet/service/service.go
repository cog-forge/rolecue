package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type WalletRepository interface {
	ExecTx(ctx context.Context, fn func(q *repository.Queries) error) error
	GetWalletByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error)
	CreateWallet(ctx context.Context, userID uuid.UUID) (domain.Wallet, error)
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
			return s.repo.CreateWallet(ctx, userID)
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

	orderCode, err := generateOrderCode()
	if err != nil {
		return domain.DepositCheckoutResponse{}, apperror.Wrap(apperror.CodeInternal, "failed to generate order code", err)
	}

	description := cp.Name
	if len(description) > 25 {
		description = description[:25]
	}

	// Create pending transaction record first
	var txRecord domain.Transaction
	err = s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		created, err := s.repo.InsertTransaction(ctx, q, domain.Transaction{
			From:           nil,
			To:             &userID,
			Amount:         cp.CoinAmount,
			Currency:       "VND",
			Description:    description,
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
		Description: description,
		ReturnURL:   returnURL,
		CancelURL:   cancelURL,
	}

	linkData, err := s.payos.CreatePaymentLink(ctx, payosReq)
	if err != nil {
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

	return s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		txRecord, err := s.repo.GetTransactionByPayOSOrderCodeForUpdate(ctx, q, webhookData.Data.OrderCode)
		if err != nil {
			return err
		}

		if txRecord.Status != domain.StatusPending {
			// Already processed — idempotent, do nothing.
			return nil
		}

		switch {
		case webhookData.Success || webhookData.Code == "00" || webhookData.Data.Code == "00":
			// Payment confirmed by PayOS.
			_, err = s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusSuccess)
			if err != nil {
				return err
			}
			if txRecord.To != nil {
				wallet, err := s.repo.GetWalletForUpdate(ctx, q, *txRecord.To)
				if err != nil {
					if apperror.IsCode(err, apperror.CodeWalletNotFound) {
						wallet, err = s.repo.CreateWallet(ctx, *txRecord.To)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				}
				newBalance := wallet.Balance + txRecord.Amount
				_, err = s.repo.UpdateWalletBalance(ctx, q, wallet.ID, newBalance)
				if err != nil {
					return err
				}
			}
		case webhookData.Code == "01" || webhookData.Data.Code == "01":
			// PayOS signals payment was cancelled by the user.
			_, err = s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusCancelled)
			if err != nil {
				return err
			}
		default:
			// Any other non-success code is treated as a failed payment.
			_, err = s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusFailed)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ChargePracticeInterview(ctx context.Context, candidateID uuid.UUID, amount int32) error {
	return s.deductBalance(ctx, candidateID, amount, "Practice interview charge")
}

func (s *Service) ChargeRecruitmentCapacity(ctx context.Context, recruiterID uuid.UUID, amount int32) error {
	return s.deductBalance(ctx, recruiterID, amount, "Recruitment interview capacity charge")
}

func (s *Service) RefundRecruitmentCapacity(ctx context.Context, recruiterID uuid.UUID, amount int32) error {
	return s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		wallet, err := s.repo.GetWalletForUpdate(ctx, q, recruiterID)
		if err != nil {
			if apperror.IsCode(err, apperror.CodeWalletNotFound) {
				wallet, err = s.repo.CreateWallet(ctx, recruiterID)
				if err != nil {
					return err
				}
			} else {
				return err
			}
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
		min   int64 = 100_000_000_000
		range_ int64 = 899_999_999_999
	)
	nBig, err := rand.Int(rand.Reader, big.NewInt(range_))
	if err != nil {
		// Fallback: derive from nanosecond timestamp, keeping it in range.
		return min + (time.Now().UnixNano()%range_ + range_)%range_, nil
	}
	return min + nBig.Int64(), nil
}
