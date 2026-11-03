package provider

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideQuestionBankRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideQuestionBankService(cfg *config.Config, repo *repository.Repository) (*service.Service, error) {
	model, err := ai.NewTaskAdapter(context.Background(), cfg, config.TaskBank)
	if err != nil {
		return nil, err
	}
	retries := 0
	if cfg.AI.Route(config.TaskBank) == (config.TaskRoute{}) {
		retries = cfg.LLM.MaxRetries
	}
	return service.New(repo, model, retries, cfg.QuestionBank.Size)
}
func ProvideQuestionBankHandler(cfg *config.Config, svc *service.Service) *handler.QuestionBankHandler {
	return handler.NewQuestionBankHandler(svc, generationBudget(cfg))
}

// generationBudget leaves time to persist and respond after the task deadline. Unconfigured routes
// retain the existing per-service-attempt budget.
func generationBudget(cfg *config.Config) time.Duration {
	if cfg.AI.Route(config.TaskBank) == (config.TaskRoute{}) {
		return time.Duration(cfg.LLM.MaxRetries+1)*cfg.LLM.Timeout + 5*time.Second
	}
	primary, _ := cfg.TaskLLM(config.TaskBank)
	return primary.Timeout + 5*time.Second
}
