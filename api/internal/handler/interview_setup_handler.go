package handler

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InterviewSetupService is the pre-interview setup use case of the Candidate (KAN-231).
type InterviewSetupService interface {
	Practice(ctx context.Context, userID uuid.UUID, role string, profileID uuid.UUID) (domain.Setup, error)
	SavePractice(ctx context.Context, userID uuid.UUID, role string, profileID uuid.UUID, in domain.PracticeInput) (domain.Setup, error)
	Recruitment(ctx context.Context, userID uuid.UUID, role string, applicationID uuid.UUID) (domain.Setup, error)
	SaveRecruitment(ctx context.Context, userID uuid.UUID, role string, applicationID uuid.UUID, environment *string) (domain.Setup, error)
}

type InterviewSetupHandler struct{ service InterviewSetupService }

func NewInterviewSetupHandler(service InterviewSetupService) *InterviewSetupHandler {
	return &InterviewSetupHandler{service: service}
}

// PracticeSetupRequest replaces a practice setup. A missing or null field means "not chosen yet". Unknown
// fields are refused.
type PracticeSetupRequest struct {
	AvatarID    *uuid.UUID `json:"avatar_id" format:"uuid"`
	VoiceID     *uuid.UUID `json:"voice_id" format:"uuid"`
	Environment *string    `json:"environment" example:"office-3d" enums:"cafe-3d,office-3d,meeting-3d,office,balcony,cafe"`
}

// RecruitmentSetupRequest replaces the Candidate's part of a recruitment setup: the environment only. The
// interviewer avatar and voice are locked by the Recruiter, so a request that names them is refused.
type RecruitmentSetupRequest struct {
	Environment *string `json:"environment" example:"office-3d" enums:"cafe-3d,office-3d,meeting-3d,office,balcony,cafe"`
}

// InterviewSetupChoice is an avatar or a voice a setup points at.
type InterviewSetupChoice struct {
	ID   uuid.UUID `json:"id" format:"uuid"`
	Name string    `json:"name"`
}

// InterviewSetupBlocker is one reason a setup is not ready to start.
type InterviewSetupBlocker struct {
	Code  string `json:"code" example:"avatar_required"`
	Field string `json:"field,omitempty" example:"avatar"`
}

// InterviewSetupResponse is a setup with its live readiness. Never carries core questions, bank size or
// evaluation weights.
type InterviewSetupResponse struct {
	Mode          string     `json:"mode" enums:"practice,recruitment"`
	RoleProfileID *uuid.UUID `json:"role_profile_id,omitempty" format:"uuid"`
	ApplicationID *uuid.UUID `json:"application_id,omitempty" format:"uuid"`
	// TargetTitle is the Role Profile title (practice) or the posting title (recruitment).
	TargetTitle string                `json:"target_title"`
	Avatar      *InterviewSetupChoice `json:"avatar"`
	Voice       *InterviewSetupChoice `json:"voice"`
	Environment *string               `json:"environment"`
	// LockedFields are the fields the Candidate cannot change: avatar and voice for a recruitment interview.
	LockedFields      []string                `json:"locked_fields"`
	InterviewDeadline *time.Time              `json:"interview_deadline,omitempty"`
	Saved             bool                    `json:"saved"`
	UpdatedAt         *time.Time              `json:"updated_at,omitempty"`
	Ready             bool                    `json:"ready"`
	Blockers          []InterviewSetupBlocker `json:"blockers"`
}

func newInterviewSetupResponse(s domain.Setup) InterviewSetupResponse {
	blockers := make([]InterviewSetupBlocker, 0, len(s.Blockers))
	for _, b := range s.Blockers {
		blockers = append(blockers, InterviewSetupBlocker{Code: string(b.Code), Field: b.Field})
	}
	locked := s.LockedFields
	if locked == nil {
		locked = []string{}
	}
	return InterviewSetupResponse{
		Mode: string(s.Mode), RoleProfileID: s.RoleProfileID, ApplicationID: s.ApplicationID, TargetTitle: s.TargetTitle,
		Avatar: setupChoice(s.Avatar), Voice: setupChoice(s.Voice), Environment: s.Environment, LockedFields: locked,
		InterviewDeadline: s.InterviewDeadline, Saved: s.Saved, UpdatedAt: s.UpdatedAt, Ready: s.Ready, Blockers: blockers,
	}
}

func setupChoice(c *domain.Choice) *InterviewSetupChoice {
	if c == nil {
		return nil
	}
	return &InterviewSetupChoice{ID: c.ID, Name: c.Name}
}

// setupResource reads the signed-in user and the :id path parameter; what the id names is the route's.
func setupResource(c *gin.Context, what string) (userID uuid.UUID, role string, id uuid.UUID, ok bool) {
	user, ok := roleUser(c)
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "Invalid "+what+" ID."))
		return uuid.Nil, "", uuid.Nil, false
	}
	return user.ID, user.Role, id, true
}

func respondSetup(c *gin.Context, setup domain.Setup, err error) {
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, newInterviewSetupResponse(setup))
}

// GetPractice godoc
// @Summary Read the saved practice setup of a Role Profile and revalidate it
// @Description Candidate only, owner of the Role Profile (a foreign or missing one is JD_NOT_FOUND). Returns the saved avatar, voice and environment (null: not chosen yet) and recomputes the readiness from the data as it is now: ready is true exactly when blockers is empty. Blockers: role_profile_not_confirmed, question_bank_not_ready, avatar_required, avatar_unavailable, voice_required, voice_unavailable, environment_required, environment_unsupported. A stored choice that went stale (an avatar no longer usable, a voice deactivated) is reported and kept; a deleted avatar or voice is cleared by the database and shows as required. Never writes, debits coins, uses a recruitment slot or starts an interview. Never returns core questions, bank size or evaluation weights. See api/docs/interview-setup-api.md.
// @Tags interview-setups
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Success 200 {object} response.Envelope{data=InterviewSetupResponse}
// @Failure 400,401,403,404,500 {object} response.Envelope
// @Router /interview-setups/practice/{id} [get]
func (h *InterviewSetupHandler) GetPractice(c *gin.Context) {
	userID, role, id, ok := setupResource(c, "Role Profile")
	if !ok {
		return
	}
	setup, err := h.service.Practice(c.Request.Context(), userID, role, id)
	respondSetup(c, setup, err)
}

// PutPractice godoc
// @Summary Save the practice setup of a confirmed Role Profile
// @Description Candidate only. A full replacement (null or a missing field: not chosen yet, so a partial draft can be saved), idempotent: repeating it changes nothing. Refused with nothing saved: JD_NOT_FOUND (foreign or missing Role Profile), ROLE_PROFILE_REQUIRED (not confirmed), QUESTION_BANK_NOT_READY (no ready bank), AVATAR_NOT_FOUND (not in the Candidate's own inventory, deleted, or its model blocked), VOICE_NOT_FOUND (unknown or inactive voice), VALIDATION_ERROR (environment outside the catalog, unknown JSON field, bad UUID). Answers with the revalidated setup, like GET. Never debits coins, uses a recruitment slot or creates an interview: starting still goes through POST /interviews.
// @Tags interview-setups
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Role Profile UUID"
// @Param body body PracticeSetupRequest true "Setup"
// @Success 200 {object} response.Envelope{data=InterviewSetupResponse}
// @Failure 400,401,403,404,409,500 {object} response.Envelope
// @Router /interview-setups/practice/{id} [put]
func (h *InterviewSetupHandler) PutPractice(c *gin.Context) {
	userID, role, id, ok := setupResource(c, "Role Profile")
	if !ok {
		return
	}
	var request PracticeSetupRequest
	if !roleJSON(c, &request) {
		return
	}
	setup, err := h.service.SavePractice(c.Request.Context(), userID, role, id, domain.PracticeInput{
		AvatarID: request.AvatarID, VoiceID: request.VoiceID, Environment: request.Environment,
	})
	respondSetup(c, setup, err)
}

// GetRecruitment godoc
// @Summary Read the recruitment setup of an Application and revalidate it
// @Description Candidate only, owner of the Application (a foreign or missing one is APPLICATION_NOT_FOUND). Returns the avatar and voice the Recruiter locked on the posting (read-only, listed in locked_fields; id and name only, no model or grant), the Candidate's saved environment and the interview deadline, and recomputes the readiness now. Blockers: application_not_eligible, interview_deadline_passed, interview_already_started, locked_avatar_unavailable, locked_voice_unavailable, environment_required, environment_unsupported. Never writes, uses a recruitment slot or starts an interview. See api/docs/interview-setup-api.md.
// @Tags interview-setups
// @Produce json
// @Security SessionCookie
// @Param id path string true "Application UUID"
// @Success 200 {object} response.Envelope{data=InterviewSetupResponse}
// @Failure 400,401,403,404,500 {object} response.Envelope
// @Router /interview-setups/recruitment/{id} [get]
func (h *InterviewSetupHandler) GetRecruitment(c *gin.Context) {
	userID, role, id, ok := setupResource(c, "Application")
	if !ok {
		return
	}
	setup, err := h.service.Recruitment(c.Request.Context(), userID, role, id)
	respondSetup(c, setup, err)
}

// PutRecruitment godoc
// @Summary Save the environment of a recruitment setup
// @Description Candidate only. The environment is the only Candidate-editable field; the Recruiter-locked avatar and voice cannot be sent (unknown fields are VALIDATION_ERROR). A full replacement (null: not chosen yet), idempotent. Refused with nothing saved: APPLICATION_NOT_FOUND (foreign or missing), APPLICATION_NOT_ELIGIBLE (CV not approved, deadline passed, posting not accepting, or the interview already started), VALIDATION_ERROR (environment outside the catalog). Answers with the revalidated setup. Never uses a recruitment slot or creates an interview: the first start still goes through POST /interviews.
// @Tags interview-setups
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path string true "Application UUID"
// @Param body body RecruitmentSetupRequest true "Setup"
// @Success 200 {object} response.Envelope{data=InterviewSetupResponse}
// @Failure 400,401,403,404,409,500 {object} response.Envelope
// @Router /interview-setups/recruitment/{id} [put]
func (h *InterviewSetupHandler) PutRecruitment(c *gin.Context) {
	userID, role, id, ok := setupResource(c, "Application")
	if !ok {
		return
	}
	var request RecruitmentSetupRequest
	if !roleJSON(c, &request) {
		return
	}
	setup, err := h.service.SaveRecruitment(c.Request.Context(), userID, role, id, request.Environment)
	respondSetup(c, setup, err)
}
