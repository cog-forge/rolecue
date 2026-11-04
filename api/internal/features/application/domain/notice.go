package domain

import (
	"time"

	"github.com/google/uuid"
)

// NoticeContext is what an Application email needs: who receives it and which posting it is about.
// It carries no CV, transcript, recording, evaluation or storage data (KAN-219).
type NoticeContext struct {
	ApplicationID     uuid.UUID
	JobPostingID      uuid.UUID
	PostingTitle      string
	CandidateName     string
	CandidateEmail    string
	RecruiterEmail    string
	InterviewDeadline *time.Time
}
