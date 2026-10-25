package provider

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
)

// The application Repository provides the final decision and the expiry pass. The assertions fail the
// build when either method goes missing, instead of at the first request.
var (
	_ service.DecisionRepository = (*repository.Repository)(nil)
	_ service.ExpiryRepository   = (*repository.Repository)(nil)
)

// ProvideApplicationExpiryRunner builds the auto-reject runner over the application service. The
// interval is service.ExpiryInterval, so there is no configuration to read.
func ProvideApplicationExpiryRunner(svc *service.Service) (*service.ExpiryRunner, error) {
	return service.NewExpiryRunner(svc)
}

// StartApplicationExpiryRunner runs the expiry pass in the background until ctx is done. Call it once
// after the runner is built, with the context that is cancelled on shutdown.
func StartApplicationExpiryRunner(ctx context.Context, runner *service.ExpiryRunner) {
	go runner.Run(ctx, service.ExpiryInterval, time.Now)
}
