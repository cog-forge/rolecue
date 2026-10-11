package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type QuestionBankService interface {
	List(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) ([]domain.CoreQuestion, error)
	Generate(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) ([]domain.CoreQuestion, error)
	Add(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, content string) (domain.CoreQuestion, error)
	Update(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID, content string) (domain.CoreQuestion, error)
	Delete(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID) error
}
type QuestionBankHandler struct{ service QuestionBankService }

func NewQuestionBankHandler(service QuestionBankService) *QuestionBankHandler {
	return &QuestionBankHandler{service: service}
}

// QuestionRequest is the body of add and edit. Content is a pointer so a missing field differs from "".
type QuestionRequest struct {
	Content *string `json:"content" binding:"required"`
}

// QuestionBankResponse wraps the list so an empty bank is `{"questions": []}`.
type QuestionBankResponse struct {
	Questions []domain.CoreQuestion `json:"questions"`
}

func questionBankResponse(questions []domain.CoreQuestion) QuestionBankResponse {
	if questions == nil {
		questions = []domain.CoreQuestion{}
	}
	return QuestionBankResponse{Questions: questions}
}

func questionBankUser(c *gin.Context) (uuid.UUID, string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", false
	}
	return user.ID, user.Role, true
}

func questionBankID(c *gin.Context, param, thing string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid "+thing+" ID"))
		return uuid.Nil, false
	}
	return id, true
}

func questionBankJSON(c *gin.Context, target any) bool {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(target) != nil || decoder.Decode(&extra) != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return false
	}
	return true
}

func questionBankContent(c *gin.Context) (string, bool) {
	var req QuestionRequest
	if !questionBankJSON(c, &req) {
		return "", false
	}
	if req.Content == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return "", false
	}
	return *req.Content, true
}

// List godoc
// @Summary List the core-question bank of your own job posting
// @Description Recruiter only. Candidates never receive the bank.
// @Tags job-posting-questions
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job posting ID"
// @Success 200 {object} response.Envelope{data=QuestionBankResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/questions [get]
func (h *QuestionBankHandler) List(c *gin.Context) {
	userID, role, ok := questionBankUser(c)
	if !ok {
		return
	}
	postingID, ok := questionBankID(c, "id", "job posting")
	if !ok {
		return
	}
	questions, err := h.service.List(c.Request.Context(), userID, role, postingID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, questionBankResponse(questions))
}

// Generate godoc
// @Summary Generate the core-question bank of your own job posting
// @Description Recruiter only. Runs only while the bank is empty: a posting that already has questions returns 409 (a bank emptied by deleting every question can be generated again). Only while the posting is pending. Requires confirmed requirements. Trusted Origin required.
// @Tags job-posting-questions
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Success 201 {object} response.Envelope{data=QuestionBankResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 502 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/questions/generate [post]
func (h *QuestionBankHandler) Generate(c *gin.Context) {
	userID, role, ok := questionBankUser(c)
	if !ok {
		return
	}
	postingID, ok := questionBankID(c, "id", "job posting")
	if !ok {
		return
	}
	questions, err := h.service.Generate(c.Request.Context(), userID, role, postingID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, questionBankResponse(questions))
}

// Add godoc
// @Summary Add a question to the bank of your own job posting
// @Description Recruiter only, while the posting is pending. Trusted Origin required. Unknown fields reject the whole request.
// @Tags job-posting-questions
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Param request body QuestionRequest true "Question; body limit 16 KiB"
// @Success 201 {object} response.Envelope{data=domain.CoreQuestion}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/questions [post]
func (h *QuestionBankHandler) Add(c *gin.Context) {
	userID, role, ok := questionBankUser(c)
	if !ok {
		return
	}
	postingID, ok := questionBankID(c, "id", "job posting")
	if !ok {
		return
	}
	content, ok := questionBankContent(c)
	if !ok {
		return
	}
	question, err := h.service.Add(c.Request.Context(), userID, role, postingID, content)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, question)
}

// Update godoc
// @Summary Edit a question in the bank of your own job posting
// @Description Recruiter only, while the posting is pending. Trusted Origin required. Unknown fields reject the whole request.
// @Tags job-posting-questions
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Param questionId path string true "Question ID"
// @Param request body QuestionRequest true "Question; body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=domain.CoreQuestion}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/questions/{questionId} [patch]
func (h *QuestionBankHandler) Update(c *gin.Context) {
	userID, role, ok := questionBankUser(c)
	if !ok {
		return
	}
	postingID, ok := questionBankID(c, "id", "job posting")
	if !ok {
		return
	}
	questionID, ok := questionBankID(c, "questionId", "question")
	if !ok {
		return
	}
	content, ok := questionBankContent(c)
	if !ok {
		return
	}
	question, err := h.service.Update(c.Request.Context(), userID, role, postingID, questionID, content)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, question)
}

// Delete godoc
// @Summary Delete a question from the bank of your own job posting
// @Description Recruiter only, while the posting is pending. A question already used by an interview cannot be deleted. Trusted Origin required.
// @Tags job-posting-questions
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Param questionId path string true "Question ID"
// @Success 204
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /job-postings/{id}/questions/{questionId} [delete]
func (h *QuestionBankHandler) Delete(c *gin.Context) {
	userID, role, ok := questionBankUser(c)
	if !ok {
		return
	}
	postingID, ok := questionBankID(c, "id", "job posting")
	if !ok {
		return
	}
	questionID, ok := questionBankID(c, "questionId", "question")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, role, postingID, questionID); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
