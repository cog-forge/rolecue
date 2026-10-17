package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestGetReturnsStoredPDFText(t *testing.T) {
	record := openRecord(ptr("JD text read from the PDF"))
	record.AlreadyApplied = true
	repo, texts := &fakeRepository{record: record}, &fakeTexts{}
	candidateID, id := uuid.New(), record.ID
	got, err := newService(t, repo, texts).Get(context.Background(), candidateID, "candidate", id)
	if err != nil {
		t.Fatal(err)
	}
	if repo.candidateID != candidateID || repo.id != id {
		t.Fatalf("repository got candidate %s, id %s", repo.candidateID, repo.id)
	}
	if got.RawText != "JD text read from the PDF" || !got.AlreadyApplied || got.Title != "Backend Engineer" || len(got.Requirements) != 1 {
		t.Fatalf("detail = %+v", got)
	}
	if texts.calls != 0 {
		t.Fatal("storage was read although the PDF text is stored")
	}
}

func TestGetReadsTextObjectOfOlderPostings(t *testing.T) {
	record := openRecord(nil)
	record.ContentPath = "job-postings/old.txt"
	texts := &fakeTexts{text: "JD stored as a text object"}
	got, err := newService(t, &fakeRepository{record: record}, texts).Get(context.Background(), uuid.New(), "candidate", record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RawText != "JD stored as a text object" || texts.key != "job-postings/old.txt" {
		t.Fatalf("raw text = %q, key = %q", got.RawText, texts.key)
	}
}

func TestGetWithoutAnyJDShowsNoText(t *testing.T) {
	record := openRecord(nil)
	record.ContentPath = ""
	texts := &fakeTexts{}
	got, err := newService(t, &fakeRepository{record: record}, texts).Get(context.Background(), uuid.New(), "candidate", record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RawText != "" || texts.calls != 0 {
		t.Fatalf("raw text = %q, storage calls = %d", got.RawText, texts.calls)
	}
}

func TestGetReportsStorageFailures(t *testing.T) {
	for name, texts := range map[string]*fakeTexts{
		"download fails": {err: errors.New("bucket unavailable")},
		"text too large": {text: strings.Repeat("a", maxStoredTextBytes+1)},
	} {
		_, err := newService(t, &fakeRepository{record: openRecord(nil)}, texts).Get(context.Background(), uuid.New(), "candidate", uuid.New())
		assertCode(t, err, apperror.CodeInternal)
		if !strings.Contains(err.Error(), "storage") {
			t.Fatalf("%s: error = %v", name, err)
		}
	}
}

func TestGetReturnsEmptyRequirementsNotNil(t *testing.T) {
	record := openRecord(ptr("text"))
	record.Requirements = nil
	got, err := newService(t, &fakeRepository{record: record}, nil).Get(context.Background(), uuid.New(), "candidate", record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Requirements == nil {
		t.Fatal("requirements is nil; want []")
	}
}

func TestGetRefusesOtherRoles(t *testing.T) {
	for _, role := range []string{"recruiter", "admin", ""} {
		repo := &fakeRepository{record: openRecord(ptr("text"))}
		_, err := newService(t, repo, nil).Get(context.Background(), uuid.New(), role, uuid.New())
		assertCode(t, err, apperror.CodeForbidden)
		if repo.calls != 0 {
			t.Fatalf("%q: repository was called", role)
		}
	}
}

// A posting that is missing or not open comes back from the repository as not found; the service
// must pass that through unchanged and never fall back to storage.
func TestGetPassesNotFoundThrough(t *testing.T) {
	notFound := apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
	texts := &fakeTexts{}
	_, err := newService(t, &fakeRepository{err: notFound}, texts).Get(context.Background(), uuid.New(), "candidate", uuid.New())
	if !errors.Is(err, notFound) {
		t.Fatalf("error = %v, want %v", err, notFound)
	}
	if texts.calls != 0 {
		t.Fatal("storage was read for a missing posting")
	}
}

// The Candidate DTOs must never carry recruiter-private data: the question bank, evaluation
// weights, capacity or funding, the reject reason, or the required avatar and voice.
func TestCandidateDTOsDoNotExposePrivateFields(t *testing.T) {
	detail, err := newService(t, &fakeRepository{record: openRecord(ptr("text"))}, nil).Get(context.Background(), uuid.New(), "candidate", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"list item": detail.BoardPosting, "detail": detail} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		for key := range fields {
			for _, banned := range []string{"reject", "slot", "capacity", "question", "metric", "weight", "avatar", "voice", "fund", "wallet", "status", "recruiter", "content_path", "jd_id"} {
				if strings.Contains(key, banned) {
					t.Fatalf("%s exposes %q", name, key)
				}
			}
		}
	}
	want := []string{"id", "title", "company_name", "location", "employment_type", "salary_min", "salary_max", "seniority_level", "created_at",
		"company_website", "interview_duration", "raw_text", "requirements", "already_applied"}
	raw, _ := json.Marshal(detail)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if len(fields) != len(want) {
		t.Fatalf("detail has %d fields, want exactly %v: %s", len(fields), want, raw)
	}
	for _, key := range want {
		if _, ok := fields[key]; !ok {
			t.Fatalf("detail is missing %q: %s", key, raw)
		}
	}
}
