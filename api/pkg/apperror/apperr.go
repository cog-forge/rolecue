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
	CodeAvatarConversionQuota      Code = "AVATAR_CONVERSION_QUOTA"
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
	CodeAdminMFARequired           Code = "ADMIN_MFA_REQUIRED"
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

	// Avatar Marketplace listing codes (KAN-363/365/367). Kept together so the lanes merge cleanly.
	CodeMarketplaceListingNotFound   Code = "MARKETPLACE_LISTING_NOT_FOUND"
	CodeMarketplaceAssetNotEligible  Code = "MARKETPLACE_ASSET_NOT_ELIGIBLE"
	CodeMarketplaceAssetNotReady     Code = "MARKETPLACE_ASSET_NOT_READY"
	CodeMarketplaceListingExists     Code = "MARKETPLACE_LISTING_EXISTS"
	CodeMarketplaceListingStale      Code = "MARKETPLACE_LISTING_STALE"
	CodeMarketplaceInvalidTransition Code = "MARKETPLACE_INVALID_TRANSITION"

	// Avatar Marketplace checkout codes (KAN-370), see api/docs/marketplace-checkout-api.md.
	CodeMarketplaceListingUnavailable Code = "MARKETPLACE_LISTING_UNAVAILABLE"
	CodeMarketplacePriceChanged       Code = "MARKETPLACE_PRICE_CHANGED"
	CodeMarketplaceAlreadyOwned       Code = "MARKETPLACE_ALREADY_OWNED"
	CodeMarketplaceOwnListing         Code = "MARKETPLACE_OWN_LISTING"

	CodeJobPostingNotPending Code = "JOB_POSTING_NOT_PENDING"
	// CodeGuidelineVersionConflict is an Interview Behavior Guidelines save made against a version that is no
	// longer the current one.
	CodeGuidelineVersionConflict Code = "GUIDELINE_VERSION_CONFLICT"
	CodeQuestionBankNotReady     Code = "QUESTION_BANK_NOT_READY"
	// CodeQuestionBankCoverageInsufficient means a session cannot start because the question bank cannot
	// cover every metric the posting weights within the session's question limit (KAN-333).
	CodeQuestionBankCoverageInsufficient Code = "QUESTION_BANK_COVERAGE_INSUFFICIENT"
	// CodeMetricCoverageIncomplete means a job posting's question bank cannot cover every active metric within
	// the interview's question limit, so it can be neither submitted nor approved (KAN-377). Recruiter and
	// Admin only; the Candidate-facing refusal at interview start is CodeQuestionBankCoverageInsufficient.
	CodeMetricCoverageIncomplete Code = "METRIC_COVERAGE_INCOMPLETE"
	// CodeGradingConfigIncomplete means a question lacks a usable rubric or a valid expected answer (KAN-331).
	CodeGradingConfigIncomplete Code = "GRADING_CONFIG_INCOMPLETE"

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

	// Learning Roadmap read API (KAN-352). A topic that is not part of the profile's Roadmap, or a profile the
	// caller does not own, is reported as not found, exactly like a missing one.
	CodeRoadmapItemNotFound Code = "ROADMAP_ITEM_NOT_FOUND"

	// Scoring rubric (Barem) management codes (KAN-326). A rubric the caller may not see is reported as not
	// found, exactly like a missing one, so a guessed UUID reveals nothing.
	CodeRubricNotFound               Code = "RUBRIC_NOT_FOUND"
	CodeRubricLocked                 Code = "RUBRIC_LOCKED"
	CodeRubricInUse                  Code = "RUBRIC_IN_USE"
	CodeRubricStale                  Code = "RUBRIC_STALE"
	CodeQuestionStale                Code = "QUESTION_STALE"
	CodeExpectedAnswerReviewRequired Code = "EXPECTED_ANSWER_REVIEW_REQUIRED"

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
	CodeRoleProfileRequired        Code = "ROLE_PROFILE_REQUIRED"
	CodeInterviewCompleted         Code = "INTERVIEW_COMPLETED"
	CodeInterviewTurnConflict      Code = "INTERVIEW_TURN_CONFLICT"
	CodeInterviewNotCompleted      Code = "INTERVIEW_NOT_COMPLETED"
	CodeEvaluationAttemptsExceeded Code = "EVALUATION_ATTEMPTS_EXCEEDED"
	CodeRateLimited                Code = "RATE_LIMITED"
	CodeLLMBudgetExceeded          Code = "LLM_BUDGET_EXCEEDED"
	CodeFinalReviewLocked          Code = "FINAL_REVIEW_LOCKED"
	CodeApplicationNotInterviewed  Code = "APPLICATION_NOT_INTERVIEWED"
	CodeRecordingNotFound          Code = "RECORDING_NOT_FOUND"
	CodeRecordingNotAllowed        Code = "RECORDING_NOT_ALLOWED"
	CodeRecordingUploadLimit       Code = "RECORDING_UPLOAD_LIMIT"
	CodeRecordingIntegrity         Code = "RECORDING_INTEGRITY"
	CodeRecordingConsentRequired   Code = "RECORDING_CONSENT_REQUIRED"

	// Account erasure (KAN-280).
	CodeAccountHasActiveWork  Code = "ACCOUNT_HAS_ACTIVE_WORK"
	CodeAdminCannotSelfDelete Code = "ADMIN_CANNOT_SELF_DELETE"

	// Learning Roadmap generation (KAN-343): topics generated from the confirmed requirements.
	CodeRoadmapGenerating              Code = "ROADMAP_GENERATING"
	CodeRoadmapGenerationFailed        Code = "ROADMAP_GENERATION_FAILED"
	CodeInvalidRoadmapGenerationOutput Code = "INVALID_ROADMAP_GENERATION_OUTPUT"
	CodeRoadmapRegenerateNotAllowed    Code = "ROADMAP_REGENERATE_NOT_ALLOWED"
	CodeRoadmapNotRetryable            Code = "ROADMAP_NOT_RETRYABLE"
	CodeRoadmapNotReady                Code = "ROADMAP_NOT_READY"
	CodeInterviewStartIntentConflict   Code = "INTERVIEW_START_INTENT_CONFLICT"
	// CodeSetupNotReady means a new interview cannot start because the saved interview setup of its Role
	// Profile or Application is missing or not ready (KAN-231). The error details list the blockers.
	CodeSetupNotReady Code = "SETUP_NOT_READY"

	// Coin transfer between users (KAN-369), see api/docs/marketplace-coins.md. The key is malformed or
	// not a Marketplace key; the key is well formed but already names another transfer; the recipient's
	// wallet cannot hold the amount.
	CodeCoinTransferKeyInvalid    Code = "COIN_TRANSFER_KEY_INVALID"
	CodeCoinTransferKeyConflict   Code = "COIN_TRANSFER_KEY_CONFLICT"
	CodeCoinRecipientBalanceLimit Code = "COIN_RECIPIENT_BALANCE_LIMIT"
)

type AppError struct {
	Code    Code
	Message string
	Err     error
	// Details is optional, client-safe structured data that explains the error (for example the blockers of a
	// setup that is not ready). It is sent as error.details; it must never hold anything the message could not.
	Details any
}

// WithDetails returns a copy of the error that carries details. The receiver is not changed, so a shared
// error value can be given details per request.
func (e *AppError) WithDetails(details any) *AppError {
	if e == nil {
		return nil
	}
	copied := *e
	copied.Details = details
	return &copied
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
