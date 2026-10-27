package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InventoryService interface {
	Summary(context.Context, authdomain.User) (domain.Capacity, error)
	List(context.Context, authdomain.User, int32, int32) ([]domain.Avatar, int64, error)
	Get(context.Context, authdomain.User, uuid.UUID) (domain.AvatarDetail, error)
	PurchaseSlot(ctx context.Context, user authdomain.User, amount int32) (domain.SlotPurchase, error)
	Delete(context.Context, authdomain.User, uuid.UUID) error
}

type InventoryHandler struct{ service InventoryService }

func NewInventoryHandler(service InventoryService) *InventoryHandler {
	return &InventoryHandler{service: service}
}

// inventoryInput rejects bodies and query parameters not in the endpoint contract.
func inventoryInput(c *gin.Context, paginated bool) (url.Values, bool) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err == nil {
		for key := range values {
			if !paginated || (key != "limit" && key != "offset") {
				err = apperror.New(apperror.CodeValidation, "unsupported query parameter")
				break
			}
		}
	}
	if err == nil && c.Request.Body != nil {
		var body []byte
		body, err = io.ReadAll(io.LimitReader(c.Request.Body, 1))
		if len(body) != 0 {
			err = apperror.New(apperror.CodeValidation, "request body is not supported")
		}
	}
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid inventory request"))
		return nil, false
	}
	return values, true
}

func inventoryUser(c *gin.Context) (authdomain.User, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
	}
	return user, ok
}

func inventoryAvatarID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid avatar ID"))
		return uuid.Nil, false
	}
	return id, true
}

// Summary godoc
// @Summary Get your personal avatar capacity
// @Description Candidate/Recruiter only. Initializes a missing inventory with 3 slots; preserves existing capacity.
// @Tags inventory
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=domain.Capacity}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /inventory [get]
func (h *InventoryHandler) Summary(c *gin.Context) {
	user, ok := inventoryUser(c)
	if !ok {
		return
	}
	if _, ok := inventoryInput(c, false); !ok {
		return
	}
	capacity, err := h.service.Summary(c.Request.Context(), user)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, capacity)
}

// List godoc
// @Summary List your personal avatars
// @Description Candidate/Recruiter only. Ordered by created_at DESC and id DESC. No storage lookup; no system avatars.
// @Tags inventory
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Success 200 {object} response.Envelope{data=response.Page[domain.Avatar]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /inventory/avatars [get]
func (h *InventoryHandler) List(c *gin.Context) {
	user, ok := inventoryUser(c)
	if !ok {
		return
	}
	values, ok := inventoryInput(c, true)
	if !ok {
		return
	}
	limit, offset := int32(20), int32(0)
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &limit, 1, 100}, {"offset", &offset, 0, 2147483647}} {
		if raw, present := values[param.name]; present {
			if len(raw) != 1 {
				response.Error(c, apperror.New(apperror.CodeValidation, "invalid pagination parameters"))
				return
			}
			value, err := strconv.ParseInt(raw[0], 10, 32)
			if err != nil || value < param.min || value > param.max {
				response.Error(c, apperror.New(apperror.CodeValidation, "invalid pagination parameters"))
				return
			}
			*param.value = int32(value)
		}
	}
	items, total, err := h.service.List(c.Request.Context(), user, limit, offset)
	if err != nil {
		response.Error(c, err)
		return
	}
	if items == nil {
		items = []domain.Avatar{}
	}
	response.OK(c, response.Page[domain.Avatar]{Items: items, Pagination: response.Pagination{Limit: limit, Offset: offset, Total: total}})
}

// Get godoc
// @Summary View your personal avatar
// @Description Returns metadata and a presigned VRM GET valid for five minutes. An unowned avatar is indistinguishable from a missing avatar.
// @Tags inventory
// @Produce json
// @Security SessionCookie
// @Param id path string true "Avatar UUID"
// @Success 200 {object} response.Envelope{data=domain.AvatarDetail}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /inventory/avatars/{id} [get]
func (h *InventoryHandler) Get(c *gin.Context) {
	user, ok := inventoryUser(c)
	if !ok {
		return
	}
	if _, ok := inventoryInput(c, false); !ok {
		return
	}
	id, ok := inventoryAvatarID(c)
	if !ok {
		return
	}
	avatar, err := h.service.Get(c.Request.Context(), user, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, avatar)
}

// PurchaseSlot godoc
// @Summary Purchase inventory slots
// @Description Candidate/Recruiter only. Buys 1 to 10 slots at 1 coin each. The balance check, the coin debit, its ledger row and the capacity increase commit together. Returns the GET /inventory summary after the purchase plus balance, the wallet balance after the debit. An amount outside 1..10 is VALIDATION_ERROR (400); a missing wallet or a balance below the price is INSUFFICIENT_BALANCE (422) and changes nothing.
// @Tags inventory
// @Produce json
// @Security SessionCookie
// @Accept json
// @Param request body domain.SlotPurchaseRequest true "Number of slots to purchase, 1 to 10"
// @Success 200 {object} response.Envelope{data=domain.SlotPurchase}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 422 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /inventory/purchase [post]
func (h *InventoryHandler) PurchaseSlot(c *gin.Context) {
	user, ok := inventoryUser(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid inventory request"))
		return
	}
	var request domain.SlotPurchaseRequest
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Amount < 1 || request.Amount > domain.MaxSlotsPerPurchase {
		response.Error(c, apperror.New(apperror.CodeValidation, "amount must be between 1 and 10"))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid inventory request"))
		return
	}
	purchase, err := h.service.PurchaseSlot(c.Request.Context(), user, request.Amount)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, purchase)
}

// Delete godoc
// @Summary Delete an unused personal avatar
// @Description Rejects avatars referenced by any posting or interview with AVATAR_IN_USE. Frees an inventory slot without changing capacity; does not physically purge the VRM object. Repeated deletion returns 404.
// @Tags inventory
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Avatar UUID"
// @Success 204 "Deleted"
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /inventory/avatars/{id} [delete]
func (h *InventoryHandler) Delete(c *gin.Context) {
	user, ok := inventoryUser(c)
	if !ok {
		return
	}
	if _, ok := inventoryInput(c, false); !ok {
		return
	}
	id, ok := inventoryAvatarID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), user, id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
