package handler

import (
	"context"
	"encoding/json"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strconv"
)

type AccountsService interface {
	List(context.Context, domain.ListFilter) (domain.List, error)
	Get(context.Context, uuid.UUID) (domain.Account, error)
	SetLock(context.Context, string, uuid.UUID, bool, domain.LockRequest) (domain.Transition, error)
}

type AccountsHandler struct{ service AccountsService }

func NewAccountsHandler(service AccountsService) *AccountsHandler {
	return &AccountsHandler{service: service}
}

// List godoc
// @Summary List system accounts
// @Tags admin-users
// @Produce json
// @Security SessionCookie
// @Param search query string false "Name or email substring"
// @Param filter query string false "all, locked, candidate, recruiter, admin"
// @Param limit query int false "Page size" default(20)
// @Param offset query int false "Page offset" default(0)
// @Success 200 {object} response.Envelope{data=domain.List}
// @Failure 400,401,403,500 {object} response.Envelope
// @Router /admin/users [get]
func (h *AccountsHandler) List(c *gin.Context) {
	limit, e1 := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 32)
	offset, e2 := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if e1 != nil || e2 != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid account pagination"))
		return
	}
	result, err := h.service.List(c.Request.Context(), domain.ListFilter{Search: c.Query("search"), Filter: c.DefaultQuery("filter", "all"), Limit: int32(limit), Offset: offset})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Get godoc
// @Summary Get system account details
// @Tags admin-users
// @Produce json
// @Security SessionCookie
// @Param id path string true "Account ID"
// @Success 200 {object} response.Envelope{data=domain.Account}
// @Failure 400,401,403,404,500 {object} response.Envelope
// @Router /admin/users/{id} [get]
func (h *AccountsHandler) Get(c *gin.Context) {
	id, ok := accountID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Lock godoc
// @Summary Lock a Candidate or Recruiter account
// @Tags admin-users
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Account ID"
// @Param body body domain.LockRequest true "User-visible reason and optional expiry in seconds"
// @Success 200 {object} response.Envelope{data=domain.Transition}
// @Failure 400,401,403,404,500,503 {object} response.Envelope
// @Router /admin/users/{id}/lock [post]
func (h *AccountsHandler) Lock(c *gin.Context) {
	var input domain.LockRequest
	if !accountJSON(c, &input) {
		return
	}
	h.setLock(c, true, input)
}

// Unlock godoc
// @Summary Unlock a Candidate or Recruiter account
// @Tags admin-users
// @Produce json
// @Security SessionCookie
// @Param id path string true "Account ID"
// @Success 200 {object} response.Envelope{data=domain.Transition}
// @Failure 400,401,403,404,500,503 {object} response.Envelope
// @Router /admin/users/{id}/unlock [post]
func (h *AccountsHandler) Unlock(c *gin.Context) {
	var input struct{}
	if !accountJSON(c, &input) {
		return
	}
	h.setLock(c, false, domain.LockRequest{})
}

func (h *AccountsHandler) setLock(c *gin.Context, locking bool, input domain.LockRequest) {
	id, ok := accountID(c)
	if !ok {
		return
	}
	result, err := h.service.SetLock(c.Request.Context(), c.GetHeader("Cookie"), id, locking, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

func accountID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid account ID"))
		return uuid.Nil, false
	}
	return id, true
}

func accountJSON(c *gin.Context, target any) bool {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid account request"))
		return false
	}
	return true
}
