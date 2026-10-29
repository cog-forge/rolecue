package provider

import (
	"context"
	"sync"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
)

// FakeName is the provider name the fake reports.
const FakeName = "fake"

// Fake is an in-memory voice provider for tests: it returns the voices it was given, or the error.
type Fake struct {
	mu     sync.Mutex
	voices []domain.ProviderVoice
	err    error
	calls  int
}

var _ service.VoiceProvider = (*Fake)(nil)

func NewFake(voices ...domain.ProviderVoice) *Fake { return &Fake{voices: voices} }

// Set replaces the voices the fake offers and clears any error.
func (f *Fake) Set(voices ...domain.ProviderVoice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.voices, f.err = voices, nil
}

// Fail makes every following call return err.
func (f *Fake) Fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *Fake) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *Fake) Name() string { return FakeName }

func (f *Fake) ListVoices(ctx context.Context) ([]domain.ProviderVoice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.ProviderVoice(nil), f.voices...), nil
}
