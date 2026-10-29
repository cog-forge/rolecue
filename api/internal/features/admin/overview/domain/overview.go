// Package domain holds the Admin overview aggregates: read-only counts for the platform and finance
// dashboard. It carries no personal data.
//
// Definitions shared by every type below:
//   - window is [now-days, now): from is inclusive, to (now) is exclusive.
//   - accounts count Candidate and Recruiter users only; Admin accounts are excluded.
//   - locked means is_locked and the lock has not expired (no lock_expires_at, or one still in the future).
//   - job postings and applications by status are all-time counts.
//   - interviews: *_started_in_window use started_at by type, completed_in_window uses completed_at,
//     evaluation_failed is all-time.
//   - finance is the wallet billing summary over the window; revenue is successful PayOS top-ups
//     only, coin debits and refunds are not revenue.
package domain

import "time"

// DefaultWindowDays is the window used when the request names no days.
const DefaultWindowDays = 30

// WindowDays lists the windows the Admin can ask for.
var WindowDays = []int{7, 30, 90}

// Overview is the whole Admin overview response.
type Overview struct {
	GeneratedAt  time.Time            `json:"generated_at"`
	Window       OverviewWindow       `json:"window"`
	Accounts     OverviewAccounts     `json:"accounts"`
	JobPostings  OverviewPostings     `json:"job_postings"`
	Applications OverviewApplications `json:"applications"`
	Interviews   OverviewInterviews   `json:"interviews"`
	Finance      OverviewFinance      `json:"finance"`
}

// OverviewWindow is the time window the windowed counts cover: [from, to).
type OverviewWindow struct {
	Days int       `json:"days"`
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// OverviewAccounts counts Candidate and Recruiter accounts. Admin accounts are excluded.
// Locked is an active lock (is_locked and not yet expired). NewInWindow counts accounts created in the window.
type OverviewAccounts struct {
	Candidates  int64 `json:"candidates"`
	Recruiters  int64 `json:"recruiters"`
	Locked      int64 `json:"locked"`
	NewInWindow int64 `json:"new_in_window"`
}

// OverviewPostings counts Job Postings by status, all-time.
type OverviewPostings struct {
	Pending      int64 `json:"pending"`
	Open         int64 `json:"open"`
	IntakeClosed int64 `json:"intake_closed"`
	Closed       int64 `json:"closed"`
	Rejected     int64 `json:"rejected"`
}

// OverviewApplications counts Applications by status, all-time. SubmittedInWindow counts
// applications created in the window.
type OverviewApplications struct {
	Pending           int64 `json:"pending"`
	InterviewEligible int64 `json:"interview_eligible"`
	Interviewed       int64 `json:"interviewed"`
	Approved          int64 `json:"approved"`
	Rejected          int64 `json:"rejected"`
	SubmittedInWindow int64 `json:"submitted_in_window"`
}

// OverviewInterviews counts interview sessions. See the package comment for the time basis of each count.
type OverviewInterviews struct {
	PracticeStartedInWindow    int64 `json:"practice_started_in_window"`
	RecruitmentStartedInWindow int64 `json:"recruitment_started_in_window"`
	CompletedInWindow          int64 `json:"completed_in_window"`
	EvaluationFailed           int64 `json:"evaluation_failed"`
}

// OverviewFinance is the wallet billing summary over the window. RevenueVND is successful PayOS
// top-ups only. SuccessRate is the share of started top-ups that succeeded, from 0 to 1.
type OverviewFinance struct {
	RevenueVND         int64   `json:"revenue_vnd"`
	SuccessfulDeposits int64   `json:"successful_deposits"`
	CoinsSold          int64   `json:"coins_sold"`
	CoinsSpent         int64   `json:"coins_spent"`
	CoinsRefunded      int64   `json:"coins_refunded"`
	UniquePayers       int64   `json:"unique_payers"`
	SuccessRate        float64 `json:"success_rate"`
}
