package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DocumentRepository interface {
	GetProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	CreatePDFProfile(context.Context, uuid.UUID, string, string) (domain.RoleProfile, error)
	RecordUpload(context.Context, uuid.UUID, uuid.UUID) error
}

type RoleProfileDocumentApplication struct {
	repo   DocumentRepository
	store  storage.Store
	parser PDFParser
}

func NewRoleProfileDocumentApplication(repo DocumentRepository, store storage.Store, parser PDFParser) *RoleProfileDocumentApplication {
	return &RoleProfileDocumentApplication{repo: repo, store: store, parser: parser}
}

type UploadRequest struct {
	FileName    string `json:"fileName"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType"`
}
type UploadTicket struct {
	UploadID uuid.UUID             `json:"uploadId"`
	Request  storage.SignedRequest `json:"request"`
}

// storageUnavailable keeps the storage failure as the cause so logs show what the store reported.
func storageUnavailable(cause error) error {
	return apperror.Wrap(apperror.CodeDocumentStorage, "Document storage is unavailable. Please retry.", cause)
}

// uploadRule names the first upload rule a request breaks, or "" when the request is valid.
func uploadRule(name string, input UploadRequest) string {
	switch {
	case !boundedText(name, 1, 200):
		return "filename length"
	case filepath.Base(name) != name || strings.ContainsAny(name, "\\/"):
		return "filename path"
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return "filename control character"
	case !strings.EqualFold(filepath.Ext(name), ".pdf"):
		return "filename extension"
	case input.ContentType != "application/pdf":
		return "content type"
	case input.SizeBytes <= 0:
		return "size"
	}
	return ""
}

// storedObjectRule names the first rule a stored upload breaks, or "" when it is valid.
func storedObjectRule(object storage.Object) string {
	switch {
	case object.Size <= 0:
		return "stored size is empty"
	case object.Size > MaxPDFBytes:
		return "stored size exceeds limit"
	case !strings.EqualFold(object.ContentType, "application/pdf"):
		return "stored content type"
	}
	return ""
}

func (a *RoleProfileDocumentApplication) Create(ctx context.Context, user uuid.UUID, role string, input UploadRequest) (UploadTicket, error) {
	if err := requireCandidate(role); err != nil {
		return UploadTicket{}, err
	}
	if input.SizeBytes > MaxPDFBytes {
		return UploadTicket{}, apperror.New(apperror.CodePDFTooLarge, "Choose a PDF up to 5 MB.")
	}
	name := strings.TrimSpace(input.FileName)
	if rule := uploadRule(name, input); rule != "" {
		return UploadTicket{}, apperror.Wrap(apperror.CodeInvalidPDF, "Choose a PDF with a valid filename, up to 5 MB.", fmt.Errorf("upload rule failed: %s", rule))
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	grant, err := a.store.PresignPut(ctx, "application/pdf", input.SizeBytes, 5*time.Minute)
	if err != nil {
		return UploadTicket{}, storageUnavailable(err)
	}
	id, err := uuid.Parse(grant.Key)
	if err != nil {
		return UploadTicket{}, storageUnavailable(err)
	}
	// Binds the key to this Candidate; Complete refuses a key issued to anyone else.
	if err := a.repo.RecordUpload(ctx, user, id); err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{UploadID: id, Request: grant.Request}, nil
}

func (a *RoleProfileDocumentApplication) Complete(ctx context.Context, user uuid.UUID, role string, id uuid.UUID) (domain.RoleProfile, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoleProfile{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	objectKey := id.String()
	downloadDone := logger.Start(ctx, "roleprofile.document_download", logger.String("upload_id", objectKey))
	object, err := a.store.Download(ctx, objectKey)
	if err != nil {
		downloadDone(err)
		if errors.Is(err, storage.ErrNotFound) {
			return domain.RoleProfile{}, apperror.New(apperror.CodeInvalidPDF, "The PDF has not finished uploading. Retry the upload.")
		}
		return domain.RoleProfile{}, storageUnavailable(err)
	}
	downloadDone(nil, zap.Int64("size_bytes", object.Size))
	defer func() { _ = object.Body.Close() }()
	if rule := storedObjectRule(object); rule != "" {
		return domain.RoleProfile{}, apperror.Wrap(apperror.CodeInvalidPDF, "Choose a valid PDF up to 5 MB.", fmt.Errorf("upload rule failed: %s", rule))
	}
	data, err := io.ReadAll(io.LimitReader(object.Body, MaxPDFBytes+1))
	if err != nil {
		return domain.RoleProfile{}, storageUnavailable(err)
	}
	if int64(len(data)) != object.Size {
		return domain.RoleProfile{}, apperror.Wrap(apperror.CodeInvalidPDF, "The uploaded PDF is incomplete. Retry the upload.",
			fmt.Errorf("read %d bytes, stored size %d", len(data), object.Size))
	}
	raw, err := a.parser.Parse(ctx, data)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	return a.repo.CreatePDFProfile(ctx, user, raw, objectKey)
}

func (a *RoleProfileDocumentApplication) Document(ctx context.Context, user uuid.UUID, role string, id uuid.UUID) (storage.SignedRequest, error) {
	if err := requireCandidate(role); err != nil {
		return storage.SignedRequest{}, err
	}
	profile, err := a.repo.GetProfile(ctx, user, id)
	if err != nil {
		return storage.SignedRequest{}, err
	}
	if profile.ContentPath == "" {
		return storage.SignedRequest{}, apperror.New(apperror.CodeJDNotFound, "This Role Profile has no original PDF.")
	}
	request, err := a.store.PresignGet(ctx, profile.ContentPath, time.Minute)
	if err != nil {
		return storage.SignedRequest{}, storageUnavailable(err)
	}
	return request, nil
}
