package domain

import (
	"bytes"
	"encoding/json"
)

// StringChange preserves absent, explicit null, and a supplied string.
type StringChange struct {
	Present bool
	Value   *string
}

func (v *StringChange) UnmarshalJSON(data []byte) error {
	v.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	v.Value = &s
	return nil
}

type Patch struct {
	FullName       StringChange `json:"full_name" swaggertype:"string"`
	Image          StringChange `json:"image" swaggertype:"string" extensions:"x-nullable"`
	CompanyName    StringChange `json:"company_name" swaggertype:"string" extensions:"x-nullable"`
	CompanyWebsite StringChange `json:"company_website" swaggertype:"string" extensions:"x-nullable"`
}
