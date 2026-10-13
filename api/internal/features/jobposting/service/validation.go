package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// The JD text uses the same range as the Candidate JD flow. The other caps only bound storage and
// payload size; they are not product rules.
const (
	MinTextLength      = 100
	MaxTextLength      = 20_000
	maxStoredTextBytes = 4 * MaxTextLength
	maxTitleLength     = 200
	maxDetailLength    = 200
	maxNameLength      = 200
	maxRequirements    = 100
)

func invalid(msg string) error { return apperror.New(apperror.CodeValidation, msg) }

func validateCreate(in domain.CreateInput) (domain.CreateInput, error) {
	var err error
	if in.Title, err = requiredText(in.Title, "title", maxTitleLength); err != nil {
		return in, err
	}
	if err := checkText(in.RawText); err != nil {
		return in, err
	}
	if !in.SeniorityLevel.Valid() {
		return in, invalid("seniority_level must be one of intern, junior, mid, senior, lead")
	}
	if in.Requirements, err = checkRequirements(in.Requirements); err != nil {
		return in, err
	}
	if in.Location, err = optionalText(in.Location, "location"); err != nil {
		return in, err
	}
	if in.EmploymentType, err = optionalText(in.EmploymentType, "employment_type"); err != nil {
		return in, err
	}
	if err := checkNumbers(in.SalaryMin, in.SalaryMax, in.InterviewDuration); err != nil {
		return in, err
	}
	if err := domain.CheckSalary(in.SalaryMin, in.SalaryMax); err != nil {
		return in, err
	}
	// The interviewer and voice are locked before Admin approval, and a posting is submitted for
	// approval when it is created, so both are required here.
	if in.RequiredAvatarID == uuid.Nil {
		return in, invalid("required_avatar_id is required")
	}
	if in.RequiredVoiceID == uuid.Nil {
		return in, invalid("required_voice_id is required")
	}
	return in, nil
}

func validatePatch(p domain.Edit) (domain.Edit, error) {
	var err error
	if p.Title.Present {
		if p.Title.Value == nil {
			return p, invalid("title cannot be null")
		}
		title, err := requiredText(*p.Title.Value, "title", maxTitleLength)
		if err != nil {
			return p, err
		}
		p.Title.Value = &title
	}
	if p.SeniorityLevel.Present {
		if p.SeniorityLevel.Value == nil || !domain.Level(*p.SeniorityLevel.Value).Valid() {
			return p, invalid("seniority_level must be one of intern, junior, mid, senior, lead")
		}
	}
	if p.Requirements.Present {
		if p.Requirements.Value == nil {
			return p, invalid("requirements cannot be null")
		}
		rows, err := checkRequirements(*p.Requirements.Value)
		if err != nil {
			return p, err
		}
		p.Requirements.Value = &rows
	}
	if p.Location.Value, err = optionalText(p.Location.Value, "location"); err != nil {
		return p, err
	}
	if p.EmploymentType.Value, err = optionalText(p.EmploymentType.Value, "employment_type"); err != nil {
		return p, err
	}
	if err := checkNumbers(p.SalaryMin.Value, p.SalaryMax.Value, p.InterviewDuration.Value); err != nil {
		return p, err
	}
	if p.RequiredAvatarID.Present && (p.RequiredAvatarID.Value == nil || *p.RequiredAvatarID.Value == uuid.Nil) {
		return p, invalid("required_avatar_id cannot be cleared")
	}
	if p.RequiredVoiceID.Present && (p.RequiredVoiceID.Value == nil || *p.RequiredVoiceID.Value == uuid.Nil) {
		return p, invalid("required_voice_id cannot be cleared")
	}
	return p, nil
}

func validateFilter(f domain.ListFilter) (domain.ListFilter, error) {
	if f.Status != nil && !f.Status.Valid() {
		return f, invalid("invalid status filter")
	}
	search, err := optionalText(f.Search, "search")
	if err != nil {
		return f, err
	}
	f.Search = search
	if search != nil {
		// ILIKE treats % and _ as wildcards; a search for them should match them literally.
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(*search)
		f.Search = &escaped
	}
	return f, nil
}

// checkText bounds the JD text the same way the Candidate JD flow does. Whitespace is not trimmed:
// the original text is what gets stored and shown back.
func checkText(raw string) error {
	if !utf8.ValidString(raw) {
		return apperror.New(apperror.CodeInvalidJDInput, "Job description input is invalid.")
	}
	length := utf8.RuneCountInString(strings.TrimSpace(raw))
	switch {
	case length < MinTextLength:
		return apperror.New(apperror.CodeJDTooShort, "Job description must contain at least 100 Unicode characters.")
	case length > MaxTextLength || len(raw) > maxStoredTextBytes:
		return apperror.New(apperror.CodeJDTooLong, "Job description must contain at most 20000 Unicode characters.")
	}
	return nil
}

// checkRequirements trims and checks the confirmed requirements. At least one is needed, since the
// question bank is generated from them.
func checkRequirements(rows []domain.PostingRequirement) ([]domain.PostingRequirement, error) {
	if len(rows) == 0 {
		return nil, invalid("at least one requirement is required")
	}
	if len(rows) > maxRequirements {
		return nil, invalid(fmt.Sprintf("at most %d requirements are allowed", maxRequirements))
	}
	out := make([]domain.PostingRequirement, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		name, err := requiredText(row.Name, "requirement name", maxNameLength)
		if err != nil {
			return nil, err
		}
		if !domain.ValidTarget(row.Target) {
			return nil, invalid("requirement target is invalid")
		}
		if row.Type != domain.Required && row.Type != domain.Preferred {
			return nil, invalid("requirement type must be required or preferred")
		}
		key := strings.ToLower(name) + "\x00" + row.Target
		if seen[key] {
			return nil, invalid("requirements must not repeat the same name and target")
		}
		seen[key] = true
		out = append(out, domain.PostingRequirement{Name: name, Target: row.Target, Type: row.Type})
	}
	return out, nil
}

func checkNumbers(salaryMin, salaryMax, duration *int32) error {
	if (salaryMin != nil && *salaryMin < 0) || (salaryMax != nil && *salaryMax < 0) {
		return invalid("salary must not be negative")
	}
	if duration != nil && *duration <= 0 {
		return invalid("interview_duration must be a positive number of minutes")
	}
	return nil
}

func requiredText(value, field string, max int) (string, error) {
	if !utf8.ValidString(value) {
		return "", invalid(field + " must be valid UTF-8")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", invalid(field + " is required")
	}
	if utf8.RuneCountInString(value) > max {
		return "", invalid(fmt.Sprintf("%s must be at most %d characters", field, max))
	}
	return value, nil
}

// optionalText trims a nullable string and turns an empty one into nil.
func optionalText(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if !utf8.ValidString(*value) {
		return nil, invalid(field + " must be valid UTF-8")
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > maxDetailLength {
		return nil, invalid(fmt.Sprintf("%s must be at most %d characters", field, maxDetailLength))
	}
	return &trimmed, nil
}
