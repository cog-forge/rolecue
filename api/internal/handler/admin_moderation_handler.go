package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ModerationService interface {
	List(ctx context.Context, role string, filter domain.ListFilter) ([]domain.PostingSummary, int64, error)
	Get(ctx context.Context, role string, id uuid.UUID) (domain.ReviewPosting, error)
	Approve(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.ReviewPosting, error)
	Reject(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID, reason string) (domain.ReviewPosting, error)
}
type ModerationHandler struct{ service ModerationService }

func NewModerationHandler(service ModerationService) *ModerationHandler {
	return &ModerationHandler{service: service}
}

// RejectPostingRequest is the body of reject. RejectReason is a pointer so a missing field differs from "".
// The name matches the reject_reason field returned in the posting.
type RejectPostingRequest struct {
	RejectReason *string `json:"reject_reason" binding:"required"`
}

func moderationUser(c *gin.Context) (uuid.UUID, string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", false
	}
	return user.ID, user.Role, true
}

func moderationID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return uuid.Nil, false
	}
	return id, true
}

func moderationJSON(c *gin.Context, target any) bool {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}

// moderationFilter reads the queue filters. Each parameter may appear at most once; a missing
// status means the pending queue and "all" removes the status filter.
func moderationFilter(c *gin.Context) (domain.ListFilter, bool) {
	pending := domain.PostingStatusPending
	filter := domain.ListFilter{Status: &pending, Limit: 20}
	fail := func(msg string) (domain.ListFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return domain.ListFilter{}, false
	}
	single := func(name string) (string, bool, bool) {
		raw, exists := c.GetQueryArray(name)
		if !exists {
			return "", false, true
		}
		return raw[0], true, len(raw) == 1
	}
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &filter.Limit, 1, 100}, {"offset", &filter.Offset, 0, 2147483647}} {
		raw, exists, once := single(param.name)
		if !exists {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 32)
		if !once || err != nil || value < param.min || value > param.max {
			return fail("invalid pagination parameters")
		}
		*param.value = int32(value)
	}
	if raw, exists, once := single("status"); exists {
		status := domain.PostingStatus(raw)
		switch {
		case !once:
			return fail("invalid status filter")
		case raw == "all":
			filter.Status = nil
		case status.Valid():
			filter.Status = &status
		default:
			return fail("invalid status filter")
		}
	}
	if raw, exists, once := single("recruiter_id"); exists {
		id, err := uuid.Parse(raw)
		if !once || err != nil {
			return fail("invalid recruiter ID")
		}
		filter.RecruiterID = &id
	}
	for _, param := range []struct {
		name  string
		value **time.Time
	}{{"from", &filter.From}, {"to", &filter.To}} {
		raw, exists, once := single(param.name)
		if !exists {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if !once || err != nil {
			return fail("invalid " + param.name + " date; use RFC 3339, e.g. 2026-10-01T00:00:00Z")
		}
		*param.value = &t
	}
	return filter, true
}

// List godoc
// @Summary List Job Postings for moderation
// @Description Admin only. Newest first. Without status, returns the pending queue; status=all removes the filter. The date window is [from, to) on created_at.
// @Tags admin-job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param status query string false "pending (default), open, intake_closed, closed, rejected or all"
// @Param recruiter_id query string false "Recruiter user ID" format(uuid)
// @Param from query string false "Created at or after (RFC 3339)" format(date-time)
// @Param to query string false "Created before (RFC 3339)" format(date-time)
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Success 200 {object} response.Envelope{data=response.Page[domain.PostingSummary]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/job-postings [get]
func (h *ModerationHandler) List(c *gin.Context) {
	_, role, ok := moderationUser(c)
	if !ok {
		return
	}
	filter, ok := moderationFilter(c)
	if !ok {
		return
	}
	result, total, err := h.service.List(c.Request.Context(), role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	if result == nil {
		result = []domain.PostingSummary{}
	}
	response.OK(c, response.Page[domain.PostingSummary]{Items: result, Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total}})
}

// Get godoc
// @Summary Get a Job Posting for review
// @Description Admin only. Read-only: the Admin cannot edit posting content, the 3D interviewer or the voice.
// @Tags admin-job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.ReviewPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/job-postings/{id} [get]
func (h *ModerationHandler) Get(c *gin.Context) {
	_, role, ok := moderationUser(c)
	if !ok {
		return
	}
	id, ok := moderationID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Approve godoc
// @Summary Approve a pending Job Posting
// @Description Admin only. Trusted Origin required. Moves pending to open, which makes the posting available. Approving an already open posting returns it unchanged; any other non-pending state is 409.
// @Tags admin-job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.ReviewPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/job-postings/{id}/approve [post]
func (h *ModerationHandler) Approve(c *gin.Context) {
	adminID, role, ok := moderationUser(c)
	if !ok {
		return
	}
	id, ok := moderationID(c)
	if !ok {
		return
	}
	result, err := h.service.Approve(c.Request.Context(), adminID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Reject godoc
// @Summary Reject a pending Job Posting
// @Description Admin only. Trusted Origin required. Moves pending to rejected and stores reject_reason (1-1000 characters) for the Recruiter. Rejecting an already rejected posting returns it unchanged and keeps the first reason; any other non-pending state is 409. Unknown fields reject the whole request.
// @Tags admin-job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID" format(uuid)
// @Param request body RejectPostingRequest true "Reject reason; body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=domain.ReviewPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/job-postings/{id}/reject [post]
func (h *ModerationHandler) Reject(c *gin.Context) {
	adminID, role, ok := moderationUser(c)
	if !ok {
		return
	}
	id, ok := moderationID(c)
	if !ok {
		return
	}
	var req RejectPostingRequest
	if !moderationJSON(c, &req) {
		return
	}
	if req.RejectReason == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	result, err := h.service.Reject(c.Request.Context(), adminID, role, id, *req.RejectReason)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
