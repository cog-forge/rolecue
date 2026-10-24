package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const evaluationBodyLimit = 16 << 10

type EvaluationService interface {
	Metrics(ctx context.Context) ([]domain.Metric, error)
	Weights(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.PostingWeights, error)
	ReplaceWeights(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, inputs []domain.WeightInput) (domain.PostingWeights, error)
	ResetWeights(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.PostingWeights, error)
}
type EvaluationHandler struct{ service EvaluationService }

func NewEvaluationHandler(service EvaluationService) *EvaluationHandler {
	return &EvaluationHandler{service: service}
}

// EvaluationWeightsRequest replaces a posting's whole weight set.
type EvaluationWeightsRequest struct {
	// Weights is the full set; at least one entry. Metrics left out are not weighted.
	Weights *[]EvaluationWeightEntry `json:"weights" binding:"required"`
}

type EvaluationWeightEntry struct {
	MetricID   *uuid.UUID `json:"metric_id" binding:"required" format:"uuid"`
	Percentage *float64   `json:"percentage" binding:"required"`
}

func evaluationUser(c *gin.Context) (uuid.UUID, string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", false
	}
	return user.ID, user.Role, true
}

func evaluationPostingID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return uuid.Nil, false
	}
	return id, true
}

func evaluationJSON(c *gin.Context, target any) bool {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, evaluationBodyLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}

// Metrics godoc
// @Summary List evaluation metrics
// @Description The evaluation dimensions a Recruiter can weight on a Job Posting, from the seeded metrics catalog.
// @Tags evaluation
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=domain.MetricCatalog}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /evaluation-metrics [get]
func (h *EvaluationHandler) Metrics(c *gin.Context) {
	if _, _, ok := evaluationUser(c); !ok {
		return
	}
	metrics, err := h.service.Metrics(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, domain.MetricCatalog{Items: metrics})
}

// Weights godoc
// @Summary View a Job Posting's evaluation weights
// @Description Recruiter only, own postings. When the Recruiter set no weights, configured is false, using_defaults is true and weights holds the defaults the posting will be scored with (the total split equally across all metrics); show the Recruiter a warning. Defaults are never saved. total is the sum the weights must reach (0 = no sum rule).
// @Tags evaluation
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.PostingWeights}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/evaluation-weights [get]
func (h *EvaluationHandler) Weights(c *gin.Context) {
	userID, role, ok := evaluationUser(c)
	if !ok {
		return
	}
	id, ok := evaluationPostingID(c)
	if !ok {
		return
	}
	weights, err := h.service.Weights(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, weights)
}

// ReplaceWeights godoc
// @Summary Replace a Job Posting's evaluation weights
// @Description Recruiter only, own postings that are pending or rejected. The list replaces the whole set in one write. Errors: EVALUATION_METRIC_UNKNOWN, EVALUATION_METRIC_DUPLICATE, EVALUATION_WEIGHT_INVALID (negative or above the total), EVALUATION_WEIGHT_TOTAL_MISMATCH, EVALUATION_WEIGHTS_LOCKED (posting open, intake_closed or closed).
// @Tags evaluation
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Param body body EvaluationWeightsRequest true "The full weight set"
// @Success 200 {object} response.Envelope{data=domain.PostingWeights}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/evaluation-weights [put]
func (h *EvaluationHandler) ReplaceWeights(c *gin.Context) {
	userID, role, ok := evaluationUser(c)
	if !ok {
		return
	}
	id, ok := evaluationPostingID(c)
	if !ok {
		return
	}
	var request EvaluationWeightsRequest
	if !evaluationJSON(c, &request) {
		return
	}
	if request.Weights == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "weights is required"))
		return
	}
	inputs := make([]domain.WeightInput, 0, len(*request.Weights))
	for _, entry := range *request.Weights {
		inputs = append(inputs, domain.WeightInput{MetricID: entry.MetricID, Percentage: entry.Percentage})
	}
	weights, err := h.service.ReplaceWeights(c.Request.Context(), userID, role, id, inputs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, weights)
}

// ResetWeights godoc
// @Summary Reset a Job Posting's evaluation weights to the defaults
// @Description Recruiter only, own postings that are pending or rejected. Deletes the saved weights, so the posting is scored with the defaults again (the total split equally across all metrics); the response is the same view as GET with configured false and using_defaults true. Resetting a posting that already uses the defaults changes nothing. Errors: EVALUATION_WEIGHTS_LOCKED (posting open, intake_closed or closed).
// @Tags evaluation
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job Posting ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.PostingWeights}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/evaluation-weights [delete]
func (h *EvaluationHandler) ResetWeights(c *gin.Context) {
	userID, role, ok := evaluationUser(c)
	if !ok {
		return
	}
	id, ok := evaluationPostingID(c)
	if !ok {
		return
	}
	weights, err := h.service.ResetWeights(c.Request.Context(), userID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, weights)
}
