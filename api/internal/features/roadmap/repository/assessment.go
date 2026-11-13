package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/google/uuid"
)

// LinkCores links the listed assignment positions of a completed practice interview to the topic of their core
// question in one statement (see LinkInterviewCores). Written is the rows newly written: a position that was
// linked before adds nothing, and neither does one that fails the owner, practice, topic and profile checks.
// Present is how many of the listed positions have a link afterwards, whoever wrote it, and Mappable how many have a
// topic in the profile's Roadmap, so the caller can tell a position that was already linked, or has no topic, from
// one the rules refused.
func (r *Repository) LinkCores(ctx context.Context, candidateID, interviewID uuid.UUID, positions []int32) (domain.LinkOutcome, error) {
	if len(positions) == 0 {
		return domain.LinkOutcome{}, nil
	}
	written, err := r.queries.LinkInterviewCores(ctx, LinkInterviewCoresParams{
		InterviewID: interviewID, CandidateID: candidateID, Positions: positions,
	})
	if err != nil {
		return domain.LinkOutcome{}, persistence(err)
	}
	counts, err := r.queries.CountInterviewLinks(ctx, CountInterviewLinksParams{InterviewID: interviewID, Positions: positions})
	if err != nil {
		return domain.LinkOutcome{}, persistence(err)
	}
	return domain.LinkOutcome{Written: written, Present: counts.Present, Mappable: counts.Mappable}, nil
}

// ListSessionsMissingAssessments returns at most limit practice sessions that still lack Roadmap links for a
// countable result, oldest evaluation first, skipping the excluded ids (see the query for the exact rule).
func (r *Repository) ListSessionsMissingAssessments(ctx context.Context, limit int32, exclude []uuid.UUID) ([]uuid.UUID, error) {
	if exclude == nil {
		exclude = []uuid.UUID{}
	}
	ids, err := r.queries.ListSessionsMissingAssessments(ctx, ListSessionsMissingAssessmentsParams{BatchSize: limit, Exclude: exclude})
	if err != nil {
		return nil, persistence(err)
	}
	return ids, nil
}
