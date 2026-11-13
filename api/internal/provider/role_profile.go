package provider

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/config"
	questionbankservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	roadmapservice "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
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
	model, err := ai.NewTaskAdapter(context.Background(), cfg, config.TaskJD)
	if err != nil {
		return nil, err
	}
	retries := 0
	if cfg.AI.Route(config.TaskJD) == (config.TaskRoute{}) {
		retries = cfg.LLM.MaxRetries
	}
	return service.NewExtractionService(model, retries)
}
func ProvideRoleProfileService(cfg *config.Config, extraction *service.ExtractionService, repo *repository.RoleProfileRepository, questionBank *questionbankservice.Service, roadmap *roadmapservice.Service) (*service.RoleProfileApplication, error) {
	primary, _ := cfg.TaskLLM(config.TaskJD)
	app, err := service.NewRoleProfileApplication(extraction, repo, questionBank, primary.Timeout)
	if err != nil {
		return nil, err
	}
	return app.WithRoadmap(roadmap), nil
}
func ProvideRoleProfileDocumentApplication(repo *repository.RoleProfileRepository, store storage.Store) *service.RoleProfileDocumentApplication {
	return service.NewRoleProfileDocumentApplication(repo, store, service.PopplerParser{})
}
func ProvideRoleProfileHandler(app *service.RoleProfileApplication, documents *service.RoleProfileDocumentApplication, roadmap *roadmapservice.Service) *handler.RoleProfileHandler {
	return handler.NewRoleProfileHandler(app, documents).WithRoadmap(roadmap)
}
