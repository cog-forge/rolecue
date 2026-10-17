package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	roleprofiledomain "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
)

// JDExtractor is the shared JD extraction pipeline (the Role Profile one), which owns retries and
// validates the model output.
type JDExtractor interface {
	Extract(ctx context.Context, raw string) (roleprofiledomain.StructuredJD, error)
}

// Extraction suggests a title, seniority and requirements from a Recruiter's JD text. It stores
// nothing: the Recruiter confirms or edits the suggestion and then creates the posting.
type Extraction struct {
	extractor JDExtractor
	budget    time.Duration
}

func NewExtraction(extractor JDExtractor, budget time.Duration) (*Extraction, error) {
	if extractor == nil || budget <= 0 {
		return nil, fmt.Errorf("job posting extraction needs an extractor and a positive time budget")
	}
	return &Extraction{extractor: extractor, budget: budget}, nil
}

func (e *Extraction) Suggest(ctx context.Context, role, raw string) (domain.Suggestion, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.Suggestion{}, err
	}
	if err := checkText(raw); err != nil {
		return domain.Suggestion{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, e.budget)
	defer cancel()
	jd, err := e.extractor.Extract(ctx, raw)
	if err != nil {
		return domain.Suggestion{}, err
	}
	return suggestionFrom(jd), nil
}

// suggestionFrom maps extraction output onto posting requirements: a skill keeps its category as
// the target, and plain names go to the technologies and domain_knowledge lists. A skill whose JD
// does not say required or preferred is suggested as required; the Recruiter decides in review.
// The result is kept within the limits Create checks, so confirming it unchanged is accepted.
func suggestionFrom(jd roleprofiledomain.StructuredJD) domain.Suggestion {
	out := domain.Suggestion{Title: clip(strings.TrimSpace(jd.Title), maxTitleLength), Requirements: []domain.PostingRequirement{}}
	if jd.SeniorityLevel != nil {
		if level := domain.Level(*jd.SeniorityLevel); level.Valid() {
			out.SeniorityLevel = &level
		}
	}
	seen := map[string]bool{}
	add := func(name, target string, kind domain.RequirementType) {
		name = clip(strings.TrimSpace(name), maxNameLength)
		key := strings.ToLower(name)
		if name == "" || seen[key] || len(out.Requirements) >= maxRequirements {
			return
		}
		seen[key] = true
		out.Requirements = append(out.Requirements, domain.PostingRequirement{Name: name, Target: target, Type: kind})
	}
	for _, skill := range jd.Skills {
		target := string(skill.Category)
		if !domain.ValidTarget(target) {
			target = "other"
		}
		kind := domain.Required
		if skill.Requirement == roleprofiledomain.Preferred {
			kind = domain.Preferred
		}
		add(skill.Name, target, kind)
	}
	for _, name := range jd.Technologies {
		add(name, "technologies", domain.Required)
	}
	for _, name := range jd.DomainKnowledge {
		add(name, "domain_knowledge", domain.Required)
	}
	return out
}

func clip(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	return strings.TrimSpace(string([]rune(value)[:max]))
}
