package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	calls         int
	updates       int
	user, id      uuid.UUID
	saved         domain.Record
	stored        domain.Record
	err           error
	limit, offset int32
}

func (f *fakeRepository) Create(_ context.Context, u uuid.UUID, in domain.Record) (domain.Record, error) {
	f.calls++
	f.user = u
	f.saved = in
	in.ID, in.CreatedAt = uuid.New(), time.Now()
	return in, f.err
}
func (f *fakeRepository) List(_ context.Context, u uuid.UUID, limit, offset int32) ([]domain.ListItem, int64, error) {
	f.calls++
	f.user = u
	f.limit, f.offset = limit, offset
	return nil, 0, f.err
}
func (f *fakeRepository) Get(_ context.Context, u, id uuid.UUID) (domain.Record, error) {
	f.calls++
	f.user = u
	f.id = id
	return f.stored, f.err
}
func (f *fakeRepository) Update(_ context.Context, u, id uuid.UUID, in domain.Record) (domain.Record, error) {
	f.calls++
	f.updates++
	f.user = u
	f.id = id
	f.saved = in
	in.ID, in.ContentPath = id, f.stored.ContentPath
	return in, f.err
}
func (f *fakeRepository) Delete(_ context.Context, u, id uuid.UUID) error {
	f.calls++
	f.user = u
	f.id = id
	return f.err
}

type fakeStore struct {
	objects     map[string]string
	contentType string
	uploadErr   error
	downloadErr error
	uploads     int
}

func newFakeStore() *fakeStore { return &fakeStore{objects: map[string]string{}} }

func (f *fakeStore) Upload(_ context.Context, body io.ReadSeeker, contentType string) (string, error) {
	f.uploads++
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	key := uuid.NewString()
	f.objects[key] = string(data)
	f.contentType = contentType
	return key, nil
}
func (f *fakeStore) Download(_ context.Context, key string) (storage.Object, error) {
	if f.downloadErr != nil {
		return storage.Object{}, f.downloadErr
	}
	text, ok := f.objects[key]
	if !ok {
		return storage.Object{}, storage.ErrNotFound
	}
	return storage.Object{Body: io.NopCloser(strings.NewReader(text)), Size: int64(len(text))}, nil
}

func newApplication(t *testing.T, repo Repository, store TextStore) *Application {
	t.Helper()
	app, err := NewApplication(newService(t, &fakeExtractor{fn: func(context.Context, string) (domain.ExtractionCandidate, error) {
		return validCandidate(), nil
	}}, 0), repo, store)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func reviewedJD() domain.StructuredJD {
	level := domain.Senior
	jd := domain.StructuredJD(validCandidate())
	jd.SeniorityLevel = &level
	jd.DomainKnowledge = []string{"Fintech"}
	return jd
}

func TestApplicationRequiresDependencies(t *testing.T) {
	if _, err := NewApplication(newService(t, &fakeExtractor{}, 0), &fakeRepository{}, nil); err == nil {
		t.Fatal("a missing text store must be rejected")
	}
}

func TestAnalyzeDoesNotPersist(t *testing.T) {
	repo, store := &fakeRepository{}, newFakeStore()
	result, err := newApplication(t, repo, store).Analyze(context.Background(), " "+strings.Repeat("a", 100)+" ")
	if err != nil || result.SeniorityLevel != nil || repo.calls != 0 || store.uploads != 0 {
		t.Fatalf("analyze boundary: %+v %v", result, err)
	}
}

func TestCreateStoresOriginalTextAndRows(t *testing.T) {
	repo, store := &fakeRepository{}, newFakeStore()
	user := uuid.New()
	raw := "  " + strings.Repeat("original ", 20) + "\r\n"
	got, err := newApplication(t, repo, store).Create(context.Background(), user, domain.CreateInput{RawText: raw, StructuredJD: reviewedJD()})
	if err != nil {
		t.Fatal(err)
	}
	if store.objects[repo.saved.ContentPath] != raw || store.contentType != jdTextContentType {
		t.Fatalf("the original, un-normalized text must be stored as plain text; got %q (%s)", store.objects[repo.saved.ContentPath], store.contentType)
	}
	if repo.user != user || repo.saved.Title != "Backend Engineer" || repo.saved.Seniority != domain.Senior {
		t.Fatalf("saved record = %+v", repo.saved)
	}
	// Sorted by target, then name: the same order ListRequirements returns.
	want := []domain.RequirementRow{
		{Name: "PostgreSQL", Target: "database", Type: domain.Preferred},
		{Name: "Fintech", Target: domain.TargetDomainKnowledge, Type: domain.Preferred},
		{Name: "Go", Target: "programming_language", Type: domain.Required},
		{Name: "Kafka", Target: domain.TargetTechnologies, Type: domain.Preferred},
	}
	if len(repo.saved.Requirements) != len(want) {
		t.Fatalf("rows = %+v", repo.saved.Requirements)
	}
	for i, row := range want {
		if repo.saved.Requirements[i] != row {
			t.Fatalf("row %d = %+v, want %+v", i, repo.saved.Requirements[i], row)
		}
	}
	if got.RawText != raw || got.StructuredJD.Title != "Backend Engineer" || len(got.StructuredJD.Technologies) != 1 {
		t.Fatalf("response = %+v", got)
	}
}

func TestCreateRejectsInvalidInputBeforeStorage(t *testing.T) {
	repo, store := &fakeRepository{}, newFakeStore()
	app := newApplication(t, repo, store)
	valid := reviewedJD()
	raw := strings.Repeat("original ", 20)
	for _, tc := range []struct {
		raw  string
		code apperror.Code
	}{
		{"", apperror.CodeInvalidJDInput}, {"short", apperror.CodeJDTooShort},
		{string([]byte{0xff}), apperror.CodeInvalidJDInput},
		{strings.Repeat("a", MaxJDLength+1), apperror.CodeJDTooLong},
	} {
		_, err := app.Create(context.Background(), uuid.New(), domain.CreateInput{RawText: tc.raw, StructuredJD: valid})
		assertCode(t, err, tc.code)
	}
	noLevel := valid
	noLevel.SeniorityLevel = nil
	_, err := app.Create(context.Background(), uuid.New(), domain.CreateInput{RawText: raw, StructuredJD: noLevel})
	assertCode(t, err, apperror.CodeValidation)
	if repo.calls != 0 || store.uploads != 0 {
		t.Fatal("invalid input reached storage or persistence")
	}
}

func TestCreateStorageFailureSavesNothing(t *testing.T) {
	repo, store := &fakeRepository{}, newFakeStore()
	store.uploadErr = errors.New("bucket unreachable")
	_, err := newApplication(t, repo, store).Create(context.Background(), uuid.New(), domain.CreateInput{RawText: strings.Repeat("original ", 20), StructuredJD: reviewedJD()})
	assertCode(t, err, apperror.CodeInternal)
	if repo.calls != 0 {
		t.Fatal("rows must not be written without the stored text")
	}
}

func TestGetRebuildsJDFromRowsAndText(t *testing.T) {
	store := newFakeStore()
	key, _ := store.Upload(context.Background(), strings.NewReader("Original JD text"), jdTextContentType)
	repo := &fakeRepository{stored: domain.Record{ID: uuid.New(), ContentPath: key, Title: "Data Engineer", Seniority: domain.Mid, Requirements: []domain.RequirementRow{
		{Name: "Airflow", Target: "tool", Type: domain.Required},
		{Name: "Docker", Target: domain.TargetTechnologies, Type: domain.Preferred},
		{Name: "Banking", Target: domain.TargetDomainKnowledge, Type: domain.Preferred},
		{Name: "Mystery", Target: "not_a_category", Type: domain.Preferred},
	}}}
	user, id := uuid.New(), uuid.New()
	got, err := newApplication(t, repo, store).Get(context.Background(), user, id)
	if err != nil {
		t.Fatal(err)
	}
	if repo.user != user || repo.id != id {
		t.Fatal("owner or ID not forwarded")
	}
	jd := got.StructuredJD
	if got.RawText != "Original JD text" || jd.Title != "Data Engineer" || *jd.SeniorityLevel != domain.Mid {
		t.Fatalf("jd = %+v", got)
	}
	if len(jd.Skills) != 2 || jd.Skills[0] != (domain.ExtractedSkill{Name: "Airflow", Category: domain.Tool, Requirement: domain.Required}) || jd.Skills[1].Category != domain.Other {
		t.Fatalf("skills = %+v", jd.Skills)
	}
	if len(jd.Technologies) != 1 || jd.Technologies[0] != "Docker" || len(jd.DomainKnowledge) != 1 || jd.DomainKnowledge[0] != "Banking" {
		t.Fatalf("lists = %+v / %+v", jd.Technologies, jd.DomainKnowledge)
	}
}

func TestGetEmptyListsAreNotNil(t *testing.T) {
	store := newFakeStore()
	key, _ := store.Upload(context.Background(), strings.NewReader("text"), jdTextContentType)
	repo := &fakeRepository{stored: domain.Record{ContentPath: key, Title: "T", Seniority: domain.Junior}}
	got, err := newApplication(t, repo, store).Get(context.Background(), uuid.New(), uuid.New())
	if err != nil || got.StructuredJD.Skills == nil || got.StructuredJD.Technologies == nil || got.StructuredJD.DomainKnowledge == nil {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestGetStorageFailures(t *testing.T) {
	repo := &fakeRepository{stored: domain.Record{ContentPath: "missing", Seniority: domain.Junior}}
	_, err := newApplication(t, repo, newFakeStore()).Get(context.Background(), uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeInternal)

	store := newFakeStore()
	store.objects["big"] = strings.Repeat("a", maxStoredJDBytes+1)
	repo.stored.ContentPath = "big"
	_, err = newApplication(t, repo, store).Get(context.Background(), uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeInternal)
}

func TestRepositoryErrorsPassThrough(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	sentinel := apperror.New(apperror.CodeJDNotFound, "job description not found")
	repo := &fakeRepository{err: sentinel}
	app := newApplication(t, repo, store)
	user, id := uuid.New(), uuid.New()
	raw := strings.Repeat("original ", 20)
	operations := []func() error{
		func() error {
			_, e := app.Create(ctx, user, domain.CreateInput{RawText: raw, StructuredJD: reviewedJD()})
			return e
		},
		func() error { _, _, e := app.List(ctx, user, 10, 5); return e },
		func() error { _, e := app.Get(ctx, user, id); return e },
		func() error {
			_, e := app.Update(ctx, user, id, domain.UpdateInput{StructuredJD: reviewedJD()})
			return e
		},
		func() error { return app.Delete(ctx, user, id) },
	}
	for i, op := range operations {
		if err := op(); !errors.Is(err, sentinel) || repo.user != user {
			t.Fatalf("operation %d: %v", i, err)
		}
		if i >= 2 && repo.id != id {
			t.Fatal("ID lost")
		}
	}
	if repo.limit != 10 || repo.offset != 5 {
		t.Fatal("pagination not forwarded")
	}
}

func TestUpdateNormalizesAndKeepsText(t *testing.T) {
	store := newFakeStore()
	key, _ := store.Upload(context.Background(), strings.NewReader("Original JD text"), jdTextContentType)
	repo := &fakeRepository{stored: domain.Record{ContentPath: key}}
	app := newApplication(t, repo, store)

	invalid := reviewedJD()
	invalid.Skills = []domain.ExtractedSkill{{Name: "Go", Category: domain.Tool}, {Name: "go", Category: domain.Database}}
	_, err := app.Update(context.Background(), uuid.New(), uuid.New(), domain.UpdateInput{StructuredJD: invalid})
	assertCode(t, err, apperror.CodeValidation)
	if repo.calls != 0 {
		t.Fatal("invalid input reached persistence")
	}

	valid := reviewedJD()
	valid.Title = "  Engineer  "
	valid.Technologies = []string{" Go ", "go", " "}
	got, err := app.Update(context.Background(), uuid.New(), uuid.New(), domain.UpdateInput{StructuredJD: valid})
	if err != nil {
		t.Fatal(err)
	}
	if repo.saved.Title != "Engineer" || repo.saved.ContentPath != "" {
		t.Fatalf("saved = %+v; update must not touch the stored text", repo.saved)
	}
	if got.RawText != "Original JD text" || len(got.StructuredJD.Technologies) != 1 || got.StructuredJD.Technologies[0] != "Go" {
		t.Fatalf("response = %+v", got)
	}
	if store.uploads != 1 {
		t.Fatal("update must not upload a new text object")
	}
}

func TestCreateRejectsRawTextOverTheStorageCap(t *testing.T) {
	repo, store := &fakeRepository{}, newFakeStore()
	// Under the normalized limit (whitespace collapses), but too big to read back once stored.
	raw := strings.Repeat("Go backend ", 20) + strings.Repeat(" ", maxStoredJDBytes)
	_, err := newApplication(t, repo, store).Create(context.Background(), uuid.New(), domain.CreateInput{RawText: raw, StructuredJD: reviewedJD()})
	assertCode(t, err, apperror.CodeJDTooLong)
	if store.uploads != 0 || repo.calls != 0 {
		t.Fatal("an oversized original must not be stored")
	}
}

func TestUpdateReadsTextBeforeWriting(t *testing.T) {
	store := newFakeStore()
	store.downloadErr = errors.New("bucket unreachable")
	repo := &fakeRepository{stored: domain.Record{ContentPath: "key", Seniority: domain.Senior}}
	_, err := newApplication(t, repo, store).Update(context.Background(), uuid.New(), uuid.New(), domain.UpdateInput{StructuredJD: reviewedJD()})
	assertCode(t, err, apperror.CodeInternal)
	if repo.updates != 0 {
		t.Fatal("a storage failure must leave the JD unchanged, not report an error after saving")
	}
}

func TestCreateAndGetReturnTheSameOrder(t *testing.T) {
	repo, store := &fakeRepository{}, newFakeStore()
	app := newApplication(t, repo, store)
	jd := reviewedJD()
	jd.Skills = []domain.ExtractedSkill{
		{Name: "Redis", Category: domain.Database}, {Name: "Go", Category: domain.ProgrammingLanguage, Requirement: domain.Required},
		{Name: "MySQL", Category: domain.Database}, {Name: "gRPC", Category: domain.Framework},
	}
	created, err := app.Create(context.Background(), uuid.New(), domain.CreateInput{RawText: strings.Repeat("original ", 20), StructuredJD: jd})
	if err != nil {
		t.Fatal(err)
	}
	repo.stored = repo.saved // what Get reads back is exactly what Create wrote
	got, err := app.Get(context.Background(), uuid.New(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for i, skill := range got.StructuredJD.Skills {
		if created.StructuredJD.Skills[i] != skill {
			t.Fatalf("skill %d: create %+v, get %+v", i, created.StructuredJD.Skills[i], skill)
		}
		names = append(names, skill.Name)
	}
	if strings.Join(names, ",") != "MySQL,Redis,gRPC,Go" {
		t.Fatalf("order = %v, want database, framework, programming_language with names sorted", names)
	}
}

func TestUnknownLevelIsAnInternalError(t *testing.T) {
	store := newFakeStore()
	key, _ := store.Upload(context.Background(), strings.NewReader("text"), jdTextContentType)
	repo := &fakeRepository{stored: domain.Record{ContentPath: key, Title: "T", Seniority: "mid-level"}}
	_, err := newApplication(t, repo, store).Get(context.Background(), uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeInternal)
}
