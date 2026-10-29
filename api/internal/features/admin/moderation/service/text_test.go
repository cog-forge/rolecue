package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newServiceWithTexts(t *testing.T, repo Repository, texts TextStore) (*Service, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.WarnLevel)
	s, err := New(repo, texts, &logger.Logger{Logger: zap.New(core)})
	if err != nil {
		t.Fatal(err)
	}
	return s, logs
}

// A posting with a JD PDF: the Admin gets a short-lived link to it and the text read from it,
// without downloading the PDF.
func TestGetLinksJDPDF(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	pdfText := "Text read from the PDF."
	repo.posting.ContentPath, repo.posting.StoredText = "0b6c3a52-6a3d-4b7e-9d55-8c1f2a3b4c5d", &pdfText
	texts := storedTexts()
	s, _ := newServiceWithTexts(t, repo, texts)
	got, err := s.Get(context.Background(), "admin", repo.posting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DocumentURL == nil || !strings.Contains(*got.DocumentURL, repo.posting.ContentPath) || got.DocumentExpiresAt == nil {
		t.Fatalf("document_url=%v expires=%v", got.DocumentURL, got.DocumentExpiresAt)
	}
	if got.RawText == nil || *got.RawText != pdfText {
		t.Fatalf("raw_text = %v, want the PDF text", got.RawText)
	}
	if len(texts.keys) != 0 || len(texts.presigned) != 1 || texts.presigned[0] != repo.posting.ContentPath {
		t.Fatalf("downloads=%v presigned=%v", texts.keys, texts.presigned)
	}
}

// content_path is nullable (Role Profile drafts), so a posting can hold its text without any file. The
// Admin still gets the posting and its text, with no document link, instead of a failed signing.
func TestGetReturnsStoredTextWithoutLinkWhenContentPathIsEmpty(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	text := "Text without a file."
	repo.posting.ContentPath, repo.posting.StoredText = "", &text
	texts := storedTexts()
	s, _ := newServiceWithTexts(t, repo, texts)
	got, err := s.Get(context.Background(), "admin", repo.posting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RawText == nil || *got.RawText != text {
		t.Fatalf("raw_text = %v, want the stored text", got.RawText)
	}
	if got.DocumentURL != nil || got.DocumentExpiresAt != nil {
		t.Fatalf("document_url=%v expires=%v, want no document link", got.DocumentURL, got.DocumentExpiresAt)
	}
	if len(texts.presigned) != 0 || len(texts.keys) != 0 {
		t.Fatalf("presigned=%v downloads=%v, want no storage call", texts.presigned, texts.keys)
	}
}

func TestGetFailsWhenPDFLinkFails(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	pdfText := "Text read from the PDF."
	repo.posting.StoredText = &pdfText
	s, _ := newServiceWithTexts(t, repo, &fakeTextStore{presignErr: errors.New("signing failed")})
	_, err := s.Get(context.Background(), "admin", repo.posting.ID)
	assertCode(t, err, apperror.CodeInternal)
}

// Postings created before PDF upload keep a plain-text object under content_path (KAN-230).
func TestGetReadsStoredJDText(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	texts := storedTexts()
	s, _ := newServiceWithTexts(t, repo, texts)
	got, err := s.Get(context.Background(), "admin", repo.posting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RawText == nil || *got.RawText != storedText || got.DocumentURL != nil {
		t.Fatalf("raw_text=%v document_url=%v, want the stored text and no PDF link", got.RawText, got.DocumentURL)
	}
	if got.ContentPath != storedKey || len(texts.keys) != 1 || texts.keys[0] != storedKey {
		t.Fatalf("content_path=%q downloads=%v", got.ContentPath, texts.keys)
	}
}

// Older rows hold a full URL; the Admin opens the file, so storage isn't asked for it.
func TestGetLeavesURLContentPathAlone(t *testing.T) {
	for _, path := range []string{"https://files.example.com/jd/java.pdf", "http://localhost:8099/files/go-backend.pdf"} {
		t.Run(path, func(t *testing.T) {
			repo := postingIn(domain.PostingStatusPending)
			repo.posting.ContentPath = path
			texts := storedTexts()
			s, _ := newServiceWithTexts(t, repo, texts)
			got, err := s.Get(context.Background(), "admin", repo.posting.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.RawText != nil || got.ContentPath != path || len(texts.keys) != 0 {
				t.Fatalf("raw_text=%v content_path=%q downloads=%v", got.RawText, got.ContentPath, texts.keys)
			}
		})
	}
}

// A key that only looks like a URL (no scheme or host) is still a storage key.
func TestGetTreatsSchemelessPathAsKey(t *testing.T) {
	for _, path := range []string{"files/java.pdf", "ftp://files.example.com/jd.txt", "https:///no-host"} {
		t.Run(path, func(t *testing.T) {
			repo := postingIn(domain.PostingStatusPending)
			repo.posting.ContentPath = path
			texts := &fakeTextStore{objects: map[string]string{path: storedText}}
			s, _ := newServiceWithTexts(t, repo, texts)
			got, err := s.Get(context.Background(), "admin", repo.posting.ID)
			if err != nil || got.RawText == nil || *got.RawText != storedText {
				t.Fatalf("raw_text=%v err=%v", got.RawText, err)
			}
		})
	}
}

// A missing object must not hide the posting: the Admin still sees it and can reject it.
func TestGetWithMissingTextReturnsPostingWithoutText(t *testing.T) {
	for _, storeErr := range []error{storage.ErrNotFound, storage.ErrInvalidInput} {
		t.Run(storeErr.Error(), func(t *testing.T) {
			repo := postingIn(domain.PostingStatusPending)
			s, logs := newServiceWithTexts(t, repo, &fakeTextStore{err: storeErr})
			got, err := s.Get(context.Background(), "admin", repo.posting.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.RawText != nil || got.ID != repo.posting.ID {
				t.Fatalf("raw_text=%v id=%s", got.RawText, got.ID)
			}
			if logs.FilterMessage("job posting JD text not found").Len() != 1 {
				t.Fatalf("warning lines = %d, want 1", logs.Len())
			}
		})
	}
}

func TestGetFailsOnStorageError(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	s, _ := newServiceWithTexts(t, repo, &fakeTextStore{err: errors.New("connection reset")})
	_, err := s.Get(context.Background(), "admin", repo.posting.ID)
	assertCode(t, err, apperror.CodeInternal)
}

func TestGetRefusesOversizedText(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	texts := &fakeTextStore{objects: map[string]string{storedKey: strings.Repeat("a", maxJDTextBytes+1)}}
	s, _ := newServiceWithTexts(t, repo, texts)
	_, err := s.Get(context.Background(), "admin", repo.posting.ID)
	assertCode(t, err, apperror.CodeInternal)
}

// The decision response replaces the review page, so it carries the text too, including the
// idempotent retry path.
func TestDecisionsReturnStoredJDText(t *testing.T) {
	ctx := context.Background()
	repo := postingIn(domain.PostingStatusPending)
	s, _ := newServiceWithTexts(t, repo, storedTexts())
	for i := 0; i < 2; i++ {
		got, err := s.Approve(ctx, uuid.New(), "admin", repo.posting.ID)
		if err != nil || got.RawText == nil || *got.RawText != storedText {
			t.Fatalf("approval %d: raw_text=%v err=%v", i+1, got.RawText, err)
		}
	}

	repo = postingIn(domain.PostingStatusPending)
	s, _ = newServiceWithTexts(t, repo, storedTexts())
	got, err := s.Reject(ctx, uuid.New(), "admin", repo.posting.ID, "Duplicate posting")
	if err != nil || got.RawText == nil || *got.RawText != storedText {
		t.Fatalf("rejection: raw_text=%v err=%v", got.RawText, err)
	}
}

// Non-admins are refused before storage is read.
func TestNonAdminGetDoesNotReadStorage(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	texts := storedTexts()
	s, _ := newServiceWithTexts(t, repo, texts)
	_, err := s.Get(context.Background(), "recruiter", repo.posting.ID)
	assertCode(t, err, apperror.CodeForbidden)
	if len(texts.keys) != 0 {
		t.Fatal("a non-admin request reached storage")
	}
}
