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
}

const statusPending = "pending"

// EditableStatus is the one place that decides in which posting statuses the weights can change.
// The repository applies it again under the posting's row lock.
func EditableStatus(status string) bool {
	return status == statusPending
}

var ErrWeightsLocked = apperror.New(apperror.CodeEvaluationWeightsLocked, "evaluation weights can only be changed while the job posting is pending")
