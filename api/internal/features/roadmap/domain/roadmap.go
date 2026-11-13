// Package domain holds the Learning Roadmap types shared by the Roadmap features (generation, sampling, scoring).
package domain

import (
	"time"

	"github.com/google/uuid"
)

// GenerationStatus is the state of the Roadmap's content, not of the candidate's learning (that is computed on read).
type GenerationStatus string

const (
	StatusPending    GenerationStatus = "pending"
	StatusGenerating GenerationStatus = "generating"
	StatusReady      GenerationStatus = "ready"
	StatusFailed     GenerationStatus = "failed"
)

// Roadmap is the one Roadmap of a Role Profile. UpdatedAt doubles as the fencing token of a generation.
type Roadmap struct {
	ID               uuid.UUID
	RoleProfileID    uuid.UUID
	GenerationStatus GenerationStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Item is one topic of a Roadmap. Order is Position, then ID.
type Item struct {
	ID          uuid.UUID
	RoadmapID   uuid.UUID
	Title       string
	Description string
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewItem is a topic about to be stored. The backend assigns the ID before the topic is stored, so the question
// bank can be generated for it and published in the same transaction; a nil ID is replaced by a new one.
type NewItem struct {
	ID          uuid.UUID
	Title       string
	Description string
	Position    int
}

// Fence is the token a worker holds while it generates. It is the Roadmap's updated_at at claim time, and
// Previous is the status the Roadmap had before the claim (a regeneration that fails puts a ready Roadmap back).
type Fence struct {
	UpdatedAt time.Time
	Previous  GenerationStatus
}

// ProfileState is what the generation entry points need to know about a candidate's Role Profile.
type ProfileState struct {
	JDID uuid.UUID
	// Confirmed is true once the candidate confirmed the requirements.
	Confirmed bool
	// HasSessions is true once any interview exists for the profile, finished or not.
	HasSessions bool
}
