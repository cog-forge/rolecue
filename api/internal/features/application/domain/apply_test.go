package domain

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestValidateCVAcceptsPDFUpToLimit(t *testing.T) {
	atLimit := append([]byte("%PDF-1.7\n"), make([]byte, MaxCVBytes-9)...)
	for _, data := range [][]byte{[]byte("%PDF-1.4 small"), atLimit} {
		if err := ValidateCV(data); err != nil {
			t.Fatalf("ValidateCV(%d bytes) error = %v", len(data), err)
		}
	}
}

func TestValidateCVRejectsEverythingElseAsValidation(t *testing.T) {
	overLimit := append([]byte("%PDF-1.7\n"), make([]byte, MaxCVBytes)...)
	for name, data := range map[string][]byte{
		"empty":     {},
		"nil":       nil,
		"plain":     []byte("just text"),
		"wrong sig": []byte("PDF-1.4 without the percent"),
		"over 5 MB": overLimit,
	} {
		if err := ValidateCV(data); !apperror.IsCode(err, apperror.CodeValidation) {
			t.Fatalf("%s: ValidateCV() = %v, want VALIDATION_ERROR", name, err)
		}
	}
}

func TestCanStartInterview(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	earlier := now.Add(-time.Hour)
	for _, tc := range []struct {
		name     string
		status   ApplicationStatus
		deadline *time.Time
		want     bool
	}{
		{"eligible before deadline", StatusInterviewEligible, &later, true},
		{"eligible after deadline", StatusInterviewEligible, &earlier, false},
		{"eligible without deadline", StatusInterviewEligible, nil, false},
		{"pending", StatusPending, &later, false},
		{"rejected", StatusRejected, &later, false},
		{"approved", StatusApproved, &later, false},
		{"interviewed", StatusInterviewed, &later, false},
	} {
		if got := CanStartInterview(tc.status, tc.deadline, now); got != tc.want {
			t.Fatalf("%s: CanStartInterview() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
