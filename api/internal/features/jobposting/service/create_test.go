package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestUploadDocumentStoresPDFAndReturnsText(t *testing.T) {
	store := newStore()
	got, err := newService(t, &fakeRepository{}, store).UploadDocument(context.Background(), "recruiter", fakePDF(validText()))
	if err != nil {
		t.Fatal(err)
	}
	object, ok := store.objects[got.DocumentID]
	if !ok || object.contentType != pdfContentType || string(object.data) != string(fakePDF(validText())) {
		t.Fatalf("stored object = %+v (found %t)", object.contentType, ok)
	}
	if got.RawText != validText() {
		t.Fatal("the PDF text was not returned")
	}
}

// An unreadable or out-of-range PDF is refused before anything is stored.
func TestUploadDocumentRefusesBadPDFs(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
		code apperror.Code
	}{
		{"not a PDF", []byte("hello"), apperror.CodeInvalidPDF},
		{"too large", append(fakePDF(validText()), make([]byte, MaxPDFBytes)...), apperror.CodePDFTooLarge},
		{"too little text", fakePDF("Go developer"), apperror.CodeJDTooShort},
		{"too much text", fakePDF(strings.Repeat("a", MaxTextLength+1)), apperror.CodeJDTooLong},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := newStore()
			_, err := newService(t, &fakeRepository{}, store).UploadDocument(context.Background(), "recruiter", tt.data)
			assertCode(t, err, tt.code)
			if store.uploads != 0 {
				t.Fatal("a refused PDF was stored")
			}
		})
	}
}

func TestUploadDocumentStorageFailure(t *testing.T) {
	store := newStore()
	store.uploadErr = errBoom
	_, err := newService(t, &fakeRepository{}, store).UploadDocument(context.Background(), "recruiter", fakePDF(validText()))
	assertCode(t, err, apperror.CodeInternal)
}

func TestCreateStoresPDFTextFromServerRead(t *testing.T) {
	repo, store := &fakeRepository{}, newStore()
	documentID := store.putPDF(validText())
	recruiter := uuid.New()
	input := validCreate(documentID)
	input.Title = "  Backend Engineer  "
	input.Location = ptr("  ")
	input.EmploymentType = ptr(" Full-time ")
	// A client-sent text never reaches storage: the text comes from the PDF.
	input.RawText = "ignored"

	got, err := newService(t, repo, store).Create(context.Background(), recruiter, "recruiter", input)
	if err != nil {
		t.Fatal(err)
	}
	if repo.recruiter != recruiter {
		t.Fatal("the posting was not stored for the session user")
	}
	if repo.created.ContentPath != documentID || repo.created.RawText != validText() {
		t.Fatalf("content_path=%q text matches=%t", repo.created.ContentPath, repo.created.RawText == validText())
	}
	if repo.created.Title != "Backend Engineer" || repo.created.Location != nil || *repo.created.EmploymentType != "Full-time" {
		t.Fatalf("fields were not normalized: %+v", repo.created.CreateInput)
	}
	if got.RawText != validText() || !got.HasDocument || got.Status != domain.StatusPending {
		t.Fatalf("response: status=%s has_document=%t text matches=%t", got.Status, got.HasDocument, got.RawText == validText())
	}
	// The repository returns the stored posting; a separate read after commit could fail and make
	// a client retry create a duplicate.
	if repo.getCalls != 0 || store.uploads != 0 {
		t.Fatalf("get calls=%d uploads=%d", repo.getCalls, store.uploads)
	}
}

func TestCreateValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*domain.CreateInput)
		code   apperror.Code
	}{
		{"blank title", func(in *domain.CreateInput) { in.Title = "  " }, apperror.CodeValidation},
		{"title too long", func(in *domain.CreateInput) { in.Title = strings.Repeat("界", 201) }, apperror.CodeValidation},
		{"missing document", func(in *domain.CreateInput) { in.DocumentID = "" }, apperror.CodeValidation},
		{"document path", func(in *domain.CreateInput) { in.DocumentID = "../other/key.pdf" }, apperror.CodeValidation},
		{"unknown seniority", func(in *domain.CreateInput) { in.SeniorityLevel = "principal" }, apperror.CodeValidation},
		{"no requirements", func(in *domain.CreateInput) { in.Requirements = nil }, apperror.CodeValidation},
		{"blank requirement", func(in *domain.CreateInput) { in.Requirements[0].Name = " " }, apperror.CodeValidation},
		{"unknown target", func(in *domain.CreateInput) { in.Requirements[0].Target = "soft_skill" }, apperror.CodeValidation},
		{"unknown type", func(in *domain.CreateInput) { in.Requirements[0].Type = "optional" }, apperror.CodeValidation},
		{"duplicate requirement", func(in *domain.CreateInput) {
			in.Requirements[1] = domain.PostingRequirement{Name: " go ", Target: "programming_language", Type: domain.Preferred}
		}, apperror.CodeValidation},
		{"too many requirements", func(in *domain.CreateInput) {
			in.Requirements = nil
			for i := 0; i <= maxRequirements; i++ {
				in.Requirements = append(in.Requirements, domain.PostingRequirement{Name: uuid.NewString(), Target: "tool", Type: domain.Required})
			}
		}, apperror.CodeValidation},
		{"negative salary", func(in *domain.CreateInput) { in.SalaryMin = ptr(int32(-1)) }, apperror.CodeValidation},
		{"salary range reversed", func(in *domain.CreateInput) {
			in.SalaryMin, in.SalaryMax = ptr(int32(2000)), ptr(int32(1000))
		}, apperror.CodeValidation},
		{"zero duration", func(in *domain.CreateInput) { in.InterviewDuration = ptr(int32(0)) }, apperror.CodeValidation},
		{"long location", func(in *domain.CreateInput) { in.Location = ptr(strings.Repeat("x", 201)) }, apperror.CodeValidation},
		{"missing avatar", func(in *domain.CreateInput) { in.RequiredAvatarID = uuid.Nil }, apperror.CodeValidation},
		{"missing voice", func(in *domain.CreateInput) { in.RequiredVoiceID = uuid.Nil }, apperror.CodeValidation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := &fakeRepository{}, newStore()
			input := validCreate(store.putPDF(validText()))
			tt.change(&input)
			_, err := newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", input)
			assertCode(t, err, tt.code)
			if store.downloads+repo.createCalls != 0 {
				t.Fatal("invalid input reached storage or the repository")
			}
		})
	}
}

func TestCreateRefusesMissingOrInvalidDocument(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(*fakeStore) string
		code  apperror.Code
	}{
		{"never uploaded", func(*fakeStore) string { return uuid.NewString() }, apperror.CodeInvalidPDF},
		{"not a PDF object", func(s *fakeStore) string {
			key := uuid.NewString()
			s.objects[key] = storedObject{data: []byte(validText()), contentType: "text/plain; charset=utf-8"}
			return key
		}, apperror.CodeInvalidPDF},
		{"unreadable PDF", func(s *fakeStore) string {
			key := uuid.NewString()
			s.objects[key] = storedObject{data: []byte("%PDF-broken"), contentType: pdfContentType}
			return key
		}, apperror.CodeInvalidPDF},
		{"too little text", func(s *fakeStore) string { return s.putPDF("Go developer") }, apperror.CodeJDTooShort},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, store := &fakeRepository{}, newStore()
			_, err := newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", validCreate(tt.setup(store)))
			assertCode(t, err, tt.code)
			if repo.createCalls != 0 {
				t.Fatal("rows were written without a readable PDF")
			}
		})
	}
}

func TestCreateStorageFailure(t *testing.T) {
	repo, store := &fakeRepository{}, newStore()
	documentID := store.putPDF(validText())
	store.downloadErr = errBoom
	_, err := newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", validCreate(documentID))
	assertCode(t, err, apperror.CodeInternal)
	if repo.createCalls != 0 {
		t.Fatal("rows were written without the PDF")
	}

	store.downloadErr = storage.ErrInvalidInput
	_, err = newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", validCreate(documentID))
	assertCode(t, err, apperror.CodeInvalidPDF)
}

func TestCreatePassesRepositoryErrorsThrough(t *testing.T) {
	repo, store := &fakeRepository{err: domain.ErrAvatarNotOwned}, newStore()
	_, err := newService(t, repo, store).Create(context.Background(), uuid.New(), "recruiter", validCreate(store.putPDF(validText())))
	assertCode(t, err, apperror.CodeValidation)
}
