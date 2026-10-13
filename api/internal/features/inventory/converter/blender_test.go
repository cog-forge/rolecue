package converter

import (
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
