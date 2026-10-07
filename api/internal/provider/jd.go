package provider

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/cog-forge/rolecue/api/internal/config"
	jdprovider "github.com/cog-forge/rolecue/api/internal/features/jd/provider"
	"github.com/cog-forge/rolecue/api/internal/features/jd/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jd/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
)

func ProvideJDRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideJDService(cfg *config.Config, repo *repository.Repository) (*service.Application, error) {
	model, err := ai.NewChatModel(context.Background(), cfg.LLM)
	if err != nil {
		return nil, err
	}
	extractor, err := jdprovider.New(model)
	if err != nil {
		return nil, err
	}
	extraction, err := service.New(extractor, cfg.LLM.MaxRetries)
	if err != nil {
		return nil, err
	}
	return service.NewApplication(extraction, repo)
}
func ProvideJDHandler(app *service.Application) *handler.JDHandler { return handler.NewJDHandler(app) }
