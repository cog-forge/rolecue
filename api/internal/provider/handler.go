package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/auth"
	"github.com/cog-forge/rolecue/api/internal/handler"
)

func ProvideAuthHandler(authService auth.AuthService) *handler.AuthHandler {
	return handler.NewAuthHandler(authService)
}

func ProvideHealthHandler() *handler.HealthHandler {
	return handler.NewHealthHandler()
}
