package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideInterviewRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideInterviewService takes the practice price and the question limit from the interview config.
func ProvideInterviewService(repo *repository.Repository, cfg *config.Config) (*service.Service, error) {
	return service.New(repo, service.Settings{
		PracticeCoins: cfg.Interview.PracticeCoins,
		QuestionLimit: int32(cfg.Interview.QuestionLimit),
	})
}

func ProvideInterviewHandler(svc *service.Service) *handler.InterviewHandler {
	return handler.NewInterviewHandler(svc)
}
