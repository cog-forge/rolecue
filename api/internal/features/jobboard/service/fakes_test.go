package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	items       []domain.BoardPosting
	total       int64
	record      domain.BoardRecord
	err         error
	calls       int
	filter      domain.BoardFilter
	candidateID uuid.UUID
	id          uuid.UUID
}

func (f *fakeRepository) ListOpen(_ context.Context, filter domain.BoardFilter) ([]domain.BoardPosting, int64, error) {
	f.calls++
	f.filter = filter
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.items, f.total, nil
}

func (f *fakeRepository) GetOpen(_ context.Context, candidateID, id uuid.UUID) (domain.BoardRecord, error) {
	f.calls++
	f.candidateID, f.id = candidateID, id
	if f.err != nil {
		return domain.BoardRecord{}, f.err
	}
	return f.record, nil
}

type fakeTexts struct {
	text  string
	err   error
	calls int
	key   string
}

func (f *fakeTexts) Download(_ context.Context, key string) (storage.Object, error) {
	f.calls++
	f.key = key
	if f.err != nil {
		return storage.Object{}, f.err
	}
	return storage.Object{Body: io.NopCloser(strings.NewReader(f.text)), ContentType: "text/plain", Size: int64(len(f.text))}, nil
}

func newService(t *testing.T, repo *fakeRepository, texts *fakeTexts) *Service {
	t.Helper()
	if texts == nil {
		texts = &fakeTexts{}
	}
	svc, err := New(repo, texts)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func ptr[T any](v T) *T { return &v }

func openRecord(storedText *string) domain.BoardRecord {
	return domain.BoardRecord{
		ContentPath: "job-postings/jd.pdf",
		StoredText:  storedText,
		BoardPostingDetail: domain.BoardPostingDetail{
			BoardPosting: domain.BoardPosting{
				ID: uuid.New(), Title: "Backend Engineer", CompanyName: ptr("Acme"), Location: ptr("Ha Noi"),
				EmploymentType: ptr("Full-time"), SalaryMin: ptr[int32](1000), SalaryMax: ptr[int32](2000),
				SeniorityLevel: "mid", CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			},
			CompanyWebsite: ptr("https://acme.test"), InterviewDuration: ptr[int32](30),
			Requirements: []domain.BoardRequirement{{Name: "Go", Target: "programming_language", Type: "required"}},
		},
	}
}
