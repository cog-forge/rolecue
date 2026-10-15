package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/converter"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/repository"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideInventoryRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

func ProvideAvatarConverter(cfg *config.Config) *converter.Blender {
	c := cfg.AvatarConversion
	return &converter.Blender{Binary: c.BlenderBinary, Timeout: c.Timeout, MaxOutput: c.MaxOutputBytes, CommercialUsage: c.CommercialUsage, LicenseURL: c.LicenseURL}
}

func ProvideAvatarConversionService(repo *repository.Repository, store storage.Store, runner *converter.Blender, cfg *config.Config) *service.AvatarConversionService {
	return service.NewAvatarConversionService(repo, store, runner, cfg.AvatarConversion)
}

func ProvideAvatarCreateHandler(svc *service.AvatarConversionService, cfg *config.Config) *handler.AvatarCreateHandler {
	return handler.NewAvatarCreateHandler(svc, cfg.AvatarConversion.MaxInputBytes, cfg.AvatarConversion.Timeout)
}
func ProvideInventoryService(repo *repository.Repository, store storage.Store) *service.Service {
	return service.NewService(repo, store)
}
func ProvideInventoryHandler(svc *service.Service) *handler.InventoryHandler {
	return handler.NewInventoryHandler(svc)
}
