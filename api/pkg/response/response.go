package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    apperror.Code `json:"code"`
	Message string        `json:"message"`
}

var httpStatusByCode = map[apperror.Code]int{
	apperror.CodeInvalidPDF:                      http.StatusBadRequest,
	apperror.CodePDFTooLarge:                     http.StatusRequestEntityTooLarge,
	apperror.CodePDFUnavailable:                  http.StatusServiceUnavailable,
	apperror.CodeDocumentStorage:                 http.StatusBadGateway,
	apperror.CodeRoleConflict:                    http.StatusConflict,
	apperror.CodeInventoryFull:                   http.StatusConflict,
	apperror.CodeAvatarSourceTooLarge:            http.StatusRequestEntityTooLarge,
	apperror.CodeAvatarSourceUnsupported:         http.StatusUnprocessableEntity,
	apperror.CodeAvatarConverterUnavailable:      http.StatusServiceUnavailable,
	apperror.CodeAvatarConversionFailed:          http.StatusBadGateway,
	apperror.CodeAvatarConversionTimeout:         http.StatusGatewayTimeout,
	apperror.CodeAvatarPersistenceFailed:         http.StatusServiceUnavailable,
	apperror.CodeAvatarNotFound:                  http.StatusNotFound,
	apperror.CodeAvatarInUse:                     http.StatusConflict,
	apperror.CodeAvatarAssetUnavailable:          http.StatusServiceUnavailable,
	apperror.CodeJDInUse:                         http.StatusConflict,
	apperror.CodeJDNotFound:                      http.StatusNotFound,
	apperror.CodeInvalidJDInput:                  http.StatusBadRequest,
	apperror.CodeJDTooShort:                      http.StatusBadRequest,
	apperror.CodeJDTooLong:                       http.StatusBadRequest,
	apperror.CodeExtractionFailed:                http.StatusBadGateway,
	apperror.CodeInvalidExtractionOutput:         http.StatusBadGateway,
	apperror.CodeJobPostingNotFound:              http.StatusNotFound,
	apperror.CodeVoiceNotFound:                   http.StatusNotFound,
	apperror.CodeCoreQuestionNotFound:            http.StatusNotFound,
	apperror.CodeQuestionBankExists:              http.StatusConflict,
	apperror.CodeQuestionBankLocked:              http.StatusConflict,
	apperror.CodeCoreQuestionInUse:               http.StatusConflict,
	apperror.CodeRequirementsNotConfirmed:        http.StatusConflict,
	apperror.CodeQuestionGenerationFailed:        http.StatusBadGateway,
	apperror.CodeInvalidQuestionGenerationOutput: http.StatusBadGateway,
	apperror.CodeQuestionBankUnavailable:         http.StatusServiceUnavailable,
	apperror.CodeJobPostingNotPending:            http.StatusConflict,
	apperror.CodeQuestionBankNotReady:            http.StatusConflict,
	apperror.CodeJobPostingRequirementsLocked:    http.StatusConflict,
	apperror.CodeJobPostingNotEditable:           http.StatusConflict,
	apperror.CodeJobPostingTextLocked:            http.StatusConflict,
	apperror.CodeJobPostingNotRejected:           http.StatusConflict,
	apperror.CodeEvaluationMetricUnknown:         http.StatusBadRequest,
	apperror.CodeEvaluationMetricDuplicate:       http.StatusBadRequest,
	apperror.CodeEvaluationWeightInvalid:         http.StatusBadRequest,
	apperror.CodeEvaluationWeightTotalMismatch:   http.StatusBadRequest,
	apperror.CodeEvaluationWeightsLocked:         http.StatusConflict,
	apperror.CodeApplicationNotFound:             http.StatusNotFound,
	apperror.CodeApplicationNotPending:           http.StatusConflict,
	apperror.CodeApplicationNotEligible:          http.StatusConflict,
	apperror.CodeIntakeNotOpen:                   http.StatusConflict,
	apperror.CodeInterviewCapacityNotFunded:      http.StatusConflict,
	apperror.CodeInterviewCapacityFull:           http.StatusConflict,
	apperror.CodeInvalidCredentials:              http.StatusUnauthorized,
	apperror.CodeAccountNotFound:                 http.StatusNotFound,
	apperror.CodeAccountLocked:                   http.StatusForbidden,
	apperror.CodeInvalidToken:                    http.StatusUnauthorized,
	apperror.CodeAuthUnavailable:                 http.StatusServiceUnavailable,
	apperror.CodeForbidden:                       http.StatusForbidden,
	apperror.CodeOnboardingRoleConflict:          http.StatusConflict,
	apperror.CodeValidation:                      http.StatusBadRequest,
	apperror.CodeInternal:                        http.StatusInternalServerError,

	// Wallet / payment domain.
	apperror.CodeWalletNotFound:          http.StatusNotFound,
	apperror.CodeInsufficientBalance:     http.StatusUnprocessableEntity,
	apperror.CodePackageNotFound:         http.StatusNotFound,
	apperror.CodeCoinPackageNotFound:     http.StatusNotFound,
	apperror.CodeTransactionNotFound:     http.StatusNotFound,
	apperror.CodePaymentGatewayError:     http.StatusBadGateway,
	apperror.CodePayOSError:              http.StatusBadGateway,
	apperror.CodeWebhookSignatureInvalid: http.StatusBadRequest,
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data})
}

func Error(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) && appErr != nil {
		status, known := httpStatusByCode[appErr.Code]
		if !known {
			status = http.StatusInternalServerError
		}
		c.JSON(status, Envelope{
			Success: false,
			Error:   &ErrorBody{Code: appErr.Code, Message: appErr.Message},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, Envelope{
		Success: false,
		Error:   &ErrorBody{Code: apperror.CodeInternal, Message: "something wrong happen"},
	})
}
