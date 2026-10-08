package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
)

func ProvideAuthHandler(authService service.AuthService) *handler.AuthHandler {
	return handler.NewAuthHandler(authService)
}

func ProvideHealthHandler() *handler.HealthHandler {
	return handler.NewHealthHandler()
}
