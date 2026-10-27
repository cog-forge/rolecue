package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
)

// The recording service uses the interview repository (ProvideInterviewRepository) and the storage Store.
// These assertions make the build fail if a signature drifts from the interface the service declares.
var (
	_ service.RecordingRepository       = (*repository.Repository)(nil)
	_ handler.InterviewRecordingService = (*service.RecordingService)(nil)
)

func ProvideInterviewRecordingService(repo *repository.Repository, store storage.Store) (*service.RecordingService, error) {
	return service.NewRecordingService(repo, store)
}

func ProvideInterviewRecordingHandler(svc *service.RecordingService) *handler.InterviewRecordingHandler {
	return handler.NewInterviewRecordingHandler(svc)
}
