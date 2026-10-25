package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// interviewBodyLimit caps a session request body. The largest body is one answer of 8000 characters.
const interviewBodyLimit = 16 << 10

// InterviewService is the Candidate's interview session use case. The interview service implements it.
type InterviewService interface {
	Create(ctx context.Context, candidateID uuid.UUID, role string, input domain.CreateInput) (domain.Session, bool, error)
	Get(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error)
	Resume(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error)
	AppendTurn(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID, position int32, answer string) (domain.Session, error)
	Complete(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error)
}

type InterviewHandler struct{ service InterviewService }

func NewInterviewHandler(service InterviewService) *InterviewHandler {
	return &InterviewHandler{service: service}
}

// maxIdempotencyKeyLength bounds the Idempotency-Key header of a session start.
const maxIdempotencyKeyLength = 128

// CreateInterviewRequest starts a session. type is required. role_profile_id is required for practice,
// application_id for recruitment. avatar_id and voice_id are optional.
type CreateInterviewRequest struct {
	// Type is the session kind.
	Type string `json:"type" enums:"practice,recruitment" example:"practice"`
	// RoleProfileID is the role profile to practise with. For recruitment it is optional.
	RoleProfileID *uuid.UUID `json:"role_profile_id,omitempty"`
	// ApplicationID is the Application a recruitment session is for.
	ApplicationID *uuid.UUID `json:"application_id,omitempty"`
	AvatarID      *uuid.UUID `json:"avatar_id,omitempty"`
	VoiceID       *uuid.UUID `json:"voice_id,omitempty"`
}

// AppendTurnRequest is the answer to one question. position is the 1-based number of the question.
type AppendTurnRequest struct {
	Position *int32  `json:"position" example:"1"`
	Answer   *string `json:"answer" example:"I led a team of four on a payments service."`
}

// Create godoc
// @Summary Start an interview session
// @Description Candidate only. Creates a session, or returns the existing recruitment session of the Application (200, no side effects). A practice session needs a confirmed role profile of yours, charges the practice price once and asks the first questions of its bank. A recruitment session needs an interview_eligible Application inside its 24-hour deadline, a funded posting with a free interview slot, and a question bank; it is free. Errors: VALIDATION_ERROR (bad type, missing or unknown reference), FORBIDDEN (not a Candidate), JOB_POSTING_NOT_FOUND, APPLICATION_NOT_FOUND (not yours or missing), APPLICATION_NOT_ELIGIBLE, INTERVIEW_CAPACITY_NOT_FUNDED, INTERVIEW_CAPACITY_FULL, ROLE_PROFILE_REQUIRED, JD_NOT_FOUND (role profile missing or not yours), QUESTION_BANK_NOT_READY, INSUFFICIENT_BALANCE.
// @Tags interviews
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param Idempotency-Key header string false "Optional. A practice start repeated with the same key returns the first session and charges once; up to 128 characters"
// @Param body body CreateInterviewRequest true "Session to start; body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=domain.Session}
// @Success 201 {object} response.Envelope{data=domain.Session}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews [post]
func (h *InterviewHandler) Create(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	var request CreateInterviewRequest
	if !interviewJSON(c, &request) {
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) > maxIdempotencyKeyLength {
		response.Error(c, apperror.New(apperror.CodeValidation, "Idempotency-Key is too long"))
		return
	}
	session, created, err := h.service.Create(c.Request.Context(), user.ID, user.Role, domain.CreateInput{
		Type: domain.Type(request.Type), RoleProfileID: request.RoleProfileID, ApplicationID: request.ApplicationID,
		AvatarID: request.AvatarID, VoiceID: request.VoiceID, IdempotencyKey: key,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	if created {
		response.Created(c, session)
		return
	}
	response.OK(c, session)
}

// Get godoc
// @Summary Read one of your interview sessions
// @Description Candidate only, your own sessions. Returns the session with its question count, answered count and current question (null when every question is answered or the session is completed). Practice sessions also return their answered turns; recruitment sessions never do. Another Candidate's session is not found.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param id path string true "Interview session ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Session}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id} [get]
func (h *InterviewHandler) Get(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	session, err := h.service.Get(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, session)
}

// Resume godoc
// @Summary Resume an interview session
// @Description Candidate only. Returns the same session with no side effects and no charge. A completed session is INTERVIEW_COMPLETED.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Interview session ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Session}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id}/resume [post]
func (h *InterviewHandler) Resume(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	session, err := h.service.Resume(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, session)
}

// AppendTurn godoc
// @Summary Answer the next question of an interview session
// @Description Candidate only. position must be the next unanswered question (1-based). Sending the same position with the same answer again returns the session unchanged, so a retry is safe. Another answer for a position already answered, or a position that is not next, is INTERVIEW_TURN_CONFLICT. The answer is trimmed and must be 1 to 8000 characters. A completed session is INTERVIEW_COMPLETED. Errors: VALIDATION_ERROR, FORBIDDEN, INTERVIEW_NOT_FOUND, INTERVIEW_COMPLETED, INTERVIEW_TURN_CONFLICT.
// @Tags interviews
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Interview session ID" format(uuid)
// @Param body body AppendTurnRequest true "Answer to the next question; body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=domain.Session}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id}/turns [post]
func (h *InterviewHandler) AppendTurn(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	var request AppendTurnRequest
	if !interviewJSON(c, &request) {
		return
	}
	if request.Position == nil || request.Answer == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	session, err := h.service.AppendTurn(c.Request.Context(), user.ID, user.Role, id, *request.Position, *request.Answer)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, session)
}

// Complete godoc
// @Summary Complete an interview session
// @Description Candidate only. Idempotent: the first call sets completed_at, and later calls return the session as it is. A recruitment session also moves its Application to interviewed. The evaluation starts in the background after the first completion.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Interview session ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Session}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id}/complete [post]
func (h *InterviewHandler) Complete(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	session, err := h.service.Complete(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, session)
}

// interviewTarget reads the Candidate and the session ID of a session route.
func interviewTarget(c *gin.Context) (authdomain.User, uuid.UUID, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return authdomain.User{}, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid interview ID"))
		return authdomain.User{}, uuid.Nil, false
	}
	return user, id, true
}

// interviewJSON decodes one JSON object from the body. Unknown fields and trailing values are rejected,
// and a decode error is never echoed to the client.
func interviewJSON(c *gin.Context, target any) bool {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, interviewBodyLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}
