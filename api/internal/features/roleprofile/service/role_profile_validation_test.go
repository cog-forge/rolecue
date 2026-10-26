package service

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
)

func TestReviewFromExtractionOnlyKeepsProficiencyWithMatchingExplicitEvidence(t *testing.T) {
	dockerEvidence := "Engineers should be proficient with Docker."
	redisEvidence := "Engineers should be familiar with Redis caching."
	goEvidence := "Required skills include Go concurrency."
	jd := domain.StructuredJD{
		Title: "Backend Engineer",
		Skills: []domain.ExtractedSkill{
			{Name: "Docker", Category: domain.Tool, Target: "Proficient", EvidenceText: &dockerEvidence},
			{Name: "Redis caching", Category: domain.Database, Target: "Familiar", EvidenceText: &redisEvidence},
			{Name: "Go concurrency", Category: domain.ProgrammingLanguage, Target: "Expert", EvidenceText: &goEvidence},
			{Name: "REST API design", Category: domain.Other, Target: "Expert"},
		},
		Technologies: []string{}, DomainKnowledge: []string{},
	}

	review := ReviewFromExtraction(jd)
	got := make(map[string]string, len(review.Requirements))
	for _, requirement := range review.Requirements {
		got[requirement.Name] = requirement.Target
	}
	for name, want := range map[string]string{
		"Docker":          "Proficient",
		"Redis caching":   "Familiar",
		"Go concurrency":  "Unspecified",
		"REST API design": "Unspecified",
	} {
		if got[name] != want {
			t.Errorf("target for %q = %q, want %q", name, got[name], want)
		}
	}
}
