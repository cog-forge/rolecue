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
	apperror.CodeJDInUse:                 http.StatusConflict,
	apperror.CodeJDNotFound:              http.StatusNotFound,
	apperror.CodeInvalidJDInput:          http.StatusBadRequest,
	apperror.CodeJDTooShort:              http.StatusBadRequest,
	apperror.CodeJDTooLong:               http.StatusBadRequest,
	apperror.CodeExtractionFailed:        http.StatusBadGateway,
	apperror.CodeInvalidExtractionOutput: http.StatusBadGateway,
	apperror.CodeInvalidCredentials:      http.StatusUnauthorized,
	apperror.CodeAccountNotFound:         http.StatusNotFound,
	apperror.CodeAccountLocked:           http.StatusForbidden,
	apperror.CodeInvalidToken:            http.StatusUnauthorized,
	apperror.CodeAuthUnavailable:         http.StatusServiceUnavailable,
	apperror.CodeForbidden:               http.StatusForbidden,
	apperror.CodeValidation:              http.StatusBadRequest,
	apperror.CodeInternal:                http.StatusInternalServerError,
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
