package apperror

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeJDInUse                 Code = "JD_IN_USE"
	CodeJDNotFound              Code = "JD_NOT_FOUND"
	CodeInvalidCredentials      Code = "INVALID_CREDENTIALS"
	CodeAccountNotFound         Code = "USER_NOT_FOUND"
	CodeAccountLocked           Code = "USER_INACTIVE"
	CodeInvalidToken            Code = "INVALID_TOKEN"
	CodeAuthUnavailable         Code = "AUTH_UNAVAILABLE"
	CodeForbidden               Code = "FORBIDDEN"
	CodeValidation              Code = "VALIDATION_ERROR"
	CodeInternal                Code = "INTERNAL_ERROR"
	CodeInvalidJDInput          Code = "INVALID_JD_INPUT"
	CodeJDTooShort              Code = "JD_TOO_SHORT"
	CodeJDTooLong               Code = "JD_TOO_LONG"
	CodeExtractionFailed        Code = "EXTRACTION_FAILED"
	CodeInvalidExtractionOutput Code = "INVALID_EXTRACTION_OUTPUT"

	// Question bank domain codes.
	CodeJobPostingNotFound              Code = "JOB_POSTING_NOT_FOUND"
	CodeCoreQuestionNotFound            Code = "CORE_QUESTION_NOT_FOUND"
	CodeQuestionBankExists              Code = "QUESTION_BANK_EXISTS"
	CodeQuestionBankLocked              Code = "QUESTION_BANK_LOCKED"
	CodeCoreQuestionInUse               Code = "CORE_QUESTION_IN_USE"
	CodeRequirementsNotConfirmed        Code = "REQUIREMENTS_NOT_CONFIRMED"
	CodeQuestionGenerationFailed        Code = "QUESTION_GENERATION_FAILED"
	CodeInvalidQuestionGenerationOutput Code = "INVALID_QUESTION_GENERATION_OUTPUT"
	CodeQuestionBankUnavailable         Code = "QUESTION_BANK_UNAVAILABLE"

	// Wallet / payment domain codes.
	CodeWalletNotFound          Code = "WALLET_NOT_FOUND"
	CodeInsufficientBalance     Code = "INSUFFICIENT_BALANCE"
	CodePackageNotFound         Code = "PACKAGE_NOT_FOUND"
	CodeCoinPackageNotFound     Code = "COIN_PACKAGE_NOT_FOUND"
	CodeTransactionNotFound     Code = "TRANSACTION_NOT_FOUND"
	CodePaymentGatewayError     Code = "PAYMENT_GATEWAY_ERROR"
	CodePayOSError              Code = "PAYOS_ERROR"
	CodeWebhookSignatureInvalid Code = "WEBHOOK_SIGNATURE_INVALID"
)

type AppError struct {
	Code    Code
	Message string
	Err     error
}

func New(code Code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

func Wrap(code Code, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Err == nil {
		return e.Message
	}

	if e.Message == "" {
		return e.Err.Error()
	}

	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func IsCode(err error, code Code) bool {
	var appErr *AppError
	if errors.As(err, &appErr) && appErr != nil {
		return appErr.Code == code
	}
	return false
}
