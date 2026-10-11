package provider

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/config"
	qbprovider "github.com/cog-forge/rolecue/api/internal/features/questionbank/provider"
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
	model, err := ai.NewChatModel(context.Background(), cfg.LLM)
	if err != nil {
		return nil, err
	}
	generator, err := qbprovider.New(model)
	if err != nil {
		return nil, err
	}
	return service.New(repo, generator, cfg.LLM.MaxRetries, cfg.QuestionBank.Size)
}
func ProvideQuestionBankHandler(svc *service.Service) *handler.QuestionBankHandler {
	return handler.NewQuestionBankHandler(svc)
}
