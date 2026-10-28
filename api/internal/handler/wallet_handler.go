package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	walletservice "github.com/cog-forge/rolecue/api/internal/features/wallet/service"
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
	GetBillingSummary(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.BillingSummary, error)
	ListBillingTransactions(ctx context.Context, filter walletdomain.AdminTransactionFilter) (walletdomain.AdminTransactionPage, error)
	GetBillingTransaction(ctx context.Context, id uuid.UUID) (walletdomain.AdminTransactionItem, error)
	GetRevenueReport(ctx context.Context, req walletdomain.RevenueReportRequest) (walletdomain.RevenueReport, error)
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

func requireAdminUser(c *gin.Context) bool {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return false
	}
	if user.Role != "admin" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "admin access required"))
		return false
	}
	return true
}

func singleQueryParam(c *gin.Context, name string) (string, bool, bool) {
	vals, exists := c.GetQueryArray(name)
	if !exists {
		return "", false, true
	}
	if len(vals) != 1 {
		return "", true, false
	}
	return vals[0], true, true
}

// dateQueryParam reads an optional RFC 3339 instant. It returns a message when the value is invalid or
// given twice; an empty value counts as absent.
func dateQueryParam(c *gin.Context, name string) (*time.Time, string) {
	raw, present, once := singleQueryParam(c, name)
	if !present {
		return nil, ""
	}
	if !once {
		return nil, "duplicate " + name + " parameter"
	}
	if strings.TrimSpace(raw) == "" {
		return nil, ""
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return nil, "invalid " + name + ": use RFC 3339, e.g. 2026-10-01T00:00:00+07:00"
	}
	return &t, ""
}

func parseBillingFilter(c *gin.Context, maxPageSize int) (walletdomain.AdminTransactionFilter, bool) {
	fail := func(msg string) (walletdomain.AdminTransactionFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return walletdomain.AdminTransactionFilter{}, false
	}

	filter := walletdomain.AdminTransactionFilter{
		Page:      1,
		PageSize:  20,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	if rawPage, present, once := singleQueryParam(c, "page"); present {
		if !once {
			return fail("duplicate page parameter")
		}
		v, err := strconv.Atoi(strings.TrimSpace(rawPage))
		if err != nil || v < 1 {
			return fail("invalid page parameter: must be a positive integer")
		}
		filter.Page = v
	}

	if rawPageSize, present, once := singleQueryParam(c, "page_size"); present {
		if !once {
			return fail("duplicate page_size parameter")
		}
		v, err := strconv.Atoi(strings.TrimSpace(rawPageSize))
		if err != nil || v < 1 || v > maxPageSize {
			return fail(fmt.Sprintf("invalid page_size parameter: must be between 1 and %d", maxPageSize))
		}
		filter.PageSize = v
	}

	if rawMin, present, once := singleQueryParam(c, "amount_min"); present && strings.TrimSpace(rawMin) != "" {
		if !once {
			return fail("duplicate amount_min parameter")
		}
		v, err := strconv.ParseInt(strings.TrimSpace(rawMin), 10, 64)
		if err != nil || v < 0 {
			return fail("invalid amount_min parameter: must be a non-negative integer")
		}
		filter.AmountMin = &v
	}

	if rawMax, present, once := singleQueryParam(c, "amount_max"); present && strings.TrimSpace(rawMax) != "" {
		if !once {
			return fail("duplicate amount_max parameter")
		}
		v, err := strconv.ParseInt(strings.TrimSpace(rawMax), 10, 64)
		if err != nil || v < 0 {
			return fail("invalid amount_max parameter: must be a non-negative integer")
		}
		filter.AmountMax = &v
	}

	for _, param := range []struct {
		name   string
		target **time.Time
	}{{"date_from", &filter.DateFrom}, {"date_to", &filter.DateTo}} {
		t, msg := dateQueryParam(c, param.name)
		if msg != "" {
			return fail(msg)
		}
		*param.target = t
	}

	for _, param := range []struct {
		name   string
		target *string
	}{
		{"search", &filter.Search},
		{"status", &filter.Status},
		{"type", &filter.Type},
		{"package", &filter.PackageName},
		{"sort_by", &filter.SortBy},
		{"sort_order", &filter.SortOrder},
	} {
		raw, present, once := singleQueryParam(c, param.name)
		if !present {
			continue
		}
		if !once {
			return fail("duplicate " + param.name + " parameter")
		}
		*param.target = raw
	}

	if err := walletservice.ValidateAdminFilter(&filter, maxPageSize); err != nil {
		response.Error(c, err)
		return walletdomain.AdminTransactionFilter{}, false
	}

	return filter, true
}

// AdminGetBillingSummary godoc
// @Summary Get shared admin billing & revenue summary
// @Description Admin only. Read-only. Computes revenue strictly from status='success' transactions with payos_order_code (KAN-192). Accepts the same filters as the transaction list so both pages always agree. The report time window is [date_from, date_to) on created_at; without it the figures cover all time.
// @Tags admin-billing
// @Produce json
// @Security SessionCookie
// @Param search query string false "Search by transaction ID, user name/email/ID, order code, or description"
// @Param status query string false "Filter by status (pending, success, failed, cancelled)"
// @Param type query string false "Filter by transaction type (topup, spend, refund)"
// @Param package query string false "Filter by package or feature name"
// @Param amount_min query int false "Minimum amount filter"
// @Param amount_max query int false "Maximum amount filter"
// @Param date_from query string false "Created at or after (RFC 3339 with offset, e.g. 2026-10-01T00:00:00+07:00)" format(date-time)
// @Param date_to query string false "Created before (RFC 3339 with offset, exclusive)" format(date-time)
// @Success 200 {object} response.Envelope{data=walletdomain.BillingSummary}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/billing/summary [get]
func (h *WalletHandler) AdminGetBillingSummary(c *gin.Context) {
	if !requireAdminUser(c) {
		return
	}
	filter, ok := parseBillingFilter(c, 100)
	if !ok {
		return
	}

	summary, err := h.service.GetBillingSummary(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, summary)
}

// AdminListBillingTransactions godoc
// @Summary List admin payment and coin ledger transactions
// @Description Admin only. Read-only paginated list of transactions enriched with user and package details, newest first by default. The date window is [date_from, date_to) on created_at.
// @Tags admin-billing
// @Produce json
// @Security SessionCookie
// @Param page query int false "Page number" default(1) minimum(1) maximum(10000)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
// @Param search query string false "Search by transaction ID, user name/email/ID, order code, or description"
// @Param status query string false "Filter by status (pending, success, failed, cancelled)"
// @Param type query string false "Filter by transaction type (topup, spend, refund)"
// @Param package query string false "Filter by package or feature name"
// @Param amount_min query int false "Minimum amount filter"
// @Param amount_max query int false "Maximum amount filter"
// @Param date_from query string false "Created at or after (RFC 3339 with offset, e.g. 2026-10-01T00:00:00+07:00)" format(date-time)
// @Param date_to query string false "Created before (RFC 3339 with offset, exclusive)" format(date-time)
// @Param sort_by query string false "Sort field (created_at, id, amount, coins, status)" default(created_at)
// @Param sort_order query string false "Sort order (asc, desc)" default(desc)
// @Success 200 {object} response.Envelope{data=walletdomain.AdminTransactionPage}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/billing/transactions [get]
func (h *WalletHandler) AdminListBillingTransactions(c *gin.Context) {
	if !requireAdminUser(c) {
		return
	}
	filter, ok := parseBillingFilter(c, 100)
	if !ok {
		return
	}

	result, err := h.service.ListBillingTransactions(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, result)
}

// AdminGetBillingTransaction godoc
// @Summary Get a single transaction's detail for Admin inspection
// @Description Admin only. Read-only transaction detail by UUID.
// @Tags admin-billing
// @Produce json
// @Security SessionCookie
// @Param id path string true "Transaction UUID"
// @Success 200 {object} response.Envelope{data=walletdomain.AdminTransactionItem}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/billing/transactions/{id} [get]
func (h *WalletHandler) AdminGetBillingTransaction(c *gin.Context) {
	if !requireAdminUser(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid transaction id"))
		return
	}

	item, err := h.service.GetBillingTransaction(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, item)
}

// sanitizeCSVCell prevents CSV formula injection (CWE-1236) in spreadsheet applications
// by prefixing cells that begin with '=', '+', '-', '@', '\t', or '\r' with a single quote.
func sanitizeCSVCell(val string) string {
	if val == "" {
		return ""
	}
	switch val[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + val
	default:
		return val
	}
}

// vietnamTime is the admin billing business timezone. Vietnam has no DST, so a fixed offset is exact
// and the export does not depend on the server's tzdata.
var vietnamTime = time.FixedZone("UTC+07:00", 7*60*60)

// AdminExportBillingCSV godoc
// @Summary Export filtered admin transactions as CSV
// @Description Admin only. Read-only CSV export respecting active filters, including the [date_from, date_to) window on created_at (up to 5,000 rows), with CSV formula-injection protection. Created At is RFC 3339 in Vietnam time (+07:00). Sets X-Total-Count and X-Export-Truncated headers.
// @Tags admin-billing
// @Produce text/csv
// @Security SessionCookie
// @Param search query string false "Search by transaction ID, user name/email/ID, order code, or description"
// @Param status query string false "Filter by status (pending, success, failed, cancelled)"
// @Param type query string false "Filter by transaction type (topup, spend, refund)"
// @Param package query string false "Filter by package or feature name"
// @Param amount_min query int false "Minimum amount filter"
// @Param amount_max query int false "Maximum amount filter"
// @Param date_from query string false "Created at or after (RFC 3339 with offset, e.g. 2026-10-01T00:00:00+07:00)" format(date-time)
// @Param date_to query string false "Created before (RFC 3339 with offset, exclusive)" format(date-time)
// @Param sort_by query string false "Sort field (created_at, id, amount, coins, status)" default(created_at)
// @Param sort_order query string false "Sort order (asc, desc)" default(desc)
// @Success 200 {file} file "CSV file"
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/billing/export [get]
func (h *WalletHandler) AdminExportBillingCSV(c *gin.Context) {
	if !requireAdminUser(c) {
		return
	}
	filter, ok := parseBillingFilter(c, 5000)
	if !ok {
		return
	}
	filter.Page = 1
	if _, hasCustomPageSize := c.GetQuery("page_size"); !hasCustomPageSize {
		filter.PageSize = 5000
	}

	result, err := h.service.ListBillingTransactions(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{
		"Transaction ID",
		"Created At (UTC+07:00)",
		"Order Code",
		"User ID",
		"User Name",
		"User Email",
		"Type",
		"Package / Feature",
		"Paid (VND)",
		"Coins",
		"Status",
		"From",
		"To",
		"Description",
	}); err != nil {
		response.Error(c, apperror.Wrap(apperror.CodeInternal, "failed to write csv header", err))
		return
	}

	for _, item := range result.Items {
		orderCode := ""
		if item.PayOSOrderCode != nil {
			orderCode = strconv.FormatInt(*item.PayOSOrderCode, 10)
		}
		if err := writer.Write([]string{
			sanitizeCSVCell(item.ID.String()),
			item.CreatedAt.In(vietnamTime).Format(time.RFC3339),
			sanitizeCSVCell(orderCode),
			sanitizeCSVCell(item.UserID),
			sanitizeCSVCell(item.UserName),
			sanitizeCSVCell(item.UserEmail),
			sanitizeCSVCell(item.Type),
			sanitizeCSVCell(item.PackageOrFeature),
			strconv.FormatInt(item.PaidVND, 10),
			strconv.FormatInt(item.CoinAmount, 10),
			sanitizeCSVCell(string(item.Status)),
			sanitizeCSVCell(item.From),
			sanitizeCSVCell(item.To),
			sanitizeCSVCell(item.Description),
		}); err != nil {
			response.Error(c, apperror.Wrap(apperror.CodeInternal, "failed to write csv row", err))
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		response.Error(c, apperror.Wrap(apperror.CodeInternal, "failed to flush csv export", err))
		return
	}

	truncated := result.Total > int64(len(result.Items))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=rolecue-transactions.csv")
	c.Header("X-Total-Count", strconv.FormatInt(result.Total, 10))
	c.Header("X-Export-Truncated", strconv.FormatBool(truncated))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(buf.Bytes())
}

// AdminGetRevenueReport godoc
// @Summary Get the admin revenue report with a period comparison
// @Description Admin only. Read-only. Revenue is the VND of successful PayOS top-ups in [date_from, date_to); coin movements are not revenue. Compares the window with the same dates one year earlier (default) or with the period of the same length just before it. The series is by day for windows up to 62 days and by month otherwise, in Vietnam time (UTC+07:00). Outstanding coins are what users hold now, valued at the all-time average price per coin.
// @Tags admin-billing
// @Produce json
// @Security SessionCookie
// @Param date_from query string true "Window start, inclusive (RFC 3339 with offset, e.g. 2026-10-01T00:00:00+07:00)" format(date-time)
// @Param date_to query string true "Window end, exclusive (RFC 3339 with offset)" format(date-time)
// @Param compare query string false "previous_year (default) or previous_period"
// @Success 200 {object} response.Envelope{data=walletdomain.RevenueReport}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/billing/revenue-report [get]
func (h *WalletHandler) AdminGetRevenueReport(c *gin.Context) {
	if !requireAdminUser(c) {
		return
	}
	var req walletdomain.RevenueReportRequest
	for _, param := range []struct {
		name   string
		target *time.Time
	}{{"date_from", &req.From}, {"date_to", &req.To}} {
		t, msg := dateQueryParam(c, param.name)
		if msg != "" {
			response.Error(c, apperror.New(apperror.CodeValidation, msg))
			return
		}
		if t != nil {
			*param.target = *t
		}
	}
	compare, present, once := singleQueryParam(c, "compare")
	if present && !once {
		response.Error(c, apperror.New(apperror.CodeValidation, "duplicate compare parameter"))
		return
	}
	req.Compare = strings.TrimSpace(compare)

	report, err := h.service.GetRevenueReport(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, report)
}
