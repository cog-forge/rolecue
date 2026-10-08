package provider

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/database"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tracer"
)

func ProvideLogger(cfg *config.Config) (*logger.Logger, func(), error) {
	log, err := logger.New(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		Output:     cfg.Logger.Output,
		TimeFormat: cfg.Logger.TimeFormat,
	})
	if err != nil {
		return nil, nil, err
	}
	return log, func() { _ = log.Sync() }, nil
}

func ProvideTracer(cfg *config.Config, log *logger.Logger) (*tracer.Tracer, func(), error) {
	t, err := tracer.New(tracer.Config{
		Enabled:     cfg.Tracer.Enabled,
		ServiceName: cfg.Tracer.ServiceName,
		Endpoint:    cfg.Tracer.Endpoint,
		Insecure:    cfg.Tracer.Insecure,
		SampleRate:  cfg.Tracer.SampleRate,
		Environment: cfg.App.Environment,
		Version:     cfg.App.Version,
	})
	if err != nil {
		return nil, nil, err
	}
	return t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := t.Shutdown(ctx); err != nil {
			log.Error("Tracer shutdown failed", logger.Error(err))
		}
	}, nil
}

func ProvideDatabasePool(cfg *config.Config, log *logger.Logger) (*pgxpool.Pool, func(), error) {
	pool, err := database.NewDatabasePool(&cfg.Database)
	if err != nil {
		log.Error("Failed to connect to database", logger.Error(err))
		return nil, nil, err
	}
	log.Info("Database connection pool established successfully")
	return pool, pool.Close, nil
}
