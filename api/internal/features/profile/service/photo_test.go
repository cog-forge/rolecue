package service

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestVerifiedPhotoChecksStoredBytes(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	pngBytes := encoded.Bytes()
	for _, tc := range []struct {
		name        string
		contentType string
		data        []byte
		size        int
		valid       bool
	}{
		{"valid photo", "image/png", pngBytes, len(pngBytes), true},
		{"spoofed content type", "image/jpeg", pngBytes, len(pngBytes), false},
		{"truncated photo", "image/png", pngBytes[:40], 40, false},
		{"incomplete download", "image/png", pngBytes, len(pngBytes) + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := verifiedPhoto(storage.Object{
				Body: io.NopCloser(bytes.NewReader(tc.data)), ContentType: tc.contentType, Size: int64(tc.size),
			})
			if tc.valid {
				if err != nil || !bytes.Equal(data, tc.data) {
					t.Fatalf("expected the verified photo bytes, got %d bytes, %v", len(data), err)
				}
			} else if !apperror.IsCode(err, apperror.CodeInvalidProfilePhoto) {
				t.Fatalf("expected invalid profile photo, got %v", err)
			}
		})
	}
}
