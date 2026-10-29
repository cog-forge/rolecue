//go:build wireinject
// +build wireinject

package main

import (
	applicationservice "github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
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
		provider.ProvideRoleProfileExtraction,
		provider.ProvideRoleProfileService,
		provider.ProvideRoleProfileDocumentApplication,
		provider.ProvideRoleProfileHandler,
		provider.ProvideProfileRepository,
		provider.ProvideProfileService,
		provider.ProvideProfileHandler,
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
		provider.ProvideApplicationRepository,
		provider.ProvideApplicationService,
		provider.ProvideApplicationHandler,
		provider.ProvideAdminInterviewsRepository,
		provider.ProvideAdminInterviewsService,
		provider.ProvideAdminInterviewsHandler,
		provider.ProvideInterviewRepository,
		provider.ProvideInterviewScorer,
		provider.ProvideInterviewEvaluationService,
		provider.ProvideInterviewService,
		provider.ProvideInterviewHandler,
		provider.ProvideInterviewHistoryHandler,
		provider.ProvideInterviewReportHandler,
		provider.ProvideInterviewRecordingService,
		provider.ProvideInterviewRecordingHandler,
		provider.ProvideInterviewHandlers,
		provider.ProvideApplicationExpiryRunner,
		provider.ProvideRouter,
		provider.ProvideHTTPServer,
		wire.Struct(new(Application), "*"),
	)
	return &Application{}, nil, nil
}
