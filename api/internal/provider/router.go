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
	jdHandler *handler.JDHandler,
	profileHandler *handler.ProfileHandler,
) *gin.Engine {
	appRouter := router.NewRouter(cfg, log, authHandler, healthHandler, jdHandler, profileHandler)
	return appRouter.Setup()
}

func ProvideHTTPServer(
	cfg *config.Config,
	engine *gin.Engine,
	log *logger.Logger,
) *server.Server {
	return server.NewServer(cfg.Server, engine, log)
}
