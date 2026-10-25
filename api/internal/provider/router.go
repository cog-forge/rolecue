package provider

import (
	"github.com/gin-gonic/gin"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/router"
	"github.com/cog-forge/rolecue/api/internal/server"
)

func ProvideRouter(
	cfg *config.Config,
	log *logger.Logger,
	authHandler *handler.AuthHandler,
	healthHandler *handler.HealthHandler,
	roleProfileHandler *handler.RoleProfileHandler,
	profileHandler *handler.ProfileHandler,
	walletHandler *handler.WalletHandler,
	questionBankHandler *handler.QuestionBankHandler,
	moderationHandler *handler.ModerationHandler,
	onboardingHandler *handler.OnboardingHandler,
	accountsHandler *handler.AccountsHandler,
	inventoryHandler *handler.InventoryHandler,
	avatarCreateHandler *handler.AvatarCreateHandler,
	voiceCatalogHandler *handler.VoiceCatalogHandler,
	jobPostingHandler *handler.JobPostingHandler,
	voiceHandler *handler.VoiceHandler,
	jobBoardHandler *handler.JobBoardHandler,
	evaluationHandler *handler.EvaluationHandler,
	applicationHandler *handler.ApplicationHandler,
	adminInterviewsHandler *handler.AdminInterviewsHandler,
	interviewHandlers *handler.InterviewHandlers,
) *gin.Engine {
	appRouter := router.NewRouter(cfg, log, authHandler, healthHandler, roleProfileHandler, profileHandler, walletHandler, questionBankHandler, moderationHandler, onboardingHandler, accountsHandler, inventoryHandler, avatarCreateHandler, voiceCatalogHandler, jobPostingHandler, voiceHandler, jobBoardHandler, evaluationHandler, applicationHandler, adminInterviewsHandler, interviewHandlers)
	return appRouter.Setup()
}

func ProvideHTTPServer(
	cfg *config.Config,
	engine *gin.Engine,
	log *logger.Logger,
) *server.Server {
	return server.NewServer(cfg.Server, engine, log)
}
