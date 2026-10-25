package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	evaluationservice "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideInterviewRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideInterviewService takes the practice price and the question limit from the interview config
// and hands completed sessions to the evaluation engine.
func ProvideInterviewService(repo *repository.Repository, cfg *config.Config, scoring *evaluationservice.Scoring) (*service.Service, error) {
	svc, err := service.New(repo, service.Settings{
		PracticeCoins: cfg.Interview.PracticeCoins,
		QuestionLimit: int32(cfg.Interview.QuestionLimit),
	})
	if err != nil {
		return nil, err
	}
	svc.SetEvaluator(scoring)
	return svc, nil
}

func ProvideInterviewHandler(svc *service.Service) *handler.InterviewHandler {
	return handler.NewInterviewHandler(svc)
}

// ProvideInterviewHandlers bundles every interview handler for the router.
func ProvideInterviewHandlers(
	sessions *handler.InterviewHandler,
	history *handler.InterviewHistoryHandler,
	report *handler.InterviewReportHandler,
	recording *handler.InterviewRecordingHandler,
) *handler.InterviewHandlers {
	return handler.NewInterviewHandlers(sessions, history, report, recording)
}
