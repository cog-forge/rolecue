package domain

import "github.com/google/uuid"

// Voice is an active voice profile as Candidates and Recruiters see it when choosing one. Usage
// counts and inactive voices stay in the Admin catalog.
type Voice struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ListFilter pages the active voices.
type ListFilter struct {
	Limit  int32
	Offset int32
}
