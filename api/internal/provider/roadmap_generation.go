package provider

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/config"
	questionbankrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	questionbankservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	roadmaprepository "github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	roadmapservice "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvideRoadmapRepository is the one provider of the Roadmap repository; the read API and the generation share it.
func ProvideRoadmapRepository(pool *pgxpool.Pool) *roadmaprepository.Repository {
	return roadmaprepository.NewRepository(pool)
}

// ProvideRoadmapGeneration builds the service Confirm uses to generate a Roadmap and its bank. The topic step
// uses the question bank's model route (timeout, retries, fallback): both steps are the same kind of
// generation, and no new AI task needs configuring.
func ProvideRoadmapGeneration(cfg *config.Config, repo *roadmaprepository.Repository, inputs *questionbankrepository.Repository,
	bank *questionbankservice.Service) (*roadmapservice.Service, error) {
	model, err := ai.NewTaskAdapter(context.Background(), cfg, config.TaskBank)
	if err != nil {
		return nil, err
	}
	retries := 0
	if cfg.AI.Route(config.TaskBank) == (config.TaskRoute{}) {
		retries = cfg.LLM.MaxRetries
	}
	return roadmapservice.New(repo, model, inputs, bank, inputs, retries)
}
