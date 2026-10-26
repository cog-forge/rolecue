package service

import (
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func ValidateRoleReview(in domain.RoleReview, confirming bool) (domain.RoleReview, error) {
	invalid := func(message string) (domain.RoleReview, error) {
		return domain.RoleReview{}, apperror.New(apperror.CodeValidation, message)
	}
	in.Title = strings.TrimSpace(in.Title)
	in.RefinementNote = strings.TrimSpace(in.RefinementNote)
	if !boundedText(in.Title, 1, 200) {
		return invalid("Role title must contain 1–200 characters.")
	}
	if !boundedText(in.RefinementNote, 0, 2000) {
		return invalid("Refinement note must contain at most 2000 characters.")
	}
	if confirming && in.SeniorityLevel == nil {
		return invalid("Select a seniority level before confirming.")
	}
	if in.SeniorityLevel != nil {
		switch *in.SeniorityLevel {
		case domain.Intern, domain.Junior, domain.Mid, domain.Senior, domain.Lead:
		default:
			return invalid("Invalid seniority level.")
		}
	}
	if len(in.Requirements) == 0 || len(in.Requirements) > 200 {
		return invalid("Keep between 1 and 200 technical requirements.")
	}
	items := make([]domain.RoleRequirement, 0, len(in.Requirements))
	seen := make(map[string]domain.RoleRequirement)
	for _, item := range in.Requirements {
		item.Name = strings.TrimSpace(item.Name)
		item.Target = strings.TrimSpace(item.Target)
		if !boundedText(item.Name, 1, 200) || !boundedText(item.Target, 1, 2000) {
			return invalid("Each requirement needs a name and preparation target.")
		}
		if isSoftSkill(item.Name) {
			return invalid("Keep technical requirements only; remove behavioral or soft skills.")
		}
		switch item.Kind {
		case "skill":
			if item.Category == nil || !validSkillCategory(*item.Category) {
				return invalid("Select a valid technical skill category.")
			}
		case "technology", "domain_knowledge":
			if item.Category != nil {
				return invalid("Only skills have a category.")
			}
		default:
			return invalid("Invalid requirement kind.")
		}
		if item.Type != nil && *item.Type != domain.Required && *item.Type != domain.Preferred {
			return invalid("Invalid requirement type.")
		}
		if item.EvidenceText != nil {
			value := strings.TrimSpace(*item.EvidenceText)
			if !boundedText(value, 0, 4000) {
				return invalid("Requirement evidence is too long or invalid.")
			}
			item.EvidenceText = &value
		}
		key := strings.ToLower(item.Name)
		if previous, ok := seen[key]; ok {
			previous.Name = item.Name
			if !reflect.DeepEqual(previous, item) {
				return invalid("Duplicate requirements have conflicting details.")
			}
			continue
		}
		seen[key] = item
		items = append(items, item)
	}
	in.Requirements = items
	return in, nil
}

func boundedText(value string, min, max int) bool {
	length := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && length >= min && length <= max
}

func isSoftSkill(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "communication", "communication skills", "teamwork", "team work", "punctuality", "time management", "interpersonal skills", "leadership", "adaptability", "work ethic", "attention to detail", "giao tiếp", "kỹ năng giao tiếp", "làm việc nhóm", "đúng giờ":
		return true
	default:
		return false
	}
}

func ReviewFromExtraction(jd domain.StructuredJD) domain.RoleReview {
	result := domain.RoleReview{Title: jd.Title, SeniorityLevel: jd.SeniorityLevel, Requirements: make([]domain.RoleRequirement, 0)}
	seen := make(map[string]bool)
	for _, skill := range jd.Skills {
		category := skill.Category
		var requirement *domain.Requirement
		if skill.Requirement != "" {
			value := skill.Requirement
			requirement = &value
		}
		target := skill.Target
		if strings.TrimSpace(target) == "" {
			target = "Unspecified"
		}
		if !targetHasExplicitEvidence(target, skill.Name, skill.EvidenceText) {
			target = "Unspecified"
		}
		result.Requirements = append(result.Requirements, domain.RoleRequirement{Name: skill.Name, Target: target, Kind: "skill", Category: &category, Type: requirement, EvidenceText: skill.EvidenceText})
		seen[strings.ToLower(skill.Name)] = true
	}
	for _, group := range []struct {
		kind  string
		names []string
	}{{"technology", jd.Technologies}, {"domain_knowledge", jd.DomainKnowledge}} {
		for _, name := range group.names {
			if seen[strings.ToLower(name)] {
				continue
			}
			result.Requirements = append(result.Requirements, domain.RoleRequirement{Name: name, Target: "Unspecified", Kind: group.kind})
			seen[strings.ToLower(name)] = true
		}
	}
	return result
}

func targetHasExplicitEvidence(target, name string, evidence *string) bool {
	if strings.EqualFold(strings.TrimSpace(target), "Unspecified") {
		return true
	}
	if evidence == nil {
		return false
	}
	quote := strings.ToLower(strings.Join(strings.Fields(*evidence), " "))
	if !strings.Contains(quote, strings.ToLower(strings.TrimSpace(name))) {
		return false
	}
	var cues []string
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "new":
		cues = []string{"new to", "beginner in", "beginner with", "no experience with", "no prior experience with"}
	case "familiar":
		cues = []string{"familiar with", "familiarity with", "basic familiarity with", "basic knowledge of", "working knowledge of"}
	case "proficient":
		cues = []string{"proficient with", "proficient in", "proficiency with", "proficiency in"}
	case "expert":
		cues = []string{"expert in", "expert with", "expertise in", "expertise with", "deep expertise in"}
	default:
		return false
	}
	for _, cue := range cues {
		if strings.Contains(quote, cue) {
			return true
		}
	}
	return false
}

// Evidence is a quote from source data, not a second source of AI-authored facts.
func ValidateEvidence(review domain.RoleReview, raw string) error {
	source := strings.Join(strings.Fields(raw), " ")
	for _, item := range review.Requirements {
		if item.EvidenceText == nil || strings.TrimSpace(*item.EvidenceText) == "" {
			continue
		}
		excerpt := strings.Join(strings.Fields(*item.EvidenceText), " ")
		if !strings.Contains(source, excerpt) {
			return apperror.New(apperror.CodeValidation, "Requirement evidence must be an excerpt of the original job description.")
		}
	}
	return nil
}
