package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// pdfPosting puts a posting with a JD PDF in the fakes.
func pdfPosting(t *testing.T, text string) (*fakeRepository, *fakeStore) {
	t.Helper()
	store := newStore()
	key := store.putPDF(text)
	repo := &fakeRepository{record: domain.Record{ContentPath: key, StoredText: &text, JobPosting: domain.JobPosting{ID: uuid.New(), Status: domain.StatusPending}}}
	return repo, store
}

// textPosting puts a posting created before PDF upload in the fakes: its JD is a plain-text object.
func textPosting(t *testing.T, text string) (*fakeRepository, *fakeStore) {
	t.Helper()
	store := newStore()
	store.objects["key"] = storedObject{data: []byte(text), contentType: "text/plain; charset=utf-8"}
	repo := &fakeRepository{record: domain.Record{ContentPath: "key", JobPosting: domain.JobPosting{ID: uuid.New(), Status: domain.StatusPending}}}
	return repo, store
}

func TestGetUsesStoredPDFText(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	recruiter := uuid.New()
	got, err := newService(t, repo, store).Get(context.Background(), recruiter, "recruiter", repo.record.ID)
	if err != nil || got.RawText != validText() || !got.HasDocument {
		t.Fatalf("has_document=%t text matches=%t err=%v", got.HasDocument, got.RawText == validText(), err)
	}
	if store.downloads != 0 {
		t.Fatal("the PDF was downloaded to show text that is already stored")
	}
	if repo.recruiter != recruiter {
		t.Fatal("the read was not scoped to the session user")
	}
}

func TestGetReadsOlderTextObjectBack(t *testing.T) {
	repo, store := textPosting(t, validText())
	got, err := newService(t, repo, store).Get(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	if err != nil || got.RawText != validText() || got.HasDocument {
		t.Fatalf("has_document=%t text matches=%t err=%v", got.HasDocument, got.RawText == validText(), err)
	}
}

func TestGetRejectsOversizedStoredText(t *testing.T) {
	repo, store := textPosting(t, strings.Repeat("a", maxStoredTextBytes+1))
	_, err := newService(t, repo, store).Get(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	assertCode(t, err, apperror.CodeInternal)
}

func TestGetPassesNotFoundThrough(t *testing.T) {
	repo := &fakeRepository{err: apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")}
	_, err := newService(t, repo, newStore()).Get(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestDocumentLinksPDF(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	got, err := newService(t, repo, store).Document(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	if err != nil || !strings.Contains(got.URL, repo.record.ContentPath) || got.ExpiresAt.IsZero() {
		t.Fatalf("link=%+v err=%v", got, err)
	}
}

func TestDocumentErrors(t *testing.T) {
	repo, store := textPosting(t, validText())
	_, err := newService(t, repo, store).Document(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	assertCode(t, err, apperror.CodeJDNotFound)

	repo, store = pdfPosting(t, validText())
	store.presignErr = errBoom
	_, err = newService(t, repo, store).Document(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	assertCode(t, err, apperror.CodeInternal)

	repo = &fakeRepository{err: apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")}
	_, err = newService(t, repo, newStore()).Document(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}

func TestUpdateNormalizesPatch(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	patch := domain.Edit{
		Title:    domain.StringChange{Present: true, Value: ptr("  Senior Backend Engineer ")},
		Location: domain.StringChange{Present: true, Value: ptr("   ")},
		Requirements: domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{
			{Name: " Kafka ", Target: "technology", Type: domain.Required},
		}},
	}
	got, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID, patch)
	if err != nil {
		t.Fatal(err)
	}
	if *repo.patch.Title.Value != "Senior Backend Engineer" {
		t.Fatalf("title = %q", *repo.patch.Title.Value)
	}
	if !repo.patch.Location.Present || repo.patch.Location.Value != nil {
		t.Fatal("a blank location should clear the field")
	}
	if (*repo.patch.Requirements.Value)[0].Name != "Kafka" {
		t.Fatal("requirements were not trimmed")
	}
	if repo.document != nil || got.RawText != validText() {
		t.Fatalf("document=%v text matches=%t", repo.document, got.RawText == validText())
	}
}

func TestUpdateValidation(t *testing.T) {
	id := uuid.New()
	for _, tt := range []struct {
		name  string
		patch domain.Edit
	}{
		{"null title", domain.Edit{Title: domain.StringChange{Present: true}}},
		{"blank title", domain.Edit{Title: domain.StringChange{Present: true, Value: ptr(" ")}}},
		{"null seniority", domain.Edit{SeniorityLevel: domain.StringChange{Present: true}}},
		{"unknown seniority", domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("staff")}}},
		{"empty requirements", domain.Edit{Requirements: domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{}}}},
		{"null requirements", domain.Edit{Requirements: domain.RequirementsChange{Present: true}}},
		{"negative salary", domain.Edit{SalaryMax: domain.Int32Change{Present: true, Value: ptr(int32(-5))}}},
		{"zero duration", domain.Edit{InterviewDuration: domain.Int32Change{Present: true, Value: ptr(int32(0))}}},
		{"cleared avatar", domain.Edit{RequiredAvatarID: domain.UUIDChange{Present: true}}},
		{"cleared voice", domain.Edit{RequiredVoiceID: domain.UUIDChange{Present: true}}},
		{"nil voice", domain.Edit{RequiredVoiceID: domain.UUIDChange{Present: true, Value: &uuid.Nil}}},
		{"null document", domain.Edit{DocumentID: domain.StringChange{Present: true}}},
		{"document path", domain.Edit{DocumentID: domain.StringChange{Present: true, Value: ptr("some/other/key")}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := pdfPosting(t, validText())
			repo.record.Status = domain.StatusRejected
			_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", id, tt.patch)
			assertCode(t, err, apperror.CodeValidation)
			if store.downloads != 0 || repo.updateCalls != 0 {
				t.Fatal("an invalid patch reached storage or the repository")
			}
		})
	}
}

// The repository enforces pending-only edits and the requirements lock under a row lock; the
// service must report them unchanged and must not read the posting after the failed write.
func TestUpdatePassesStateErrorsThrough(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		code apperror.Code
	}{
		{"not editable", domain.ErrNotEditable, apperror.CodeJobPostingNotEditable},
		{"text locked", domain.ErrTextLocked, apperror.CodeJobPostingTextLocked},
		{"requirements locked", domain.ErrRequirementsLocked, apperror.CodeJobPostingRequirementsLocked},
		{"salary after merge", domain.ErrSalaryRange, apperror.CodeValidation},
		{"voice inactive", domain.ErrVoiceNotActive, apperror.CodeValidation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := pdfPosting(t, validText())
			repo.updateErr = tt.err
			_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", uuid.New(),
				domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("senior")}})
			assertCode(t, err, tt.code)
			if repo.getCalls != 0 {
				t.Fatalf("get calls = %d, want none after a failed write", repo.getCalls)
			}
		})
	}
}

func TestListFilter(t *testing.T) {
	bad := domain.Status("archived")
	_, _, err := newService(t, &fakeRepository{}, newStore()).List(context.Background(), uuid.New(), "recruiter", domain.ListFilter{Status: &bad, Limit: 20})
	assertCode(t, err, apperror.CodeValidation)

	repo := &fakeRepository{}
	_, _, err = newService(t, repo, newStore()).List(context.Background(), uuid.New(), "recruiter", domain.ListFilter{Search: ptr(" 100%_off "), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if *repo.filter.Search != `100\%\_off` {
		t.Fatalf("search = %q, want wildcards escaped", *repo.filter.Search)
	}

	repo = &fakeRepository{}
	_, _, err = newService(t, repo, newStore()).List(context.Background(), uuid.New(), "recruiter", domain.ListFilter{Search: ptr("  "), Limit: 20})
	if err != nil || repo.filter.Search != nil {
		t.Fatalf("a blank search should mean no filter: search=%v err=%v", repo.filter.Search, err)
	}
}

func correctedText() string {
	return strings.Repeat("Design and operate Go APIs with PostgreSQL and Kafka. ", 4)
}

func TestUpdateReplacesPDFOfRejectedPosting(t *testing.T) {
	// An older text posting can be moved to a PDF too.
	for name, setup := range map[string]func(*testing.T, string) (*fakeRepository, *fakeStore){"pdf": pdfPosting, "text": textPosting} {
		t.Run(name, func(t *testing.T) {
			repo, store := setup(t, validText())
			repo.record.Status = domain.StatusRejected
			newID := store.putPDF(correctedText())
			got, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID,
				domain.Edit{DocumentID: domain.StringChange{Present: true, Value: &newID}})
			if err != nil {
				t.Fatal(err)
			}
			if repo.document == nil || repo.document.ContentPath != newID || repo.document.RawText != correctedText() {
				t.Fatalf("document = %+v", repo.document)
			}
			if got.RawText != correctedText() || !got.HasDocument || got.Status != domain.StatusRejected {
				t.Fatalf("status=%s has_document=%t text matches=%t", got.Status, got.HasDocument, got.RawText == correctedText())
			}
		})
	}
}

func TestUpdateRefusesUnreadableReplacementPDF(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	repo.record.Status = domain.StatusRejected
	shortID := store.putPDF("Too short.")
	_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID,
		domain.Edit{DocumentID: domain.StringChange{Present: true, Value: &shortID}})
	assertCode(t, err, apperror.CodeJDTooShort)

	missing := uuid.NewString()
	_, err = newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID,
		domain.Edit{DocumentID: domain.StringChange{Present: true, Value: &missing}})
	assertCode(t, err, apperror.CodeInvalidPDF)
	if repo.updateCalls != 0 {
		t.Fatal("rows were written without a readable PDF")
	}
}

// A whole-form submit resends the current PDF; that is not a change and works while pending.
func TestUpdateIgnoresUnchangedDocument(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	current := repo.record.ContentPath
	_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID,
		domain.Edit{DocumentID: domain.StringChange{Present: true, Value: &current}, Title: domain.StringChange{Present: true, Value: ptr("Same PDF")}})
	if err != nil || store.downloads != 0 || repo.document != nil || repo.updateCalls != 1 {
		t.Fatalf("err=%v downloads=%d document=%v updates=%d", err, store.downloads, repo.document, repo.updateCalls)
	}
}

func TestUpdateRefusesDocumentChangeUnlessRejected(t *testing.T) {
	for _, status := range []domain.Status{domain.StatusPending, domain.StatusOpen} {
		repo, store := pdfPosting(t, validText())
		repo.record.Status = status
		newID := store.putPDF(correctedText())
		_, err := newService(t, repo, store).Update(context.Background(), uuid.New(), "recruiter", repo.record.ID,
			domain.Edit{DocumentID: domain.StringChange{Present: true, Value: &newID}})
		assertCode(t, err, apperror.CodeJobPostingTextLocked)
		if store.downloads != 0 || repo.updateCalls != 0 {
			t.Fatalf("%s: downloads=%d updates=%d", status, store.downloads, repo.updateCalls)
		}
	}
}

func TestResubmitRejectedPosting(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	repo.record.Status, repo.record.RejectReason = domain.StatusRejected, ptr("Too vague")
	recruiter := uuid.New()
	s := newService(t, repo, store)
	// Repeating it (a retry or a double click) succeeds and stays pending.
	for call := 1; call <= 2; call++ {
		got, err := s.Resubmit(context.Background(), recruiter, "recruiter", repo.record.ID)
		if err != nil || got.Status != domain.StatusPending || got.RejectReason != nil || got.RawText != validText() {
			t.Fatalf("call %d: status=%s reason=%v err=%v", call, got.Status, got.RejectReason, err)
		}
	}
	if repo.recruiter != recruiter {
		t.Fatal("resubmit was not scoped to the session user")
	}
}

func TestResubmitPassesStateErrorsThrough(t *testing.T) {
	repo, store := pdfPosting(t, validText())
	repo.record.Status = domain.StatusOpen
	_, err := newService(t, repo, store).Resubmit(context.Background(), uuid.New(), "recruiter", repo.record.ID)
	assertCode(t, err, apperror.CodeJobPostingNotRejected)

	repo = &fakeRepository{err: apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")}
	_, err = newService(t, repo, newStore()).Resubmit(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeJobPostingNotFound)
}
