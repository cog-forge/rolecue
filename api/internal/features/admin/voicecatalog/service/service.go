package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const adminRole = "admin"

// Repository is the persistence contract consumed by the Admin voice catalog.
type Repository interface {
	List(ctx context.Context, filter domain.VoiceListFilter) ([]domain.VoiceProfile, int64, error)
	Get(ctx context.Context, id uuid.UUID) (domain.VoiceProfile, error)
	Remove(ctx context.Context, id uuid.UUID) (domain.VoiceDeleteOutcome, error)
}

type Service struct {
	repository Repository
	log        *logger.Logger
}

// New builds the service. Removals are logged because the brain asks for an admin audit trail
// while the frozen schema has no audit table.
func New(repository Repository, log *logger.Logger) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("voice catalog repository is required")
	}
	if log == nil {
		log = &logger.Logger{Logger: zap.NewNop()}
	}
	return &Service{repository: repository, log: log}, nil
}

func requireAdmin(role string) error {
	if role != adminRole {
		return apperror.New(apperror.CodeForbidden, "only administrators can manage voice profiles")
	}
	return nil
}

func invalid(message string) error { return apperror.New(apperror.CodeValidation, message) }

func (s *Service) List(ctx context.Context, role string, filter domain.VoiceListFilter) ([]domain.VoiceProfile, int64, error) {
	if err := requireAdmin(role); err != nil {
		return nil, 0, err
	}
	filter, err := normalizeFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	return s.repository.List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, role string, id uuid.UUID) (domain.VoiceProfile, error) {
	if err := requireAdmin(role); err != nil {
		return domain.VoiceProfile{}, err
	}
	return s.repository.Get(ctx, id)
}

// Remove takes a voice out of the catalog. A voice that any interview or Job Posting references is
// deactivated, never deleted, so past sessions keep their data; an unused one is deleted. Calling
// it again on a deactivated voice reports "deactivated" again.
func (s *Service) Remove(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.VoiceDeleteResult, error) {
	if err := requireAdmin(role); err != nil {
		return domain.VoiceDeleteResult{}, err
	}
	voice, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.VoiceDeleteResult{}, err
	}
	outcome, err := s.repository.Remove(ctx, id)
	if err != nil {
		return domain.VoiceDeleteResult{}, err
	}
	if outcome == domain.VoiceDeactivated {
		// Read it again: a reference added since the first read is what made Remove deactivate,
		// and the counts in the response should show it.
		if fresh, err := s.repository.Get(ctx, id); err == nil {
			voice = fresh
		}
		voice.IsActive = false
	}
	s.log.WithContext(ctx).Info("voice profile removed",
		logger.String("voice_id", id.String()),
		logger.String("admin_id", adminID.String()),
		logger.String("outcome", string(outcome)))
	return domain.VoiceDeleteResult{Outcome: outcome, Voice: voice}, nil
}

func normalizeFilter(f domain.VoiceListFilter) (domain.VoiceListFilter, error) {
	if f.Status == "" {
		f.Status = domain.VoiceStatusAll
	}
	if !f.Status.Valid() {
		return f, invalid("invalid status filter")
	}
	if f.Limit < 1 || f.Limit > 100 || f.Offset < 0 {
		return f, invalid("invalid pagination parameters")
	}
	if !utf8.ValidString(f.Query) || strings.ContainsRune(f.Query, 0) {
		return f, invalid("invalid search text")
	}
	f.Query = strings.TrimSpace(f.Query)
	if utf8.RuneCountInString(f.Query) > domain.MaxVoiceQueryRunes {
		return f, invalid("search text must contain at most 100 characters")
	}
	return f, nil
}
