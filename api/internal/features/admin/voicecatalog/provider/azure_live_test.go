//go:build azurelive

package provider

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestAzureLive makes one real call with AZURE_SPEECH_KEY and AZURE_SPEECH_REGION from the
// environment. Run: go test -tags azurelive -run TestAzureLive -v ./internal/features/admin/voicecatalog/provider/
func TestAzureLive(t *testing.T) {
	key, region := os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION")
	if key == "" || region == "" {
		t.Skip("AZURE_SPEECH_KEY and AZURE_SPEECH_REGION are not set")
	}
	a, err := NewAzure(key, region, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	voices, err := a.ListVoices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	english := 0
	for _, v := range voices {
		if v.ProviderVoiceID == "" || v.Name == "" || v.Locale == "" {
			t.Fatalf("incomplete voice: %+v", v)
		}
		if len(v.Locale) >= 2 && v.Locale[:2] == "en" {
			english++
		}
	}
	t.Logf("azure returned %d voices, %d English", len(voices), english)
	if english == 0 {
		t.Fatal("expected English voices")
	}
}
