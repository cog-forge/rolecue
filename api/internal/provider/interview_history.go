package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
)

// The history read is served by the interview repository in its own file. The assertion fails the build,
// not the first request, when that method is missing.
var _ service.HistoryRepository = (*repository.Repository)(nil)

func ProvideInterviewHistoryHandler(svc *service.Service) *handler.InterviewHistoryHandler {
	return handler.NewInterviewHistoryHandler(svc)
}
