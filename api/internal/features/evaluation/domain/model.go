package domain

import (
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Metric is one evaluation dimension from the seeded metrics catalog.
type Metric struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// MetricCatalog is the response of the metrics catalog endpoint.
type MetricCatalog struct {
	Items []Metric `json:"items"`
}

// Weight is a posting's percentage for one metric.
type Weight struct {
	MetricID   uuid.UUID `json:"metric_id"`
	Name       string    `json:"name"`
	Percentage float64   `json:"percentage"`
}

// WeightInput is one entry of a replace request. Both fields are pointers so a missing field is
// told apart from a zero value.
type WeightInput struct {
	MetricID   *uuid.UUID
	Percentage *float64
}

// PostingWeights is the Recruiter's view of a posting's evaluation weights. When the Recruiter set
// none, Configured is false, UsingDefaults is true and Weights holds the default weights the
// posting will be scored with, so the UI can warn the Recruiter. Defaults are never stored.
type PostingWeights struct {
	Configured    bool     `json:"configured"`
	UsingDefaults bool     `json:"using_defaults"`
	Editable      bool     `json:"editable"`
	PostingStatus string   `json:"posting_status"`
	Total         float64  `json:"total"`
	Weights       []Weight `json:"weights"`
	// Coverage tells the Recruiter whether the posting's bank still covers every active metric under
	// these weights (KAN-377). It is set on the Recruiter's views only and is absent when the server
	// cannot read the bank.
	Coverage *CoverageReport `json:"coverage,omitempty"`
	// SubmissionWithdrawn is true on the answer to a weight change that took the posting out of the Admin
	// queue because the bank no longer covers every active metric (KAN-377): the Recruiter fixes the bank or
	// the weights and submits again. It is absent on reads.
	SubmissionWithdrawn bool `json:"submission_withdrawn,omitempty"`
}

const (
	statusPending  = "pending"
	statusRejected = "rejected"
)

// EditableStatus is the one place that decides in which posting statuses the weights can change:
// before approval and in the rejected correction window, like the posting's own details. Once
// published (open, intake_closed, closed) they are locked so every Candidate is scored alike. The
// repository applies it again under the posting's row lock.
func EditableStatus(status string) bool {
	return status == statusPending || status == statusRejected
}

var ErrWeightsLocked = apperror.New(apperror.CodeEvaluationWeightsLocked, "evaluation weights can only be changed while the job posting is pending or rejected")
