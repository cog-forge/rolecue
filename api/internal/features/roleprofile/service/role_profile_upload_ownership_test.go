package service

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/internal/pkg/uploadgrant"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type grantRepo struct {
	owners   map[uuid.UUID]uuid.UUID
	consumed map[uuid.UUID]bool
	profiles int
}

func (r *grantRepo) GetProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error) {
	return domain.RoleProfile{}, nil
}
func (r *grantRepo) RecordUpload(_ context.Context, user, key uuid.UUID) error {
	r.owners[key] = user
	return nil
}

// CreatePDFProfile mirrors the repository: owner match and single use, claimed with the insert.
func (r *grantRepo) CreatePDFProfile(_ context.Context, user uuid.UUID, _, key string) (domain.RoleProfile, error) {
	id := uuid.MustParse(key)
	if owner, ok := r.owners[id]; !ok || owner != user || r.consumed[id] {
		return domain.RoleProfile{}, uploadgrant.ErrUnavailable
	}
	r.consumed[id] = true
	r.profiles++
	return domain.RoleProfile{ID: uuid.New()}, nil
}

type grantStore struct{ storage.Store }

func (grantStore) PresignPut(context.Context, string, int64, time.Duration) (storage.UploadGrant, error) {
	return storage.UploadGrant{Key: uuid.NewString()}, nil
}
func (grantStore) Download(context.Context, string) (storage.Object, error) {
	data := []byte("%PDF-1.4 body")
	return storage.Object{Body: io.NopCloser(bytes.NewReader(data)), ContentType: "application/pdf", Size: int64(len(data))}, nil
}

type textParser struct{}

func (textParser) Parse(context.Context, []byte) (string, error) { return "parsed text", nil }

func TestCompleteRequiresOwnUnusedUpload(t *testing.T) {
	ctx := context.Background()
	repo := &grantRepo{owners: map[uuid.UUID]uuid.UUID{}, consumed: map[uuid.UUID]bool{}}
	app := NewRoleProfileDocumentApplication(repo, grantStore{}, textParser{})
	owner, other := uuid.New(), uuid.New()
	ticket, err := app.Create(ctx, owner, "candidate", UploadRequest{FileName: "cv.pdf", SizeBytes: 100, ContentType: "application/pdf"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = app.Complete(ctx, other, "candidate", ticket.UploadID)
	assertCode(t, err, apperror.CodeInvalidPDF)
	if _, err = app.Complete(ctx, owner, "candidate", ticket.UploadID); err != nil {
		t.Fatal(err)
	}
	_, err = app.Complete(ctx, owner, "candidate", ticket.UploadID)
	assertCode(t, err, apperror.CodeInvalidPDF)
	_, err = app.Complete(ctx, owner, "candidate", uuid.New())
	assertCode(t, err, apperror.CodeInvalidPDF)
	if repo.profiles != 1 {
		t.Fatalf("profiles created = %d, want 1", repo.profiles)
	}
}
