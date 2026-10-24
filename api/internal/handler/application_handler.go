package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

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
	ListForPosting(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, filter domain.ListFilter) ([]domain.ListItem, int64, error)
	ListOwn(ctx context.Context, recruiterID uuid.UUID, role string, filter domain.ListFilter) ([]domain.ListItem, int64, error)
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

// applicationListFilter reads the list filters. Each parameter may appear at most once. The posting
// filter is read only when allowPosting is set (the cross-posting list); the single-posting list
// takes its posting from the path.
func applicationListFilter(c *gin.Context, allowPosting bool) (domain.ListFilter, bool) {
	filter := domain.ListFilter{Limit: 20}
	fail := func(msg string) (domain.ListFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return domain.ListFilter{}, false
	}
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &filter.Limit, 1, 100}, {"offset", &filter.Offset, 0, 2147483647}} {
		raw, exists := c.GetQueryArray(param.name)
		if !exists {
			continue
		}
		value, err := strconv.ParseInt(raw[0], 10, 32)
		if len(raw) != 1 || err != nil || value < param.min || value > param.max {
			return fail("invalid pagination parameters")
		}
		*param.value = int32(value)
	}
	if raw, exists := c.GetQueryArray("status"); exists {
		if len(raw) != 1 {
			return fail("invalid status")
		}
		status := domain.ApplicationStatus(raw[0])
		filter.Status = &status
	}
	if raw, exists := c.GetQueryArray("q"); exists {
		if len(raw) != 1 {
			return fail("invalid search")
		}
		filter.Search = &raw[0]
	}
	if raw, exists := c.GetQueryArray("job_posting_id"); exists && allowPosting {
		id, err := uuid.Parse(raw[0])
		if len(raw) != 1 || err != nil {
			return fail("invalid job posting ID")
		}
		filter.JobPostingID = &id
	}
	return filter, true
}

// ListForPosting godoc
// @Summary List the Applications of one Job Posting
// @Description Recruiter only, own postings; a posting that is not yours returns 404 even when it has no Applications. Newest first. Filter by status and search the Candidate's name or email (case-insensitive, part of the text). Works while intake is open and after it closes; Applications the system rejected show as rejected. Items never carry the CV file, transcript or recording: evidence is reached through Application Detail.
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Param status query string false "Application status" Enums(pending, interview_eligible, interviewed, approved, rejected)
// @Param q query string false "Part of the Candidate's name or email" maxLength(200)
// @Success 200 {object} response.Envelope{data=response.Page[domain.ListItem]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/applications [get]
func (h *ApplicationHandler) ListForPosting(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	postingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return
	}
	filter, ok := applicationListFilter(c, false)
	if !ok {
		return
	}
	items, total, err := h.service.ListForPosting(c.Request.Context(), user.ID, user.Role, postingID, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.Page[domain.ListItem]{
		Items:      items,
		Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total},
	})
}

// ListOwn godoc
// @Summary List the Applications across your Job Postings
// @Description Recruiter only. Every Application on the Recruiter's own postings, newest first, optionally narrowed to one of them with job_posting_id (a posting that is not yours returns 404). Same filters, ordering and redaction as the per-posting list.
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Param status query string false "Application status" Enums(pending, interview_eligible, interviewed, approved, rejected)
// @Param q query string false "Part of the Candidate's name or email" maxLength(200)
// @Param job_posting_id query string false "Only this Job Posting" format(uuid)
// @Success 200 {object} response.Envelope{data=response.Page[domain.ListItem]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /recruiter/applications [get]
func (h *ApplicationHandler) ListOwn(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	filter, ok := applicationListFilter(c, true)
	if !ok {
		return
	}
	items, total, err := h.service.ListOwn(c.Request.Context(), user.ID, user.Role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.Page[domain.ListItem]{
		Items:      items,
		Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total},
	})
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
