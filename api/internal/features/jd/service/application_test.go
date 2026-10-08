package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"strings"
	"testing"
)

type fakeRepository struct {
	calls         int
	user, id      uuid.UUID
	create        domain.CreateInput
	update        domain.UpdateInput
	err           error
	limit, offset int32
}

func (f *fakeRepository) Create(_ context.Context, u uuid.UUID, in domain.CreateInput) (domain.JD, error) {
	f.calls++
	f.user = u
	f.create = in
	return domain.JD{}, f.err
}
func (f *fakeRepository) List(_ context.Context, u uuid.UUID, limit, offset int32) ([]domain.ListItem, int64, error) {
	f.calls++
	f.user = u
	f.limit, f.offset = limit, offset
	return nil, 0, f.err
}
func (f *fakeRepository) Get(_ context.Context, u, id uuid.UUID) (domain.JD, error) {
	f.calls++
	f.user = u
	f.id = id
	return domain.JD{}, f.err
}
func (f *fakeRepository) Update(_ context.Context, u, id uuid.UUID, in domain.UpdateInput) (domain.JD, error) {
	f.calls++
	f.user = u
	f.id = id
	f.update = in
	return domain.JD{}, f.err
}
func (f *fakeRepository) Delete(_ context.Context, u, id uuid.UUID) error {
	f.calls++
	f.user = u
	f.id = id
	return f.err
}

func TestApplicationBoundaries(t *testing.T) {
	ctx := context.Background()
	raw := strings.Repeat("a", 100)
	extractor := &fakeExtractor{fn: func(_ context.Context, got string) (domain.ExtractionCandidate, error) {
		if got != raw {
			t.Fatal("normalization changed")
		}
		return validCandidate(), nil
	}}
	repo := &fakeRepository{}
	app, err := NewApplication(newService(t, extractor, 0), repo)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Analyze(ctx, " "+raw+" ")
	if err != nil || result.SeniorityLevel != nil || extractor.calls != 1 || repo.calls != 0 {
		t.Fatalf("analyze boundary: %+v %v", result, err)
	}
	user, id := uuid.New(), uuid.New()
	seniority := domain.Senior
	result.SeniorityLevel = &seniority
	create := domain.CreateInput{RawText: " " + raw + "\r\n", StructuredJD: result}
	update := domain.UpdateInput{StructuredJD: result}
	sentinel := apperror.New(apperror.CodeJDNotFound, "job description not found")
	repo.err = sentinel
	operations := []func() error{
		func() error { _, e := app.Create(ctx, user, create); return e },
		func() error { _, _, e := app.List(ctx, user, 10, 5); return e },
		func() error { _, e := app.Get(ctx, user, id); return e },
		func() error { _, e := app.Update(ctx, user, id, update); return e },
		func() error { return app.Delete(ctx, user, id) },
	}
	for i, op := range operations {
		before := repo.calls
		if err := op(); !errors.Is(err, sentinel) || repo.user != user || repo.calls != before+1 {
			t.Fatalf("operation %d: %v", i, err)
		}
		if i >= 2 && repo.id != id {
			t.Fatal("ID lost")
		}
	}
	if repo.create.RawText != create.RawText || repo.update.StructuredJD.Title != result.Title {
		t.Fatal("inputs changed")
	}
	if repo.limit != 10 || repo.offset != 5 {
		t.Fatal("pagination not forwarded")
	}
}

func TestReviewedApplicationGates(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepository{}
	app, err := NewApplication(newService(t, &fakeExtractor{}, 0), repo)
	if err != nil {
		t.Fatal(err)
	}
	level := domain.Senior
	valid := domain.StructuredJD(validCandidate())
	valid.SeniorityLevel = &level
	raw := "  " + strings.Repeat("original ", 20) + "\r\n"
	for _, tc := range []struct {
		raw  string
		code apperror.Code
	}{
		{"", apperror.CodeInvalidJDInput}, {"short", apperror.CodeJDTooShort},
		{string([]byte{0xff}), apperror.CodeInvalidJDInput},
		{strings.Repeat("a", MaxJDLength+1), apperror.CodeJDTooLong},
	} {
		_, err := app.Create(ctx, uuid.New(), domain.CreateInput{RawText: tc.raw, StructuredJD: valid})
		assertCode(t, err, tc.code)
	}
	invalid := valid
	invalid.SeniorityLevel = nil
	_, err = app.Create(ctx, uuid.New(), domain.CreateInput{RawText: raw, StructuredJD: invalid})
	assertCode(t, err, apperror.CodeValidation)
	invalid = valid
	invalid.Skills = []domain.ExtractedSkill{{Name: "Go", Category: domain.Tool}, {Name: "go", Category: domain.Database}}
	_, err = app.Update(ctx, uuid.New(), uuid.New(), domain.UpdateInput{StructuredJD: invalid})
	assertCode(t, err, apperror.CodeValidation)
	if repo.calls != 0 {
		t.Fatal("invalid input reached persistence")
	}
	valid.Title = "  Engineer  "
	valid.Technologies = []string{" Go ", "go", " "}
	valid.Skills = []domain.ExtractedSkill{{Name: " SQL ", Category: domain.Database}, {Name: "sql", Category: domain.Database}}
	for _, update := range []bool{false, true} {
		if update {
			_, err = app.Update(ctx, uuid.New(), uuid.New(), domain.UpdateInput{StructuredJD: valid})
		} else {
			_, err = app.Create(ctx, uuid.New(), domain.CreateInput{RawText: raw, StructuredJD: valid})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, got := range []domain.StructuredJD{repo.create.StructuredJD, repo.update.StructuredJD} {
		if got.Title != "Engineer" || len(got.Technologies) != 1 || got.Technologies[0] != "Go" || len(got.Skills) != 1 || got.Skills[0].Name != "SQL" {
			t.Fatalf("not normalized: %+v", got)
		}
	}
	if repo.create.RawText != raw {
		t.Fatal("original raw text was normalized")
	}
}
