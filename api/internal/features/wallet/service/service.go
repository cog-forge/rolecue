package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	opPayOSWebhook = "wallet.payos_webhook"
	opDepositSync  = "wallet.deposit_sync"
)

type WalletRepository interface {
	ExecTx(ctx context.Context, fn func(q *repository.Queries) error) error
	GetWalletByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error)
	CreateWallet(ctx context.Context, q *repository.Queries, userID uuid.UUID) (domain.Wallet, error)
	// CreditPaidOrder adds a paid order's coins to the wallet through the shared ledger and writes no row.
	CreditPaidOrder(ctx context.Context, q *repository.Queries, userID uuid.UUID, amount int32) error
	InsertTransaction(ctx context.Context, q *repository.Queries, tx domain.Transaction) (domain.Transaction, error)
	GetTransactionByID(ctx context.Context, id uuid.UUID) (domain.Transaction, error)
	GetTransactionByPayOSOrderCode(ctx context.Context, orderCode int64) (domain.Transaction, error)
	GetTransactionByPayOSOrderCodeForUpdate(ctx context.Context, q *repository.Queries, orderCode int64) (domain.Transaction, error)
	UpdateTransactionStatus(ctx context.Context, q *repository.Queries, id uuid.UUID, status domain.TransactionStatus) (domain.Transaction, error)
	ListTransactionsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]domain.Transaction, int64, error)
	ListCoinPackages(ctx context.Context) ([]domain.CoinPackage, error)
	GetCoinPackageByID(ctx context.Context, id uuid.UUID) (domain.CoinPackage, error)
	// Admin read-only methods (KAN-191, KAN-192, KAN-241)
	ListAdminTransactions(ctx context.Context, filter domain.AdminTransactionFilter) (domain.AdminTransactionPage, error)
	GetAdminTransactionByID(ctx context.Context, id uuid.UUID) (domain.AdminTransactionItem, error)
	GetBillingSummary(ctx context.Context, filter domain.AdminTransactionFilter) (domain.BillingSummary, error)
	// Revenue report: one window's summary, successful top-ups and new payers, and the coin totals now.
	GetRevenuePeriod(ctx context.Context, from, to time.Time) (domain.RevenuePeriodData, error)
	GetCoinTotals(ctx context.Context) (outstanding, allTimeRevenueVND, allTimeCoinsSold int64, err error)
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
			created, err := s.repo.CreateWallet(ctx, nil, userID)
			if err == nil {
				logger.FromContext(ctx).Info("wallet created", logger.Op("wallet.create"), zap.String("user_id", userID.String()))
			}
			return created, err
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

	done := logger.Start(ctx, "wallet.payos_create_link", zap.Int64("order_code", orderCode), zap.String("transaction_id", txRecord.ID.String()))
	linkData, err := s.payos.CreatePaymentLink(ctx, payosReq)
	done(err)
	if err != nil {
		// The error can be ambiguous: PayOS may have created the link before our call timed out. The
		// order stays pending so a later PAID webhook or sync still credits it; the user retries with a
		// new order.
		return domain.DepositCheckoutResponse{}, apperror.Wrap(apperror.CodePayOSError, "failed to create PayOS payment link", err)
	}
	logger.FromContext(ctx).Info("deposit checkout created", logger.Op("wallet.checkout"),
		zap.String("user_id", userID.String()), zap.String("transaction_id", txRecord.ID.String()),
		zap.Int64("order_code", orderCode), zap.Int32("amount", cp.PriceVND), zap.String("currency", "VND"),
		zap.Int32("coin_amount", cp.CoinAmount))

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
		// Only the reason is logged: the body and the signature stay out of the log.
		logger.FromContext(ctx).Warn("payos webhook rejected", logger.Op(opPayOSWebhook), zap.String("reason", "signature_or_format_invalid"))
		return apperror.Wrap(apperror.CodePayOSError, "invalid webhook signature or format", err)
	}
	orderCode := webhookData.Data.OrderCode

	done := logger.Start(ctx, "wallet.payos_get_link", zap.Int64("order_code", orderCode))
	payment, err := s.payos.GetPaymentLink(ctx, orderCode)
	done(err)
	if err != nil {
		return apperror.Wrap(apperror.CodePayOSError, "failed to get PayOS payment link", err)
	}

	if payment.OrderCode != orderCode {
		logger.FromContext(ctx).Warn("payos webhook rejected", logger.Op(opPayOSWebhook),
			zap.String("reason", "order_code_mismatch"), zap.Int64("order_code", orderCode), zap.Int64("link_order_code", payment.OrderCode))
		return apperror.New(apperror.CodePayOSError, "payos order code mismatch")
	}

	var updated domain.Transaction
	settled := false
	err = s.repo.ExecTx(ctx, func(q *repository.Queries) error {
		txRecord, err := s.repo.GetTransactionByPayOSOrderCodeForUpdate(ctx, q, orderCode)
		if err != nil {
			if apperror.IsCode(err, apperror.CodeTransactionNotFound) {
				logger.FromContext(ctx).Warn("payos webhook rejected", logger.Op(opPayOSWebhook),
					zap.String("reason", "unknown_order"), zap.Int64("order_code", orderCode))
			}
			return err
		}

		if txRecord.Status != domain.StatusPending {
			// Already processed — idempotent, do nothing.
			logger.FromContext(ctx).Info("payos webhook replay ignored", logger.Op(opPayOSWebhook),
				zap.String("reason", "already_processed"), zap.Int64("order_code", orderCode),
				zap.String("transaction_id", txRecord.ID.String()), zap.String("status", string(txRecord.Status)))
			return nil
		}

		res, err := s.processPaymentStatus(ctx, q, txRecord, payment, true)
		if err != nil {
			return err
		}
		updated, settled = res, true
		return nil
	})
	if err != nil {
		return err
	}
	if settled {
		logDepositSettled(ctx, opPayOSWebhook, updated, payment)
	}
	return nil
}

// logDepositSettled records a top-up that left pending. Identifiers, amounts and statuses only.
func logDepositSettled(ctx context.Context, op string, tx domain.Transaction, payment *payos.PaymentLink) {
	fields := []zap.Field{logger.Op(op), logger.String("transaction_id", tx.ID.String()),
		zap.Int64("order_code", payment.OrderCode), zap.String("from_status", string(domain.StatusPending)),
		zap.String("to_status", string(tx.Status)), zap.Int("amount", payment.AmountPaid), zap.String("currency", "VND"),
		zap.Int32("coin_amount", tx.Amount)}
	if tx.To != nil {
		fields = append(fields, zap.String("user_id", tx.To.String()))
	}
	logger.FromContext(ctx).Info("deposit status changed", fields...)
}

// logPaymentRejected records a paid link whose amounts do not match the order.
func logPaymentRejected(ctx context.Context, op, reason string, payment *payos.PaymentLink, expectedVND int) {
	logger.FromContext(ctx).Warn("payos payment rejected", logger.Op(op), zap.String("reason", reason),
		zap.Int64("order_code", payment.OrderCode), zap.Int("amount", payment.Amount),
		zap.Int("amount_paid", payment.AmountPaid), zap.Int("expected_amount", expectedVND), zap.String("currency", "VND"))
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
	settled := false
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
		settled = res.Status != domain.StatusPending
		return nil
	})
	if err != nil {
		return domain.Transaction{}, err
	}
	if settled {
		logDepositSettled(ctx, opDepositSync, updatedTx, payment)
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
	op := opDepositSync
	if isWebhook {
		op = opPayOSWebhook
	}
	switch payment.Status {
	case payos.PaymentLinkStatusPaid:
		if payment.Amount <= 0 || payment.AmountPaid < payment.Amount {
			logPaymentRejected(ctx, op, "amount_invalid_or_underpaid", payment, 0)
			return domain.Transaction{}, apperror.New(apperror.CodePayOSError, "payment amount is invalid or underpaid")
		}
		if expectedVND, ok := parseExpectedVND(txRecord.Description); ok {
			if payment.AmountPaid < expectedVND {
				logPaymentRejected(ctx, op, "amount_below_expected", payment, expectedVND)
				return domain.Transaction{}, apperror.New(apperror.CodePayOSError, fmt.Sprintf("amount paid %d VND is less than expected %d VND", payment.AmountPaid, expectedVND))
			}
		}
		// Payment confirmed by PayOS.
		updated, err := s.repo.UpdateTransactionStatus(ctx, q, txRecord.ID, domain.StatusSuccess)
		if err != nil {
			return domain.Transaction{}, err
		}
		if txRecord.To != nil {
			// The VND order row is the record of this top-up, so the ledger only moves the balance.
			if err := s.repo.CreditPaidOrder(ctx, q, *txRecord.To, txRecord.Amount); err != nil {
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
			logger.FromContext(ctx).Info("payos webhook deferred", logger.Op(op), zap.String("reason", "payment_in_progress"),
				zap.Int64("order_code", payment.OrderCode), zap.Any("status", payment.Status))
			return domain.Transaction{}, apperror.New(apperror.CodePayOSError, fmt.Sprintf("payment is in progress (status: %s)", payment.Status))
		}
		return txRecord, nil
	default:
		// Unknown status: do not mark failed destructively.
		logger.FromContext(ctx).Warn("payos payment status unhandled", logger.Op(op), zap.String("reason", "unknown_status"),
			zap.Int64("order_code", payment.OrderCode), zap.Any("status", payment.Status))
		return domain.Transaction{}, apperror.New(apperror.CodePayOSError, fmt.Sprintf("unhandled payment link status: %s", payment.Status))
	}
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

// ValidateAdminFilter validates and normalizes read-only Admin billing/revenue filter parameters.
func ValidateAdminFilter(filter *domain.AdminTransactionFilter, maxPageSize int) error {
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.Page < 1 || filter.Page > 10_000 {
		return apperror.New(apperror.CodeValidation, "page must be between 1 and 10000")
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.PageSize < 1 || filter.PageSize > maxPageSize {
		return apperror.New(apperror.CodeValidation, fmt.Sprintf("page_size must be between 1 and %d", maxPageSize))
	}

	status := strings.ToLower(strings.TrimSpace(filter.Status))
	switch status {
	case "", string(domain.StatusPending), string(domain.StatusSuccess), string(domain.StatusFailed), string(domain.StatusCancelled):
		filter.Status = status
	default:
		return apperror.New(apperror.CodeValidation, "invalid status filter: must be pending, success, failed, or cancelled")
	}

	txType := strings.ToLower(strings.TrimSpace(filter.Type))
	switch txType {
	case "", "topup", "spend", "refund":
		filter.Type = txType
	default:
		return apperror.New(apperror.CodeValidation, "invalid type filter: must be topup, spend, or refund")
	}

	sortBy := strings.ToLower(strings.TrimSpace(filter.SortBy))
	switch sortBy {
	case "", "created_at":
		filter.SortBy = "created_at"
	case "id", "amount", "coins", "status":
		filter.SortBy = sortBy
	default:
		return apperror.New(apperror.CodeValidation, "invalid sort_by: must be created_at, id, amount, coins, or status")
	}

	sortOrder := strings.ToLower(strings.TrimSpace(filter.SortOrder))
	switch sortOrder {
	case "", "desc":
		filter.SortOrder = "desc"
	case "asc":
		filter.SortOrder = "asc"
	default:
		return apperror.New(apperror.CodeValidation, "invalid sort_order: must be asc or desc")
	}

	if filter.AmountMin != nil && *filter.AmountMin < 0 {
		return apperror.New(apperror.CodeValidation, "amount_min cannot be negative")
	}
	if filter.AmountMax != nil && *filter.AmountMax < 0 {
		return apperror.New(apperror.CodeValidation, "amount_max cannot be negative")
	}
	if filter.AmountMin != nil && filter.AmountMax != nil && *filter.AmountMin > *filter.AmountMax {
		return apperror.New(apperror.CodeValidation, "amount_min cannot be greater than amount_max")
	}
	if filter.DateFrom != nil && filter.DateTo != nil && !filter.DateFrom.Before(*filter.DateTo) {
		return apperror.New(apperror.CodeValidation, "date_from must be before date_to")
	}

	filter.Search = strings.TrimSpace(filter.Search)
	filter.PackageName = strings.TrimSpace(filter.PackageName)
	return nil
}

func (s *Service) GetBillingSummary(ctx context.Context, filter domain.AdminTransactionFilter) (domain.BillingSummary, error) {
	if err := ValidateAdminFilter(&filter, 100); err != nil {
		return domain.BillingSummary{}, err
	}
	return s.repo.GetBillingSummary(ctx, filter)
}

func (s *Service) ListBillingTransactions(ctx context.Context, filter domain.AdminTransactionFilter) (domain.AdminTransactionPage, error) {
	if err := ValidateAdminFilter(&filter, 5000); err != nil {
		return domain.AdminTransactionPage{}, err
	}
	return s.repo.ListAdminTransactions(ctx, filter)
}

func (s *Service) GetBillingTransaction(ctx context.Context, id uuid.UUID) (domain.AdminTransactionItem, error) {
	return s.repo.GetAdminTransactionByID(ctx, id)
}
