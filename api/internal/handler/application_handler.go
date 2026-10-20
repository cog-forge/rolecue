package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const applicationBodyLimit = 16 << 10

type ApplicationService interface {
	ApproveCV(ctx context.Context, recruiterID uuid.UUID, role string, postingID, applicationID uuid.UUID) (domain.Screening, error)
	RejectCV(ctx context.Context, recruiterID uuid.UUID, role string, postingID, applicationID uuid.UUID, reason *string) (domain.Screening, error)
}
type ApplicationHandler struct{ service ApplicationService }

func NewApplicationHandler(service ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: service}
}

// RejectCVRequest is optional: an empty body rejects without a reason.
type RejectCVRequest struct {
	// RejectReason is an optional note, at most 500 characters.
	RejectReason *string `json:"reject_reason"`
}

// applicationTarget reads the caller and the posting and Application IDs from the path.
func applicationTarget(c *gin.Context) (uuid.UUID, string, uuid.UUID, uuid.UUID, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	postingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	applicationID, err := uuid.Parse(c.Param("applicationId"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid application ID"))
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	return user.ID, user.Role, postingID, applicationID, true
}

// applicationJSON decodes an optional strict JSON body; no body leaves target untouched. A body of
// unknown length (chunked) without a Content-Type counts as no body only when it is really empty.
func applicationJSON(c *gin.Context, target any) bool {
	if c.Request.ContentLength == 0 {
		return true
	}
	if c.Request.ContentLength < 0 && c.ContentType() == "" {
		var probe [1]byte
		if n, _ := io.ReadFull(io.LimitReader(c.Request.Body, 1), probe[:]); n == 0 {
			return true
		}
	}
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, applicationBodyLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}

// ApproveCV godoc
// @Summary Approve an Application's CV
// @Description Recruiter only, own open postings. The Application moves from pending to interview_eligible and gets interview_deadline = now + 24 hours. No interview slot is consumed yet; at most interview_slot Applications can ever be CV-approved per posting. Errors: VALIDATION_ERROR (bad IDs), FORBIDDEN (not a Recruiter), JOB_POSTING_NOT_FOUND (missing or not yours), APPLICATION_NOT_FOUND (not on this posting), APPLICATION_NOT_PENDING (already screened), INTAKE_NOT_OPEN (posting not open), INTERVIEW_CAPACITY_NOT_FUNDED (interview_slot not set), INTERVIEW_CAPACITY_FULL.
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param applicationId path string true "Application ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Screening}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/applications/{applicationId}/approve-cv [post]
func (h *ApplicationHandler) ApproveCV(c *gin.Context) {
	userID, role, postingID, applicationID, ok := applicationTarget(c)
	if !ok {
		return
	}
	screening, err := h.service.ApproveCV(c.Request.Context(), userID, role, postingID, applicationID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, screening)
}

// RejectCV godoc
// @Summary Reject an Application's CV
// @Description Recruiter only, own postings. A pending Application becomes terminally rejected. The body is optional. Errors: VALIDATION_ERROR (bad IDs, body or reason over 500 characters), FORBIDDEN (not a Recruiter), JOB_POSTING_NOT_FOUND (missing or not yours), APPLICATION_NOT_FOUND (not on this posting), APPLICATION_NOT_PENDING (already screened).
// @Tags applications
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param applicationId path string true "Application ID" format(uuid)
// @Param body body RejectCVRequest false "Optional reject reason"
// @Success 200 {object} response.Envelope{data=domain.Screening}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/applications/{applicationId}/reject-cv [post]
func (h *ApplicationHandler) RejectCV(c *gin.Context) {
	userID, role, postingID, applicationID, ok := applicationTarget(c)
	if !ok {
		return
	}
	var request RejectCVRequest
	if !applicationJSON(c, &request) {
		return
	}
	screening, err := h.service.RejectCV(c.Request.Context(), userID, role, postingID, applicationID, request.RejectReason)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, screening)
}
