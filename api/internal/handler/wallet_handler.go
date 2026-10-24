package handler

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"time"

	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WalletService interface {
	GetWallet(ctx context.Context, userID uuid.UUID) (walletdomain.Wallet, error)
	ListTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int32) ([]walletdomain.Transaction, int64, error)
	ListPackages(ctx context.Context) ([]walletdomain.CoinPackage, error)
	CreateDepositCheckout(ctx context.Context, userID uuid.UUID, packageID uuid.UUID, returnURL, cancelURL string) (walletdomain.DepositCheckoutResponse, error)
	SyncDepositStatus(ctx context.Context, userID uuid.UUID, orderCode int64) (walletdomain.Transaction, error)
	HandlePayOSWebhook(ctx context.Context, body []byte) error
	AdminListTransactions(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.AdminTransactionPage, error)
	GetRevenueReport(ctx context.Context, dateFrom, dateTo *time.Time) (walletdomain.RevenueReport, error)
	GetBillingSummary(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.BillingSummary, error)
	GetBillingDailySeries(ctx context.Context, filter walletdomain.AdminTransactionFilter) ([]walletdomain.BillingDailyItem, error)
	ListBillingTransactions(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.AdminTransactionPage, error)
	GetBillingTransaction(ctx context.Context, id uuid.UUID) (*walletdomain.AdminTransactionItem, error)
	SyncPayOSTransaction(ctx context.Context, id uuid.UUID, adminID uuid.UUID) (*walletdomain.AdminTransactionItem, error)
	CancelTransaction(ctx context.Context, id uuid.UUID, adminID uuid.UUID, reason string) (*walletdomain.AdminTransactionItem, error)
	IssueRefund(ctx context.Context, req walletdomain.IssueRefundRequest, adminID uuid.UUID) (*walletdomain.AdminTransactionItem, error)
}

type WalletHandler struct {
	service WalletService
}

func NewWalletHandler(service WalletService) *WalletHandler {
	return &WalletHandler{service: service}
}

type DepositCheckoutRequest struct {
	PackageID uuid.UUID `json:"package_id" binding:"required"`
	ReturnURL string    `json:"return_url"`
	CancelURL string    `json:"cancel_url"`
}

type SyncDepositRequest struct {
	OrderCode int64 `json:"order_code" binding:"required"`
}

// GetWallet handles GET /api/v1/wallet
func (h *WalletHandler) GetWallet(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if user.Role == "admin" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "admin has no wallet"))
		return
	}

	wallet, err := h.service.GetWallet(c.Request.Context(), user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, wallet)
}

// ListTransactions handles GET /api/v1/wallet/transactions
func (h *WalletHandler) ListTransactions(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if user.Role == "admin" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "admin has no wallet"))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	txs, total, err := h.service.ListTransactions(c.Request.Context(), user.ID, int32(page), int32(pageSize))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"items":     txs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListPackages handles GET /api/v1/wallet/packages
func (h *WalletHandler) ListPackages(c *gin.Context) {
	packages, err := h.service.ListPackages(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, packages)
}

// CreateDepositCheckout handles POST /api/v1/wallet/deposit/checkout
func (h *WalletHandler) CreateDepositCheckout(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if user.Role == "admin" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "admin has no wallet"))
		return
	}

	var req DepositCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid deposit checkout request"))
		return
	}

	checkoutResp, err := h.service.CreateDepositCheckout(
		c.Request.Context(),
		user.ID,
		req.PackageID,
		req.ReturnURL,
		req.CancelURL,
	)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, checkoutResp)
}

// SyncDepositStatus handles POST /api/v1/wallet/deposit/sync
func (h *WalletHandler) SyncDepositStatus(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if user.Role == "admin" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "admin has no wallet"))
		return
	}

	var req SyncDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid sync deposit request"))
		return
	}

	tx, err := h.service.SyncDepositStatus(c.Request.Context(), user.ID, req.OrderCode)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, tx)
}

// HandlePayOSWebhook handles POST /api/v1/wallet/webhook/payos
func (h *WalletHandler) HandlePayOSWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "unable to read request body"))
		return
	}

	if err := h.service.HandlePayOSWebhook(c.Request.Context(), body); err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "00", "message": "success"})
}

// AdminListTransactions handles GET /admin/transactions
func (h *WalletHandler) AdminListTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	search := c.Query("search")
	status := c.Query("status")
	txType := c.Query("type")

	var dateFrom, dateTo *time.Time
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err == nil {
			dateFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err == nil {
			dateTo = &t
		}
	}

	filter := walletdomain.AdminTransactionFilter{
		Search:   search,
		Status:   status,
		TxType:   txType,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	result, err := h.service.AdminListTransactions(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"items":     result.Items,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// AdminGetRevenueReport handles GET /admin/revenue
func (h *WalletHandler) AdminGetRevenueReport(c *gin.Context) {
	var dateFrom, dateTo *time.Time
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err == nil {
			dateFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err == nil {
			dateTo = &t
		}
	}

	result, err := h.service.GetRevenueReport(c.Request.Context(), dateFrom, dateTo)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, result)
}

func parseBillingFilter(c *gin.Context) walletdomain.AdminTransactionFilter {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	search := c.Query("search")
	status := c.Query("status")
	txType := c.Query("type")
	pkg := c.Query("package")
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	includeTest := c.Query("include_test") == "true" || c.Query("is_test") == "true"

	var amountMin, amountMax *int64
	if amin := c.Query("amount_min"); amin != "" {
		if v, err := strconv.ParseInt(amin, 10, 64); err == nil {
			amountMin = &v
		}
	}
	if amax := c.Query("amount_max"); amax != "" {
		if v, err := strconv.ParseInt(amax, 10, 64); err == nil {
			amountMax = &v
		}
	}

	var dateFrom, dateTo *time.Time
	if df := c.Query("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			dateFrom = &t
		} else if t, err := time.Parse("2006-01-02", df); err == nil {
			dateFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			dateTo = &t
		} else if t, err := time.Parse("2006-01-02", dt); err == nil {
			tEnd := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			dateTo = &tEnd
		}
	}

	return walletdomain.AdminTransactionFilter{
		Search:      search,
		Status:      status,
		TxType:      txType,
		Package:     pkg,
		AmountMin:   amountMin,
		AmountMax:   amountMax,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		IncludeTest: includeTest,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		Page:        int32(page),
		PageSize:    int32(pageSize),
	}
}

// AdminGetBillingSummary handles GET /admin/billing/summary
func (h *WalletHandler) AdminGetBillingSummary(c *gin.Context) {
	filter := parseBillingFilter(c)
	summary, err := h.service.GetBillingSummary(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, summary)
}

// AdminGetBillingDailySeries handles GET /admin/billing/daily
func (h *WalletHandler) AdminGetBillingDailySeries(c *gin.Context) {
	filter := parseBillingFilter(c)
	daily, err := h.service.GetBillingDailySeries(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, daily)
}

// AdminListBillingTransactions handles GET /admin/billing/transactions
func (h *WalletHandler) AdminListBillingTransactions(c *gin.Context) {
	filter := parseBillingFilter(c)
	result, err := h.service.ListBillingTransactions(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{
		"items":     result.Items,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// AdminGetBillingTransaction handles GET /admin/billing/transactions/:id
func (h *WalletHandler) AdminGetBillingTransaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid transaction ID format"))
		return
	}

	tx, err := h.service.GetBillingTransaction(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tx)
}

// AdminSyncTransaction handles POST /admin/billing/transactions/:id/sync
func (h *WalletHandler) AdminSyncTransaction(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid transaction ID format"))
		return
	}

	tx, err := h.service.SyncPayOSTransaction(c.Request.Context(), id, user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tx)
}

type cancelTransactionBody struct {
	Reason string `json:"reason"`
}

// AdminCancelTransaction handles POST /admin/billing/transactions/:id/cancel
func (h *WalletHandler) AdminCancelTransaction(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid transaction ID format"))
		return
	}

	var body cancelTransactionBody
	_ = c.ShouldBindJSON(&body)

	tx, err := h.service.CancelTransaction(c.Request.Context(), id, user.ID, body.Reason)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tx)
}

type issueRefundBody struct {
	TransactionID *uuid.UUID `json:"transaction_id"`
	UserID        uuid.UUID  `json:"user_id" binding:"required"`
	Amount        int32      `json:"amount" binding:"required"`
	Currency      string     `json:"currency"`
	Reason        string     `json:"reason" binding:"required"`
	ClawbackCoins bool       `json:"clawback_coins"`
}

// AdminIssueRefund handles POST /admin/billing/transactions/:id/refund or /admin/billing/refund
func (h *WalletHandler) AdminIssueRefund(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}

	var body issueRefundBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid refund request body"))
		return
	}

	if paramID := c.Param("id"); paramID != "" {
		if id, err := uuid.Parse(paramID); err == nil {
			body.TransactionID = &id
		}
	}

	req := walletdomain.IssueRefundRequest{
		TransactionID: body.TransactionID,
		UserID:        body.UserID,
		Amount:        body.Amount,
		Currency:      body.Currency,
		Reason:        body.Reason,
		ClawbackCoins: body.ClawbackCoins,
	}

	tx, err := h.service.IssueRefund(c.Request.Context(), req, user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tx)
}

// AdminExportTransactionsCSV handles GET /admin/billing/export
func (h *WalletHandler) AdminExportTransactionsCSV(c *gin.Context) {
	filter := parseBillingFilter(c)
	filter.Page = 1
	filter.PageSize = 5000 // export up to 5000 rows

	result, err := h.service.ListBillingTransactions(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=billing-transactions.csv")
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// UTF-8 BOM so Excel opens it with proper encoding for Vietnamese diacritics
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write CSV Header
	_ = writer.Write([]string{
		"Transaction ID",
		"Order Code",
		"User Name",
		"User Email",
		"Type",
		"Package or Feature",
		"Paid (VND)",
		"Coins",
		"Status",
		"Failure Reason",
		"Date (UTC+7)",
	})

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	if loc == nil {
		loc = time.FixedZone("UTC+7", 7*3600)
	}

	for _, it := range result.Items {
		orderCode := ""
		if it.PayOSOrderCode != nil {
			orderCode = strconv.FormatInt(*it.PayOSOrderCode, 10)
		}
		userName := ""
		userEmail := ""
		if it.User != nil {
			userName = it.User.Name
			userEmail = it.User.Email
		}
		paidVND := ""
		if it.PaidVND != nil {
			paidVND = strconv.FormatInt(*it.PaidVND, 10)
		}
		coins := ""
		if it.Coins != nil {
			coins = strconv.FormatInt(int64(*it.Coins), 10)
		}
		failReason := ""
		if it.FailureReason != nil {
			failReason = *it.FailureReason
		}

		createdAt := it.CreatedAt.In(loc).Format("02/01/2006 15:04:05")

		_ = writer.Write([]string{
			it.ID.String(),
			orderCode,
			userName,
			userEmail,
			it.Type,
			it.PackageOrFeature,
			paidVND,
			coins,
			string(it.Status),
			failReason,
			createdAt,
		})
	}
}
