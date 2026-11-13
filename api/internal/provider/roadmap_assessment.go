package provider

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	evaluationrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	roadmapdomain "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	roadmaprepository "github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	roadmapservice "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/google/uuid"
)

// Roadmap assessment writer and catch-up (KAN-350, KAN-351). Features do not import each other, so the Roadmap
// service reads interview facts through an interface that this adapter satisfies from the evaluation repository.

var _ roadmapservice.SessionSource = (*RoadmapSessionSource)(nil)

// RoadmapSessionSource reads the stored facts of an interview for the Roadmap: the owner and the facts that
// decide eligibility from the interview record, and the per-core facts from the evaluation adapter query.
type RoadmapSessionSource struct {
	repo *evaluationrepository.Repository
}

func ProvideRoadmapSessionSource(repo *evaluationrepository.Repository) *RoadmapSessionSource {
	return &RoadmapSessionSource{repo: repo}
}

func (s *RoadmapSessionSource) LoadSession(ctx context.Context, interviewID uuid.UUID) (roadmapservice.SessionInfo, error) {
	record, err := s.repo.InterviewRecord(ctx, interviewID)
	if err != nil {
		return roadmapservice.SessionInfo{}, err
	}
	info := roadmapservice.SessionInfo{
		CandidateID: record.CandidateID,
		Session: roadmapdomain.SessionFacts{
			Type: record.Type, CompletedAt: record.CompletedAt, EvaluationStatus: record.EvaluationStatus,
			InjectionFlagged: record.InjectionFlagged,
		},
	}
	// A session that may not feed a Roadmap needs no per-core facts.
	if !info.Session.Eligible() {
		return info, nil
	}
	facts, err := s.repo.CoreResultFacts(ctx, interviewID)
	if err != nil {
		return roadmapservice.SessionInfo{}, err
	}
	info.Cores = coreFacts(facts)
	return info, nil
}

func coreFacts(facts []evaluationdomain.CoreResultFacts) []roadmapdomain.CoreFacts {
	out := make([]roadmapdomain.CoreFacts, 0, len(facts))
	for _, f := range facts {
		out = append(out, roadmapdomain.CoreFacts{Position: f.Position, Delivered: f.Delivered, Answered: f.Answered, QuestionScore: f.Score})
	}
	return out
}

// roadmapBackoff is the wait after a session's first failed link; it doubles up to the maximum.
const (
	roadmapBackoffBase = time.Minute
	roadmapBackoffMax  = time.Hour
)

// ProvideRoadmapCatchUp builds the catch-up job's use case (KAN-351) from the jobs configuration.
func ProvideRoadmapCatchUp(cfg *config.Config, repo *roadmaprepository.Repository, assessments *roadmapservice.AssessmentService) (*roadmapservice.CatchUp, error) {
	batch := cfg.Jobs.RoadmapCatchup.BatchSize
	if batch < 1 {
		// Only reachable with the jobs switched off as a whole, when the settings are not validated; the job
		// then never runs.
		batch = 1
	}
	return roadmapservice.NewCatchUp(repo, assessments, roadmapservice.CatchUpSettings{
		BatchSize: batch, BackoffBase: roadmapBackoffBase, BackoffMax: roadmapBackoffMax,
	})
}

func ProvideRoadmapAssessmentService(source *RoadmapSessionSource, repo *roadmaprepository.Repository) *roadmapservice.AssessmentService {
	return roadmapservice.NewAssessmentService(source, repo)
}
