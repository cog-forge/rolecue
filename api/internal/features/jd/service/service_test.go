package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type fakeExtractor struct {
	calls int
	fn    func(context.Context, string) (domain.ExtractionCandidate, error)
}

func (f *fakeExtractor) Extract(ctx context.Context, jd string) (domain.ExtractionCandidate, error) {
	f.calls++
	return f.fn(ctx, jd)
}
func validCandidate() domain.ExtractionCandidate {
	return domain.ExtractionCandidate{Title: "Backend Engineer", Skills: []domain.ExtractedSkill{{Name: "Go", Category: domain.ProgrammingLanguage, Requirement: domain.Required}, {Name: "PostgreSQL", Category: domain.Database}}, Technologies: []string{"Kafka"}, DomainKnowledge: []string{}}
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}
func newService(t *testing.T, f Extractor, retries int) *Service {
	t.Helper()
	s, err := New(f, retries)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInputPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		code        apperror.Code
		length      int
	}{
		{"empty", "", apperror.CodeInvalidJDInput, 0},
		{"whitespace", " \r\n\t\u2003 ", apperror.CodeInvalidJDInput, 0},
		{"invalid UTF8", string([]byte{0xff}), apperror.CodeInvalidJDInput, 0},
		{"short unicode", strings.Repeat("界", 99), apperror.CodeJDTooShort, 0},
		{"minimum unicode", strings.Repeat("界", 100), "", 100},
		{"maximum unicode", strings.Repeat("界", MaxJDLength), "", MaxJDLength},
		{"over maximum", strings.Repeat("界", MaxJDLength+1), apperror.CodeJDTooLong, 0},
		{"length after normalization", strings.Repeat(" ", MaxJDLength) + strings.Repeat("界", 100), "", 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeExtractor{fn: func(_ context.Context, jd string) (domain.ExtractionCandidate, error) {
				if utf8.RuneCountInString(jd) != tt.length {
					t.Errorf("normalized length = %d", utf8.RuneCountInString(jd))
				}
				return validCandidate(), nil
			}}
			_, err := newService(t, f, 1).Extract(context.Background(), tt.input)
			if tt.code != "" {
				assertCode(t, err, tt.code)
				if f.calls != 0 {
					t.Fatal("invalid input invoked extractor")
				}
			} else if err != nil || f.calls != 1 {
				t.Fatalf("err=%v calls=%d", err, f.calls)
			}
		})
	}
}

func TestValidExtractionAndNormalization(t *testing.T) {
	raw := " \t" + strings.Repeat("界", 100) + "\r\n\r\n\r\n  Go\t\t developer \r API\u2003work "
	wantInput := strings.Repeat("界", 100) + "\n\nGo developer\nAPI work"
	c := validCandidate()
	c.Title = " Backend Engineer \n"
	c.Skills = append(c.Skills, domain.ExtractedSkill{Name: " go ", Category: domain.ProgrammingLanguage, Requirement: domain.Required}, domain.ExtractedSkill{Name: " ", Category: domain.Tool})
	c.Technologies = []string{" Go ", "go", " ", "SQL", "ſql"}
	c.DomainKnowledge = []string{" Finance ", "finance", ""}
	f := &fakeExtractor{fn: func(_ context.Context, jd string) (domain.ExtractionCandidate, error) {
		if jd != wantInput {
			t.Errorf("input = %q, want %q", jd, wantInput)
		}
		return c, nil
	}}
	got, err := newService(t, f, 1).Extract(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.StructuredJD{Title: "Backend Engineer", Skills: []domain.ExtractedSkill{{Name: "Go", Category: domain.ProgrammingLanguage, Requirement: domain.Required}, {Name: "PostgreSQL", Category: domain.Database}}, Technologies: []string{"Go", "SQL"}, DomainKnowledge: []string{"Finance"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	if got.SeniorityLevel != nil {
		t.Fatal("invented seniority")
	}
	if c.Technologies[0] != " Go " {
		t.Fatal("mutated candidate")
	}
}

func TestCandidateValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*domain.ExtractionCandidate)
	}{
		{"blank title", func(c *domain.ExtractionCandidate) { c.Title = " " }},
		{"missing array", func(c *domain.ExtractionCandidate) { c.DomainKnowledge = nil }},
		{"invalid seniority", func(c *domain.ExtractionCandidate) { v := domain.Seniority("expert"); c.SeniorityLevel = &v }},
		{"difficulty is not seniority", func(c *domain.ExtractionCandidate) { v := domain.Seniority("hard"); c.SeniorityLevel = &v }},
		{"blank seniority", func(c *domain.ExtractionCandidate) { v := domain.Seniority(""); c.SeniorityLevel = &v }},
		{"interpersonal category", func(c *domain.ExtractionCandidate) { c.Skills[0].Category = "soft" }},
		{"invalid requirement", func(c *domain.ExtractionCandidate) { c.Skills[0].Requirement = "mandatory" }},
		{"no usable competency", func(c *domain.ExtractionCandidate) {
			c.Skills = []domain.ExtractedSkill{}
			c.Technologies = []string{" "}
		}},
		{"conflicting duplicate", func(c *domain.ExtractionCandidate) {
			c.Skills = append(c.Skills, domain.ExtractedSkill{Name: "go", Category: domain.Database, Requirement: domain.Required})
		}},
		{"invalid UTF8 name", func(c *domain.ExtractionCandidate) { c.Technologies = []string{string([]byte{0xff})} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := validCandidate()
			tt.change(&c)
			_, err := ValidateCandidate(c)
			assertCode(t, err, apperror.CodeInvalidExtractionOutput)
		})
	}
	for _, category := range []domain.SkillCategory{domain.ProgrammingLanguage, domain.Framework, domain.Database, domain.Tool, domain.Technology, domain.Other} {
		c := validCandidate()
		c.Skills[0].Category = category
		c.Skills[0].Requirement = ""
		if _, err := ValidateCandidate(c); err != nil {
			t.Fatalf("category %s: %v", category, err)
		}
	}
	for _, v := range []domain.Seniority{domain.Intern, domain.Junior, domain.Mid, domain.Senior, domain.Lead} {
		c := validCandidate()
		c.SeniorityLevel = &v
		got, err := ValidateCandidate(c)
		if err != nil || got.SeniorityLevel == nil || *got.SeniorityLevel != v {
			t.Fatalf("seniority %s: %v", v, err)
		}
		if got.SeniorityLevel == c.SeniorityLevel {
			t.Fatal("seniority aliases candidate")
		}
	}
}

func TestRetryBudgetAndCauses(t *testing.T) {
	secretCause := errors.New("internal provider detail")
	for _, tt := range []struct {
		name           string
		first          error
		invalid        bool
		retries, calls int
		code           apperror.Code
	}{
		{"permanent provider failure", secretCause, false, 1, 1, apperror.CodeExtractionFailed},
		{"malformed recovery", apperror.Wrap(apperror.CodeInvalidExtractionOutput, invalidExtractionOutputMessage, secretCause), false, 1, 2, ""},
		{"invalid candidate recovery", nil, true, 1, 2, ""},
		{"retries disabled", apperror.Wrap(apperror.CodeInvalidExtractionOutput, invalidExtractionOutputMessage, secretCause), false, 0, 1, apperror.CodeInvalidExtractionOutput},
		{"canceled", context.Canceled, false, 1, 1, apperror.CodeExtractionFailed},
		{"deadline", context.DeadlineExceeded, false, 1, 1, apperror.CodeExtractionFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeExtractor{}
			f.fn = func(context.Context, string) (domain.ExtractionCandidate, error) {
				if f.calls == 1 {
					if tt.invalid {
						return domain.ExtractionCandidate{}, nil
					}
					return validCandidate(), tt.first
				}
				return validCandidate(), nil
			}
			_, err := newService(t, f, tt.retries).Extract(context.Background(), strings.Repeat("a", 100))
			if f.calls != tt.calls {
				t.Fatalf("calls=%d want %d", f.calls, tt.calls)
			}
			if tt.code != "" {
				assertCode(t, err, tt.code)
				if !errors.Is(err, tt.first) {
					t.Fatal("cause lost")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	f := &fakeExtractor{}
	f.fn = func(context.Context, string) (domain.ExtractionCandidate, error) {
		if f.calls == 1 {
			return domain.ExtractionCandidate{}, apperror.Wrap(apperror.CodeInvalidExtractionOutput, invalidExtractionOutputMessage, secretCause)
		}
		return domain.ExtractionCandidate{}, nil
	}
	_, err := newService(t, f, 1).Extract(context.Background(), strings.Repeat("a", 100))
	assertCode(t, err, apperror.CodeInvalidExtractionOutput)
	if f.calls != 2 {
		t.Fatalf("shared budget exceeded: %d", f.calls)
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeExtractor{fn: func(context.Context, string) (domain.ExtractionCandidate, error) {
		t.Fatal("called after cancellation")
		return domain.ExtractionCandidate{}, nil
	}}
	_, err := newService(t, f, 1).Extract(ctx, strings.Repeat("a", 100))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	f.fn = func(ctx context.Context, _ string) (domain.ExtractionCandidate, error) {
		cancel()
		return validCandidate(), ctx.Err()
	}
	_, err = newService(t, f, 1).Extract(ctx, strings.Repeat("a", 100))
	if !errors.Is(err, context.Canceled) || f.calls != 1 {
		t.Fatalf("calls=%d err=%v", f.calls, err)
	}
}

func TestConstructor(t *testing.T) {
	if _, err := New(nil, 0); err == nil {
		t.Fatal("nil extractor accepted")
	}
	for _, n := range []int{-1, 2} {
		if _, err := New(&fakeExtractor{}, n); err == nil {
			t.Fatal("invalid retry budget accepted")
		}
	}
}
