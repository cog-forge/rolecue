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

// ProvideQuestionBankRepository gives the bank's coverage reads the session question limit of the interview
// config, the same value the interview start uses.
func ProvideQuestionBankRepository(pool *pgxpool.Pool, cfg *config.Config) *repository.Repository {
	return repository.NewRepository(pool, repository.WithQuestionLimit(cfg.Interview.QuestionLimit))
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
	svc, err := service.New(repo, model, retries, cfg.QuestionBank.Size)
	if err != nil {
		return nil, err
	}
	svc.SetQuestionLimit(cfg.Interview.QuestionLimit)
	return svc, nil
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
