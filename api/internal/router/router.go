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
	cfg            *config.Config
	logger         *logger.Logger
	authHandler    *handler.AuthHandler
	healthHandler  *handler.HealthHandler
	jdHandler      *handler.JDHandler
	profileHandler *handler.ProfileHandler
}

func NewRouter(
	cfg *config.Config,
	logger *logger.Logger,
	authHandler *handler.AuthHandler,
	healthHandler *handler.HealthHandler,
	jdHandler *handler.JDHandler,
	profileHandler *handler.ProfileHandler,
) *Router {
	return &Router{
		cfg:            cfg,
		logger:         logger,
		authHandler:    authHandler,
		healthHandler:  healthHandler,
		jdHandler:      jdHandler,
		profileHandler: profileHandler,
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

	jds := router.Group("/jds", r.authHandler.RequireAuth(r.cfg.CORS.AllowOrigins))
	jds.POST("/analyze", r.jdHandler.Analyze)
	jds.POST("", r.jdHandler.Create)
	jds.GET("", r.jdHandler.List)
	jds.GET("/:id", r.jdHandler.Get)
	jds.PUT("/:id", r.jdHandler.Update)
	jds.DELETE("/:id", r.jdHandler.Delete)
	return router
}
