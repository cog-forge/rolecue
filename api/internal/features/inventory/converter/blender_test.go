package converter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestConverterUnavailable(t *testing.T) {
	b := &Blender{Binary: "/does/not/exist"}
	if !errors.Is(b.Available(context.Background()), ErrUnavailable) {
		t.Fatal("missing runtime accepted")
	}
}

func TestRealConversion(t *testing.T) {
	if os.Getenv("AVATAR_CONVERSION_RUN") != "1" {
		t.Skip("set AVATAR_CONVERSION_RUN=1 to run Blender")
	}
	f, err := os.Open(os.Getenv("AVATAR_CONVERSION_FIXTURE"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	b := &Blender{Binary: "blender", Timeout: 120 * time.Second, MaxOutput: 100 << 20, CommercialUsage: "personalNonProfit"}
	if err = b.Available(context.Background()); err != nil {
		t.Fatal(err)
	}
	asset, err := b.Convert(context.Background(), f, "KAN-122 fixture")
	if err != nil {
		t.Fatal(err)
	}
	path := asset.Name()
	if err = asset.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temporary result retained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = b.Convert(ctx, f, "cancelled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled conversion: %v", err)
	}
}

// Block the first input stream to keep a conversion occupied without Blender.
type blockedSource struct {
	*bytes.Reader
	started chan struct{}
	release chan struct{}
}

func (s *blockedSource) Seek(offset int64, whence int) (int64, error) {
	close(s.started)
	<-s.release
	return s.Reader.Seek(offset, whence)
}
func TestConcurrentConversionRejectedAndSlotReleased(t *testing.T) {
	b := &Blender{Binary: "/does/not/exist", Timeout: time.Second}
	source := &blockedSource{Reader: bytes.NewReader([]byte("input")), started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := b.Convert(context.Background(), source, "first"); done <- err }()
	select {
	case <-source.started:
	case <-time.After(5 * time.Second):
		t.Fatal("conversion did not start")
	}
	defer func() {
		close(source.release)
		if err := <-done; err == nil {
			t.Error("conversion with a missing Blender binary unexpectedly succeeded")
		}
	}()
	_, err := b.Convert(context.Background(), bytes.NewReader(nil), "second")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("busy converter = %v, want unavailable", err)
	}
}
func TestConversionFailureReleasesSlot(t *testing.T) {
	b := &Blender{Binary: "/does/not/exist", Timeout: time.Second}
	for range 2 {
		_, err := b.Convert(context.Background(), bytes.NewReader(nil), "test")
		if err == nil || errors.Is(err, ErrUnavailable) {
			t.Fatalf("failed conversion retained slot: %v", err)
		}
	}
}
