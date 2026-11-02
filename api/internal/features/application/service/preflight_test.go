package service

import (
	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"testing"
	"time"
)

func TestCandidatePreflightUsesDatabaseClockAndBlockers(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	for _, tc := range []struct {
		name        string
		status      domain.ApplicationStatus
		deadline    *time.Time
		unavailable bool
		blocker     string
		wantStatus  domain.ApplicationStatus
	}{
		{"waiting", domain.StatusPending, nil, false, "awaiting_cv_review", domain.StatusPending},
		{"closed pending", domain.StatusPending, nil, true, "posting_unavailable", domain.StatusPending},
		{"expired before worker", domain.StatusInterviewEligible, &past, false, "deadline_expired", domain.StatusRejected},
		{"unfunded or terminal", domain.StatusInterviewEligible, &future, true, "posting_unavailable", domain.StatusInterviewEligible},
		{"completed", domain.StatusInterviewed, &future, false, "awaiting_final_decision", domain.StatusInterviewed},
		{"decided", domain.StatusApproved, &future, false, "application_decided", domain.StatusApproved},
		{"eligible", domain.StatusInterviewEligible, &future, false, "", domain.StatusInterviewEligible},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := domain.CandidateApplication{Status: tc.status, InterviewDeadline: tc.deadline, PostingUnavailable: tc.unavailable, ReadAt: now}
			candidatePreflight(&a, now.Add(48*time.Hour))
			blocker := ""
			if a.Blocker != nil {
				blocker = *a.Blocker
			}
			if a.Status != tc.wantStatus || blocker != tc.blocker || a.CanStartInterview != (tc.blocker == "") {
				t.Fatalf("unexpected preflight: %+v", a)
			}
		})
	}
}
