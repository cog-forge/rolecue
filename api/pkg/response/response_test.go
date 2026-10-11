package response_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

func TestError(t *testing.T) {
	cause := errors.New("private password/database details")
	for _, tc := range []struct {
		name   string
		code   apperror.Code
		status int
	}{
		{"credentials", apperror.CodeInvalidCredentials, http.StatusUnauthorized},
		{"account missing", apperror.CodeAccountNotFound, http.StatusNotFound},
		{"account locked", apperror.CodeAccountLocked, http.StatusForbidden},
		{"token", apperror.CodeInvalidToken, http.StatusUnauthorized},
		{"validation", apperror.CodeValidation, http.StatusBadRequest},
		{"JD input", apperror.CodeInvalidJDInput, http.StatusBadRequest},
		{"JD short", apperror.CodeJDTooShort, http.StatusBadRequest},
		{"JD long", apperror.CodeJDTooLong, http.StatusBadRequest},
		{"JD missing", apperror.CodeJDNotFound, http.StatusNotFound},
		{"JD in use", apperror.CodeJDInUse, http.StatusConflict},
		{"extraction", apperror.CodeExtractionFailed, http.StatusBadGateway},
		{"invalid output", apperror.CodeInvalidExtractionOutput, http.StatusBadGateway},
		{"posting missing", apperror.CodeJobPostingNotFound, http.StatusNotFound},
		{"question missing", apperror.CodeCoreQuestionNotFound, http.StatusNotFound},
		{"bank exists", apperror.CodeQuestionBankExists, http.StatusConflict},
		{"bank locked", apperror.CodeQuestionBankLocked, http.StatusConflict},
		{"question in use", apperror.CodeCoreQuestionInUse, http.StatusConflict},
		{"requirements unconfirmed", apperror.CodeRequirementsNotConfirmed, http.StatusConflict},
		{"question generation", apperror.CodeQuestionGenerationFailed, http.StatusBadGateway},
		{"invalid question output", apperror.CodeInvalidQuestionGenerationOutput, http.StatusBadGateway},
		{"question bank unavailable", apperror.CodeQuestionBankUnavailable, http.StatusServiceUnavailable},
		{"internal", apperror.CodeInternal, http.StatusInternalServerError},
		{"unknown code", apperror.Code("FUTURE_CODE"), http.StatusInternalServerError},
	} {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wrapped=%t", tc.name, wrapped), func(t *testing.T) {
				var err error = apperror.Wrap(tc.code, "public message", cause)
				if wrapped {
					err = fmt.Errorf("private service context: %w", err)
				}
				assertResponse(t, err, tc.status, tc.code, "public message")
			})
		}
	}
	for name, err := range map[string]error{
		"plain cause": cause,
		"nil":         nil,
		"typed nil":   (*apperror.AppError)(nil),
	} {
		t.Run(name, func(t *testing.T) {
			assertResponse(t, err, http.StatusInternalServerError, apperror.CodeInternal, "something wrong happen")
		})
	}
	t.Run("constructor without cause", func(t *testing.T) {
		assertResponse(t, apperror.New(apperror.CodeValidation, "invalid input"), http.StatusBadRequest, apperror.CodeValidation, "invalid input")
	})
}

func assertResponse(t *testing.T, err error, status int, code apperror.Code, message string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	response.Error(ctx, err)
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, status, recorder.Body.String())
	}
	var body response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Success || body.Data != nil || body.Error == nil || body.Error.Code != code || body.Error.Message != message {
		t.Fatalf("unexpected envelope: %+v; body = %s", body, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "database") {
		t.Fatalf("internal cause leaked: %s", recorder.Body.String())
	}
}
