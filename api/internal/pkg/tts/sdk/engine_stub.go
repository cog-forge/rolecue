//go:build !azure_sdk

package sdk

import "github.com/cog-forge/rolecue/api/internal/pkg/logger"

const built = false

func newEngine(Config, *logger.Logger) (engine, error) { return nil, ErrNotBuilt }
