package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	candidateRole = "candidate"
	// maxFilterLength matches the longest title a Recruiter can save.
	maxFilterLength = 200
	// maxStoredTextBytes matches the Job Posting limit for a JD stored as a text object.
	maxStoredTextBytes = 80_000
)

// Repository is the persistence contract of the job board. It only ever returns postings whose
// status is open; any other posting is reported as not found.
type Repository interface {
	ListOpen(ctx context.Context, filter domain.BoardFilter) ([]domain.BoardPosting, int64, error)
	GetOpen(ctx context.Context, candidateID, id uuid.UUID) (domain.BoardRecord, error)
}

// TextStore reads the JD of a posting created before PDF upload, kept as a plain-text object.
type TextStore interface {
	Download(ctx context.Context, key string) (storage.Object, error)
}

type Service struct {
	repository Repository
	texts      TextStore
}

func New(repository Repository, texts TextStore) (*Service, error) {
	if repository == nil || texts == nil {
		return nil, fmt.Errorf("job board repository and text store are required")
	}
	return &Service{repository: repository, texts: texts}, nil
}

func requireCandidate(role string) error {
	if role != candidateRole {
		return apperror.New(apperror.CodeForbidden, "only candidates can browse the job board")
	}
	return nil
}

// List returns open Job Postings, newest first.
func (s *Service) List(ctx context.Context, role string, filter domain.BoardFilter) ([]domain.BoardPosting, int64, error) {
	if err := requireCandidate(role); err != nil {
		return nil, 0, err
	}
	filter, err := validateFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	items, total, err := s.repository.ListOpen(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []domain.BoardPosting{}
	}
	return items, total, nil
}

// Get returns an open Job Posting and whether this Candidate already applied to it.
func (s *Service) Get(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.BoardPostingDetail, error) {
	if err := requireCandidate(role); err != nil {
		return domain.BoardPostingDetail{}, err
	}
	record, err := s.repository.GetOpen(ctx, candidateID, id)
	if err != nil {
		return domain.BoardPostingDetail{}, err
	}
	detail := record.BoardPostingDetail
	if detail.Requirements == nil {
		detail.Requirements = []domain.BoardRequirement{}
	}
	if record.StoredText != nil {
		detail.RawText = *record.StoredText
		return detail, nil
	}
	// content_path became nullable with Role Profile drafts; a posting without any JD shows no text.
	if record.ContentPath == "" {
		return detail, nil
	}
	if detail.RawText, err = s.readText(ctx, record.ContentPath); err != nil {
		return domain.BoardPostingDetail{}, err
	}
	return detail, nil
}

func (s *Service) readText(ctx context.Context, key string) (string, error) {
	object, err := s.texts.Download(ctx, key)
	if err != nil {
		return "", storageError(err)
	}
	defer func() { _ = object.Body.Close() }()
	text, err := io.ReadAll(io.LimitReader(object.Body, maxStoredTextBytes+1))
	if err != nil {
		return "", storageError(err)
	}
	if len(text) > maxStoredTextBytes {
		return "", storageError(fmt.Errorf("stored JD text exceeds %d bytes", maxStoredTextBytes))
	}
	return string(text), nil
}

func storageError(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "job board file storage failed", err)
}

// validateFilter checks paging, trims the text filters and drops empty ones. search and location
// become escaped LIKE patterns, so a typed % or _ matches itself.
func validateFilter(filter domain.BoardFilter) (domain.BoardFilter, error) {
	if filter.Limit < 1 || filter.Limit > 100 || filter.Offset < 0 {
		return domain.BoardFilter{}, apperror.New(apperror.CodeValidation, "invalid pagination parameters")
	}
	var err error
	if filter.Search, err = textFilter(filter.Search, "search", true); err != nil {
		return domain.BoardFilter{}, err
	}
	if filter.Location, err = textFilter(filter.Location, "location", true); err != nil {
		return domain.BoardFilter{}, err
	}
	if filter.EmploymentType, err = textFilter(filter.EmploymentType, "employment_type", false); err != nil {
		return domain.BoardFilter{}, err
	}
	return filter, nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func textFilter(value *string, field string, pattern bool) (*string, error) {
	if value == nil {
		return nil, nil
	}
	if !utf8.ValidString(*value) || strings.ContainsRune(*value, 0) {
		return nil, apperror.New(apperror.CodeValidation, "invalid "+field)
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > maxFilterLength {
		return nil, apperror.New(apperror.CodeValidation, fmt.Sprintf("%s must be at most %d characters", field, maxFilterLength))
	}
	if pattern {
		trimmed = likeEscaper.Replace(trimmed)
	}
	return &trimmed, nil
}
