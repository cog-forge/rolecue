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
	roleProfileHandler  *handler.RoleProfileHandler
	profileHandler      *handler.ProfileHandler
	walletHandler       *handler.WalletHandler
	questionBankHandler *handler.QuestionBankHandler
	moderationHandler   *handler.ModerationHandler
}

func NewRouter(
	cfg *config.Config,
	logger *logger.Logger,
	authHandler *handler.AuthHandler,
	healthHandler *handler.HealthHandler,
	roleProfileHandler *handler.RoleProfileHandler,
	profileHandler *handler.ProfileHandler,
	walletHandler *handler.WalletHandler,
	questionBankHandler *handler.QuestionBankHandler,
	moderationHandler *handler.ModerationHandler,
) *Router {
	return &Router{
		cfg:                 cfg,
		logger:              logger,
		authHandler:         authHandler,
		healthHandler:       healthHandler,
		roleProfileHandler:  roleProfileHandler,
		profileHandler:      profileHandler,
		walletHandler:       walletHandler,
		questionBankHandler: questionBankHandler,
		moderationHandler:   moderationHandler,
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

	profilesJD := router.Group("/role-profiles", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins))
	profilesJD.POST("", r.roleProfileHandler.Create)
	profilesJD.POST("/uploads", r.roleProfileHandler.CreateUpload)
	profilesJD.POST("/uploads/:id/complete", r.roleProfileHandler.CompleteUpload)
	profilesJD.GET("", r.roleProfileHandler.List)
	profilesJD.GET("/:id", r.roleProfileHandler.Get)
	profilesJD.GET("/:id/document", r.roleProfileHandler.Document)
	profilesJD.PUT("/:id", r.roleProfileHandler.Save)
	profilesJD.DELETE("/:id", r.roleProfileHandler.Delete)
	profilesJD.POST("/:id/analyze", r.roleProfileHandler.Analyze)
	profilesJD.POST("/:id/confirm", r.roleProfileHandler.Confirm)

	wallet := router.Group("/wallet", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins))
	{
		wallet.GET("", r.walletHandler.GetWallet)
		wallet.GET("/transactions", r.walletHandler.ListTransactions)
		wallet.GET("/packages", r.walletHandler.ListPackages)
		wallet.POST("/deposit/checkout", r.walletHandler.CreateDepositCheckout)
	}

	router.POST("/wallet/webhook/payos", r.walletHandler.HandlePayOSWebhook)

	jobPostings := router.Group("/job-postings", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins))
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
