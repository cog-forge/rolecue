// Package service holds the Learning Roadmap use cases. The read use cases compute every number from stored
// rows; nothing is cached or written.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	// MaxHistoryPageSize bounds one history page; the default is the common list default of 20.
	MaxHistoryPageSize = 100
)

// ReadRepository is what the read use cases need from storage: one consistent snapshot of the Roadmap of a
// profile the candidate owns. A foreign or missing profile is JD_NOT_FOUND.
type ReadRepository interface {
	LoadReadModel(ctx context.Context, candidateID, profileID uuid.UUID) (domain.RoadmapRecord, []domain.ResultRow, error)
}

// ReadService serves the Roadmap and the topic history to the owning Candidate.
type ReadService struct {
	repository ReadRepository
	now        func() time.Time
}

func NewReadService(repository ReadRepository) *ReadService {
	return &ReadService{repository: repository, now: time.Now}
}

// WithClock replaces the clock the "delayed" update state is measured against. Tests only.
func (s *ReadService) WithClock(now func() time.Time) *ReadService {
	s.now = now
	return s
}

func requireCandidate(role string) error {
	if role != candidateRole {
		return apperror.New(apperror.CodeForbidden, "Roadmaps are available to candidates.")
	}
	return nil
}

// Get returns the Roadmap of an owned Role Profile with every score and status computed from the stored results.
func (s *ReadService) Get(ctx context.Context, userID uuid.UUID, role string, profileID uuid.UUID) (domain.RoadmapView, error) {
	if err := requireCandidate(role); err != nil {
		return domain.RoadmapView{}, err
	}
	rec, rows, err := s.repository.LoadReadModel(ctx, userID, profileID)
	if err != nil {
		return domain.RoadmapView{}, err
	}
	view, err := domain.BuildRoadmapView(rec, rows, s.now())
	if err != nil {
		return domain.RoadmapView{}, apperror.Wrap(apperror.CodeInternal, "The Roadmap could not be computed.", err)
	}
	return view, nil
}

// ItemHistory returns one page of the evaluated practice answers of a topic, newest first, and the total
// number of answers in the history. limit is 1 to MaxHistoryPageSize and offset at least 0. A topic that is not
// part of the profile's Roadmap is ROADMAP_ITEM_NOT_FOUND.
func (s *ReadService) ItemHistory(ctx context.Context, userID uuid.UUID, role string, profileID, itemID uuid.UUID, limit, offset int32) ([]domain.HistoryEntry, int64, error) {
	if err := requireCandidate(role); err != nil {
		return nil, 0, err
	}
	if limit < 1 || limit > MaxHistoryPageSize || offset < 0 {
		return nil, 0, apperror.New(apperror.CodeValidation, "Invalid pagination parameters.")
	}
	rec, rows, err := s.repository.LoadReadModel(ctx, userID, profileID)
	if err != nil {
		return nil, 0, err
	}
	history, err := domain.BuildItemHistory(rec, itemID, rows)
	if errors.Is(err, domain.ErrItemNotFound) {
		return nil, 0, apperror.New(apperror.CodeRoadmapItemNotFound, "Roadmap topic not found.")
	}
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInternal, "The topic history could not be computed.", err)
	}
	return domain.PageEntries(history, limit, offset), int64(len(history)), nil
}
