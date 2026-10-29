package apperror

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeRoleConflict               Code = "ROLE_PROFILE_CONFLICT"
	CodeInvalidPDF                 Code = "INVALID_PDF"
	CodePDFTooLarge                Code = "PDF_TOO_LARGE"
	CodePDFUnavailable             Code = "PDF_UNAVAILABLE"
	CodeDocumentStorage            Code = "DOCUMENT_STORAGE_UNAVAILABLE"
	CodeInventoryFull              Code = "INVENTORY_FULL"
	CodeAvatarSourceTooLarge       Code = "AVATAR_SOURCE_TOO_LARGE"
	CodeAvatarSourceUnsupported    Code = "AVATAR_SOURCE_UNSUPPORTED"
	CodeAvatarConverterUnavailable Code = "AVATAR_CONVERTER_UNAVAILABLE"
	CodeAvatarConversionFailed     Code = "AVATAR_CONVERSION_FAILED"
	CodeAvatarConversionTimeout    Code = "AVATAR_CONVERSION_TIMEOUT"
	CodeAvatarPersistenceFailed    Code = "AVATAR_PERSISTENCE_FAILED"
	CodeAvatarNotFound             Code = "AVATAR_NOT_FOUND"
	CodeAvatarInUse                Code = "AVATAR_IN_USE"
	CodeAvatarAssetUnavailable     Code = "AVATAR_ASSET_UNAVAILABLE"
	CodeJDInUse                    Code = "JD_IN_USE"
	CodeJDNotFound                 Code = "JD_NOT_FOUND"
	CodeInvalidCredentials         Code = "INVALID_CREDENTIALS"
	CodeAccountNotFound            Code = "USER_NOT_FOUND"
	CodeAccountLocked              Code = "USER_INACTIVE"
	CodeInvalidToken               Code = "INVALID_TOKEN"
	CodeAuthUnavailable            Code = "AUTH_UNAVAILABLE"
	CodeForbidden                  Code = "FORBIDDEN"
	CodeOnboardingRoleConflict     Code = "ONBOARDING_ROLE_CONFLICT"
	CodeValidation                 Code = "VALIDATION_ERROR"
	CodeInternal                   Code = "INTERNAL_ERROR"
	CodeInvalidJDInput             Code = "INVALID_JD_INPUT"
	CodeJDTooShort                 Code = "JD_TOO_SHORT"
	CodeJDTooLong                  Code = "JD_TOO_LONG"
	CodeExtractionFailed           Code = "EXTRACTION_FAILED"
	CodeInvalidExtractionOutput    Code = "INVALID_EXTRACTION_OUTPUT"

	// Question bank domain codes.
	CodeJobPostingNotFound              Code = "JOB_POSTING_NOT_FOUND"
	CodeVoiceSyncFailed                 Code = "VOICE_SYNC_FAILED"
	CodeVoiceProviderUnavailable        Code = "VOICE_PROVIDER_UNAVAILABLE"
	CodeVoiceNotFound                   Code = "VOICE_NOT_FOUND"
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

	CodeJobPostingNotPending Code = "JOB_POSTING_NOT_PENDING"
	CodeQuestionBankNotReady Code = "QUESTION_BANK_NOT_READY"

	// Job Posting domain codes.
	CodeJobPostingRequirementsLocked Code = "JOB_POSTING_REQUIREMENTS_LOCKED"
	CodeJobPostingNotEditable        Code = "JOB_POSTING_NOT_EDITABLE"
	CodeJobPostingTextLocked         Code = "JOB_POSTING_TEXT_LOCKED"
	CodeJobPostingNotRejected        Code = "JOB_POSTING_NOT_REJECTED"

	// Evaluation weight codes.
	CodeEvaluationMetricUnknown       Code = "EVALUATION_METRIC_UNKNOWN"
	CodeEvaluationMetricDuplicate     Code = "EVALUATION_METRIC_DUPLICATE"
	CodeEvaluationWeightInvalid       Code = "EVALUATION_WEIGHT_INVALID"
	CodeEvaluationWeightTotalMismatch Code = "EVALUATION_WEIGHT_TOTAL_MISMATCH"
	CodeEvaluationWeightsLocked       Code = "EVALUATION_WEIGHTS_LOCKED"

	// Application screening codes.
	CodeApplicationNotFound        Code = "APPLICATION_NOT_FOUND"
	CodeApplicationNotPending      Code = "APPLICATION_NOT_PENDING"
	CodeApplicationNotEligible     Code = "APPLICATION_NOT_ELIGIBLE"
	CodeIntakeNotOpen              Code = "INTAKE_NOT_OPEN"
	CodeInterviewCapacityNotFunded Code = "INTERVIEW_CAPACITY_NOT_FUNDED"
	CodeInterviewCapacityFull      Code = "INTERVIEW_CAPACITY_FULL"

	// Interview capacity funding, Candidate apply and Admin interview oversight codes.
	CodeInterviewCapacityAlreadyFunded Code = "INTERVIEW_CAPACITY_ALREADY_FUNDED"
	CodeAlreadyApplied                 Code = "ALREADY_APPLIED"
	CodeInterviewNotFound              Code = "INTERVIEW_NOT_FOUND"

	// Interview session, final decision and recording codes.
	CodeRoleProfileRequired       Code = "ROLE_PROFILE_REQUIRED"
	CodeInterviewCompleted        Code = "INTERVIEW_COMPLETED"
	CodeInterviewTurnConflict     Code = "INTERVIEW_TURN_CONFLICT"
	CodeInterviewNotCompleted     Code = "INTERVIEW_NOT_COMPLETED"
	CodeFinalReviewLocked         Code = "FINAL_REVIEW_LOCKED"
	CodeApplicationNotInterviewed Code = "APPLICATION_NOT_INTERVIEWED"
	CodeRecordingNotFound         Code = "RECORDING_NOT_FOUND"
	CodeRecordingNotAllowed       Code = "RECORDING_NOT_ALLOWED"
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
