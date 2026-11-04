package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideApplicationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideApplicationService signs the CV links of Application Detail with the same storage Store the
// Job Posting service uses for its PDF links, and sends the Application lifecycle emails (KAN-219)
// through the shared dispatcher. A template left blank disables that one notice.
func ProvideApplicationService(repo *repository.Repository, store storage.Store, dispatcher *email.Dispatcher, cfg *config.Config) (*service.Service, error) {
	svc, err := service.New(repo, store)
	if err != nil {
		return nil, err
	}
	e := cfg.Email
	if err := svc.SetNotices(dispatcher, cfg.Auth.PublicURL, service.NoticeTemplates{
		Submitted:        e.ApplicationSubmittedTemplateID,
		NewApplication:   e.ApplicationNewApplicationTemplateID,
		CVRejected:       e.ApplicationCVRejectedTemplateID,
		CVApproved:       e.ApplicationCVApprovedTemplateID,
		DecisionApproved: e.ApplicationDecisionApprovedTemplateID,
		DecisionRejected: e.ApplicationDecisionRejectedTemplateID,
		AutoRejected:     e.ApplicationAutoRejectedTemplateID,
		IntakeClosed:     e.ApplicationIntakeClosedTemplateID,
	}); err != nil {
		return nil, err
	}
	return svc, nil
}
func ProvideApplicationHandler(svc *service.Service) *handler.ApplicationHandler {
	return handler.NewApplicationHandler(svc)
}
