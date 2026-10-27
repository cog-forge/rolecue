package domain

import (
	"bytes"
	"encoding/json"

	"github.com/google/uuid"
)

// The Change types keep "absent", explicit null and a supplied value apart, as profile does.

type StringChange struct {
	Present bool
	Value   *string
}

func (v *StringChange) UnmarshalJSON(data []byte) error {
	return decodeChange(data, &v.Present, &v.Value)
}

type Int32Change struct {
	Present bool
	Value   *int32
}

func (v *Int32Change) UnmarshalJSON(data []byte) error {
	return decodeChange(data, &v.Present, &v.Value)
}

type UUIDChange struct {
	Present bool
	Value   *uuid.UUID
}

func (v *UUIDChange) UnmarshalJSON(data []byte) error {
	return decodeChange(data, &v.Present, &v.Value)
}

// RequirementsChange tells "absent" from null; null is rejected, since the list cannot be cleared.
type RequirementsChange struct {
	Present bool
	Value   *[]PostingRequirement
}

func (v *RequirementsChange) UnmarshalJSON(data []byte) error {
	return decodeChange(data, &v.Present, &v.Value)
}

func decodeChange[T any](data []byte, present *bool, value **T) error {
	*present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*value = &v
	return nil
}

// Edit changes a pending or rejected posting. Title, seniority, avatar and voice may change but not
// be cleared; the optional details and the refinement note may be cleared with null. Requirements, when present, replace
// the list. Every field may be resent unchanged (so a client can submit the whole form): an
// unchanged seniority or requirement list does not count as a change to the question bank's
// inputs, and an unchanged voice is not a new voice selection. DocumentID (a newly uploaded JD PDF)
// may change only on a rejected posting, to fix what the Admin objected to.
type Edit struct {
	Title             StringChange       `json:"title" swaggertype:"string"`
	DocumentID        StringChange       `json:"document_id" swaggertype:"string"`
	SeniorityLevel    StringChange       `json:"seniority_level" swaggertype:"string" enums:"intern,junior,mid,senior,lead"`
	Requirements      RequirementsChange `json:"requirements" swaggertype:"array,object"`
	RefinementNote    StringChange       `json:"refinement_note" swaggertype:"string" extensions:"x-nullable"`
	Location          StringChange       `json:"location" swaggertype:"string" extensions:"x-nullable"`
	EmploymentType    StringChange       `json:"employment_type" swaggertype:"string" extensions:"x-nullable"`
	SalaryMin         Int32Change        `json:"salary_min" swaggertype:"integer" extensions:"x-nullable"`
	SalaryMax         Int32Change        `json:"salary_max" swaggertype:"integer" extensions:"x-nullable"`
	InterviewDuration Int32Change        `json:"interview_duration" swaggertype:"integer" extensions:"x-nullable"`
	RequiredAvatarID  UUIDChange         `json:"required_avatar_id" swaggertype:"string" format:"uuid"`
	RequiredVoiceID   UUIDChange         `json:"required_voice_id" swaggertype:"string" format:"uuid"`
}

// SameRequirements reports whether two validated lists hold the same requirements, in any order.
func SameRequirements(a, b []PostingRequirement) bool {
	if len(a) != len(b) {
		return false
	}
	count := make(map[PostingRequirement]int, len(a))
	for _, r := range a {
		count[r]++
	}
	for _, r := range b {
		if count[r] == 0 {
			return false
		}
		count[r]--
	}
	return true
}
