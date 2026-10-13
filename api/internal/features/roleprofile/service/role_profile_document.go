package service

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type DocumentRepository interface {
	GetProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error)
	CreatePDFProfile(context.Context, uuid.UUID, string, string) (domain.RoleProfile, error)
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

func storageUnavailable() error {
	return apperror.New(apperror.CodeDocumentStorage, "Document storage is unavailable. Please retry.")
}

func (a *RoleProfileDocumentApplication) Create(ctx context.Context, input UploadRequest) (UploadTicket, error) {
	if input.SizeBytes > MaxPDFBytes {
		return UploadTicket{}, apperror.New(apperror.CodePDFTooLarge, "Choose a PDF up to 5 MB.")
	}
	name := strings.TrimSpace(input.FileName)
	if !boundedText(name, 1, 200) || filepath.Base(name) != name || strings.ContainsAny(name, "\\/") || strings.IndexFunc(name, unicode.IsControl) >= 0 || !strings.EqualFold(filepath.Ext(name), ".pdf") || input.ContentType != "application/pdf" || input.SizeBytes <= 0 {
		return UploadTicket{}, apperror.New(apperror.CodeInvalidPDF, "Choose a PDF with a valid filename, up to 5 MB.")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	grant, err := a.store.PresignPut(ctx, "application/pdf", input.SizeBytes, 5*time.Minute)
	if err != nil {
		return UploadTicket{}, storageUnavailable()
	}
	id, err := uuid.Parse(grant.Key)
	if err != nil {
		return UploadTicket{}, storageUnavailable()
	}
	return UploadTicket{UploadID: id, Request: grant.Request}, nil
}

func (a *RoleProfileDocumentApplication) Complete(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	objectKey := id.String()
	object, err := a.store.Download(ctx, objectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return domain.RoleProfile{}, apperror.New(apperror.CodeInvalidPDF, "The PDF has not finished uploading. Retry the upload.")
	}
	if err != nil {
		return domain.RoleProfile{}, storageUnavailable()
	}
	defer object.Body.Close()
	if object.Size <= 0 || object.Size > MaxPDFBytes || !strings.EqualFold(object.ContentType, "application/pdf") {
		return domain.RoleProfile{}, apperror.New(apperror.CodeInvalidPDF, "Choose a valid PDF up to 5 MB.")
	}
	data, err := io.ReadAll(io.LimitReader(object.Body, MaxPDFBytes+1))
	if err != nil {
		return domain.RoleProfile{}, storageUnavailable()
	}
	if int64(len(data)) != object.Size {
		return domain.RoleProfile{}, apperror.New(apperror.CodeInvalidPDF, "The uploaded PDF is incomplete. Retry the upload.")
	}
	raw, err := a.parser.Parse(ctx, data)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	return a.repo.CreatePDFProfile(ctx, user, raw, objectKey)
}

func (a *RoleProfileDocumentApplication) Document(ctx context.Context, user, id uuid.UUID) (storage.SignedRequest, error) {
	profile, err := a.repo.GetProfile(ctx, user, id)
	if err != nil {
		return storage.SignedRequest{}, err
	}
	if profile.ContentPath == "" {
		return storage.SignedRequest{}, apperror.New(apperror.CodeJDNotFound, "This Role Profile has no original PDF.")
	}
	request, err := a.store.PresignGet(ctx, profile.ContentPath, time.Minute)
	if err != nil {
		return storage.SignedRequest{}, storageUnavailable()
	}
	return request, nil
}
