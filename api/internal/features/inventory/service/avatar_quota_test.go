package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/converter"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// blockingRunner holds every conversion until release is closed and counts how many started.
type blockingRunner struct {
	started atomic.Int32
	release chan struct{}
	dir     string
	err     error
}

func (r *blockingRunner) Convert(ctx context.Context, _ io.ReadSeeker, _ string) (*converter.Asset, error) {
	r.started.Add(1)
	select {
	case <-r.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if r.err != nil {
		return nil, r.err
	}
	file, err := os.CreateTemp(r.dir, "asset-*.vrm")
	if err != nil {
		return nil, err
	}
	return &converter.Asset{File: file}, nil
}

func quotaService(t *testing.T, repo *creationRepository, runner AvatarConverter, cfg config.AvatarConversionConfig) *AvatarConversionService {
	t.Helper()
	cfg.Enabled, cfg.MaxInputBytes = true, 50<<20
	return NewAvatarConversionService(repo, &conversionStore{objects: map[string]bool{}}, runner, cfg)
}

// N parallel uploads from one user with one free slot start one conversion, with the default
// in-flight limit and with a higher limit that the slot reservation then has to hold.
func TestParallelCreatesWithOneFreeSlotRunOneConversion(t *testing.T) {
	source := avatarSource(t)
	for name, inflight := range map[string]int{"default limit": 1, "higher limit": 4} {
		t.Run(name, func(t *testing.T) {
			const requests = 8
			repo := &creationRepository{track: true, capacity: domain.Capacity{Capacity: 4, Used: 3, Available: 1}}
			runner := &blockingRunner{release: make(chan struct{}), dir: t.TempDir()}
			svc := quotaService(t, repo, runner, config.AvatarConversionConfig{UserMaxInflight: inflight})
			user := inventoryUser()

			results := make(chan error, requests)
			for range requests {
				go func() {
					_, err := svc.Create(context.Background(), user, bytes.NewReader(source), int64(len(source)),
						domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"})
					results <- err
				}()
			}
			// Every request but the one holding the reservation returns while the conversion is blocked.
			var refused []error
			for range requests - 1 {
				select {
				case err := <-results:
					refused = append(refused, err)
				case <-time.After(10 * time.Second):
					t.Fatal("parallel requests were not refused while one conversion ran")
				}
			}
			if got := runner.started.Load(); got != 1 {
				t.Fatalf("conversions started = %d, want 1", got)
			}
			close(runner.release)
			if err := <-results; err != nil {
				t.Fatalf("the reserved request failed: %v", err)
			}
			for _, err := range refused {
				if !apperror.IsCode(err, apperror.CodeAvatarConversionQuota) && !apperror.IsCode(err, apperror.CodeInventoryFull) {
					t.Fatalf("refused request error = %v, want a quota or full-inventory refusal", err)
				}
			}
			if got := runner.started.Load(); got != 1 {
				t.Fatalf("conversions started = %d after release, want 1", got)
			}
		})
	}
}

func TestDailyQuotaCountsPersistedAvatars(t *testing.T) {
	source := avatarSource(t)
	repo := &creationRepository{recent: 2}
	runner := &conversionRunner{}
	svc := quotaService(t, repo, runner, config.AvatarConversionConfig{UserDailyQuota: 2})
	_, err := svc.Create(context.Background(), inventoryUser(), bytes.NewReader(source), int64(len(source)),
		domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"})
	if !apperror.IsCode(err, apperror.CodeAvatarConversionQuota) || runner.calls != 0 {
		t.Fatalf("Create() error = %v, conversions = %d, want the quota refusal before any conversion", err, runner.calls)
	}
}

// A conversion that fails still cost a Blender run, so it counts against the daily quota.
func TestDailyQuotaCountsFailedConversions(t *testing.T) {
	source := avatarSource(t)
	runner := &conversionRunner{err: errors.New("export failed")}
	svc := quotaService(t, &creationRepository{}, runner, config.AvatarConversionConfig{UserDailyQuota: 2})
	user := inventoryUser()
	create := func() error {
		_, err := svc.Create(context.Background(), user, bytes.NewReader(source), int64(len(source)),
			domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"})
		return err
	}
	for range 2 {
		if err := create(); !apperror.IsCode(err, apperror.CodeAvatarConversionFailed) {
			t.Fatalf("Create() error = %v, want a failed conversion", err)
		}
	}
	if err := create(); !apperror.IsCode(err, apperror.CodeAvatarConversionQuota) || runner.calls != 2 {
		t.Fatalf("third Create() error = %v, conversions = %d, want the quota refusal after 2 runs", err, runner.calls)
	}
	// Another user has their own quota.
	other := inventoryUser()
	other.ID = uuid.New()
	if _, err := svc.Create(context.Background(), other, bytes.NewReader(source), int64(len(source)),
		domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"}); !apperror.IsCode(err, apperror.CodeAvatarConversionFailed) {
		t.Fatalf("other user error = %v, want to reach the converter", err)
	}
}

func TestConversionGateWindowAndRelease(t *testing.T) {
	user := uuid.New()
	now := time.Now()
	gate := newConversionGate(1, 2)
	gate.now = func() time.Time { return now }

	first, err := gate.acquire(user, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.acquire(user, 0); !apperror.IsCode(err, apperror.CodeAvatarConversionQuota) {
		t.Fatalf("second concurrent acquire error = %v, want the in-flight refusal", err)
	}
	first.release()
	first.release() // releasing twice must not free someone else's reservation
	second, err := gate.acquire(user, 0)
	if err != nil {
		t.Fatal(err)
	}
	second.release()
	if _, err := gate.acquire(user, 0); !apperror.IsCode(err, apperror.CodeAvatarConversionQuota) {
		t.Fatalf("acquire past the daily quota error = %v, want the quota refusal", err)
	}
	now = now.Add(conversionQuotaWindow + time.Second)
	again, err := gate.acquire(user, 0)
	if err != nil {
		t.Fatalf("acquire after the window error = %v, want the history aged out", err)
	}
	again.release()
	if len(gate.inflight) != 0 {
		t.Fatalf("in-flight map = %v, want it empty after every release", gate.inflight)
	}
}

func TestConversionGateDefaultsAndUnlimitedQuota(t *testing.T) {
	gate := newConversionGate(0, 0)
	user := uuid.New()
	for range 50 {
		r, err := gate.acquire(user, 1000)
		if err != nil {
			t.Fatalf("unlimited quota refused: %v", err)
		}
		r.release()
	}
	var wg sync.WaitGroup
	var granted atomic.Int32
	hold := make(chan struct{})
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r, err := gate.acquire(user, 0); err == nil {
				granted.Add(1)
				<-hold
				r.release()
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(hold)
	wg.Wait()
	if granted.Load() != 1 {
		t.Fatalf("granted = %d, want the default of one in flight", granted.Load())
	}
}
