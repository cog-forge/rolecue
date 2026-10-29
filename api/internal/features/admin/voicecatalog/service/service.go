package service

import (
	"context"
	"errors"
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
	Activate(ctx context.Context, id uuid.UUID) error
	Sync(ctx context.Context, provider string, voices []domain.ProviderVoice) (domain.VoiceSyncResult, error)
}

// VoiceProvider is the vendor-neutral view of a text-to-speech provider's voice list. The service never
// names a vendor; the adapter in the provider package implements it.
type VoiceProvider interface {
	// Name identifies the provider in the catalog (the voices.provider column).
	Name() string
	ListVoices(ctx context.Context) ([]domain.ProviderVoice, error)
}

// ErrProviderNotConfigured is returned by an adapter that has no credentials, so the sync can say the
// provider is unavailable instead of failing.
var ErrProviderNotConfigured = errors.New("voice provider is not configured")

// maxProviderFieldRunes bounds provider-supplied text before it reaches the catalog.
const maxProviderFieldRunes = 200

type Service struct {
	repository Repository
	provider   VoiceProvider
	locales    []string
	log        *logger.Logger
}

// New builds the service. Removals and syncs are logged because the brain asks for an admin audit
// trail while the frozen schema has no audit table. syncLocales limits a sync to those locales: an
// entry matches the locale itself or any region under it ("en" matches en-US and en-GB); no entries
// means every locale.
func New(repository Repository, provider VoiceProvider, syncLocales []string, log *logger.Logger) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("voice catalog repository is required")
	}
	if provider == nil {
		return nil, fmt.Errorf("voice provider is required")
	}
	if log == nil {
		log = &logger.Logger{Logger: zap.NewNop()}
	}
	return &Service{repository: repository, provider: provider, locales: normalizeLocales(syncLocales), log: log}, nil
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

// Activate undoes a deactivation so the voice can be chosen again. It is safe to repeat; a voice
// that was deleted permanently is not found.
func (s *Service) Activate(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.VoiceProfile, error) {
	if err := requireAdmin(role); err != nil {
		return domain.VoiceProfile{}, err
	}
	if err := s.repository.Activate(ctx, id); err != nil {
		return domain.VoiceProfile{}, err
	}
	voice, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.VoiceProfile{}, err
	}
	s.log.WithContext(ctx).Info("voice profile reactivated",
		logger.String("voice_id", id.String()),
		logger.String("admin_id", adminID.String()))
	return voice, nil
}

// Sync fetches the provider's voices and brings the catalog in line with them: new voices are added,
// changed ones updated, and voices the provider no longer offers are deactivated, never deleted.
// Everything is validated before the first write, and the write is one transaction, so a provider
// failure or a malformed response leaves the catalog unchanged. Repeating a sync changes nothing.
// A sync never reactivates a voice: an Admin may have deactivated it on purpose.
func (s *Service) Sync(ctx context.Context, adminID uuid.UUID, role string) (domain.VoiceSyncResult, error) {
	if err := requireAdmin(role); err != nil {
		return domain.VoiceSyncResult{}, err
	}
	name := s.provider.Name()
	fetched, err := s.provider.ListVoices(ctx)
	if err != nil {
		s.log.WithContext(ctx).Warn("voice sync failed to fetch", logger.String("provider", name), logger.Error(err))
		if errors.Is(err, ErrProviderNotConfigured) {
			return domain.VoiceSyncResult{}, apperror.Wrap(apperror.CodeVoiceProviderUnavailable, "the voice provider is not configured", err)
		}
		return domain.VoiceSyncResult{}, apperror.Wrap(apperror.CodeVoiceSyncFailed, "the voice provider could not be reached", err)
	}
	voices, err := s.validate(fetched)
	if err != nil {
		s.log.WithContext(ctx).Warn("voice sync rejected provider response", logger.String("provider", name), logger.Error(err))
		return domain.VoiceSyncResult{}, apperror.Wrap(apperror.CodeVoiceSyncFailed, "the voice provider returned an invalid voice list", err)
	}
	result, err := s.repository.Sync(ctx, name, voices)
	if err != nil {
		return domain.VoiceSyncResult{}, err
	}
	s.log.WithContext(ctx).Info("voice catalog synced",
		logger.String("provider", name),
		logger.String("admin_id", adminID.String()),
		logger.Int("fetched", result.Fetched),
		logger.Int("added", result.Added),
		logger.Int("updated", result.Updated),
		logger.Int("deactivated", result.Deactivated),
		logger.Int("unchanged", result.Unchanged))
	return result, nil
}

// validate checks the whole provider response and keeps the voices in the sync locales. An empty
// result is rejected: deactivating the entire catalog because of an empty answer is never right.
func (s *Service) validate(fetched []domain.ProviderVoice) ([]domain.ProviderVoice, error) {
	kept := make([]domain.ProviderVoice, 0, len(fetched))
	seen := make(map[string]struct{}, len(fetched))
	for i, v := range fetched {
		v = domain.ProviderVoice{ProviderVoiceID: strings.TrimSpace(v.ProviderVoiceID), Name: strings.TrimSpace(v.Name),
			Locale: strings.TrimSpace(v.Locale), Gender: strings.TrimSpace(v.Gender)}
		for _, field := range []struct{ label, value string }{{"id", v.ProviderVoiceID}, {"name", v.Name}, {"locale", v.Locale}, {"gender", v.Gender}} {
			if !utf8.ValidString(field.value) || strings.ContainsRune(field.value, 0) || utf8.RuneCountInString(field.value) > maxProviderFieldRunes {
				return nil, fmt.Errorf("voice %d has an invalid %s", i, field.label)
			}
		}
		if v.ProviderVoiceID == "" || v.Name == "" || v.Locale == "" {
			return nil, fmt.Errorf("voice %d is missing its id, name or locale", i)
		}
		if !s.inScope(v.Locale) {
			continue
		}
		if _, dup := seen[v.ProviderVoiceID]; dup {
			return nil, fmt.Errorf("voice id %q appears more than once", v.ProviderVoiceID)
		}
		seen[v.ProviderVoiceID] = struct{}{}
		kept = append(kept, v)
	}
	if len(kept) == 0 {
		return nil, errors.New("no voices in the sync locales")
	}
	return kept, nil
}

func (s *Service) inScope(locale string) bool {
	if len(s.locales) == 0 {
		return true
	}
	locale = strings.ToLower(locale)
	for _, wanted := range s.locales {
		if locale == wanted || strings.HasPrefix(locale, wanted+"-") {
			return true
		}
	}
	return false
}

func normalizeLocales(locales []string) []string {
	out := make([]string, 0, len(locales))
	for _, l := range locales {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" {
			out = append(out, l)
		}
	}
	return out
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
