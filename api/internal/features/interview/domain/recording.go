package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Recording limits. The upload grant is valid for 15 minutes, the maximum the storage contract allows. A
// read link to a recording lasts 5 minutes and is issued on every read, never stored.
const (
	RecordingMaxBytes  int64 = 524288000
	RecordingUploadTTL       = 15 * time.Minute
	RecordingReadTTL         = 5 * time.Minute
)

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
	RecordStatus RecordStatus `json:"record_status" enums:"none,uploading,processing,ready,failed"`
}

// RecruiterRecording is the Recruiter's evidence link. URL and ExpiresAt are set only when the
// recording is ready; the link is valid for RecordingReadTTL.
type RecruiterRecording struct {
	RecordStatus RecordStatus `json:"record_status" enums:"none,uploading,processing,ready,failed"`
	URL          *string      `json:"url"`
	ExpiresAt    *time.Time   `json:"expires_at"`
}

// TranscriptTurn is one question of a completed recruitment interview and the Candidate's answer.
type TranscriptTurn struct {
	Position int32   `json:"position"`
	Question string  `json:"question"`
	Answer   *string `json:"answer"`
}

// Transcript is the list of turns of a completed recruitment interview.
type Transcript struct {
	Turns []TranscriptTurn `json:"turns"`
}

// RecordingState is the recording part of one interview as the repository reads it. Path is the object
// key and never leaves the service.
type RecordingState struct {
	InterviewID  uuid.UUID
	Type         Type
	RecordStatus RecordStatus
	RecordPath   *string
	Completed    bool
}

var (
	ErrRecordingNotFound   = apperror.New(apperror.CodeRecordingNotFound, "recording not found")
	ErrRecordingNotAllowed = apperror.New(apperror.CodeRecordingNotAllowed,
		"a recording can only be uploaded for a recruitment interview in its current state")
	ErrInterviewNotCompleted = apperror.New(apperror.CodeInterviewNotCompleted, "interview is not completed")
)
