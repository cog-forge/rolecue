package handler

import (
	"context"
	"errors"
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

// CandidateApplicationService is the Candidate side of ApplicationService: applying with a CV and
// reading the Candidate's own Applications. The application service implements it.
type CandidateApplicationService interface {
	Apply(ctx context.Context, candidateID uuid.UUID, role string, postingID uuid.UUID, data []byte) (domain.Submission, error)
	ListMine(ctx context.Context, candidateID uuid.UUID, role string, filter domain.CandidateListFilter) (domain.CandidatePage, error)
	GetMine(ctx context.Context, candidateID uuid.UUID, role string, applicationID uuid.UUID) (domain.CandidateApplication, error)
}

// applicationCVBodyLimit covers one CV of MaxCVBytes plus the multipart framing. The service checks
// the size again.
const applicationCVBodyLimit = domain.MaxCVBytes + 16<<10

// candidateService returns the Candidate side of the application service, or an internal error when
// the service was built without it.
func (h *ApplicationHandler) candidateService(c *gin.Context) (CandidateApplicationService, bool) {
	service, ok := h.service.(CandidateApplicationService)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInternal, "candidate applications are not configured"))
		return nil, false
	}
	return service, true
}

// Apply godoc
// @Summary Apply to an open Job Posting with your CV
// @Description Candidate only. Send one PDF as the multipart part "file" (at most 5 MB). The Application starts pending, is visible to the Recruiter at once and needs no interview. Submissions are not capped by interview capacity. Errors: VALIDATION_ERROR (no single PDF of at most 5 MB), FORBIDDEN (not a Candidate), JOB_POSTING_NOT_FOUND (not open to you), INTAKE_NOT_OPEN (intake closed), ALREADY_APPLIED (you already applied to this posting).
// @Tags job-board
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param file formData file true "CV as a PDF, at most 5 MB"
// @Success 201 {object} response.Envelope{data=domain.Submission}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-board/{id}/applications [post]
func (h *ApplicationHandler) Apply(c *gin.Context) {
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
	if user.Role != "candidate" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "only candidates can apply to job postings"))
		return
	}
	service, ok := h.candidateService(c)
	if !ok {
		return
	}
	data, ok := applicationCVFile(c)
	if !ok {
		return
	}
	submission, err := service.Apply(c.Request.Context(), user.ID, user.Role, postingID, data)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, submission)
}

// applicationCVFile reads the single "file" part of a multipart body, up to the CV body limit. Every
// failure is a validation error, the same answer the service gives for a bad PDF.
func applicationCVFile(c *gin.Context) ([]byte, bool) {
	fail := func(msg string) ([]byte, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return nil, false
	}
	const oneFile = "a multipart body with one PDF file is required"
	const tooLarge = "Choose a PDF up to 5 MB."
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, applicationCVBodyLimit)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return fail(oneFile)
	}
	var data []byte
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return fail(tooLarge)
		}
		if err != nil || part.FormName() != "file" || data != nil {
			return fail(oneFile)
		}
		data, err = io.ReadAll(io.LimitReader(part, domain.MaxCVBytes+1))
		_ = part.Close()
		if errors.As(err, &limit) {
			return fail(tooLarge)
		}
		if err != nil {
			return fail(oneFile)
		}
	}
	if data == nil {
		return fail(oneFile)
	}
	return data, true
}

// ListMine godoc
// @Summary List your Applications
// @Description Candidate only. Your own Applications across all postings, newest first, one page at a time. Each item shows the status, the rejection reason, the 24-hour interview deadline and whether you can start the interview now. Transcript, audio, video and CV files are never returned.
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param page query int false "Page number, starting at 1" default(1) minimum(1)
// @Param page_size query int false "Items per page" default(20) minimum(1) maximum(100)
// @Success 200 {object} response.Envelope{data=domain.CandidatePage}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /applications [get]
func (h *ApplicationHandler) ListMine(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	filter := domain.CandidateListFilter{Page: 1, PageSize: 20}
	for _, param := range []struct {
		name  string
		value *int32
	}{{"page", &filter.Page}, {"page_size", &filter.PageSize}} {
		raw, exists := c.GetQueryArray(param.name)
		if !exists {
			continue
		}
		value, err := strconv.ParseInt(raw[0], 10, 32)
		if len(raw) != 1 || err != nil {
			response.Error(c, apperror.New(apperror.CodeValidation, "invalid pagination parameters"))
			return
		}
		*param.value = int32(value)
	}
	service, ok := h.candidateService(c)
	if !ok {
		return
	}
	page, err := service.ListMine(c.Request.Context(), user.ID, user.Role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, page)
}

// DetailMine godoc
// @Summary Read one of your Applications
// @Description Candidate only, your own Applications. Returns the status, the rejection reason, the interview deadline (set on CV approval) and can_start_interview, which is true only while the Application is interview_eligible and the deadline has not passed. Another Candidate's Application is not found.
// @Tags applications
// @Produce json
// @Security SessionCookie
// @Param id path string true "Application ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.CandidateApplication}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /applications/{id} [get]
func (h *ApplicationHandler) DetailMine(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	applicationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid application ID"))
		return
	}
	service, ok := h.candidateService(c)
	if !ok {
		return
	}
	application, err := service.GetMine(c.Request.Context(), user.ID, user.Role, applicationID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, application)
}
