package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// rubricBodyLimit covers the largest valid rubric (10 criteria with 2000-character guides, several bytes per
// character) with room for the JSON around it.
const rubricBodyLimit = 256 << 10

// RubricService is what the rubric handler needs. The role and ownership rules live in the service.
type RubricService interface {
	List(ctx context.Context, userID uuid.UUID, role string, query domain.RubricListQuery) ([]domain.Rubric, int64, error)
	Get(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID) (domain.Rubric, error)
	Create(ctx context.Context, userID uuid.UUID, role string, in domain.RubricCreate) (domain.Rubric, error)
	Copy(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID, name *string) (domain.Rubric, error)
	Update(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID, patch domain.RubricPatch) (domain.Rubric, error)
	Delete(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID) error
}

// RubricHandler serves /rubrics for Admins (system rubrics) and Recruiters (their own and the system ones).
type RubricHandler struct{ service RubricService }

func NewRubricHandler(service RubricService) *RubricHandler { return &RubricHandler{service: service} }

// RubricCriterionRequest is one criterion. id is only for an update that keeps an existing criterion; a
// criterion left out of an update is removed and one without an id is added.
type RubricCriterionRequest struct {
	ID           *uuid.UUID `json:"id" format:"uuid"`
	MetricID     *uuid.UUID `json:"metric_id" format:"uuid"`
	Name         *string    `json:"name"`
	MaxPoints    *int32     `json:"max_points"`
	ScoringGuide *string    `json:"scoring_guide"`
}

// RubricCreateRequest creates a rubric from scratch. The scope is never sent: it follows the caller's role.
type RubricCreateRequest struct {
	Name         *string                  `json:"name"`
	Description  *string                  `json:"description"`
	QuestionType *string                  `json:"question_type"`
	Criteria     []RubricCriterionRequest `json:"criteria"`
}

// RubricUpdateRequest changes some fields. updated_at is required: send the value of the rubric you read.
type RubricUpdateRequest struct {
	UpdatedAt    *time.Time                `json:"updated_at" format:"date-time"`
	Name         *string                   `json:"name"`
	Description  *string                   `json:"description"`
	QuestionType *string                   `json:"question_type"`
	Criteria     *[]RubricCriterionRequest `json:"criteria"`
	IsActive     *bool                     `json:"is_active"`
}

// RubricCopyRequest is the optional body of a copy.
type RubricCopyRequest struct {
	Name *string `json:"name"`
}

func rubricUser(c *gin.Context) (uuid.UUID, string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", false
	}
	return user.ID, user.Role, true
}

// rubricID reads the path id. A malformed id is a plain not-found, like an unknown one.
func rubricID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, domain.ErrRubricNotFound)
		return uuid.Nil, false
	}
	return id, true
}

// rubricJSON decodes a strict JSON body: one object, no unknown fields (so a request cannot smuggle a scope or
// owner), within the size limit. optional lets an empty body through.
func rubricJSON(c *gin.Context, target any, optional bool) bool {
	if optional && c.Request.ContentLength == 0 {
		return true
	}
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, rubricBodyLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}

func criterionInputs(requests []RubricCriterionRequest) []domain.CriterionInput {
	out := make([]domain.CriterionInput, 0, len(requests))
	for _, request := range requests {
		out = append(out, domain.CriterionInput{
			ID: request.ID, MetricID: request.MetricID, Name: request.Name, MaxPoints: request.MaxPoints, ScoringGuide: request.ScoringGuide,
		})
	}
	return out
}

// rubricListQuery reads the list filters. Each parameter may appear at most once.
func rubricListQuery(c *gin.Context) (domain.RubricListQuery, bool) {
	query := domain.RubricListQuery{}
	bad := func(msg string) (domain.RubricListQuery, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return domain.RubricListQuery{}, false
	}
	single := func(name string) (string, bool, bool) {
		values, exists := c.GetQueryArray(name)
		if !exists {
			return "", false, true
		}
		return values[0], true, len(values) == 1
	}
	for name, target := range map[string]*string{"scope": &query.Scope, "question_type": &query.QuestionType, "status": &query.Status} {
		value, exists, once := single(name)
		if exists && !once {
			return bad("invalid " + name + " filter")
		}
		*target = value
	}
	for _, param := range []struct {
		name     string
		target   *int32
		min, max int64
	}{{"limit", &query.Limit, 1, domain.MaxRubricPageSize}, {"offset", &query.Offset, 0, 2147483647}} {
		raw, exists, once := single(param.name)
		if !exists {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 32)
		if !once || err != nil || value < param.min || value > param.max {
			return bad("invalid pagination parameters")
		}
		*param.target = int32(value)
	}
	return query, true
}

// List godoc
// @Summary List scoring rubrics
// @Description Admin: the system rubrics, any status. Recruiter: the active system rubrics and their own (private) ones. Candidates get 403. The list leaves out criteria (see criteria_count and max_points); open one rubric for them. locked and reference_count are shown only for rubrics the caller can manage.
// @Tags rubrics
// @Produce json
// @Security SessionCookie
// @Param scope query string false "system or own" Enums(system, own)
// @Param question_type query string false "Question type slug, for example concept_explanation"
// @Param status query string false "Default active" Enums(active, archived, all)
// @Param limit query int false "Page size (1-100)" default(20)
// @Param offset query int false "Items to skip" default(0)
// @Success 200 {object} response.Envelope{data=response.Page[domain.Rubric]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /rubrics [get]
func (h *RubricHandler) List(c *gin.Context) {
	userID, role, ok := rubricUser(c)
	if !ok {
		return
	}
	query, ok := rubricListQuery(c)
	if !ok {
		return
	}
	items, total, err := h.service.List(c.Request.Context(), userID, role, query)
	if err != nil {
		response.Error(c, err)
		return
	}
	limit := query.Limit
	if limit == 0 {
		limit = domain.DefaultRubricPageSize
	}
	response.OK(c, response.Page[domain.Rubric]{
		Items:      items,
		Pagination: response.Pagination{Limit: limit, Offset: query.Offset, Total: total},
	})
}

// Get godoc
// @Summary Get a scoring rubric with its criteria
// @Description A rubric the caller may not see (another Recruiter's private rubric, an archived system rubric for a Recruiter, an Admin asking for a private one) is RUBRIC_NOT_FOUND, exactly like an unknown id. updated_at is the token to send back when changing the rubric.
// @Tags rubrics
// @Produce json
// @Security SessionCookie
// @Param id path string true "Rubric ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Rubric}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /rubrics/{id} [get]
func (h *RubricHandler) Get(c *gin.Context) {
	userID, role, ok := rubricUser(c)
	if !ok {
		return
	}
	id, ok := rubricID(c)
	if !ok {
		return
	}
	rubric, err := h.service.Get(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, rubric)
}

// Create godoc
// @Summary Create a scoring rubric
// @Description A Recruiter creates a private rubric, an Admin a system rubric; the scope is not part of the request. At least 1 and at most 10 criteria, each on one metric of the catalog (GET /evaluation-metrics) with max_points 1-10 and a scoring_guide that describes every whole score from 0 to max_points. Errors: VALIDATION_ERROR, EVALUATION_METRIC_UNKNOWN.
// @Tags rubrics
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body RubricCreateRequest true "The rubric"
// @Success 201 {object} response.Envelope{data=domain.Rubric}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /rubrics [post]
func (h *RubricHandler) Create(c *gin.Context) {
	userID, role, ok := rubricUser(c)
	if !ok {
		return
	}
	var request RubricCreateRequest
	if !rubricJSON(c, &request, false) {
		return
	}
	rubric, err := h.service.Create(c.Request.Context(), userID, role, domain.RubricCreate{
		Name: request.Name, Description: request.Description, QuestionType: request.QuestionType, Criteria: criterionInputs(request.Criteria),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, rubric)
}

// Copy godoc
// @Summary Copy a scoring rubric
// @Description Creates a new, independent rubric (own criteria rows) from a rubric the caller may read: a Recruiter's copy is private, an Admin's copy is a system rubric. Changing the copy never changes the source. The body is optional; name defaults to "Copy of <source name>".
// @Tags rubrics
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Source rubric ID" format(uuid)
// @Param body body RubricCopyRequest false "Optional name of the copy"
// @Success 201 {object} response.Envelope{data=domain.Rubric}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /rubrics/{id}/copies [post]
func (h *RubricHandler) Copy(c *gin.Context) {
	userID, role, ok := rubricUser(c)
	if !ok {
		return
	}
	id, ok := rubricID(c)
	if !ok {
		return
	}
	var request RubricCopyRequest
	if !rubricJSON(c, &request, true) {
		return
	}
	rubric, err := h.service.Copy(c.Request.Context(), userID, role, id, request.Name)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, rubric)
}

// Update godoc
// @Summary Change or archive a scoring rubric
// @Description Admin for system rubrics, the owner for private ones; anyone else gets 403 (system rubric read by a Recruiter) or 404 (not visible). Send updated_at exactly as read: if the rubric changed since, the answer is 409 RUBRIC_STALE and nothing is written, so reload and retry. criteria, when present, is the whole new set in order: criteria with an id keep their row, ids left out are removed, entries without an id are added. is_active false archives (no new assignments; existing references stay), true restores. Content changes are refused with 409 RUBRIC_LOCKED while questions use the rubric (copy it instead); archiving is always allowed.
// @Tags rubrics
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Rubric ID" format(uuid)
// @Param body body RubricUpdateRequest true "The fields to change"
// @Success 200 {object} response.Envelope{data=domain.Rubric}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /rubrics/{id} [patch]
func (h *RubricHandler) Update(c *gin.Context) {
	userID, role, ok := rubricUser(c)
	if !ok {
		return
	}
	id, ok := rubricID(c)
	if !ok {
		return
	}
	var request RubricUpdateRequest
	if !rubricJSON(c, &request, false) {
		return
	}
	patch := domain.RubricPatch{
		UpdatedAt: request.UpdatedAt, Name: request.Name, Description: request.Description,
		QuestionType: request.QuestionType, IsActive: request.IsActive,
	}
	if request.Criteria != nil {
		inputs := criterionInputs(*request.Criteria)
		patch.Criteria = &inputs
	}
	rubric, err := h.service.Update(c.Request.Context(), userID, role, id, patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, rubric)
}

// Delete godoc
// @Summary Delete a scoring rubric
// @Description Hard-deletes a rubric the caller manages, with its criteria. A rubric that questions still use is 409 RUBRIC_IN_USE: archive it instead (PATCH is_active false).
// @Tags rubrics
// @Security SessionCookie
// @Param id path string true "Rubric ID" format(uuid)
// @Success 204
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /rubrics/{id} [delete]
func (h *RubricHandler) Delete(c *gin.Context) {
	userID, role, ok := rubricUser(c)
	if !ok {
		return
	}
	id, ok := rubricID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, role, id); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
