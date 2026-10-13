package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

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
}
type JobPostingHandler struct{ service JobPostingService }

func NewJobPostingHandler(service JobPostingService) *JobPostingHandler {
	return &JobPostingHandler{service: service}
}

// CreateJobPostingRequest carries the Recruiter's JD text and the requirements they confirmed in
// review. Required fields are pointers so a missing field differs from a zero value.
type CreateJobPostingRequest struct {
	Title             *string                      `json:"title" binding:"required"`
	RawText           *string                      `json:"raw_text" binding:"required"`
	SeniorityLevel    *string                      `json:"seniority_level" binding:"required" enums:"intern,junior,mid,senior,lead"`
	Requirements      *[]domain.PostingRequirement `json:"requirements" binding:"required"`
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
	return filter, true
}

// Create godoc
// @Summary Create a Job Posting
// @Description Recruiter only. Stores the JD text with the requirements confirmed in review and submits the posting for Admin approval (status pending). The interviewer avatar must be one of the Recruiter's own avatars and the voice must be active. Trusted Origin required. Unknown fields reject the whole request.
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
	if req.Title == nil || req.RawText == nil || req.SeniorityLevel == nil || req.Requirements == nil ||
		req.RequiredAvatarID == nil || req.RequiredVoiceID == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	result, err := h.service.Create(c.Request.Context(), userID, role, domain.CreateInput{
		Title: *req.Title, RawText: *req.RawText, SeniorityLevel: domain.Level(*req.SeniorityLevel),
		Requirements: *req.Requirements, Location: req.Location, EmploymentType: req.EmploymentType,
		SalaryMin: req.SalaryMin, SalaryMax: req.SalaryMax, InterviewDuration: req.InterviewDuration,
		RequiredAvatarID: *req.RequiredAvatarID, RequiredVoiceID: *req.RequiredVoiceID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, result)
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
// @Summary Edit your pending Job Posting
// @Description Recruiter only, while the posting is pending. Absent fields are unchanged; optional details may be cleared with null. Seniority and requirements are locked once the question bank exists. The JD text never changes. Trusted Origin required.
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
	result, err := h.service.Update(c.Request.Context(), userID, role, id, patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
