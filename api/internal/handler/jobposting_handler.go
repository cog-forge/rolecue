package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type JobPostingService interface {
	Create(ctx context.Context, recruiterID uuid.UUID, role string, input domain.CreateInput) (domain.JobPosting, error)
	List(ctx context.Context, recruiterID uuid.UUID, role string, filter domain.ListFilter) ([]domain.Summary, int64, error)
	Get(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.JobPosting, error)
	Update(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID, patch domain.Edit) (domain.JobPosting, error)
	Submit(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.JobPosting, error)
	Resubmit(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.JobPosting, error)
	UploadDocument(ctx context.Context, role string, data []byte) (domain.Document, error)
	Document(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.DocumentLink, error)
}
type JobPostingExtractor interface {
	Suggest(ctx context.Context, role, raw string) (domain.Suggestion, error)
}
type JobPostingHandler struct {
	service    JobPostingService
	extraction JobPostingExtractor
}

func NewJobPostingHandler(service JobPostingService, extraction JobPostingExtractor) *JobPostingHandler {
	return &JobPostingHandler{service: service, extraction: extraction}
}

// ExtractJobPostingRequest carries the JD text to read requirements from.
type ExtractJobPostingRequest struct {
	RawText *string `json:"raw_text" binding:"required"`
}

// CreateJobPostingRequest carries the Recruiter's uploaded JD PDF and the requirements they
// confirmed in review. Required fields are pointers so a missing field differs from a zero value.
type CreateJobPostingRequest struct {
	Title             *string                      `json:"title" binding:"required"`
	DocumentID        *string                      `json:"document_id" binding:"required" format:"uuid"`
	SeniorityLevel    *string                      `json:"seniority_level" binding:"required" enums:"intern,junior,mid,senior,lead"`
	Requirements      *[]domain.PostingRequirement `json:"requirements" binding:"required"`
	RefinementNote    string                       `json:"refinement_note"`
	Location          *string                      `json:"location"`
	EmploymentType    *string                      `json:"employment_type"`
	SalaryMin         *int32                       `json:"salary_min"`
	SalaryMax         *int32                       `json:"salary_max"`
	InterviewDuration *int32                       `json:"interview_duration"`
	RequiredAvatarID  *uuid.UUID                   `json:"required_avatar_id" binding:"required"`
	RequiredVoiceID   *uuid.UUID                   `json:"required_voice_id" binding:"required"`
}

// The JD text is up to 20,000 runes; escaped in JSON, a rune can take up to 12 bytes.
const jobPostingBodyLimit = 512 << 10

// A JD PDF is up to 5 MB (the service checks it again); the rest covers the multipart framing.
const (
	maxJobPostingPDFBytes  = 5 << 20
	jobPostingPDFBodyLimit = maxJobPostingPDFBytes + 16<<10
)

func jobPostingUser(c *gin.Context) (uuid.UUID, string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", false
	}
	return user.ID, user.Role, true
}

func jobPostingID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return uuid.Nil, false
	}
	return id, true
}

func jobPostingJSON(c *gin.Context, target any) bool {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, jobPostingBodyLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}

// jobPostingFilter reads the list filters. Each parameter may appear at most once.
func jobPostingFilter(c *gin.Context) (domain.ListFilter, bool) {
	filter := domain.ListFilter{Limit: 20}
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
		status := domain.Status(raw)
		if !once || !status.Valid() {
			return fail("invalid status filter")
		}
		filter.Status = &status
	}
	if raw, exists, once := single("search"); exists {
		if !once {
			return fail("invalid search")
		}
		filter.Search = &raw
	}
	for _, param := range []struct {
		name  string
		value **bool
	}{
		{"funded", &filter.Funded}, {"review_submitted", &filter.ReviewSubmitted},
	} {
		if raw, exists, once := single(param.name); exists {
			if !once || (raw != "true" && raw != "false") {
				return fail("invalid " + param.name)
			}
			value := raw == "true"
			*param.value = &value
		}
	}
	return filter, true
}

// Create godoc
// @Summary Create a Job Posting
// @Description Recruiter only. Stores the JD PDF uploaded through POST /job-postings/documents (read again on the server) with the requirements confirmed in review. The posting is created pending but not yet submitted: generate its question bank, then submit it for Admin approval. The interviewer avatar must be one of the Recruiter's own avatars and the voice must be active. Trusted Origin required. Unknown fields reject the whole request.
// @Tags job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body CreateJobPostingRequest true "Job Posting"
// @Success 201 {object} response.Envelope{data=domain.JobPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings [post]
func (h *JobPostingHandler) Create(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	var req CreateJobPostingRequest
	if !jobPostingJSON(c, &req) {
		return
	}
	if req.Title == nil || req.DocumentID == nil || req.SeniorityLevel == nil || req.Requirements == nil ||
		req.RequiredAvatarID == nil || req.RequiredVoiceID == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	// The PDF is read again on the server, which can outlast the default write timeout.
	if !roleWriteDeadline(c, 45*time.Second) {
		return
	}
	result, err := h.service.Create(c.Request.Context(), userID, role, domain.CreateInput{
		Title: *req.Title, DocumentID: *req.DocumentID, SeniorityLevel: domain.Level(*req.SeniorityLevel),
		Requirements: *req.Requirements, RefinementNote: req.RefinementNote, Location: req.Location, EmploymentType: req.EmploymentType,
		SalaryMin: req.SalaryMin, SalaryMax: req.SalaryMax, InterviewDuration: req.InterviewDuration,
		RequiredAvatarID: *req.RequiredAvatarID, RequiredVoiceID: *req.RequiredVoiceID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, result)
}

// Extract godoc
// @Summary Suggest requirements from JD text
// @Description Recruiter only. AI extraction reads the JD text and suggests a title, seniority and requirements for the Recruiter to review before creating the posting. Nothing is stored. seniority_level is null when the JD does not say. Trusted Origin required.
// @Tags job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body ExtractJobPostingRequest true "JD text"
// @Success 200 {object} response.Envelope{data=domain.Suggestion}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 502 {object} response.Envelope
// @Router /job-postings/extract [post]
func (h *JobPostingHandler) Extract(c *gin.Context) {
	_, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	var req ExtractJobPostingRequest
	if !jobPostingJSON(c, &req) {
		return
	}
	if req.RawText == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	// Extraction can outlast the server's default write timeout; give it the same deadline as
	// Role Profile analysis, which runs the same pipeline.
	if !roleWriteDeadline(c, 75*time.Second) {
		return
	}
	result, err := h.extraction.Suggest(c.Request.Context(), role, *req.RawText)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// List godoc
// @Summary List your Job Postings
// @Description Recruiter only. Newest first. Filter by status and search the title.
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param limit query int false "Page size (1-100)" default(20)
// @Param offset query int false "Items to skip" default(0)
// @Param status query string false "Status filter" Enums(pending, open, intake_closed, closed, rejected)
// @Param search query string false "Case-insensitive title search"
// @Param funded query boolean false "Filter by funded interview capacity"
// @Param review_submitted query boolean false "Filter by explicit review submission"
// @Success 200 {object} response.Envelope{data=response.Page[domain.Summary]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings [get]
func (h *JobPostingHandler) List(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	filter, ok := jobPostingFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.List(c.Request.Context(), userID, role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.Page[domain.Summary]{
		Items:      items,
		Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total},
	})
}

// Get godoc
// @Summary Get your Job Posting
// @Description Recruiter only. Includes the JD text, confirmed requirements and whether the question bank exists. Another Recruiter's posting is not found.
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.JobPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id} [get]
func (h *JobPostingHandler) Get(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	id, ok := jobPostingID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Update godoc
// @Summary Edit your pending or rejected Job Posting
// @Description Recruiter only, while the posting is pending or rejected. Absent fields are unchanged; optional details may be cleared with null; unchanged values may be resent. document_id (a new PDF from POST /job-postings/documents) can change only on a rejected posting. Changing seniority, requirements or the JD while pending is refused once the question bank exists; on a rejected posting it clears the bank, which is regenerated after resubmitting. Saving does not resubmit. Trusted Origin required.
// @Tags job-postings
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job Posting ID" format(uuid)
// @Param request body domain.Edit true "Fields to change"
// @Success 200 {object} response.Envelope{data=domain.JobPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id} [patch]
func (h *JobPostingHandler) Update(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	id, ok := jobPostingID(c)
	if !ok {
		return
	}
	var patch domain.Edit
	if !jobPostingJSON(c, &patch) {
		return
	}
	// A new PDF is read again on the server, which can outlast the default write timeout.
	if !roleWriteDeadline(c, 45*time.Second) {
		return
	}
	result, err := h.service.Update(c.Request.Context(), userID, role, id, patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Submit godoc
// @Summary Submit your prepared Job Posting for Admin review
// @Description Recruiter only. Puts a pending posting in the Admin review queue. Needs a non-empty question bank; generating the bank never submits on its own. Repeating it changes nothing; a rejected posting uses resubmit. Trusted Origin required.
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.JobPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/submit [post]
func (h *JobPostingHandler) Submit(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	id, ok := jobPostingID(c)
	if !ok {
		return
	}
	result, err := h.service.Submit(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Resubmit godoc
// @Summary Send your rejected Job Posting for review again
// @Description Recruiter only. Moves a rejected posting back to pending review and clears the rejection reason. Needs a non-empty question bank. Repeating it once the posting is pending review changes nothing; open or closed postings return 409. Trusted Origin required.
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.JobPosting}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/resubmit [post]
func (h *JobPostingHandler) Resubmit(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	id, ok := jobPostingID(c)
	if !ok {
		return
	}
	result, err := h.service.Resubmit(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// UploadDocument godoc
// @Summary Upload a JD PDF
// @Description Recruiter only. Stores the PDF (up to 5 MB, with selectable text) and returns its document_id and the text read from it. Pass document_id to POST /job-postings, or to PATCH /job-postings/{id} on a rejected posting; pass raw_text to POST /job-postings/extract for suggestions. Trusted Origin required.
// @Tags job-postings
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param file formData file true "JD PDF"
// @Success 201 {object} response.Envelope{data=domain.Document}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 413 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/documents [post]
func (h *JobPostingHandler) UploadDocument(c *gin.Context) {
	_, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	if !roleWriteDeadline(c, 60*time.Second) {
		return
	}
	data, ok := jobPostingPDF(c)
	if !ok {
		return
	}
	result, err := h.service.UploadDocument(c.Request.Context(), role, data)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, result)
}

// jobPostingPDF reads the single "file" part of a multipart body, up to the PDF size limit.
func jobPostingPDF(c *gin.Context) ([]byte, bool) {
	fail := func(code apperror.Code, msg string) ([]byte, bool) {
		response.Error(c, apperror.New(code, msg))
		return nil, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, jobPostingPDFBodyLimit)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return fail(apperror.CodeValidation, "a multipart body with one PDF file is required")
	}
	var data []byte
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return fail(apperror.CodePDFTooLarge, "Choose a PDF up to 5 MB.")
		}
		if err != nil || part.FormName() != "file" || data != nil {
			return fail(apperror.CodeValidation, "a multipart body with one PDF file is required")
		}
		data, err = io.ReadAll(io.LimitReader(part, maxJobPostingPDFBytes+1))
		_ = part.Close()
		if errors.As(err, &tooLarge) || int64(len(data)) > maxJobPostingPDFBytes {
			return fail(apperror.CodePDFTooLarge, "Choose a PDF up to 5 MB.")
		}
		if err != nil || len(data) == 0 {
			return fail(apperror.CodeInvalidPDF, "Choose a valid PDF up to 5 MB.")
		}
	}
	if data == nil {
		return fail(apperror.CodeValidation, "a multipart body with one PDF file is required")
	}
	return data, true
}

// Document godoc
// @Summary Open your Job Posting's JD PDF
// @Description Recruiter only. Returns a link to the PDF that expires after a few minutes. A posting created before PDF upload has none (404).
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.DocumentLink}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/document [get]
func (h *JobPostingHandler) Document(c *gin.Context) {
	userID, role, ok := jobPostingUser(c)
	if !ok {
		return
	}
	id, ok := jobPostingID(c)
	if !ok {
		return
	}
	result, err := h.service.Document(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
