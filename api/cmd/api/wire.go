//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/internal/pkg/tracer"
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
}

func InitializeApplication(configPath string) (*Application, func(), error) {
	wire.Build(
		provider.ProvideConfig,
		provider.ProvideStorage,
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
		provider.ProvideRouter,
		provider.ProvideHTTPServer,
		wire.Struct(new(Application), "*"),
	)
	return &Application{}, nil, nil
}
