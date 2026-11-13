package handler

import (
	"context"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RoadmapReadService is the Roadmap read use case of the owning Candidate (KAN-352).
type RoadmapReadService interface {
	Get(ctx context.Context, userID uuid.UUID, role string, profileID uuid.UUID) (domain.RoadmapView, error)
	ItemHistory(ctx context.Context, userID uuid.UUID, role string, profileID, itemID uuid.UUID, limit, offset int32) ([]domain.HistoryEntry, int64, error)
}

type RoadmapHandler struct{ service RoadmapReadService }

func NewRoadmapHandler(service RoadmapReadService) *RoadmapHandler {
	return &RoadmapHandler{service: service}
}

// Get godoc
// @Summary Read the Learning Roadmap of an owned Role Profile
// @Description Candidate only, owner of the Role Profile. Foreign and missing Role Profiles both return JD_NOT_FOUND, exactly like GET /role-profiles/{id}. Returns generation_status (content preparation: pending, generating, ready, failed) separately from learning_status (not_started, in_progress, completed) and progress_percent (null unless generation_status is ready and there is at least one topic). Items are sorted by position, then id. Every score, count and status is computed from stored practice results on each read: score is the mean of the five most recent distinct core questions of the topic (provisional below five questions, null without results), a topic is achieved at five questions and a mean of at least 80 (compared before rounding), and a repeated question counts once with its latest result. practice_started tells whether regenerating is still possible. update_state is updating while evaluated practice results are not part of the Roadmap yet (pending_result_count says how many) and delayed when one has waited more than 10 minutes since its session ended. Practice results only: recruitment never feeds the Roadmap. Never returns bank questions, expected answers, rubrics or feedback text. Poll while generation_status is generating; see api/docs/roadmap-api.md.
// @Tags roadmap
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=domain.RoadmapView}
// @Failure 400,401,403,404,500 {object} response.Envelope
// @Router /role-profiles/{id}/roadmap [get]
func (h *RoadmapHandler) Get(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	view, err := h.service.Get(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, view)
}

// History godoc
// @Summary List the evaluated practice answers of a Roadmap topic
// @Description Candidate only, owner of the Role Profile. One page of the practice answers that feed the topic, newest first (evaluation time, then interview id, then question number). Each row is an answer that was evaluated: interview_id (open the practice report with GET /interviews/{interview_id}/report), question_number (the number the report uses), result (scored, or skipped for a question that was shown and not answered, which counts as 0), score, question_text (as the report shows it; null for a skipped question), evaluated_at and in_current_window (true for the five answers the topic score is computed from now; a repeated question's older answers and answers beyond the five most recent questions are false). Never returns expected answers, rubric details, feedback, hidden bank questions or recruitment data. A topic outside the profile's Roadmap returns ROADMAP_ITEM_NOT_FOUND.
// @Tags roadmap
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Param item_id path string true "Roadmap topic UUID"
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Items to skip" default(0) minimum(0)
// @Success 200 {object} response.Envelope{data=response.Page[domain.HistoryEntry]}
// @Failure 400,401,403,404,500 {object} response.Envelope
// @Router /role-profiles/{id}/roadmap/items/{item_id}/history [get]
func (h *RoadmapHandler) History(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "Invalid Roadmap topic ID."))
		return
	}
	limit, offset := int32(20), int32(0)
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &limit, 1, 100}, {"offset", &offset, 0, 2147483647}} {
		if raw, exists := c.GetQueryArray(param.name); exists {
			if len(raw) != 1 {
				response.Error(c, apperror.New(apperror.CodeValidation, "Invalid pagination."))
				return
			}
			value, err := strconv.ParseInt(raw[0], 10, 32)
			if err != nil || value < param.min || value > param.max {
				response.Error(c, apperror.New(apperror.CodeValidation, "Invalid pagination."))
				return
			}
			*param.value = int32(value)
		}
	}
	entries, total, err := h.service.ItemHistory(c.Request.Context(), user.ID, user.Role, id, itemID, limit, offset)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, response.Page[domain.HistoryEntry]{Items: entries, Pagination: response.Pagination{Limit: limit, Offset: offset, Total: total}})
}
