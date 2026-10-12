package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"

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
	HandlePayOSWebhook(ctx context.Context, body []byte) error
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
