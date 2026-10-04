package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/swp391-group3/ai-interview-practice/api/internal/features/jd/domain"
	"github.com/swp391-group3/ai-interview-practice/api/internal/middleware"
	"github.com/swp391-group3/ai-interview-practice/api/pkg/apperror"
	"github.com/swp391-group3/ai-interview-practice/api/pkg/response"
)

type JDService interface {
	Analyze(context.Context, string) (domain.StructuredJD, error)
	Create(context.Context, uuid.UUID, domain.CreateInput) (domain.JD, error)
	List(context.Context, uuid.UUID, int32, int32) ([]domain.ListItem, int64, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.JD, error)
	Update(context.Context, uuid.UUID, uuid.UUID, domain.UpdateInput) (domain.JD, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
type JDHandler struct{ service JDService }

func NewJDHandler(service JDService) *JDHandler { return &JDHandler{service: service} }

type AnalyzeJDRequest struct {
	RawText *string `json:"rawText" binding:"required"`
}
type CreateJDRequest struct {
	RawText      *string              `json:"rawText" binding:"required"`
	StructuredJD *domain.StructuredJD `json:"structuredJD" binding:"required"`
}
type UpdateJDRequest struct {
	StructuredJD *domain.StructuredJD `json:"structuredJD" binding:"required"`
}
type JDResponse struct {
	ID           uuid.UUID           `json:"id"`
	RawText      string              `json:"rawText"`
	StructuredJD domain.StructuredJD `json:"structuredJD"`
	Status       domain.Status       `json:"status"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

func jdResponse(jd domain.JD) JDResponse {
	return JDResponse{ID: jd.ID, RawText: jd.RawText, StructuredJD: jd.StructuredJD, Status: jd.Status, CreatedAt: jd.CreatedAt, UpdatedAt: jd.UpdatedAt}
}
func jdUser(c *gin.Context) (uuid.UUID, bool) {
	id, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
	}
	return id, ok
}
func jdID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job description ID"))
		return uuid.Nil, false
	}
	return id, true
}
func jdJSON(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		jdBadRequest(c)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		jdBadRequest(c)
		return false
	}
	return true
}
func jdBadRequest(c *gin.Context) {
	response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
}

// Analyze godoc
// @Summary Analyze a raw JD without saving
// @Tags jds
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body AnalyzeJDRequest true "JD request"
// @Success 200 {object} response.Envelope{data=domain.StructuredJD}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 502 {object} response.Envelope
// @Router /jds/analyze [post]
func (h *JDHandler) Analyze(c *gin.Context) {
	if _, ok := jdUser(c); !ok {
		return
	}
	var req AnalyzeJDRequest
	if !jdJSON(c, &req) {
		return
	}
	if req.RawText == nil {
		jdBadRequest(c)
		return
	}
	result, err := h.service.Analyze(c.Request.Context(), *req.RawText)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Create godoc
// @Summary Save a reviewed JD
// @Tags jds
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreateJDRequest true "JD request"
// @Success 201 {object} response.Envelope{data=JDResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /jds [post]
func (h *JDHandler) Create(c *gin.Context) {
	user, ok := jdUser(c)
	if !ok {
		return
	}
	var req CreateJDRequest
	if !jdJSON(c, &req) {
		return
	}
	if req.RawText == nil || req.StructuredJD == nil {
		jdBadRequest(c)
		return
	}
	result, err := h.service.Create(c.Request.Context(), user, domain.CreateInput{RawText: *req.RawText, StructuredJD: *req.StructuredJD})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, jdResponse(result))
}

// List godoc
// @Summary List current user's JDs
// @Tags jds
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Success 200 {object} response.Envelope{data=response.Page[domain.ListItem]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /jds [get]
func (h *JDHandler) List(c *gin.Context) {
	user, ok := jdUser(c)
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
			value, err := strconv.ParseInt(raw[0], 10, 32)
			if len(raw) != 1 || err != nil || value < param.min || value > param.max {
				response.Error(c, apperror.New(apperror.CodeValidation, "invalid pagination parameters"))
				return
			}
			*param.value = int32(value)
		}
	}
	result, total, err := h.service.List(c.Request.Context(), user, limit, offset)
	if err != nil {
		response.Error(c, err)
		return
	}
	if result == nil {
		result = []domain.ListItem{}
	}
	response.OK(c, response.Page[domain.ListItem]{Items: result, Pagination: response.Pagination{Limit: limit, Offset: offset, Total: total}})
}

// Get godoc
// @Summary Get an owned JD
// @Tags jds
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "JD UUID"
// @Success 200 {object} response.Envelope{data=JDResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /jds/{id} [get]
func (h *JDHandler) Get(c *gin.Context) {
	user, ok := jdUser(c)
	if !ok {
		return
	}
	id, ok := jdID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), user, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, jdResponse(result))
}

// Update godoc
// @Summary Update reviewed fields; original raw text is immutable
// @Tags jds
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body UpdateJDRequest true "JD request"
// @Param id path string true "JD UUID"
// @Success 200 {object} response.Envelope{data=JDResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /jds/{id} [put]
func (h *JDHandler) Update(c *gin.Context) {
	user, ok := jdUser(c)
	if !ok {
		return
	}
	id, ok := jdID(c)
	if !ok {
		return
	}
	var req UpdateJDRequest
	if !jdJSON(c, &req) {
		return
	}
	if req.StructuredJD == nil {
		jdBadRequest(c)
		return
	}
	result, err := h.service.Update(c.Request.Context(), user, id, domain.UpdateInput{StructuredJD: *req.StructuredJD})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, jdResponse(result))
}

// Delete godoc
// @Summary Delete an owned JD
// @Tags jds
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "JD UUID"
// @Success 204 "Deleted"
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Router /jds/{id} [delete]
func (h *JDHandler) Delete(c *gin.Context) {
	user, ok := jdUser(c)
	if !ok {
		return
	}
	id, ok := jdID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), user, id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
