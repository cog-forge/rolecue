package provider

import (
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/jobposting/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jobposting/service"
	roleprofileservice "github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideJobPostingRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideJobPostingService(repo *repository.Repository, store storage.Store) (*service.Service, error) {
	return service.New(repo, store)
}

// ProvideJobPostingExtraction reuses the Role Profile extraction pipeline, with the same time budget
// the Role Profile analysis gives it: every attempt plus a second of slack.
func ProvideJobPostingExtraction(cfg *config.Config, extraction *roleprofileservice.ExtractionService) (*service.Extraction, error) {
	return service.NewExtraction(extraction, time.Duration(cfg.LLM.MaxRetries+1)*cfg.LLM.Timeout+time.Second)
}
func ProvideJobPostingHandler(svc *service.Service, extraction *service.Extraction) *handler.JobPostingHandler {
	return handler.NewJobPostingHandler(svc, extraction)
}
