package domain

import (
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

var ErrPhotoUploadUnavailable = apperror.New(apperror.CodeProfilePhotoUploadUnavailable, "This photo upload is missing, expired, or already used. Upload the photo again.")

type PhotoUploadRequest struct {
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type PhotoUpload struct {
	UploadID uuid.UUID             `json:"upload_id"`
	Request  storage.SignedRequest `json:"request"`
}

func PhotoPath(owner, key uuid.UUID) string {
	return "/profile/photos/" + owner.String() + "/" + key.String()
}
