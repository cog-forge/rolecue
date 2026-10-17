package router

import (
	_ "github.com/cog-forge/rolecue/api/docs"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

type Router struct {
	cfg                 *config.Config
	logger              *logger.Logger
	authHandler         *handler.AuthHandler
	healthHandler       *handler.HealthHandler
	jdHandler           *handler.JDHandler
	profileHandler      *handler.ProfileHandler
	walletHandler       *handler.WalletHandler
	questionBankHandler *handler.QuestionBankHandler
	moderationHandler   *handler.ModerationHandler
	onboardingHandler   *handler.OnboardingHandler
}

func NewRouter(
	cfg *config.Config,
	logger *logger.Logger,
	authHandler *handler.AuthHandler,
	healthHandler *handler.HealthHandler,
	jdHandler *handler.JDHandler,
	profileHandler *handler.ProfileHandler,
	walletHandler *handler.WalletHandler,
	questionBankHandler *handler.QuestionBankHandler,
	moderationHandler *handler.ModerationHandler,
	onboardingHandler *handler.OnboardingHandler,
) *Router {
	return &Router{
		cfg:                 cfg,
		logger:              logger,
		authHandler:         authHandler,
		healthHandler:       healthHandler,
		jdHandler:           jdHandler,
		profileHandler:      profileHandler,
		walletHandler:       walletHandler,
		questionBankHandler: questionBankHandler,
		moderationHandler:   moderationHandler,
		onboardingHandler:   onboardingHandler,
	}
}

func (r *Router) Setup() *gin.Engine {
	if r.cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Core middlewares
	router.Use(middleware.RecoveryMiddleware(r.logger))
	router.Use(middleware.LoggingMiddleware(r.logger))
	router.Use(middleware.CORSMiddleware(r.cfg.CORS))

	router.GET("/health", r.healthHandler.Health)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	auth := router.Group("/auth")
	{
		auth.GET("/me", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), r.authHandler.Me)
	}

	profiles := router.Group("/profile", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins))
	profiles.GET("", r.profileHandler.Get)
	profiles.PATCH("", r.profileHandler.Update)

	router.POST("/onboarding/role", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), r.onboardingHandler.SelectRole)

	jds := router.Group("/jds", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
	jds.POST("/analyze", r.jdHandler.Analyze)
	jds.POST("", r.jdHandler.Create)
	jds.GET("", r.jdHandler.List)
	jds.GET("/:id", r.jdHandler.Get)
	jds.PUT("/:id", r.jdHandler.Update)
	jds.DELETE("/:id", r.jdHandler.Delete)

	wallet := router.Group("/wallet", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
	{
		wallet.GET("", r.walletHandler.GetWallet)
		wallet.GET("/transactions", r.walletHandler.ListTransactions)
		wallet.GET("/packages", r.walletHandler.ListPackages)
		wallet.POST("/deposit/checkout", r.walletHandler.CreateDepositCheckout)
	}

	router.POST("/wallet/webhook/payos", r.walletHandler.HandlePayOSWebhook)

	jobPostings := router.Group("/job-postings", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
	jobPostings.GET("/:id/questions", r.questionBankHandler.List)
	jobPostings.POST("/:id/questions/generate", r.questionBankHandler.Generate)
	jobPostings.POST("/:id/questions", r.questionBankHandler.Add)
	jobPostings.PATCH("/:id/questions/:questionId", r.questionBankHandler.Update)
	jobPostings.DELETE("/:id/questions/:questionId", r.questionBankHandler.Delete)

	// Every admin route sits under /admin so one role check guards the whole group.
	admin := router.Group("/admin", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireRole("admin"))
	admin.GET("/job-postings", r.moderationHandler.List)
	admin.GET("/job-postings/:id", r.moderationHandler.Get)
	admin.POST("/job-postings/:id/approve", r.moderationHandler.Approve)
	admin.POST("/job-postings/:id/reject", r.moderationHandler.Reject)
	return router
}
