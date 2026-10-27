package handler

import (
	"context"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InterviewRecordingService is the recording and evidence use case of recruitment interviews. The
// interview recording service implements it.
type InterviewRecordingService interface {
	GrantUpload(ctx context.Context, candidateID uuid.UUID, role string, interviewID uuid.UUID, input domain.RecordingUploadInput) (domain.RecordingUploadGrant, error)
	CompleteUpload(ctx context.Context, candidateID uuid.UUID, role string, interviewID uuid.UUID) (domain.RecordingStatus, error)
	Recording(ctx context.Context, recruiterID uuid.UUID, role string, jobPostingID, applicationID uuid.UUID) (domain.RecruiterRecording, error)
	Transcript(ctx context.Context, recruiterID uuid.UUID, role string, jobPostingID, applicationID uuid.UUID) (domain.Transcript, error)
}

type InterviewRecordingHandler struct{ service InterviewRecordingService }

func NewInterviewRecordingHandler(service InterviewRecordingService) *InterviewRecordingHandler {
	return &InterviewRecordingHandler{service: service}
}

// CreateRecordingRequest declares the recording before upload. content_type and size are required; size is
// the exact byte count the upload grant binds.
type CreateRecordingRequest struct {
	// ContentType is the media type of the recording.
	ContentType *string `json:"content_type" enums:"video/webm,audio/webm,video/mp4" example:"video/webm"`
	// Size is the byte size of the recording, 1 to 524288000.
	Size *int64 `json:"size" example:"1048576"`
}

// CreateUpload godoc
// @Summary Get an upload grant for your recruitment recording
// @Description Candidate only, owner of the recruitment interview. Returns a one-time upload to send the recording to, valid 15 minutes, with the headers to send. The grant carries no storage key and no read link. A practice interview is RECORDING_NOT_ALLOWED. A recording that is processing or ready cannot be replaced (RECORDING_NOT_ALLOWED). Errors: VALIDATION_ERROR, FORBIDDEN, INTERVIEW_NOT_FOUND, RECORDING_NOT_ALLOWED.
// @Tags interviews
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Interview session ID" format(uuid)
// @Param body body CreateRecordingRequest true "Recording to upload; body limit 16 KiB"
// @Success 201 {object} response.Envelope{data=domain.RecordingUploadGrant}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id}/recording [post]
func (h *InterviewRecordingHandler) CreateUpload(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	var request CreateRecordingRequest
	if !interviewJSON(c, &request) {
		return
	}
	if request.ContentType == nil || request.Size == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	grant, err := h.service.GrantUpload(c.Request.Context(), user.ID, user.Role, id, domain.RecordingUploadInput{
		ContentType: *request.ContentType, Size: *request.Size,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, grant)
}

// CompleteUpload godoc
// @Summary Confirm your recruitment recording upload
// @Description Candidate only, owner of the recruitment interview. Checks that the uploaded object is present and within the size limit. Present sets the recording ready; absent sets it failed and returns RECORDING_NOT_FOUND. Ready again returns ready. Errors: FORBIDDEN, INTERVIEW_NOT_FOUND, RECORDING_NOT_ALLOWED, RECORDING_NOT_FOUND, VALIDATION_ERROR.
// @Tags interviews
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Interview session ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.RecordingStatus}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /interviews/{id}/recording/complete [post]
func (h *InterviewRecordingHandler) CompleteUpload(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	status, err := h.service.CompleteUpload(c.Request.Context(), user.ID, user.Role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, status)
}

// Recording godoc
// @Summary Read the recording of an application you are reviewing
// @Description Recruiter only, owner of the posting. url and expires_at are set only when the recording is ready; the link is valid 5 minutes and is issued on each read. Any other caller, Admin included, and a missing Application get APPLICATION_NOT_FOUND. An Application without an interview gets INTERVIEW_NOT_FOUND.
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job posting ID" format(uuid)
// @Param applicationId path string true "Application ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.RecruiterRecording}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/applications/{applicationId}/recording [get]
func (h *InterviewRecordingHandler) Recording(c *gin.Context) {
	user, postingID, applicationID, ok := recordingTarget(c)
	if !ok {
		return
	}
	recording, err := h.service.Recording(c.Request.Context(), user.ID, user.Role, postingID, applicationID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, recording)
}

// Transcript godoc
// @Summary Read the transcript of a completed application interview
// @Description Recruiter only, owner of the posting. Returns the questions and answers of a completed recruitment interview. An interview that is not completed is INTERVIEW_NOT_COMPLETED. Any other caller, Admin included, and a missing Application get APPLICATION_NOT_FOUND.
// @Tags job-postings
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job posting ID" format(uuid)
// @Param applicationId path string true "Application ID" format(uuid)
// @Success 200 {object} response.Envelope{data=domain.Transcript}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/applications/{applicationId}/transcript [get]
func (h *InterviewRecordingHandler) Transcript(c *gin.Context) {
	user, postingID, applicationID, ok := recordingTarget(c)
	if !ok {
		return
	}
	transcript, err := h.service.Transcript(c.Request.Context(), user.ID, user.Role, postingID, applicationID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, transcript)
}

// recordingTarget reads the caller and the posting and Application IDs of a Recruiter route.
func recordingTarget(c *gin.Context) (authdomain.User, uuid.UUID, uuid.UUID, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return authdomain.User{}, uuid.Nil, uuid.Nil, false
	}
	postingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid job posting ID"))
		return authdomain.User{}, uuid.Nil, uuid.Nil, false
	}
	applicationID, err := uuid.Parse(c.Param("applicationId"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid application ID"))
		return authdomain.User{}, uuid.Nil, uuid.Nil, false
	}
	return user, postingID, applicationID, true
}
