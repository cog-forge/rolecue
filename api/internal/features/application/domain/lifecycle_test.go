package domain

import (
	"testing"
	"time"
)

func TestInterviewStatusIsInProgressUntilCompleted(t *testing.T) {
	completed := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if got := InterviewStatus(nil, "pending"); got != "in_progress" {
		t.Fatalf("started session = %q, want in_progress", got)
	}
	if got := InterviewStatus(&completed, "completed"); got != "completed" {
		t.Fatalf("completed session = %q, want completed", got)
	}
	if got := InterviewStatus(&completed, "failed"); got != "failed" {
		t.Fatalf("failed evaluation = %q, want failed", got)
	}
}

func TestCanDecideOpensAtTheLatestDeadline(t *testing.T) {
	opens := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		status ApplicationStatus
		opens  *time.Time
		now    time.Time
		want   bool
	}{
		{"before the window opens", StatusInterviewed, &opens, opens.Add(-time.Second), false},
		{"exactly when the window opens", StatusInterviewed, &opens, opens, true},
		{"after the window opens", StatusInterviewed, &opens, opens.Add(time.Hour), true},
		{"no deadline at all", StatusInterviewed, nil, opens, false},
		{"not interviewed yet", StatusInterviewEligible, &opens, opens.Add(time.Hour), false},
		{"already decided", StatusApproved, &opens, opens.Add(time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanDecide(tc.status, tc.opens, tc.now); got != tc.want {
				t.Fatalf("CanDecide = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExpiredForRejectionBoundaryAndInterviewState(t *testing.T) {
	deadline := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		status    ApplicationStatus
		deadline  *time.Time
		completed bool
		now       time.Time
		want      bool
	}{
		{"before the deadline", StatusInterviewEligible, &deadline, false, deadline.Add(-time.Second), false},
		{"exactly at the deadline is not yet expired", StatusInterviewEligible, &deadline, false, deadline, false},
		{"after the deadline, no interview", StatusInterviewEligible, &deadline, false, deadline.Add(time.Second), true},
		{"after the deadline, started but not completed", StatusInterviewEligible, &deadline, false, deadline.Add(time.Hour), true},
		{"after the deadline, interview completed", StatusInterviewEligible, &deadline, true, deadline.Add(time.Hour), false},
		{"after the deadline, interviewed", StatusInterviewed, &deadline, true, deadline.Add(time.Hour), false},
		{"after the deadline, pending CV", StatusPending, &deadline, false, deadline.Add(time.Hour), false},
		{"after the deadline, already rejected", StatusRejected, &deadline, false, deadline.Add(time.Hour), false},
		{"no deadline recorded", StatusInterviewEligible, nil, false, deadline.Add(time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExpiredForRejection(tc.status, tc.deadline, tc.completed, tc.now); got != tc.want {
				t.Fatalf("ExpiredForRejection = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecisionValidityAndStatus(t *testing.T) {
	if !ValidDecision(DecisionApprove) || !ValidDecision(DecisionReject) {
		t.Fatal("approve and reject must be valid decisions")
	}
	if ValidDecision("") || ValidDecision("maybe") {
		t.Fatal("empty and unknown decisions must be invalid")
	}
	if DecisionApprove.Status() != StatusApproved || DecisionReject.Status() != StatusRejected {
		t.Fatal("decisions must map to approved and rejected")
	}
}
