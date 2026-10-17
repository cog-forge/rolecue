package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProfileStatus string

const (
	Draft     ProfileStatus = "draft"
	Analyzing ProfileStatus = "analyzing"
	Review    ProfileStatus = "review"
	Confirmed ProfileStatus = "confirmed"
)

type RoleRequirement struct {
	Name         string         `json:"name"`
	Target       string         `json:"target"`
	Kind         string         `json:"kind"`
	Category     *SkillCategory `json:"category"`
	Type         *Requirement   `json:"type"`
	EvidenceText *string        `json:"evidenceText"`
}

type RoleReview struct {
	Title          string            `json:"title"`
	SeniorityLevel *Seniority        `json:"seniorityLevel"`
	Requirements   []RoleRequirement `json:"requirements"`
	RefinementNote string            `json:"refinementNote"`
}

// RoleProfile is the candidate-safe resource. Internal fields cannot be serialized.
type RoleProfile struct {
	ID                  uuid.UUID `json:"id"`
	JDID                uuid.UUID `json:"-"`
	RawText             string    `json:"rawText"`
	ContentPath         string    `json:"-"`
	HasOriginalDocument bool      `json:"hasOriginalDocument"`
	RoleReview
	Status    ProfileStatus `json:"status"`
	CreatedAt time.Time     `json:"createdAt"`
}

type SaveRoleProfile struct {
	RoleReview
}
