package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type QuestionBankService interface {
	WriteQuestion(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, questionID *uuid.UUID, patch domain.QuestionPatch) (domain.BankQuestion, error)
	ApplyRubric(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, request domain.BulkRubricRequest) ([]domain.BankQuestion, error)
	List(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) (domain.Bank, error)
	Generate(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID) ([]domain.BankQuestion, error)
	Add(ctx context.Context, recruiterID uuid.UUID, role string, postingID uuid.UUID, content string) (domain.BankQuestion, error)
	Update(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID, content string) (domain.BankQuestion, error)
	Delete(ctx context.Context, recruiterID uuid.UUID, role string, postingID, questionID uuid.UUID) error
}
type QuestionBankHandler struct {
	service          QuestionBankService
	generationBudget time.Duration
}

func NewQuestionBankHandler(service QuestionBankService, generationBudget time.Duration) *QuestionBankHandler {
	return &QuestionBankHandler{service: service, generationBudget: generationBudget}
}

// QuestionRequest is the body of add and edit. Content is a pointer so a missing field differs from "".
type QuestionRequest struct {
	Content          *string         `json:"content"`
	RubricID         json.RawMessage `json:"rubric_id" swaggertype:"string" format:"uuid"`
	ExpectedAnswer   json.RawMessage `json:"expected_answer" swaggertype:"string"`
	PreviousRevision string          `json:"previous_revision,omitempty"`
}

// QuestionBankResponse wraps the list so an empty bank is `{"questions": []}`.
type QuestionBankResponse struct {
	Questions []domain.BankQuestion `json:"questions"`
}

func questionBankResponse(questions []domain.BankQuestion) QuestionBankResponse {
	if questions == nil {
		questions = []domain.BankQuestion{}
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

func questionBankPatch(c *gin.Context) (domain.QuestionPatch, bool) {
	var req QuestionRequest
	if !questionBankJSON(c, &req) {
		return domain.QuestionPatch{}, false
	}
	p := domain.QuestionPatch{Content: req.Content, SetRubric: req.RubricID != nil, SetExpectedAnswer: req.ExpectedAnswer != nil, PreviousRevision: req.PreviousRevision}
	if (p.SetRubric && json.Unmarshal(req.RubricID, &p.RubricID) != nil) || (p.SetExpectedAnswer && json.Unmarshal(req.ExpectedAnswer, &p.ExpectedAnswer) != nil) {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid grading fields"))
		return p, false
	}
	return p, true
}

// List godoc
// @Summary List the core-question bank of your own job posting
// @Description Recruiter only. Candidates never receive the bank. Each question carries its rubric_id, expected_answer (both null until set) and grading_incomplete, which is true until the question has both. Also returns whether the bank can still change (while a confirmed bank is pending or rejected) and the server limits.
// @Tags job-posting-questions
// @Produce json
// @Security SessionCookie
// @Param id path string true "Job posting ID"
// @Success 200 {object} response.Envelope{data=domain.Bank}
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
	bank, err := h.service.List(c.Request.Context(), userID, role, postingID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, bank)
}

// Generate godoc
// @Summary Generate the core-question bank of your own job posting
// @Description Recruiter only. Runs only while the bank is empty: a posting that already has questions returns 409 (a bank emptied by deleting every question can be generated again). Only while the posting is pending or rejected. Requires confirmed requirements. One generation call also picks, for each question, an allowed rubric (an active system template or one of your own active rubrics) and writes its expected answer; a question without a suitable rubric is saved with rubric_id null and grading_incomplete true. Generating never submits the posting for review. Trusted Origin required.
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
	cancel, ok := generationDeadline(c, h.generationBudget)
	if !ok {
		return
	}
	defer cancel()
	questions, err := h.service.Generate(c.Request.Context(), userID, role, postingID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, questionBankResponse(questions))
}

// Add godoc
// @Summary Add a question to the bank of your own job posting
// @Description Recruiter only, while the posting is pending or rejected and its current bank has been delivered. Trusted Origin required. Unknown fields reject the whole request.
// @Tags job-posting-questions
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Param request body QuestionRequest true "Fields to set (at least one on PATCH); null clears grading fields; changing rubric requires expected_answer; previous_revision detects stale edits; body limit 16 KiB"
// @Success 201 {object} response.Envelope{data=domain.BankQuestion}
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
	patch, ok := questionBankPatch(c)
	if !ok {
		return
	}
	question, err := h.service.WriteQuestion(c.Request.Context(), userID, role, postingID, nil, patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, question)
}

// Update godoc
// @Summary Edit question content, rubric or expected answer in your own bank
// @Description Recruiter only, while the posting is pending or rejected and its current bank has been delivered. Trusted Origin required. Unknown fields reject the whole request.
// @Tags job-posting-questions
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Param questionId path string true "Question ID"
// @Param request body QuestionRequest true "Fields to set (at least one on PATCH); null clears grading fields; changing rubric requires expected_answer; previous_revision detects stale edits; body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=domain.BankQuestion}
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
	patch, ok := questionBankPatch(c)
	if !ok {
		return
	}
	question, err := h.service.WriteQuestion(c.Request.Context(), userID, role, postingID, &questionID, patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, question)
}

// Delete godoc
// @Summary Delete a question from the bank of your own job posting
// @Description Recruiter only, while the posting is pending or rejected and its current bank has been delivered. A question already used by an interview cannot be deleted. Trusted Origin required.
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

// generationDeadline bounds a request that may call the LLM by budget: the configured timeout and
// retry budget plus time to save and respond. Call the returned cancel when ok.
func generationDeadline(c *gin.Context, budget time.Duration) (context.CancelFunc, bool) {
	if !roleWriteDeadline(c, budget) {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), budget)
	c.Request = c.Request.WithContext(ctx)
	return cancel, true
}

// ApplyRubric godoc
// @Summary Apply an active system or own rubric to selected questions atomically
// @Description Recruiter only. Pending/rejected posting only. Clears expected_answer only when the rubric changes, requiring a new review. Previous revisions are mandatory for every selected question. Foreign, missing and archived rubrics all return RUBRIC_NOT_FOUND. Trusted Origin required; body limit 16 KiB.
// @Tags job-posting-questions
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Job posting ID"
// @Param request body domain.BulkRubricRequest true "1 to 100 distinct question IDs and previous revisions"
// @Success 200 {object} response.Envelope{data=QuestionBankResponse}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /job-postings/{id}/questions/apply-rubric [post]
func (h *QuestionBankHandler) ApplyRubric(c *gin.Context) {
	userID, role, ok := questionBankUser(c)
	if !ok {
		return
	}
	postingID, ok := questionBankID(c, "id", "job posting")
	if !ok {
		return
	}
	var req domain.BulkRubricRequest
	if !questionBankJSON(c, &req) {
		return
	}
	questions, err := h.service.ApplyRubric(c.Request.Context(), userID, role, postingID, req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, questionBankResponse(questions))
}
