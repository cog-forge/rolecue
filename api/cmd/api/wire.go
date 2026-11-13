//go:build wireinject
// +build wireinject

package main

import (
	applicationservice "github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/jobs"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/internal/pkg/tracer"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/provider"
	"github.com/cog-forge/rolecue/api/internal/server"
)

type Application struct {
	Server    *server.Server
	Pool      *pgxpool.Pool
	Logger    *logger.Logger
	Tracer    *tracer.Tracer
	Config    *config.Config
	Storage   storage.Store
	Documents *service.RoleProfileDocumentApplication
	// Synthesizer is the text-to-speech adapter; the interview engine and voice preview inject it.
	Synthesizer tts.Synthesizer
	// Expiry rejects Applications whose interview deadline passed; main starts it.
	Expiry *applicationservice.ExpiryRunner
	// Email sends transactional mail asynchronously; its consumers (KAN-219) inject it, and cleanup drains it.
	Email *email.Dispatcher
	// LiveInterviews drives the live interview sockets; main closes them on shutdown.
	LiveInterviews *runtime.Engine
	// Jobs runs the System Handler's background jobs; main starts it and stops it on shutdown.
	Jobs *jobs.Runner
}

func InitializeApplication(configPath string) (*Application, func(), error) {
	wire.Build(
		provider.ProvideConfig,
		provider.ProvideStorage,
		provider.ProvideSynthesizer,
		provider.ProvideInventoryRepository,
		provider.ProvideInventoryService,
		provider.ProvideInventoryHandler,
		provider.ProvideAvatarConverter,
		provider.ProvideAvatarConversionService,
		provider.ProvideAvatarCreateHandler,
		provider.ProvideLogger,
		provider.ProvideTracer,
		provider.ProvideDatabasePool,
		provider.ProvideAuthRepository,
		provider.ProvideAuthService,
		provider.ProvideAuthHandler,
		provider.ProvideHealthHandler,
		provider.ProvideRoleProfileRepository,
		provider.ProvideRoadmapRepository,
		provider.ProvideRoadmapGeneration,
		provider.ProvideRoleProfileExtraction,
		provider.ProvideRoleProfileService,
		provider.ProvideRoleProfileDocumentApplication,
		provider.ProvideRoleProfileHandler,
		provider.ProvideProfileRepository,
		provider.ProvideProfileService,
		provider.ProvideProfileHandler,
		provider.ProvideAccountRepository,
		provider.ProvideAccountService,
		provider.ProvideAccountHandler,
		provider.ProvideWalletRepository,
		provider.ProvidePayOSClient,
		provider.ProvideWalletService,
		provider.ProvideWalletHandler,
		provider.ProvideQuestionBankRepository,
		provider.ProvideQuestionBankService,
		provider.ProvideQuestionBankHandler,
		provider.ProvideModerationRepository,
		provider.ProvideModerationService,
		provider.ProvideModerationHandler,
		provider.ProvideOnboardingHandler,
		provider.ProvideAccountsHandler,
		provider.ProvideVoiceCatalogRepository,
		provider.ProvideVoiceCatalogProvider,
		provider.ProvideVoiceCatalogService,
		provider.ProvideVoiceCatalogHandler,
		provider.ProvideVoiceRepository,
		provider.ProvideVoiceService,
		provider.ProvideVoiceHandler,
		provider.ProvideJobPostingRepository,
		provider.ProvideJobPostingService,
		provider.ProvideJobPostingExtraction,
		provider.ProvideJobPostingHandler,
		provider.ProvideJobBoardRepository,
		provider.ProvideJobBoardService,
		provider.ProvideJobBoardHandler,
		provider.ProvideEvaluationRepository,
		provider.ProvideEvaluationService,
		provider.ProvideEvaluationHandler,
		provider.ProvideRubricService,
		provider.ProvideRubricHandler,
		provider.ProvideRoadmapReadService,
		provider.ProvideRoadmapHandler,
		provider.ProvideRoadmapSessionSource,
		provider.ProvideRoadmapAssessmentService,
		provider.ProvideRoadmapCatchUp,
		provider.ProvideApplicationRepository,
		provider.ProvideApplicationService,
		provider.ProvideApplicationHandler,
		provider.ProvideAdminInterviewsRepository,
		provider.ProvideAdminInterviewsService,
		provider.ProvideAdminInterviewsHandler,
		provider.ProvideAdminOverviewRepository,
		provider.ProvideAdminOverviewService,
		provider.ProvideAdminOverviewHandler,
		provider.ProvideAdminGuidelinesRepository,
		provider.ProvideAdminGuidelinesService,
		provider.ProvideAdminGuidelinesHandler,
		provider.ProvideLLMBudget,
		provider.ProvideInterviewRepository,
		provider.ProvideInterviewGrader,
		provider.ProvideInterviewEvaluationService,
		provider.ProvideInterviewService,
		provider.ProvideInterviewHandler,
		provider.ProvideInterviewHistoryHandler,
		provider.ProvideInterviewReportHandler,
		provider.ProvideInterviewRecordingService,
		provider.ProvideInterviewRecordingHandler,
		provider.ProvideTranscriber,
		provider.ProvideInterviewLiveEngine,
		provider.ProvideInterviewLiveHandler,
		provider.ProvideInterviewHandlers,
		provider.ProvideApplicationExpiryRunner,
		provider.ProvideRecordingUpkeep,
		provider.ProvideSystemJobs,
		provider.ProvideEmailSender,
		provider.ProvideEmailDispatcher,
		provider.ProvideRouter,
		provider.ProvideHTTPServer,
		wire.Struct(new(Application), "*"),
	)
	return &Application{}, nil, nil
}
