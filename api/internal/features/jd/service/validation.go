package service

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

const (
	MinJDLength = 100
	MaxJDLength = 20_000

	invalidJDInputMessage          = "Job description input is invalid."
	jdTooShortMessage              = "Job description must contain at least 100 Unicode characters."
	jdTooLongMessage               = "Job description must contain at most 20000 Unicode characters."
	invalidExtractionOutputMessage = "Extraction did not produce a usable job description."
)

// NormalizeInput validates UTF-8, normalizes line endings/whitespace,
// and enforces the JD character-count range.
func NormalizeInput(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", apperror.Wrap(apperror.CodeInvalidJDInput, invalidJDInputMessage, fmt.Errorf("input is not valid UTF-8"))
	}
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	lines := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" && (len(lines) == 0 || lines[len(lines)-1] == "") {
			continue
		}
		lines = append(lines, line)
	}
	normalized := strings.TrimSpace(strings.Join(lines, "\n"))
	length := utf8.RuneCountInString(normalized)
	if length == 0 {
		return "", apperror.Wrap(apperror.CodeInvalidJDInput, invalidJDInputMessage, fmt.Errorf("input is empty"))
	}
	if length < MinJDLength {
		return "", apperror.Wrap(apperror.CodeJDTooShort, jdTooShortMessage, fmt.Errorf("normalized length %d", length))
	}
	if length > MaxJDLength {
		return "", apperror.Wrap(apperror.CodeJDTooLong, jdTooLongMessage, fmt.Errorf("normalized length %d exceeds %d", length, MaxJDLength))
	}
	return normalized, nil
}

// ValidateCandidate never relies on the provider's schema enforcement. First
// occurrence wins for case-insensitive duplicate names. Conflicting duplicate
// skill metadata is rejected rather than silently inventing a resolution.
func ValidateCandidate(c domain.ExtractionCandidate) (domain.StructuredJD, error) {
	return validateStructure(c, false)
}

// ValidateReviewed applies the shared competency contract to user-reviewed input.
// Unlike extraction, persistence requires an explicitly selected seniority.
func ValidateReviewed(jd domain.StructuredJD) (domain.StructuredJD, error) {
	return validateStructure(domain.ExtractionCandidate(jd), true)
}

func validateStructure(c domain.ExtractionCandidate, reviewed bool) (domain.StructuredJD, error) {
	invalid := func(reason string) (domain.StructuredJD, error) {
		if reviewed {
			return domain.StructuredJD{}, apperror.New(apperror.CodeValidation, reason)
		}
		return domain.StructuredJD{}, apperror.Wrap(apperror.CodeInvalidExtractionOutput, invalidExtractionOutputMessage, errors.New(reason))
	}
	if reviewed && c.SeniorityLevel == nil {
		return invalid("reviewed JD requires seniority")
	}
	if !utf8.ValidString(c.Title) || strings.TrimSpace(c.Title) == "" {
		return invalid("title is required")
	}
	if c.Skills == nil || c.Technologies == nil || c.DomainKnowledge == nil {
		return invalid("skills, technologies and domainKnowledge arrays are required")
	}
	var seniority *domain.Seniority
	if c.SeniorityLevel != nil {
		switch *c.SeniorityLevel {
		case domain.Intern, domain.Junior, domain.Mid, domain.Senior, domain.Lead:
			value := *c.SeniorityLevel
			seniority = &value
		default:
			return invalid("invalid seniorityLevel")
		}
	}
	skills := make([]domain.ExtractedSkill, 0, len(c.Skills))
	for _, skill := range c.Skills {
		if !validSkillCategory(skill.Category) {
			return invalid("invalid skill category")
		}
		if skill.Requirement != "" && skill.Requirement != domain.Required && skill.Requirement != domain.Preferred {
			return invalid("invalid skill requirement")
		}
		if !utf8.ValidString(skill.Name) {
			return invalid("invalid UTF-8 skill name")
		}
		skill.Name = strings.TrimSpace(skill.Name)
		if skill.Name == "" {
			continue
		}
		duplicate := false
		for _, existing := range skills {
			if strings.EqualFold(existing.Name, skill.Name) {
				if existing.Category != skill.Category || existing.Requirement != skill.Requirement {
					return invalid("conflicting duplicate skill metadata")
				}
				duplicate = true
				break
			}
		}
		if !duplicate {
			skills = append(skills, skill)
		}
	}
	technologies, err := normalizeNames(c.Technologies)
	if err != nil {
		return invalid(err.Error())
	}
	domains, err := normalizeNames(c.DomainKnowledge)
	if err != nil {
		return invalid(err.Error())
	}
	if len(skills)+len(technologies)+len(domains) == 0 {
		return invalid("at least one nonblank competency is required")
	}
	return domain.StructuredJD{Title: strings.TrimSpace(c.Title), SeniorityLevel: seniority, Skills: skills, Technologies: technologies, DomainKnowledge: domains}, nil
}

func validSkillCategory(category domain.SkillCategory) bool {
	switch category {
	case domain.ProgrammingLanguage, domain.Framework, domain.Database, domain.Tool, domain.Technology, domain.Other:
		return true
	default:
		return false
	}
}

func normalizeNames(names []string) ([]string, error) {
	result := make([]string, 0, len(names))
	for _, name := range names {
		if !utf8.ValidString(name) {
			return nil, fmt.Errorf("invalid UTF-8 competency name")
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		duplicate := false
		for _, existing := range result {
			if strings.EqualFold(existing, name) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, name)
		}
	}
	return result, nil
}
