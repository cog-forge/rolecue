package service

import (
	"cmp"
	"slices"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
)

// requirementRows flattens a reviewed JD into requirements rows. The table has no "unknown"
// requirement type, so a skill whose requirement the JD doesn't state is stored as preferred:
// it isn't explicitly required. Technologies and domain knowledge carry no flag at all.
func requirementRows(jd domain.StructuredJD) []domain.RequirementRow {
	rows := make([]domain.RequirementRow, 0, len(jd.Skills)+len(jd.Technologies)+len(jd.DomainKnowledge))
	for _, skill := range jd.Skills {
		requirement := skill.Requirement
		if requirement == "" {
			requirement = domain.Preferred
		}
		rows = append(rows, domain.RequirementRow{Name: skill.Name, Target: string(skill.Category), Type: requirement})
	}
	for _, name := range jd.Technologies {
		rows = append(rows, domain.RequirementRow{Name: name, Target: domain.TargetTechnologies, Type: domain.Preferred})
	}
	for _, name := range jd.DomainKnowledge {
		rows = append(rows, domain.RequirementRow{Name: name, Target: domain.TargetDomainKnowledge, Type: domain.Preferred})
	}
	sortRows(rows)
	return rows
}

// sortRows puts rows in the order ListRequirements returns them (target, then name, byte-wise:
// the query sorts with COLLATE "C"). The table has no position column, so this is the one
// stable order, and Create answers in it too.
func sortRows(rows []domain.RequirementRow) {
	slices.SortStableFunc(rows, func(a, b domain.RequirementRow) int {
		return cmp.Or(cmp.Compare(a.Target, b.Target), cmp.Compare(a.Name, b.Name))
	})
}

// structuredJD rebuilds the reviewed structure from stored rows. Arrays are never nil, so the
// JSON shows [] for an empty list.
func structuredJD(record domain.Record) domain.StructuredJD {
	level := record.Seniority
	jd := domain.StructuredJD{Title: record.Title, SeniorityLevel: &level,
		Skills: []domain.ExtractedSkill{}, Technologies: []string{}, DomainKnowledge: []string{}}
	for _, row := range record.Requirements {
		switch row.Target {
		case domain.TargetTechnologies:
			jd.Technologies = append(jd.Technologies, row.Name)
		case domain.TargetDomainKnowledge:
			jd.DomainKnowledge = append(jd.DomainKnowledge, row.Name)
		default:
			category := domain.SkillCategory(row.Target)
			if !validSkillCategory(category) {
				category = domain.Other
			}
			jd.Skills = append(jd.Skills, domain.ExtractedSkill{Name: row.Name, Category: category, Requirement: row.Type})
		}
	}
	return jd
}
