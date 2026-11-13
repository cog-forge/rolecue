package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
)

// Learning Roadmap read API (KAN-352).

func ProvideRoadmapReadService(repo *repository.Repository) *service.ReadService {
	return service.NewReadService(repo)
}

func ProvideRoadmapHandler(svc *service.ReadService) *handler.RoadmapHandler {
	return handler.NewRoadmapHandler(svc)
}
