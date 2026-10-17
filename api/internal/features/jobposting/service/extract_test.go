package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	roleprofiledomain "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type fakeExtractor struct {
	result roleprofiledomain.StructuredJD
	err    error
	calls  int
	raw    string
	ctx    context.Context
}

func (f *fakeExtractor) Extract(ctx context.Context, raw string) (roleprofiledomain.StructuredJD, error) {
	f.calls++
	f.raw, f.ctx = raw, ctx
	return f.result, f.err
}

func newExtraction(t *testing.T, extractor *fakeExtractor) *Extraction {
	t.Helper()
	e, err := NewExtraction(extractor, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func jdText() string { return strings.Repeat("We build Go services on PostgreSQL. ", 5) }

func TestSuggestMapsExtractionOntoPostingRequirements(t *testing.T) {
	senior := roleprofiledomain.Senior
	extractor := &fakeExtractor{result: roleprofiledomain.StructuredJD{
		Title:          "  Backend Engineer  ",
		SeniorityLevel: &senior,
		Skills: []roleprofiledomain.ExtractedSkill{
			{Name: "Go", Category: roleprofiledomain.ProgrammingLanguage, Requirement: roleprofiledomain.Required},
			{Name: "Kubernetes", Category: roleprofiledomain.Tool, Requirement: roleprofiledomain.Preferred},
			{Name: "PostgreSQL", Category: roleprofiledomain.Database},
		},
		Technologies:    []string{"gRPC", "go"},
		DomainKnowledge: []string{"Payments"},
	}}
	got, err := newExtraction(t, extractor).Suggest(context.Background(), "recruiter", jdText())
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Backend Engineer" || got.SeniorityLevel == nil || *got.SeniorityLevel != domain.Senior {
		t.Fatalf("title/seniority = %q/%v", got.Title, got.SeniorityLevel)
	}
	want := []domain.PostingRequirement{
		{Name: "Go", Target: "programming_language", Type: domain.Required},
		{Name: "Kubernetes", Target: "tool", Type: domain.Preferred},
		{Name: "PostgreSQL", Target: "database", Type: domain.Required},
		{Name: "gRPC", Target: "technologies", Type: domain.Required},
		{Name: "Payments", Target: "domain_knowledge", Type: domain.Required},
	}
	if len(got.Requirements) != len(want) {
		t.Fatalf("requirements = %+v", got.Requirements)
	}
	for i := range want {
		if got.Requirements[i] != want[i] {
			t.Fatalf("requirement %d = %+v, want %+v", i, got.Requirements[i], want[i])
		}
	}
	if extractor.raw != jdText() {
		t.Fatal("extractor did not get the JD text")
	}
	if _, ok := extractor.ctx.Deadline(); !ok {
		t.Fatal("extraction ran without a time budget")
	}
}

// Confirming a suggestion unchanged must pass Create's checks, so the suggestion stays inside them.
func TestSuggestStaysWithinCreateLimits(t *testing.T) {
	skills := make([]roleprofiledomain.ExtractedSkill, 0, maxRequirements+5)
	for i := 0; i < maxRequirements+5; i++ {
		skills = append(skills, roleprofiledomain.ExtractedSkill{Name: "skill " + strings.Repeat("x", i), Category: "unknown"})
	}
	skills = append(skills, roleprofiledomain.ExtractedSkill{Name: "   "})
	extractor := &fakeExtractor{result: roleprofiledomain.StructuredJD{Title: strings.Repeat("t", maxTitleLength+10), Skills: skills}}
	got, err := newExtraction(t, extractor).Suggest(context.Background(), "recruiter", jdText())
	if err != nil {
		t.Fatal(err)
	}
	if got.SeniorityLevel != nil {
		t.Fatalf("seniority = %v, want nil", *got.SeniorityLevel)
	}
	if n := len([]rune(got.Title)); n != maxTitleLength {
		t.Fatalf("title has %d runes", n)
	}
	if _, err := checkRequirements(got.Requirements); err != nil {
		t.Fatalf("suggested requirements fail Create's checks: %v", err)
	}
	if len(got.Requirements) != maxRequirements || got.Requirements[0].Target != "other" {
		t.Fatalf("got %d requirements, first %+v", len(got.Requirements), got.Requirements[0])
	}
}

func TestSuggestReturnsEmptyListWhenNothingFound(t *testing.T) {
	got, err := newExtraction(t, &fakeExtractor{}).Suggest(context.Background(), "recruiter", jdText())
	if err != nil {
		t.Fatal(err)
	}
	if got.Requirements == nil || len(got.Requirements) != 0 {
		t.Fatalf("requirements = %#v, want empty list", got.Requirements)
	}
}

func TestSuggestIsRecruiterOnly(t *testing.T) {
	for _, role := range []string{"candidate", "admin", ""} {
		extractor := &fakeExtractor{}
		_, err := newExtraction(t, extractor).Suggest(context.Background(), role, jdText())
		assertCode(t, err, apperror.CodeForbidden)
		if extractor.calls != 0 {
			t.Fatalf("%q: extractor was called", role)
		}
	}
}

func TestSuggestChecksTextBeforeCallingTheModel(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		code apperror.Code
	}{
		{"too short", apperror.CodeJDTooShort},
		{strings.Repeat("a", MaxTextLength+1), apperror.CodeJDTooLong},
		{"\xff" + jdText(), apperror.CodeInvalidJDInput},
	} {
		extractor := &fakeExtractor{}
		_, err := newExtraction(t, extractor).Suggest(context.Background(), "recruiter", tc.raw)
		assertCode(t, err, tc.code)
		if extractor.calls != 0 {
			t.Fatalf("%s: extractor was called", tc.code)
		}
	}
}

func TestSuggestPassesExtractionErrorsThrough(t *testing.T) {
	want := apperror.New(apperror.CodeExtractionFailed, "Job description extraction failed.")
	_, err := newExtraction(t, &fakeExtractor{err: want}).Suggest(context.Background(), "recruiter", jdText())
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestNewExtractionNeedsExtractorAndBudget(t *testing.T) {
	if _, err := NewExtraction(nil, time.Minute); err == nil {
		t.Fatal("nil extractor accepted")
	}
	if _, err := NewExtraction(&fakeExtractor{}, 0); err == nil {
		t.Fatal("zero budget accepted")
	}
}
