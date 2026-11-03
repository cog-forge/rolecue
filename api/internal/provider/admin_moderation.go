package provider

import (
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/service"
	evaluationservice "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideModerationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideModerationService(repo *repository.Repository, store storage.Store, log *logger.Logger, evaluation *evaluationservice.Service, dispatcher *email.Dispatcher, cfg *config.Config) (*service.Service, error) {
	svc, err := service.New(repo, store, log)
	if err != nil {
		return nil, err
	}
	svc.SetEvaluationReader(evaluation)
	approvedID, rejectedID := cfg.Email.ModerationApprovedTemplateID, cfg.Email.ModerationRejectedTemplateID
	if cfg.Email.Provider == "resend" && (approvedID == "" || rejectedID == "") {
		return nil, fmt.Errorf("moderation email requires EMAIL_MODERATION_APPROVED_TEMPLATE_ID and EMAIL_MODERATION_REJECTED_TEMPLATE_ID")
	}
	// Log-only development never contacts the provider and needs no published templates.
	if approvedID == "" {
		approvedID = "rolecue-posting-approved"
	}
	if rejectedID == "" {
		rejectedID = "rolecue-posting-rejected"
	}
	if err := svc.SetNotices(dispatcher, cfg.Auth.PublicURL, approvedID, rejectedID); err != nil {
		return nil, err
	}
	return svc, nil
}
func ProvideModerationHandler(svc *service.Service) *handler.ModerationHandler {
	return handler.NewModerationHandler(svc)
}
