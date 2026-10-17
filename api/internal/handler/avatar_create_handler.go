package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type AvatarCreateService interface {
	Create(context.Context, authdomain.User, io.ReadSeeker, int64, domain.CreateAvatarInput) (domain.Avatar, error)
}

type AvatarCreateHandler struct {
	service AvatarCreateService
	maxSize int64
	timeout time.Duration
}

func NewAvatarCreateHandler(service AvatarCreateService, maxSize int64, timeout time.Duration) *AvatarCreateHandler {
	return &AvatarCreateHandler{service: service, maxSize: maxSize, timeout: timeout}
}

// Create godoc
// @Summary Create a personal VRM avatar from an Avaturn T2 GLB
// @Description Converts the supplied lip-sync GLB and stores the VRM before creating an inventory entry. Candidate/Recruiter only.
// @Tags inventory
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param file formData file true "Avaturn T2 binary GLB"
// @Param metadata formData string true "JSON with name and personality"
// @Success 201 {object} response.Envelope{data=domain.Avatar}
// @Failure 400,401,403,409,413,422,503 {object} response.Envelope
// @Router /inventory/avatars [post]
func (h *AvatarCreateHandler) Create(c *gin.Context) {
	user, ok := inventoryUser(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" {
		response.Error(c, apperror.New(apperror.CodeValidation, "query parameters are not supported"))
		return
	}
	controller := http.NewResponseController(c.Writer)
	deadline := time.Now().Add(h.timeout + 90*time.Second)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
	defer func() { _ = controller.SetReadDeadline(time.Time{}); _ = controller.SetWriteDeadline(time.Time{}) }()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxSize+(16<<10))
	reader, err := c.Request.MultipartReader()
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "multipart file and metadata required"))
		return
	}
	source, err := os.CreateTemp("", "rolecue-avatar-upload-")
	if err != nil {
		response.Error(c, err)
		return
	}
	defer func() { _ = source.Close(); _ = os.Remove(source.Name()) }()
	var metadata domain.CreateAvatarInput
	var size int64
	fileSeen, metadataSeen := false, false
	for {
		part, readErr := reader.NextPart()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			avatarInputError(c, readErr)
			return
		}
		switch part.FormName() {
		case "file":
			if fileSeen {
				_ = part.Close()
				avatarInputError(c, nil)
				return
			}
			fileSeen = true
			size, err = io.Copy(source, io.LimitReader(part, h.maxSize+1))
			if size > h.maxSize {
				_ = part.Close()
				response.Error(c, apperror.New(apperror.CodeAvatarSourceTooLarge, "avatar GLB exceeds the upload limit"))
				return
			}
		case "metadata":
			if metadataSeen {
				_ = part.Close()
				avatarInputError(c, nil)
				return
			}
			metadataSeen = true
			data, readErr := io.ReadAll(io.LimitReader(part, 16385))
			if readErr != nil || len(data) > 16384 {
				_ = part.Close()
				avatarInputError(c, readErr)
				return
			}
			err = decodeAvatarMetadata(data, &metadata)
		default:
			_ = part.Close()
			avatarInputError(c, nil)
			return
		}
		_ = part.Close()
		if err != nil {
			avatarInputError(c, err)
			return
		}
	}
	if !fileSeen || !metadataSeen || size == 0 {
		avatarInputError(c, nil)
		return
	}
	if err := metadata.Validate(); err != nil {
		response.Error(c, err)
		return
	}
	avatar, err := h.service.Create(c.Request.Context(), user, source, size, metadata)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusCreated, response.Envelope{Success: true, Data: avatar})
}

func decodeAvatarMetadata(data []byte, target *domain.CreateAvatarInput) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("metadata must contain one JSON object")
	}
	return nil
}

func avatarInputError(c *gin.Context, err error) {
	var oversized *http.MaxBytesError
	if errors.As(err, &oversized) {
		response.Error(c, apperror.New(apperror.CodeAvatarSourceTooLarge, "avatar upload exceeds the limit"))
		return
	}
	response.Error(c, apperror.New(apperror.CodeValidation, "invalid avatar multipart request"))
}
