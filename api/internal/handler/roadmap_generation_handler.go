package handler

import (
	"context"
	"time"

	roadmapdomain "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RoadmapGeneration is the part of the Roadmap generation service the Role Profile routes call.
type RoadmapGeneration interface {
	Retry(ctx context.Context, candidate uuid.UUID, role string, profile uuid.UUID) (roadmapdomain.Roadmap, error)
	Regenerate(ctx context.Context, candidate uuid.UUID, role string, profile uuid.UUID) (roadmapdomain.Roadmap, error)
}

// RoadmapGenerationResult is what a finished retry or regeneration returns: which Roadmap, and its status.
// The topics themselves are read with the Roadmap read API; nothing about the question bank is returned.
type RoadmapGenerationResult struct {
	RoadmapID        uuid.UUID `json:"roadmap_id" format:"uuid"`
	RoleProfileID    uuid.UUID `json:"role_profile_id" format:"uuid"`
	GenerationStatus string    `json:"generation_status" enums:"pending,generating,ready,failed"`
}

// WithRoadmap adds the Roadmap generation routes to the handler.
func (h *RoleProfileHandler) WithRoadmap(generation RoadmapGeneration) *RoleProfileHandler {
	h.roadmap = generation
	return h
}

// roadmapWriteDeadline is the response window of every request that generates a Roadmap (confirm, retry,
// regenerate). A generation is two sequential model steps (the topics, then the bank), each bounded by the
// 90s question_bank route, so the worst case is about 180s plus the transaction; 200s leaves room for it.
const roadmapWriteDeadline = 200 * time.Second

func (h *RoleProfileHandler) roadmapResult(c *gin.Context, roadmap roadmapdomain.Roadmap, err error) {
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, RoadmapGenerationResult{RoadmapID: roadmap.ID, RoleProfileID: roadmap.RoleProfileID, GenerationStatus: string(roadmap.GenerationStatus)})
}

// RetryRoadmap godoc
// @Summary Retry generating the learning roadmap of a confirmed Role Profile
// @Description Generates the topics and the practice question bank when the first generation did not happen or
// @Description failed. Synchronous: the response is the finished state. Idempotent: a roadmap that is already
// @Description ready is returned unchanged, and concurrent retries start one generation (the others get 409
// @Description ROADMAP_GENERATING). Never resets a ready roadmap.
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=RoadmapGenerationResult}
// @Failure 400,401,403,404,409,429,500,502 {object} response.Envelope
// @Router /role-profiles/{id}/roadmap/retry [post]
func (h *RoleProfileHandler) RetryRoadmap(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	if h.roadmap == nil {
		response.Error(c, apperror.New(apperror.CodeInternal, "Roadmap generation is not available."))
		return
	}
	if !roleWriteDeadline(c, roadmapWriteDeadline) {
		return
	}
	roadmap, err := h.roadmap.Retry(c.Request.Context(), user.ID, user.Role, id)
	h.roadmapResult(c, roadmap, err)
}

// RegenerateRoadmap godoc
// @Summary Regenerate the learning roadmap before the first practice session
// @Description Replaces the topics and the question bank of a confirmed Role Profile that has no interview at
// @Description all (a finished or abandoned session counts). The roadmap keeps its id. Synchronous and atomic:
// @Description the old topics and bank stay until the new ones are valid and are replaced in one transaction;
// @Description a failure leaves them in place. Regeneration and a practice start exclude each other.
// @Tags role-profiles
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=RoadmapGenerationResult}
// @Failure 400,401,403,404,409,429,500,502 {object} response.Envelope
// @Router /role-profiles/{id}/roadmap/regenerate [post]
func (h *RoleProfileHandler) RegenerateRoadmap(c *gin.Context) {
	user, id, ok := roleResource(c)
	if !ok {
		return
	}
	if h.roadmap == nil {
		response.Error(c, apperror.New(apperror.CodeInternal, "Roadmap generation is not available."))
		return
	}
	if !roleWriteDeadline(c, roadmapWriteDeadline) {
		return
	}
	roadmap, err := h.roadmap.Regenerate(c.Request.Context(), user.ID, user.Role, id)
	h.roadmapResult(c, roadmap, err)
}
