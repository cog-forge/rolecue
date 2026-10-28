package converter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// A failed export must name Blender's Python error instead of only "exit status 1".
func TestConversionFailureKeepsBlenderError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the Blender binary")
	}
	binary := filepath.Join(t.TempDir(), "blender")
	script := "#!/bin/sh\necho 'AttributeError: no attribute upperChest' >&2\nexit 1\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	b := &Blender{Binary: binary, Timeout: 10 * time.Second, MaxOutput: 1 << 20}
	_, err := b.Convert(context.Background(), bytes.NewReader([]byte("glb")), "test")
	if err == nil || !strings.Contains(err.Error(), "AttributeError: no attribute upperChest") {
		t.Fatalf("conversion error = %v, want Blender stderr", err)
	}
}

func TestTailWriterKeepsTheEnd(t *testing.T) {
	w := &tailWriter{limit: 5}
	for _, part := range []string{"abc", "defg", "hi"} {
		if n, err := w.Write([]byte(part)); n != len(part) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", part, n, err)
		}
	}
	if got := string(w.data); got != "efghi" {
		t.Fatalf("tail = %q, want %q", got, "efghi")
	}
}
