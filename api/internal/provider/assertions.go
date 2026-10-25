package provider

import (
	apprepository "github.com/cog-forge/rolecue/api/internal/features/application/repository"
	appservice "github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
)

// The application service and handler reach their capacity and Candidate methods through runtime
// interface checks, so the shared interfaces and their fakes stay small. These assertions make the
// build fail if a signature ever drifts, instead of every call returning an internal error.
var (
	_ handler.CapacityService             = (*appservice.Service)(nil)
	_ handler.CandidateApplicationService = (*appservice.Service)(nil)
	_ appservice.CapacityRepository       = (*apprepository.Repository)(nil)
	_ appservice.CandidateRepository      = (*apprepository.Repository)(nil)
	_ appservice.CVStore                  = (storage.Store)(nil)
)
