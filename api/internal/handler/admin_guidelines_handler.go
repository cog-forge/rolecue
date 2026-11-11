package handler

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/domain"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AdminGuidelinesService interface {
	Get(ctx context.Context, role string) (domain.Settings, error)
	Save(ctx context.Context, adminID uuid.UUID, role string, expectedVersion int32, values interviewdomain.BehaviorGuideline) (domain.Settings, error)
}

type AdminGuidelinesHandler struct{ service AdminGuidelinesService }

func NewAdminGuidelinesHandler(service AdminGuidelinesService) *AdminGuidelinesHandler {
	return &AdminGuidelinesHandler{service: service}
}

// GuidelinesAllowedView lists what each guideline field accepts.
type GuidelinesAllowedView struct {
	Tone           []string `json:"tone"`
	QuestionLength []string `json:"question_length"`
	FollowUpMax    []int32  `json:"follow_up_max"`
	FollowUpFocus  []string `json:"follow_up_focus"`
	AvoidTopics    []string `json:"avoid_topics"`
}

// GuidelinesView is the Admin response: the current values (version 0 and the defaults when nothing was
// saved), the defaults, the allowed values and the configured follow-up ceiling.
type GuidelinesView struct {
	Version                  int32                             `json:"version"`
	Values                   interviewdomain.BehaviorGuideline `json:"values"`
	Defaults                 interviewdomain.BehaviorGuideline `json:"defaults"`
	Allowed                  GuidelinesAllowedView             `json:"allowed"`
	EffectiveFollowUpCeiling int32                             `json:"effective_follow_up_ceiling"`
	UpdatedAt                *time.Time                        `json:"updated_at"`
	UpdatedByName            string                            `json:"updated_by_name"`
}

func guidelinesView(s domain.Settings) GuidelinesView {
	return GuidelinesView{
		Version: s.Version, Values: s.Values, Defaults: s.Defaults, EffectiveFollowUpCeiling: s.EffectiveFollowUpCeiling,
		UpdatedAt: s.UpdatedAt, UpdatedByName: s.UpdatedByName,
		Allowed: GuidelinesAllowedView{
			Tone: s.Allowed.Tone, QuestionLength: s.Allowed.QuestionLength, FollowUpMax: s.Allowed.FollowUpMax,
			FollowUpFocus: s.Allowed.FollowUpFocus, AvoidTopics: s.Allowed.AvoidTopics,
		},
	}
}

// SaveGuidelinesRequest is the body of the save. Both fields are pointers so a missing one differs from a
// zero value; a missing guideline field is refused by name, except follow_up_max and avoid_topics where
// null or absent means "use the configured limit" and "none".
type SaveGuidelinesRequest struct {
	ExpectedVersion *int32                             `json:"expected_version"`
	Values          *interviewdomain.BehaviorGuideline `json:"values"`
}

// Get godoc
// @Summary Get the Interview Behavior Guidelines
// @Description Admin only. Returns the current guideline values, the defaults, the allowed values of every field and the configured follow-up ceiling that follow_up_max can only lower. Version 0 with the defaults means nothing was saved yet. The guidelines shape how the live interviewer words and decides follow-ups for sessions that start after a save; running and completed interviews are not affected.
// @Tags admin-interview-guidelines
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=GuidelinesView}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/interview-guidelines [get]
func (h *AdminGuidelinesHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	result, err := h.service.Get(c.Request.Context(), user.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, guidelinesView(result))
}

// Put godoc
// @Summary Save the Interview Behavior Guidelines
// @Description Admin only. Trusted Origin required. Appends the next version (versions are never rewritten or deleted, and the saving Administrator is recorded). expected_version is the version the Admin loaded, 0 before the first save; any other current version is 409 GUIDELINE_VERSION_CONFLICT and nothing is written. Every field is an enum, a bounded integer or a subset of a closed catalog; there is no free text. follow_up_max is null (use the configured limit) or 0 to 3 and can only lower the configured interview.follow_up_limit. The new version applies to interviews that start after the save; running and completed interviews keep the version they started under. Unknown fields reject the whole request. Body limit 16 KiB.
// @Tags admin-interview-guidelines
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body SaveGuidelinesRequest true "Guideline values and the version they replace"
// @Success 200 {object} response.Envelope{data=GuidelinesView}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/interview-guidelines [put]
func (h *AdminGuidelinesHandler) Put(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	var req SaveGuidelinesRequest
	if !moderationJSON(c, &req) {
		return
	}
	if req.ExpectedVersion == nil || req.Values == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "expected_version and values are required"))
		return
	}
	result, err := h.service.Save(c.Request.Context(), user.ID, user.Role, *req.ExpectedVersion, *req.Values)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, guidelinesView(result))
}
