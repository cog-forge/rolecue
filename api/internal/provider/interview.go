package provider

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/config"
	evaluationservice "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	roadmapservice "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideInterviewRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideInterviewService takes the question and follow-up limits from the interview config, hands
// completed sessions to the evaluation engine and decides follow-ups with the configured LLM. Without a
// usable LLM the API still starts and the interview asks core questions only. After a successful evaluation
// the results of a practice session are linked to Roadmap topics; that link can never fail the evaluation
// (KAN-350). A focused practice start tops up a topic through the Roadmap service (KAN-347).
func ProvideInterviewService(repo *repository.Repository, cfg *config.Config, scoring *evaluationservice.Scoring, budget llmbudget.Guard, roadmap *roadmapservice.Service, assessment *roadmapservice.AssessmentService) (*service.Service, error) {
	svc, err := service.New(repo, service.Settings{
		QuestionLimit:          int32(cfg.Interview.QuestionLimit),
		DefaultProviderVoiceID: cfg.Interview.DefaultProviderVoiceID,
		FollowUpLimit:          int32(cfg.Interview.FollowUpLimit),
		FollowUpTimeout:        cfg.Interview.FollowUpTimeout,
		AbandonTimeout:         cfg.Jobs.AbandonedSessions.Timeout,
	})
	if err != nil {
		return nil, err
	}
	svc.SetEvaluator(roadmapservice.NewEvaluateThenLink(scoring, assessment))
	svc.SetLLMBudget(budget)
	// A focused practice start tops up a topic that has too few bank questions before it charges.
	svc.SetTopicTopUp(repo, roadmap)
	if cfg.Interview.FollowUpLimit > 0 {
		decider, err := provideFollowUpDecider(cfg)
		if err != nil {
			logger.FromContext(context.Background()).Warn("Live interview follow-ups are off: the LLM is not usable", logger.Cause(err))
		} else {
			svc.SetFollowUpDecider(decider)
		}
	}
	return svc, nil
}

// provideFollowUpDecider routes decision and text together within the live time budget. Another
// decision provider plugs in here behind service.FollowUpDecider.
func provideFollowUpDecider(cfg *config.Config) (service.FollowUpDecider, error) {
	model, err := ai.NewTaskAdapter(context.Background(), cfg, config.TaskFollowUp)
	if err != nil {
		return nil, err
	}
	return model, nil
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
	live *handler.InterviewLiveHandler,
	guidelines *handler.AdminGuidelinesHandler,
) *handler.InterviewHandlers {
	return handler.NewInterviewHandlers(sessions, history, report, recording, live, guidelines)
}
