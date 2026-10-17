package provider

import (
	authservice "github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/cog-forge/rolecue/api/internal/features/onboarding/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
)

func ProvideOnboardingHandler(auth authservice.AuthService) *handler.OnboardingHandler {
	return handler.NewOnboardingHandler(service.NewService(auth))
}
