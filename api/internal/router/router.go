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
	cfg                    *config.Config
	logger                 *logger.Logger
	authHandler            *handler.AuthHandler
	healthHandler          *handler.HealthHandler
	roleProfileHandler     *handler.RoleProfileHandler
	profileHandler         *handler.ProfileHandler
	walletHandler          *handler.WalletHandler
	questionBankHandler    *handler.QuestionBankHandler
	moderationHandler      *handler.ModerationHandler
	onboardingHandler      *handler.OnboardingHandler
	accountsHandler        *handler.AccountsHandler
	inventoryHandler       *handler.InventoryHandler
	avatarCreateHandler    *handler.AvatarCreateHandler
	voiceCatalogHandler    *handler.VoiceCatalogHandler
	jobPostingHandler      *handler.JobPostingHandler
	voiceHandler           *handler.VoiceHandler
	jobBoardHandler        *handler.JobBoardHandler
	evaluationHandler      *handler.EvaluationHandler
	applicationHandler     *handler.ApplicationHandler
	adminInterviewsHandler *handler.AdminInterviewsHandler
	interviewHandlers      *handler.InterviewHandlers
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
) *Router {
	return &Router{
		cfg:                    cfg,
		logger:                 logger,
		authHandler:            authHandler,
		healthHandler:          healthHandler,
		roleProfileHandler:     roleProfileHandler,
		profileHandler:         profileHandler,
		walletHandler:          walletHandler,
		questionBankHandler:    questionBankHandler,
		moderationHandler:      moderationHandler,
		onboardingHandler:      onboardingHandler,
		accountsHandler:        accountsHandler,
		inventoryHandler:       inventoryHandler,
		avatarCreateHandler:    avatarCreateHandler,
		voiceCatalogHandler:    voiceCatalogHandler,
		jobPostingHandler:      jobPostingHandler,
		voiceHandler:           voiceHandler,
		jobBoardHandler:        jobBoardHandler,
		evaluationHandler:      evaluationHandler,
		applicationHandler:     applicationHandler,
		adminInterviewsHandler: adminInterviewsHandler,
		interviewHandlers:      interviewHandlers,
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

	inventory := router.Group("/inventory", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding(), middleware.RequireRole("candidate", "recruiter"))
	inventory.GET("", r.inventoryHandler.Summary)
	inventory.POST("/purchase", r.inventoryHandler.PurchaseSlot)
	inventory.GET("/avatars", r.inventoryHandler.List)
	inventory.GET("/avatars/:id", r.inventoryHandler.Get)
	inventory.DELETE("/avatars/:id", r.inventoryHandler.Delete)
	inventory.POST("/avatars", r.avatarCreateHandler.Create)

	profilesJD := router.Group("/role-profiles", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
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

	wallet := router.Group("/wallet", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
	{
		wallet.GET("", r.walletHandler.GetWallet)
		wallet.GET("/transactions", r.walletHandler.ListTransactions)
		wallet.GET("/packages", r.walletHandler.ListPackages)
		wallet.POST("/deposit/checkout", r.walletHandler.CreateDepositCheckout)
		wallet.POST("/deposit/sync", r.walletHandler.SyncDepositStatus)
	}

	router.POST("/wallet/webhook/payos", r.walletHandler.HandlePayOSWebhook)

	jobPostings := router.Group("/job-postings", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
	jobPostings.POST("", r.jobPostingHandler.Create)
	jobPostings.POST("/extract", r.jobPostingHandler.Extract)
	jobPostings.POST("/documents", r.jobPostingHandler.UploadDocument)
	jobPostings.GET("", r.jobPostingHandler.List)
	jobPostings.GET("/:id", r.jobPostingHandler.Get)
	jobPostings.PATCH("/:id", r.jobPostingHandler.Update)
	jobPostings.POST("/:id/submit", r.jobPostingHandler.Submit)
	jobPostings.POST("/:id/resubmit", r.jobPostingHandler.Resubmit)
	jobPostings.GET("/:id/document", r.jobPostingHandler.Document)
	jobPostings.GET("/:id/questions", r.questionBankHandler.List)
	jobPostings.POST("/:id/questions/generate", r.questionBankHandler.Generate)
	jobPostings.POST("/:id/questions", r.questionBankHandler.Add)
	jobPostings.PATCH("/:id/questions/:questionId", r.questionBankHandler.Update)
	jobPostings.DELETE("/:id/questions/:questionId", r.questionBankHandler.Delete)
	// Evaluation weights are separate from the question bank above.
	jobPostings.GET("/:id/evaluation-weights", r.evaluationHandler.Weights)
	jobPostings.PUT("/:id/evaluation-weights", r.evaluationHandler.ReplaceWeights)
	jobPostings.DELETE("/:id/evaluation-weights", r.evaluationHandler.ResetWeights)
	jobPostings.GET("/:id/applications", r.applicationHandler.ListForPosting)
	jobPostings.GET("/:id/applications/:applicationId", r.applicationHandler.Detail)
	jobPostings.POST("/:id/applications/:applicationId/approve-cv", r.applicationHandler.ApproveCV)
	jobPostings.POST("/:id/applications/:applicationId/reject-cv", r.applicationHandler.RejectCV)
	jobPostings.POST("/:id/close-intake", r.applicationHandler.CloseIntake)
	jobPostings.GET("/:id/interview-capacity", r.applicationHandler.CapacityState)
	jobPostings.POST("/:id/interview-capacity", r.applicationHandler.FundCapacity)
	// The final decision and the recruiter evidence reads answer a non-owner with 404 from the
	// service, so they carry no RequireRole: a role middleware would turn that into a 403.
	jobPostings.POST("/:id/applications/:applicationId/decision", r.applicationHandler.Decide)
	jobPostings.GET("/:id/applications/:applicationId/recording", r.interviewHandlers.Recording.Recording)
	jobPostings.GET("/:id/applications/:applicationId/transcript", r.interviewHandlers.Recording.Transcript)

	// The Recruiter's Applications across all of their postings. /applications stays free for the
	// Candidate's own list.
	recruiter := router.Group("/recruiter", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding(), middleware.RequireRole("recruiter"))
	recruiter.GET("/applications", r.applicationHandler.ListOwn)

	router.GET("/evaluation-metrics", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding(), r.evaluationHandler.Metrics)

	router.GET("/voices", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding(), r.voiceHandler.List)

	// The job board shows Candidates the open Job Postings; /job-postings stays the Recruiter's own list.
	jobBoard := router.Group("/job-board", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding(), middleware.RequireRole("candidate"))
	jobBoard.GET("", r.jobBoardHandler.List)
	jobBoard.GET("/:id", r.jobBoardHandler.Get)
	jobBoard.POST("/:id/applications", r.applicationHandler.Apply)

	// The Candidate's own Applications. /recruiter/applications is the Recruiter's view.
	candidateApplications := router.Group("/applications", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding(), middleware.RequireRole("candidate"))
	candidateApplications.GET("", r.applicationHandler.ListMine)
	candidateApplications.GET("/:id", r.applicationHandler.DetailMine)

	// Interview sessions. Roles are checked in the services: sessions, history and recording are
	// Candidate only (403 otherwise); the report is readable by the owning Candidate or Recruiter and
	// answers anyone else with 404.
	interviews := router.Group("/interviews", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireOnboarding())
	interviews.POST("", r.interviewHandlers.Sessions.Create)
	interviews.GET("", r.interviewHandlers.History.List)
	interviews.GET("/:id", r.interviewHandlers.Sessions.Get)
	interviews.POST("/:id/resume", r.interviewHandlers.Sessions.Resume)
	interviews.POST("/:id/turns", r.interviewHandlers.Sessions.AppendTurn)
	interviews.POST("/:id/complete", r.interviewHandlers.Sessions.Complete)
	interviews.GET("/:id/report", r.interviewHandlers.Report.Get)
	interviews.POST("/:id/recording", r.interviewHandlers.Recording.CreateUpload)
	interviews.POST("/:id/recording/complete", r.interviewHandlers.Recording.CompleteUpload)

	// Every admin route sits under /admin so one role check guards the whole group.
	admin := router.Group("/admin", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins), middleware.RequireRole("admin"))
	admin.GET("/users", r.accountsHandler.List)
	admin.GET("/users/:id", r.accountsHandler.Get)
	admin.POST("/users/:id/lock", r.accountsHandler.Lock)
	admin.POST("/users/:id/unlock", r.accountsHandler.Unlock)
	admin.GET("/job-postings", r.moderationHandler.List)
	admin.GET("/job-postings/:id", r.moderationHandler.Get)
	admin.POST("/job-postings/:id/approve", r.moderationHandler.Approve)
	admin.POST("/job-postings/:id/reject", r.moderationHandler.Reject)
	admin.GET("/interviews", r.adminInterviewsHandler.List)
	admin.GET("/interviews/:id", r.adminInterviewsHandler.Get)
	admin.GET("/voices", r.voiceCatalogHandler.List)
	admin.POST("/voices/sync", r.voiceCatalogHandler.Sync)
	admin.GET("/voices/:id", r.voiceCatalogHandler.Get)
	admin.DELETE("/voices/:id", r.voiceCatalogHandler.Remove)
	admin.PATCH("/voices/:id/activate", r.voiceCatalogHandler.Activate)
	admin.GET("/billing/summary", r.walletHandler.AdminGetBillingSummary)
	admin.GET("/billing/transactions", r.walletHandler.AdminListBillingTransactions)
	admin.GET("/billing/transactions/:id", r.walletHandler.AdminGetBillingTransaction)
	admin.GET("/billing/export", r.walletHandler.AdminExportBillingCSV)
	admin.GET("/billing/revenue-report", r.walletHandler.AdminGetRevenueReport)
	return router
}
