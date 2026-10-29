package handler

import (
	"context"
	"strconv"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type VoiceCatalogService interface {
	List(ctx context.Context, role string, filter domain.VoiceListFilter) ([]domain.VoiceProfile, int64, error)
	Get(ctx context.Context, role string, id uuid.UUID) (domain.VoiceProfile, error)
	Remove(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.VoiceDeleteResult, error)
	Activate(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.VoiceProfile, error)
	Sync(ctx context.Context, adminID uuid.UUID, role string) (domain.VoiceSyncResult, error)
}
type VoiceCatalogHandler struct{ service VoiceCatalogService }

func NewVoiceCatalogHandler(service VoiceCatalogService) *VoiceCatalogHandler {
	return &VoiceCatalogHandler{service: service}
}

func voiceCatalogUser(c *gin.Context) (uuid.UUID, string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return uuid.Nil, "", false
	}
	return user.ID, user.Role, true
}

func voiceCatalogID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.New(apperror.CodeValidation, "invalid voice ID"))
		return uuid.Nil, false
	}
	return id, true
}

// voiceCatalogFilter reads the catalog filters. Each parameter may appear at most once.
func voiceCatalogFilter(c *gin.Context) (domain.VoiceListFilter, bool) {
	filter := domain.VoiceListFilter{Status: domain.VoiceStatusAll, Limit: 20}
	fail := func(msg string) (domain.VoiceListFilter, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, msg))
		return domain.VoiceListFilter{}, false
	}
	single := func(name string) (string, bool, bool) {
		raw, exists := c.GetQueryArray(name)
		if !exists {
			return "", false, true
		}
		return raw[0], true, len(raw) == 1
	}
	for _, param := range []struct {
		name     string
		value    *int32
		min, max int64
	}{{"limit", &filter.Limit, 1, 100}, {"offset", &filter.Offset, 0, 2147483647}} {
		raw, exists, once := single(param.name)
		if !exists {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 32)
		if !once || err != nil || value < param.min || value > param.max {
			return fail("invalid pagination parameters")
		}
		*param.value = int32(value)
	}
	if raw, exists, once := single("status"); exists {
		status := domain.VoiceStatusFilter(raw)
		if !once || !status.Valid() {
			return fail("invalid status filter")
		}
		filter.Status = status
	}
	if raw, exists, once := single("q"); exists {
		if !once {
			return fail("invalid search text")
		}
		filter.Query = raw
	}
	return filter, true
}

// List godoc
// @Summary List voice profiles
// @Description Admin only. Sorted by name. Each voice says how many interviews and Job Postings reference it; a referenced voice can only be deactivated.
// @Tags admin-voices
// @Produce json
// @Security SessionCookie
// @Param status query string false "all (default), active or inactive"
// @Param q query string false "Case-insensitive name search, up to 100 characters"
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Number of items to skip" default(0) minimum(0) maximum(2147483647)
// @Success 200 {object} response.Envelope{data=response.Page[domain.VoiceProfile]}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/voices [get]
func (h *VoiceCatalogHandler) List(c *gin.Context) {
	_, role, ok := voiceCatalogUser(c)
	if !ok {
		return
	}
	filter, ok := voiceCatalogFilter(c)
	if !ok {
		return
	}
	result, total, err := h.service.List(c.Request.Context(), role, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	if result == nil {
		result = []domain.VoiceProfile{}
	}
	response.OK(c, response.Page[domain.VoiceProfile]{Items: result, Pagination: response.Pagination{Limit: filter.Limit, Offset: filter.Offset, Total: total}})
}

// Get godoc
// @Summary Get a voice profile
// @Description Admin only.
// @Tags admin-voices
// @Produce json
// @Security SessionCookie
// @Param id path string true "Voice ID"
// @Success 200 {object} response.Envelope{data=domain.VoiceProfile}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/voices/{id} [get]
func (h *VoiceCatalogHandler) Get(c *gin.Context) {
	_, role, ok := voiceCatalogUser(c)
	if !ok {
		return
	}
	id, ok := voiceCatalogID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Remove godoc
// @Summary Remove a voice profile from the catalog
// @Description Admin only. A voice referenced by any interview or Job Posting is deactivated (is_active=false) so past sessions keep it; an unreferenced voice is deleted. outcome says which happened. Repeating the call on a deactivated voice returns "deactivated" again. Trusted Origin required.
// @Tags admin-voices
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Voice ID"
// @Success 200 {object} response.Envelope{data=domain.VoiceDeleteResult}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/voices/{id} [delete]
func (h *VoiceCatalogHandler) Remove(c *gin.Context) {
	adminID, role, ok := voiceCatalogUser(c)
	if !ok {
		return
	}
	id, ok := voiceCatalogID(c)
	if !ok {
		return
	}
	result, err := h.service.Remove(c.Request.Context(), adminID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Activate godoc
// @Summary Reactivate a voice profile
// @Description Admin only. Sets is_active=true so the voice can be chosen again; repeating it on an active voice changes nothing. A voice that was deleted permanently returns 404. Trusted Origin required.
// @Tags admin-voices
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param id path string true "Voice ID"
// @Success 200 {object} response.Envelope{data=domain.VoiceProfile}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/voices/{id}/activate [patch]
func (h *VoiceCatalogHandler) Activate(c *gin.Context) {
	adminID, role, ok := voiceCatalogUser(c)
	if !ok {
		return
	}
	id, ok := voiceCatalogID(c)
	if !ok {
		return
	}
	result, err := h.service.Activate(c.Request.Context(), adminID, role, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Sync godoc
// @Summary Sync the voice catalog from the TTS provider
// @Description Admin only. Fetches the provider's English voices (the locales are configuration) and upserts them by provider voice ID: new voices are added, changed ones updated, and voices the provider no longer offers are deactivated (is_active=false), never deleted. Voices an Admin deactivated stay inactive. If the provider fails or answers with an invalid or empty list, nothing changes and the call fails. Repeating it changes nothing. Trusted Origin required.
// @Tags admin-voices
// @Produce json
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Success 200 {object} response.Envelope{data=domain.VoiceSyncResult}
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Failure 502 {object} response.Envelope "The provider failed or returned an invalid list"
// @Failure 503 {object} response.Envelope "The provider is not configured"
// @Router /admin/voices/sync [post]
func (h *VoiceCatalogHandler) Sync(c *gin.Context) {
	adminID, role, ok := voiceCatalogUser(c)
	if !ok {
		return
	}
	result, err := h.service.Sync(c.Request.Context(), adminID, role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
