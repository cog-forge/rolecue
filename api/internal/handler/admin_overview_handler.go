package handler

import (
	"context"
	"slices"
	"strconv"

	overviewdomain "github.com/cog-forge/rolecue/api/internal/features/admin/overview/domain"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type AdminOverviewService interface {
	Get(ctx context.Context, role string, days int) (overviewdomain.Overview, error)
}

type AdminOverviewHandler struct{ service AdminOverviewService }

func NewAdminOverviewHandler(service AdminOverviewService) *AdminOverviewHandler {
	return &AdminOverviewHandler{service: service}
}

// adminOverviewDays reads the window. The parameter may be absent (the default window, returned as
// 0) or appear exactly once with 7, 30 or 90. Anything else is a validation error.
func adminOverviewDays(c *gin.Context) (int, bool) {
	raw, present := c.GetQueryArray("days")
	if !present {
		return 0, true
	}
	fail := func() (int, bool) {
		response.Error(c, apperror.New(apperror.CodeValidation, "days must be 7, 30 or 90"))
		return 0, false
	}
	if len(raw) != 1 {
		return fail()
	}
	days, err := strconv.Atoi(raw[0])
	if err != nil || !slices.Contains(overviewdomain.WindowDays, days) {
		return fail()
	}
	return days, true
}

// Get godoc
// @Summary Get the Admin platform and finance overview
// @Description Admin only. Read-only aggregates over a window of 7, 30 or 90 days (default 30). The window is [now-days, now). Accounts count Candidate and Recruiter users only (Admin excluded). locked means is_locked and the lock has not expired. new_in_window counts users created in the window. job_postings and applications by status are all-time counts; submitted_in_window counts applications created in the window. Interview *_started_in_window use started_at by type, completed_in_window uses completed_at, evaluation_failed is all-time. finance is the wallet billing summary over the window: revenue is successful PayOS top-ups only, and coin debits and refunds are not revenue. The response carries no personal data.
// @Tags admin-overview
// @Produce json
// @Security SessionCookie
// @Param days query int false "Window in days" Enums(7, 30, 90) default(30)
// @Success 200 {object} response.Envelope{data=domain.Overview}
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /admin/overview [get]
func (h *AdminOverviewHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, apperror.New(apperror.CodeInvalidToken, "authentication required"))
		return
	}
	days, ok := adminOverviewDays(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), user.Role, days)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}
