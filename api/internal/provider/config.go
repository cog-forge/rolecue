package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
)

func ProvideConfig(configPath string) (*config.Config, error) {
	return config.Load(configPath)
}
