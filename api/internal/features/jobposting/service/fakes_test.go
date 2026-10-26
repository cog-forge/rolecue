package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	record    domain.Record
	err       error
	updateErr error

	createCalls, listCalls, getCalls, updateCalls, resubmitCalls int
	recruiter                                                    uuid.UUID
	created                                                      domain.NewPosting
	filter                                                       domain.ListFilter
	patch                                                        domain.Edit
	document                                                     *domain.NewDocument
}

func (f *fakeRepository) Create(_ context.Context, recruiterID uuid.UUID, input domain.NewPosting) (domain.Record, error) {
	f.createCalls++
	f.recruiter, f.created = recruiterID, input
	if f.err != nil {
		return domain.Record{}, f.err
	}
	f.record = domain.Record{ContentPath: input.ContentPath, StoredText: &input.RawText, JobPosting: domain.JobPosting{
		ID: uuid.New(), Title: input.Title, Status: domain.StatusPending, SeniorityLevel: input.SeniorityLevel,
		Requirements: input.Requirements,
	}}
	return f.record, nil
}

func (f *fakeRepository) List(_ context.Context, recruiterID uuid.UUID, filter domain.ListFilter) ([]domain.Summary, int64, error) {
	f.listCalls++
	f.recruiter, f.filter = recruiterID, filter
	return []domain.Summary{}, 0, f.err
}

func (f *fakeRepository) Get(_ context.Context, recruiterID, _ uuid.UUID) (domain.Record, error) {
	f.getCalls++
	f.recruiter = recruiterID
	return f.record, f.err
}

func (f *fakeRepository) Update(_ context.Context, recruiterID, _ uuid.UUID, patch domain.Edit, document *domain.NewDocument) error {
	f.updateCalls++
	f.recruiter, f.patch, f.document = recruiterID, patch, document
	if f.err != nil {
		return f.err
	}
	if f.updateErr != nil {
		return f.updateErr
	}
	if document != nil {
		f.record.ContentPath, f.record.StoredText = document.ContentPath, &document.RawText
	}
	return nil
}

// Resubmit behaves like the SQL: rejected becomes pending without its reason, pending stays.
func (f *fakeRepository) Resubmit(_ context.Context, recruiterID, _ uuid.UUID) error {
	f.resubmitCalls++
	f.recruiter = recruiterID
	if f.err != nil {
		return f.err
	}
	switch f.record.Status {
	case domain.StatusPending:
		return nil
	case domain.StatusRejected:
		f.record.Status, f.record.RejectReason = domain.StatusPending, nil
		return nil
	}
	return domain.ErrNotRejected
}

type storedObject struct {
	data        []byte
	contentType string
}

// fakeStore keeps uploaded objects in memory.
type fakeStore struct {
	objects     map[string]storedObject
	uploadErr   error
	downloadErr error
	presignErr  error
	uploads     int
	downloads   int
}

func newStore() *fakeStore { return &fakeStore{objects: map[string]storedObject{}} }

func (s *fakeStore) Upload(_ context.Context, body io.ReadSeeker, contentType string) (string, error) {
	s.uploads++
	if s.uploadErr != nil {
		return "", s.uploadErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	key := uuid.NewString()
	s.objects[key] = storedObject{data: data, contentType: contentType}
	return key, nil
}

func (s *fakeStore) Download(_ context.Context, key string) (storage.Object, error) {
	s.downloads++
	if s.downloadErr != nil {
		return storage.Object{}, s.downloadErr
	}
	object, ok := s.objects[key]
	if !ok {
		return storage.Object{}, storage.ErrNotFound
	}
	return storage.Object{Body: io.NopCloser(bytes.NewReader(object.data)), ContentType: object.contentType, Size: int64(len(object.data))}, nil
}

func (s *fakeStore) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	if s.presignErr != nil {
		return storage.SignedRequest{}, s.presignErr
	}
	return storage.SignedRequest{URL: "https://storage.example.com/" + key + "?signed", Method: "GET", ExpiresAt: time.Unix(0, 0).Add(ttl)}, nil
}

// putPDF stores a fake PDF holding text and returns its document ID.
func (s *fakeStore) putPDF(text string) string {
	key := uuid.NewString()
	s.objects[key] = storedObject{data: fakePDF(text), contentType: pdfContentType}
	return key
}

// fakeParser reads the fake PDFs fakePDF builds; anything else is not a readable PDF.
type fakeParser struct{ calls int }

func fakePDF(text string) []byte { return []byte("%PDF-fake\n" + text) }

func (p *fakeParser) Parse(_ context.Context, data []byte) (string, error) {
	p.calls++
	text, ok := strings.CutPrefix(string(data), "%PDF-fake\n")
	if !ok {
		return "", apperror.New(apperror.CodeInvalidPDF, "Choose a valid PDF up to 5 MB.")
	}
	return text, nil
}

func newService(t *testing.T, repo Repository, store DocumentStore) *Service {
	t.Helper()
	s, err := New(repo, store, &fakeParser{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var errBoom = errors.New("boom")

func validText() string { return strings.Repeat("Build and run Go services on PostgreSQL. ", 5) }

func validCreate(documentID string) domain.CreateInput {
	return domain.CreateInput{
		Title: "Backend Engineer", DocumentID: documentID, SeniorityLevel: domain.Mid,
		Requirements: []domain.PostingRequirement{
			{Name: "Go", Target: "programming_language", Type: domain.Required},
			{Name: "PostgreSQL", Target: "database", Type: domain.Preferred},
		},
		RequiredAvatarID: uuid.New(), RequiredVoiceID: uuid.New(),
	}
}

func ptr[T any](v T) *T { return &v }

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestNewRequiresDependencies(t *testing.T) {
	if _, err := New(nil, newStore(), &fakeParser{}); err == nil {
		t.Fatal("New accepted a nil repository")
	}
	if _, err := New(&fakeRepository{}, nil, &fakeParser{}); err == nil {
		t.Fatal("New accepted a nil document store")
	}
	if _, err := New(&fakeRepository{}, newStore(), nil); err == nil {
		t.Fatal("New accepted a nil PDF parser")
	}
}

func TestNonRecruitersAreForbidden(t *testing.T) {
	for _, role := range []string{"candidate", "admin", ""} {
		t.Run(role, func(t *testing.T) {
			repo, store := &fakeRepository{}, newStore()
			s := newService(t, repo, store)
			ctx, id := context.Background(), uuid.New()
			_, err := s.UploadDocument(ctx, role, fakePDF(validText()))
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Create(ctx, uuid.New(), role, validCreate(uuid.NewString()))
			assertCode(t, err, apperror.CodeForbidden)
			_, _, err = s.List(ctx, uuid.New(), role, domain.ListFilter{Limit: 20})
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Get(ctx, uuid.New(), role, id)
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Update(ctx, uuid.New(), role, id, domain.Edit{})
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Resubmit(ctx, uuid.New(), role, id)
			assertCode(t, err, apperror.CodeForbidden)
			_, err = s.Document(ctx, uuid.New(), role, id)
			assertCode(t, err, apperror.CodeForbidden)
			if repo.createCalls+repo.listCalls+repo.getCalls+repo.updateCalls+repo.resubmitCalls+store.uploads+store.downloads != 0 {
				t.Fatal("a non-recruiter request reached the repository or storage")
			}
		})
	}
}

// Submit behaves like Resubmit here; the status rules between them live in the repository.
func (f *fakeRepository) Submit(ctx context.Context, recruiterID, id uuid.UUID) error {
	return f.Resubmit(ctx, recruiterID, id)
}
