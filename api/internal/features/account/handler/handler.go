// Package handler serves DELETE /account (erasure) and GET /account/export (data export).
package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// Service is what the handler calls.
type Service interface {
	Erase(ctx context.Context, user authdomain.User, confirm string) error
	Export(ctx context.Context, user authdomain.User) (domain.DataExport, error)
}

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// ErasureResult is the success body of DELETE /account.
type ErasureResult struct {
	Deleted bool `json:"deleted"`
}

// Erase godoc
// @Summary Permanently delete your own account and data
// @Description Requires {"confirm":"DELETE"} (or your email address). A Recruiter with open job postings or any applications on their postings is refused with ACCOUNT_HAS_ACTIVE_WORK (409, counts only); an Administrator with ADMIN_CANNOT_SELF_DELETE. Stored files are deleted first and the database rows last in one transaction, so a failed call can be repeated. Trusted Origin required; unknown fields reject the whole request.
// @Tags account
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body domain.ErasureRequest true "Confirmation; body limit 1 KiB"
// @Success 200 {object} response.Envelope{data=ErasureResult}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /account [delete]
func (h *Handler) Erase(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var body *domain.ErasureRequest
	if err := decoder.Decode(&body); err != nil || body == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	if err := h.service.Erase(c.Request.Context(), user, body.Confirm); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, ErasureResult{Deleted: true})
}

// Export godoc
// @Summary Download your own data
// @Description A Candidate gets their profile, role profiles, applications, interview questions and answers, practice reports and scores, and 15-minute links to their own CV and recording. A Recruiter gets their profile and job postings. Each export writes one audit log line (user id and time only).
// @Tags account
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=domain.DataExport}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /account/export [get]
func (h *Handler) Export(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	data, err := h.service.Export(c.Request.Context(), user)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="rolecue-data-export.json"`)
	response.OK(c, data)
}
