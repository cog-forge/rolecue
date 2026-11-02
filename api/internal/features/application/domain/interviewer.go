package domain

import (
	"time"

	inventory "github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
)

// CompanyInterviewer exposes only the locked presentation and a temporary read grant.
type CompanyInterviewer struct {
	Name        string                `json:"name"`
	VoiceName   string                `json:"voice_name"`
	ModelAccess inventory.ModelAccess `json:"model_access"`
}

type InterviewerRecord struct {
	Name      string
	VoiceName string
	ModelPath string
	Deadline  time.Time
}
