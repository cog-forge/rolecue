package provider

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/config"
	questionbankservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	roleprofileprovider "github.com/cog-forge/rolecue/api/internal/features/roleprofile/provider"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/repository"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideRoleProfileRepository(pool *pgxpool.Pool) *repository.RoleProfileRepository {
	return repository.NewRoleProfileRepository(pool)
}
func ProvideRoleProfileExtraction(cfg *config.Config) (*service.ExtractionService, error) {
	model, err := ai.NewChatModel(context.Background(), cfg.LLM)
	if err != nil {
		return nil, err
	}
	extractor, err := roleprofileprovider.New(model)
	if err != nil {
		return nil, err
	}
	return service.NewExtractionService(extractor, cfg.LLM.MaxRetries)
}
func ProvideRoleProfileService(cfg *config.Config, extraction *service.ExtractionService, repo *repository.RoleProfileRepository, questionBank *questionbankservice.Service) (*service.RoleProfileApplication, error) {
	return service.NewRoleProfileApplication(extraction, repo, questionBank, cfg.LLM.Timeout)
}
func ProvideRoleProfileDocumentApplication(repo *repository.RoleProfileRepository, store storage.Store) *service.RoleProfileDocumentApplication {
	return service.NewRoleProfileDocumentApplication(repo, store, service.PopplerParser{})
}
func ProvideRoleProfileHandler(app *service.RoleProfileApplication, documents *service.RoleProfileDocumentApplication) *handler.RoleProfileHandler {
	return handler.NewRoleProfileHandler(app, documents)
}
