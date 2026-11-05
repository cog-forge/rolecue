package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// upload runs the real upload path for a Recruiter, so the grant is recorded as in production.
func upload(t *testing.T, s *Service, recruiter uuid.UUID) string {
	t.Helper()
	doc, err := s.UploadDocument(context.Background(), recruiter, "recruiter", fakePDF(validText()))
	if err != nil {
		t.Fatal(err)
	}
	return doc.DocumentID
}

func TestCreateRequiresOwnUnusedUpload(t *testing.T) {
	ctx := context.Background()
	repo, store := &fakeRepository{}, newStore()
	s := newService(t, repo, store)
	owner, other := uuid.New(), uuid.New()
	key := upload(t, s, owner)

	t.Run("foreign key is rejected", func(t *testing.T) {
		_, err := s.Create(ctx, other, "recruiter", validCreate(key))
		assertCode(t, err, apperror.CodeInvalidPDF)
	})
	t.Run("owner succeeds", func(t *testing.T) {
		if _, err := s.Create(ctx, owner, "recruiter", validCreate(key)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("replay is rejected", func(t *testing.T) {
		_, err := s.Create(ctx, owner, "recruiter", validCreate(key))
		assertCode(t, err, apperror.CodeInvalidPDF)
	})
	t.Run("key without a grant is rejected", func(t *testing.T) {
		_, err := s.Create(ctx, owner, "recruiter", validCreate(store.putPDF(validText())))
		assertCode(t, err, apperror.CodeInvalidPDF)
	})
}

func TestUpdateDocumentRequiresOwnUnusedUpload(t *testing.T) {
	ctx := context.Background()
	repo, store := &fakeRepository{record: domain.Record{ContentPath: uuid.NewString(), JobPosting: domain.JobPosting{Status: domain.StatusRejected}}}, newStore()
	s := newService(t, repo, store)
	owner, other := uuid.New(), uuid.New()
	key := upload(t, s, owner)
	patch := domain.Edit{DocumentID: domain.StringChange{Present: true, Value: &key}}

	_, err := s.Update(ctx, other, "recruiter", uuid.New(), patch)
	assertCode(t, err, apperror.CodeInvalidPDF)
	if _, err = s.Update(ctx, owner, "recruiter", uuid.New(), patch); err != nil {
		t.Fatal(err)
	}
	// The posting now holds the key, so resending it is a no-op; a different posting is a replay.
	repo.record.ContentPath = uuid.NewString()
	_, err = s.Update(ctx, owner, "recruiter", uuid.New(), patch)
	assertCode(t, err, apperror.CodeInvalidPDF)
}
