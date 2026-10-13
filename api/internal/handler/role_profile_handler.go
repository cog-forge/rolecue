package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RoleProfileService interface {
	Create(context.Context, uuid.UUID, string) (domain.RoleProfile, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	List(context.Context, uuid.UUID, int32, int32) ([]domain.RoleProfile, int64, error)
	Save(context.Context, uuid.UUID, uuid.UUID, domain.SaveRoleProfile) (domain.RoleProfile, error)
	Confirm(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	Analyze(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
type DocumentService interface {
	Create(context.Context, service.UploadRequest) (service.UploadTicket, error)
	Complete(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	Document(context.Context, uuid.UUID, uuid.UUID) (storage.SignedRequest, error)
}
type RoleProfileHandler struct {
	service   RoleProfileService
	documents DocumentService
}

func NewRoleProfileHandler(service RoleProfileService, documents DocumentService) *RoleProfileHandler {
	return &RoleProfileHandler{service: service, documents: documents}
}

// CreateUpload godoc
// @Summary Start a private PDF upload
// @Tags role-profiles
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body service.UploadRequest true "PDF filename, MIME and byte size (maximum 5 MiB)"
// @Success 201 {object} response.Envelope{data=service.UploadTicket}
// @Failure 400,401,403,409,413,500,502 {object} response.Envelope
// @Router /role-profiles/uploads [post]
func (h *RoleProfileHandler) CreateUpload(c *gin.Context) {
	_, ok := roleUser(c)
	if !ok {
		return
	}
	var input service.UploadRequest
	if !roleJSON(c, &input) {
		return
	}
	ticket, err := h.documents.Create(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Created(c, ticket)
}

// CompleteUpload godoc
// @Summary Verify a PDF upload and create its Role Profile draft
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param id path string true "Upload UUID"
// @Success 200 {object} response.Envelope{data=domain.RoleProfile}
// @Failure 400,401,403,404,409,500,502,503 {object} response.Envelope
// @Router /role-profiles/uploads/{id}/complete [post]
func (h *RoleProfileHandler) CompleteUpload(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	profile, err := h.documents.Complete(c.Request.Context(), user, id)
	roleResult(c, profile, err)
}

// Document godoc
// @Summary Get a short-lived link to the candidate's original PDF
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=storage.SignedRequest}
// @Failure 401,403,404,500,502 {object} response.Envelope
// @Router /role-profiles/{id}/document [get]
func (h *RoleProfileHandler) Document(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	request, err := h.documents.Document(c.Request.Context(), user, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, request)
}

type CreateRoleProfileRequest struct {
	RawText *string `json:"rawText"`
}
type RoleProfileSummary struct {
	ID             uuid.UUID            `json:"id"`
	Title          string               `json:"title"`
	SeniorityLevel *domain.Seniority    `json:"seniorityLevel"`
	Status         domain.ProfileStatus `json:"status"`
	CreatedAt      time.Time            `json:"createdAt"`
}

func roleUser(c *gin.Context) (uuid.UUID, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "Authentication required."))
		return uuid.Nil, false
	}
	if user.Role != "candidate" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "Role Profiles are available to candidates."))
		return uuid.Nil, false
	}
	return user.ID, true
}
func roleResource(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	user, ok := roleUser(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "Invalid Role Profile ID."))
		return uuid.Nil, uuid.Nil, false
	}
	return user, id, true
}
func roleJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "Invalid request body."))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "Invalid request body."))
		return false
	}
	return true
}
func roleResult(c *gin.Context, profile domain.RoleProfile, err error) {
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, profile)
}

// Create godoc
// @Summary Create a private Role Profile draft
// @Tags role-profiles
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreateRoleProfileRequest true "Raw JD text"
// @Success 201 {object} response.Envelope{data=domain.RoleProfile}
// @Failure 400,401,403,409,500 {object} response.Envelope
// @Router /role-profiles [post]
func (h *RoleProfileHandler) Create(c *gin.Context) {
	user, ok := roleUser(c)
	if !ok {
		return
	}
	var req CreateRoleProfileRequest
	if !roleJSON(c, &req) {
		return
	}
	if req.RawText == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "Raw text is required."))
		return
	}
	profile, err := h.service.Create(c.Request.Context(), user, *req.RawText)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, profile)
}

// List godoc
// @Summary List current candidate Role Profiles
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Items to skip" default(0) minimum(0)
// @Success 200 {object} response.Envelope{data=response.Page[RoleProfileSummary]}
// @Failure 400,401,403,500 {object} response.Envelope
// @Router /role-profiles [get]
func (h *RoleProfileHandler) List(c *gin.Context) {
	user, ok := roleUser(c)
	if !ok {
		return
	}
	limit, offset := int32(20), int32(0)
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &limit, 1, 100}, {"offset", &offset, 0, 2147483647}} {
		if raw, exists := c.GetQueryArray(param.name); exists {
			if len(raw) != 1 {
				response.Error(c, apperror.New(apperror.CodeValidation, "Invalid pagination."))
				return
			}
			value, err := strconv.ParseInt(raw[0], 10, 32)
			if err != nil || value < param.min || value > param.max {
				response.Error(c, apperror.New(apperror.CodeValidation, "Invalid pagination."))
				return
			}
			*param.value = int32(value)
		}
	}
	profiles, total, err := h.service.List(c.Request.Context(), user, limit, offset)
	if err != nil {
		response.Error(c, err)
		return
	}
	items := make([]RoleProfileSummary, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, RoleProfileSummary{ID: p.ID, Title: p.Title, SeniorityLevel: p.SeniorityLevel, Status: p.Status, CreatedAt: p.CreatedAt})
	}
	response.OK(c, response.Page[RoleProfileSummary]{Items: items, Pagination: response.Pagination{Limit: limit, Offset: offset, Total: total}})
}

// Get godoc
// @Summary Read an owned Role Profile without hidden questions
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=domain.RoleProfile}
// @Failure 400,401,403,404,500 {object} response.Envelope
// @Router /role-profiles/{id} [get]
func (h *RoleProfileHandler) Get(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	p, e := h.service.Get(c.Request.Context(), user, id)
	roleResult(c, p, e)
}

// Save godoc
// @Summary Save reviewed requirements and refinement note
// @Tags role-profiles
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Param request body domain.SaveRoleProfile true "Review"
// @Success 200 {object} response.Envelope{data=domain.RoleProfile}
// @Failure 400,401,403,404,409,500 {object} response.Envelope
// @Router /role-profiles/{id} [put]
func (h *RoleProfileHandler) Save(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	var req domain.SaveRoleProfile
	if !roleJSON(c, &req) {
		return
	}
	p, e := h.service.Save(c.Request.Context(), user, id, req)
	roleResult(c, p, e)
}

// Analyze godoc
// @Summary Extract requirements for a persisted Role Profile
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=domain.RoleProfile}
// @Failure 400,401,403,404,409,500,502 {object} response.Envelope
// @Router /role-profiles/{id}/analyze [post]
func (h *RoleProfileHandler) Analyze(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	p, e := h.service.Analyze(c.Request.Context(), user, id)
	roleResult(c, p, e)
}

// Confirm godoc
// @Summary Confirm reviewed requirements
// @Tags role-profiles
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=domain.RoleProfile}
// @Failure 400,401,403,404,409,500 {object} response.Envelope
// @Router /role-profiles/{id}/confirm [post]
func (h *RoleProfileHandler) Confirm(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	p, e := h.service.Confirm(c.Request.Context(), user, id)
	roleResult(c, p, e)
}

// Delete godoc
// @Summary Delete an unused owned Role Profile
// @Tags role-profiles
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 204 "Deleted"
// @Failure 400,401,403,404,409,500 {object} response.Envelope
// @Router /role-profiles/{id} [delete]
func (h *RoleProfileHandler) Delete(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), user, id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
