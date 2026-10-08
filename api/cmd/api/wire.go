//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tracer"
	"github.com/cog-forge/rolecue/api/internal/provider"
	"github.com/cog-forge/rolecue/api/internal/server"
)

type Application struct {
	Server *server.Server
	Pool   *pgxpool.Pool
	Logger *logger.Logger
	Tracer *tracer.Tracer
	Config *config.Config
}

func InitializeApplication(configPath string) (*Application, func(), error) {
	wire.Build(
		provider.ProvideConfig,
		provider.ProvideLogger,
		provider.ProvideTracer,
		provider.ProvideDatabasePool,
		provider.ProvideAuthRepository,
		provider.ProvideAuthService,
		provider.ProvideAuthHandler,
		provider.ProvideHealthHandler,
		provider.ProvideJDRepository,
		provider.ProvideJDService,
		provider.ProvideJDHandler,
		provider.ProvideRouter,
		provider.ProvideHTTPServer,
		wire.Struct(new(Application), "*"),
	)
	return &Application{}, nil, nil
}
