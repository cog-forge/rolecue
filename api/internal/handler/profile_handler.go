package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	profiledomain "github.com/cog-forge/rolecue/api/internal/features/profile/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProfileService interface {
	Get(context.Context, uuid.UUID) (profiledomain.Profile, error)
	Update(context.Context, uuid.UUID, string, profiledomain.Patch) (profiledomain.Profile, error)
	CreatePhotoUpload(context.Context, uuid.UUID, profiledomain.PhotoUploadRequest) (profiledomain.PhotoUpload, error)
	ReadPhoto(context.Context, uuid.UUID, uuid.UUID) (storage.Object, error)
}
type ProfileHandler struct{ service ProfileService }

func NewProfileHandler(service ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

// Get godoc
// @Summary Get your own profile
// @Tags profile
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Envelope{data=profiledomain.Profile}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /profile [get]
func (h *ProfileHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	p, err := h.service.Get(c.Request.Context(), user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// Update godoc
// @Summary Update allowed fields in your own profile
// @Description While onboarding is pending, PATCH requires full_name and (for recruiters) company_name/company_website and atomically marks completion.
// @Description Omitted keys are unchanged. Null removes image/company values; full_name cannot be null. Send photo_upload_id to attach a finished upload, or image to update a legacy URL. Only recruiters may change company fields. Trusted Origin required. Unknown fields reject the whole request.
// @Tags profile
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param request body profiledomain.Patch true "Changed profile fields (at least one); body limit 16 KiB"
// @Success 200 {object} response.Envelope{data=profiledomain.Profile}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 429 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /profile [patch]
func (h *ProfileHandler) Update(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	patch, ok := decodeProfileJSON[profiledomain.Patch](c)
	if !ok {
		return
	}
	p, err := h.service.Update(c.Request.Context(), user.ID, user.Role, *patch)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

func decodeProfileJSON[T any](c *gin.Context) (*T, bool) {
	if c.ContentType() != "application/json" {
		response.Error(c, apperror.New(apperror.CodeValidation, "Content-Type must be application/json"))
		return nil, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var payload *T
	if err := decoder.Decode(&payload); err != nil || payload == nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid request body"))
		return nil, false
	}
	return payload, true
}

// CreatePhotoUpload godoc
// @Summary Start a private profile photo upload
// @Tags profile
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body profiledomain.PhotoUploadRequest true "JPEG, PNG or WebP MIME and size (up to 5 MiB)"
// @Success 201 {object} response.Envelope{data=profiledomain.PhotoUpload}
// @Failure 400,401,403,429,500 {object} response.Envelope
// @Router /profile/photo-uploads [post]
func (h *ProfileHandler) CreatePhotoUpload(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	if !user.OnboardingRoleSelected && user.Role != "admin" {
		response.Error(c, apperror.New(apperror.CodeForbidden, "choose your role before uploading a photo"))
		return
	}
	input, ok := decodeProfileJSON[profiledomain.PhotoUploadRequest](c)
	if !ok {
		return
	}
	ticket, err := h.service.CreatePhotoUpload(c.Request.Context(), user.ID, *input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, ticket)
}

// ReadPhoto godoc
// @Summary Read the current profile photo as its owner or an administrator
// @Tags profile
// @Security SessionCookie
// @Param owner path string true "Profile owner UUID"
// @Param key path string true "Current profile photo UUID"
// @Success 200 {file} file
// @Failure 401,403,404,500 {object} response.Envelope
// @Router /profile/photos/{owner}/{key} [get]
func (h *ProfileHandler) ReadPhoto(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	owner, ownerErr := uuid.Parse(c.Param("owner"))
	key, keyErr := uuid.Parse(c.Param("key"))
	if ownerErr != nil || keyErr != nil || (owner != user.ID && user.Role != "admin") {
		response.Error(c, apperror.New(apperror.CodeAccountNotFound, "profile photo not found"))
		return
	}
	photo, err := h.service.ReadPhoto(c.Request.Context(), owner, key)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer func() { _ = photo.Body.Close() }()
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.DataFromReader(http.StatusOK, photo.Size, photo.ContentType, photo.Body, nil)
}
