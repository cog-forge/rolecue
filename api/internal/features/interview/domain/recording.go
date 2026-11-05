package domain

import (
	"bytes"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Recording limits. The upload grant is valid for 15 minutes, the maximum the storage contract allows. A
// read link to a recording lasts 5 minutes and is issued on every read, never stored. RecordingMaxGrants
// caps the upload grants of one interview, so a Candidate cannot replace the object without end.
// RecordingSniffBytes is how much of the object the media type check reads.
const (
	RecordingMaxBytes   int64 = 524288000
	RecordingUploadTTL        = 15 * time.Minute
	RecordingReadTTL          = 5 * time.Minute
	RecordingMaxGrants  int32 = 5
	RecordingSniffBytes int64 = 12
)

// recordingEBML is the EBML header every WebM file starts with.
var recordingEBML = []byte{0x1A, 0x45, 0xDF, 0xA3}

// RecordingMatchesMagic reports whether the leading bytes of an object are the container its declared media
// type names. The declared type is signed into the upload, so a mismatch means the bytes are not that media.
func RecordingMatchesMagic(contentType string, head []byte) bool {
	switch contentType {
	case "video/webm", "audio/webm":
		return bytes.HasPrefix(head, recordingEBML)
	case "video/mp4":
		return len(head) >= 8 && string(head[4:8]) == "ftyp"
	default:
		return false
	}
}

// RecordingRetention is how long a recording is kept after its Application's final decision (Decision 18,
// 2026-11-03); the System Handler then deletes it. It is a product rule, not configuration.
const RecordingRetention = 90 * 24 * time.Hour

// RecordingChange is one recording the System Handler moved to another status, for the log.
type RecordingChange struct {
	InterviewID uuid.UUID
	From        RecordStatus
	To          RecordStatus
}

// StoredRecording is a recording object past retention. Key never leaves the service and is never logged.
type StoredRecording struct {
	InterviewID uuid.UUID
	Key         string
}

// StoredCV is the CV file of an Application past retention. Key never leaves the service and is never
// logged.
type StoredCV struct {
	ApplicationID uuid.UUID
	Key           string
}

// recordingContentTypes is the closed list of media types a recording may be uploaded as.
var recordingContentTypes = map[string]bool{
	"video/webm": true,
	"audio/webm": true,
	"video/mp4":  true,
}

// ValidRecordingContentType reports whether a recording may be uploaded as this media type.
func ValidRecordingContentType(contentType string) bool {
	return recordingContentTypes[contentType]
}

// RecordingUploadInput is what the Candidate declares before uploading: the media type and the exact
// byte size, which the grant binds.
type RecordingUploadInput struct {
	ContentType string
	Size        int64
	// ConsentVersion is the version of the consent text the Candidate accepted before recording (KAN-297).
	// Empty when the request carried none.
	ConsentVersion string
}

// MaxConsentVersionLength bounds the consent version a request may carry.
const MaxConsentVersionLength = 32

// ValidConsentVersion reports whether a consent version is a short token such as "2026-11-v1".
func ValidConsentVersion(version string) bool {
	if version == "" || len(version) > MaxConsentVersionLength {
		return false
	}
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// Recruiter access actions written to the access audit (KAN-296).
const (
	AccessRecordingView  = "recording_view"
	AccessTranscriptView = "transcript_view"
)

// AccessAuditEntry is one successful Recruiter read of a recording or transcript.
type AccessAuditEntry struct {
	ID            uuid.UUID `json:"id"`
	RecruiterID   uuid.UUID `json:"recruiter_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	InterviewID   uuid.UUID `json:"interview_id"`
	Action        string    `json:"action" enums:"recording_view,transcript_view"`
	CreatedAt     time.Time `json:"created_at"`
}

// AccessAuditFilter selects a page of the audit, newest first, optionally for one Recruiter or Application.
type AccessAuditFilter struct {
	RecruiterID   *uuid.UUID
	ApplicationID *uuid.UUID
	Page          int32
	PageSize      int32
}

// Page bounds of the access audit list.
const (
	DefaultAccessAuditPageSize int32 = 20
	MaxAccessAuditPageSize     int32 = 100
)

// AccessAuditPage is one page of the access audit.
type AccessAuditPage struct {
	Items    []AccessAuditEntry `json:"items"`
	Total    int64              `json:"total"`
	Page     int32              `json:"page"`
	PageSize int32              `json:"page_size"`
}

// RecordingUploadGrant is the one-time upload the Candidate sends the recording to. It carries no key
// and no read link.
type RecordingUploadGrant struct {
	UploadURL    string            `json:"upload_url"`
	Method       string            `json:"method" enums:"PUT"`
	Headers      map[string]string `json:"headers"`
	ExpiresAt    time.Time         `json:"expires_at"`
	RecordStatus RecordStatus      `json:"record_status" enums:"uploading"`
}

// RecordingStatus is the Candidate's view after finalization: only the state, never the object.
type RecordingStatus struct {
	RecordStatus RecordStatus `json:"record_status" enums:"none,uploading,processing,ready,failed,deleted"`
}

// RecruiterRecording is the Recruiter's evidence link. URL and ExpiresAt are set only when the
// recording is ready; the link is valid for RecordingReadTTL.
type RecruiterRecording struct {
	RecordStatus RecordStatus `json:"record_status" enums:"none,uploading,processing,ready,failed,deleted"`
	URL          *string      `json:"url"`
	ExpiresAt    *time.Time   `json:"expires_at"`
}

// TranscriptTurn is one question of a completed recruitment interview and the Candidate's answer.
// Position is its place in the conversation; QuestionNumber is the core question it belongs to, so a
// follow-up is shown under its core question.
type TranscriptTurn struct {
	Position       int32    `json:"position"`
	QuestionNumber int32    `json:"question_number"`
	Kind           TurnKind `json:"kind" enums:"core,follow_up"`
	Question       string   `json:"question"`
	Answer         *string  `json:"answer"`
}

// Transcript is the list of turns of a completed recruitment interview.
type Transcript struct {
	Turns []TranscriptTurn `json:"turns"`
}

// RecordingState is the recording part of one interview as the repository reads it. Path is the object
// key and never leaves the service. ETag is the object's ETag when its upload was verified, nil otherwise.
type RecordingState struct {
	InterviewID  uuid.UUID
	Type         Type
	RecordStatus RecordStatus
	RecordPath   *string
	Completed    bool
	RecordETag   *string
	GrantCount   int32
	// ConsentVersion is the recording consent stored for the interview, nil when none was given.
	ConsentVersion *string
}

var (
	ErrRecordingNotFound   = apperror.New(apperror.CodeRecordingNotFound, "recording not found")
	ErrRecordingNotAllowed = apperror.New(apperror.CodeRecordingNotAllowed,
		"a recording can only be uploaded for a recruitment interview in its current state")
	ErrInterviewNotCompleted = apperror.New(apperror.CodeInterviewNotCompleted, "interview is not completed")
)

var (
	ErrRecordingUploadLimit = apperror.New(apperror.CodeRecordingUploadLimit,
		"this recording has used all of its upload attempts")
	ErrRecordingConsentRequired = apperror.New(apperror.CodeRecordingConsentRequired,
		"the Candidate must accept the recording consent before a recording starts")
	ErrRecordingIntegrity = apperror.New(apperror.CodeRecordingIntegrity,
		"the stored recording no longer matches its upload and must be uploaded again")
)
