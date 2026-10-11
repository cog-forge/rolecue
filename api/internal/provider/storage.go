package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
)

func ProvideStorage(cfg *config.Config) (storage.Store, error) {
	return storage.New(cfg.Storage)
}
