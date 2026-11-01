package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
	"github.com/google/uuid"
)

const azureBody = `[
  {"Name":"Microsoft Server Speech Text to Speech Voice (en-US, AvaNeural)","DisplayName":"Ava","LocalName":"Ava","ShortName":"en-US-AvaNeural","Gender":"Female","Locale":"en-US","LocaleName":"English (United States)","StyleList":["cheerful"],"SampleRateHertz":"48000","VoiceType":"Neural","Status":"GA","WordsPerMinute":"152"},
  {"Name":"Microsoft Server Speech Text to Speech Voice (zh-CN, YunxiNeural)","DisplayName":"","LocalName":"云希","ShortName":"zh-CN-YunxiNeural","Gender":"Male","Locale":"zh-CN","VoiceType":"Neural","Status":"GA"}
]`

// mixedBody mirrors the shapes of the real list: standard neural voices, multilingual voices (own type in the
// language-support page), HD voices (":DragonHD..." names, from the HD voices page) and a non-neural type.
const mixedBody = `[
  {"DisplayName":"Jenny","ShortName":"en-US-JennyNeural","Gender":"Female","Locale":"en-US","VoiceType":"Neural","Status":"GA"},
  {"DisplayName":"Guy","ShortName":"en-US-GuyNeural","Gender":"Male","Locale":"en-US","VoiceType":"Neural","Status":"GA"},
  {"DisplayName":"Jenny Multilingual","ShortName":"en-US-JennyMultilingualNeural","Gender":"Female","Locale":"en-US","SecondaryLocaleList":["de-DE"],"VoiceType":"Neural","Status":"GA"},
  {"DisplayName":"Alloy Turbo Multilingual","ShortName":"en-US-AlloyTurboMultilingualNeural","Gender":"Male","Locale":"en-US","VoiceType":"Neural","Status":"GA"},
  {"DisplayName":"Ava HD","ShortName":"en-US-Ava:DragonHDLatestNeural","Gender":"Female","Locale":"en-US","VoiceType":"Neural","Status":"GA"},
  {"DisplayName":"Andrew3 HD","ShortName":"en-us-Andrew3:DragonHDLatestNeural","Gender":"Male","Locale":"en-US","VoiceType":"Neural","Status":"Preview"},
  {"DisplayName":"Conrad Omni","ShortName":"de-DE-Conrad:DragonHDOmniLatestNeural","Gender":"Male","Locale":"de-DE","VoiceType":"Neural","Status":"GA"},
  {"DisplayName":"Ava HD typed","ShortName":"en-US-AvaHD","Gender":"Female","Locale":"en-US","VoiceType":"NeuralHD","Status":"GA"},
  {"DisplayName":"No type","ShortName":"en-US-NoTypeNeural","Gender":"Female","Locale":"en-US","Status":"GA"},
  {"DisplayName":"Sonia","ShortName":"en-GB-SoniaNeural","Gender":"Female","Locale":"en-GB","VoiceType":"Neural","Status":"GA"}
]`

func stub(t *testing.T, handler http.HandlerFunc) *Azure {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	a, err := NewAzure("secret-key", "eastus", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return a.withBaseURL(server.URL)
}

func TestAzureListsVoices(t *testing.T) {
	var gotPath, gotKey, gotMethod string
	a := stub(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey, gotMethod = r.URL.Path, r.Header.Get("Ocp-Apim-Subscription-Key"), r.Method
		_, _ = w.Write([]byte(azureBody))
	})
	got, err := a.ListVoices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/cognitiveservices/voices/list" || gotKey != "secret-key" {
		t.Fatalf("request = %s %s key=%q", gotMethod, gotPath, gotKey)
	}
	want := []domain.ProviderVoice{
		{ProviderVoiceID: "en-US-AvaNeural", Name: "Ava", Locale: "en-US", Gender: "Female"},
		{ProviderVoiceID: "zh-CN-YunxiNeural", Name: "云希", Locale: "zh-CN", Gender: "Male"},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("voices = %+v, want %+v", got, want)
	}
	if a.Name() != AzureName {
		t.Fatalf("name = %q", a.Name())
	}
}

func TestAzureBuildsTheRegionalURL(t *testing.T) {
	a, err := NewAzure("k", "southeastasia", 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.baseURL != "https://southeastasia.tts.speech.microsoft.com" {
		t.Fatalf("baseURL = %q", a.baseURL)
	}
}

func TestAzureRejectsAHostileRegion(t *testing.T) {
	for _, region := range []string{"evil.com/x", "east us", "EastUS", "a", "eastus.evil.com", "east-us"} {
		if _, err := NewAzure("k", region, 0); err == nil {
			t.Errorf("region %q was accepted", region)
		}
	}
}

func TestAzureWithoutCredentialsIsNotConfigured(t *testing.T) {
	for _, c := range []struct{ key, region string }{{"", "eastus"}, {"k", ""}, {"", ""}} {
		a, err := NewAzure(c.key, c.region, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.ListVoices(context.Background()); !errors.Is(err, service.ErrProviderNotConfigured) {
			t.Fatalf("key=%q region=%q: err = %v, want ErrProviderNotConfigured", c.key, c.region, err)
		}
	}
}

func TestAzureFailures(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"unauthorized": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "bad key", http.StatusUnauthorized) },
		"rate limited": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "slow down", http.StatusTooManyRequests) },
		"bad gateway":  func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "oops", http.StatusBadGateway) },
		"not json":     func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>maintenance</html>")) },
		"wrong shape":  func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"voices":[]}`)) },
		"truncated":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[{"ShortName":"en-US-Ava`)) },
		"too large": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`["` + strings.Repeat("a", maxResponseBytes) + `"]`))
		},
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := stub(t, handler).ListVoices(context.Background())
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), "secret-key") {
				t.Fatalf("error leaks the key: %v", err)
			}
		})
	}
}

func TestAzureTimesOutAndHonoursCancellation(t *testing.T) {
	release := make(chan struct{})
	a := stub(t, func(http.ResponseWriter, *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	a.client.Timeout = 50 * time.Millisecond
	if _, err := a.ListVoices(context.Background()); err == nil {
		t.Fatal("a slow provider must time out")
	}

	a.client.Timeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.ListVoices(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFakeReportsItsVoicesAndFailures(t *testing.T) {
	f := NewFake(domain.ProviderVoice{ProviderVoiceID: "a"})
	got, err := f.ListVoices(context.Background())
	if err != nil || len(got) != 1 || f.Name() != FakeName || f.Calls() != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	f.Fail(errors.New("down"))
	if _, err := f.ListVoices(context.Background()); err == nil {
		t.Fatal("the fake should fail after Fail")
	}
	f.Set()
	if got, err := f.ListVoices(context.Background()); err != nil || len(got) != 0 {
		t.Fatalf("Set should clear the failure: %v, %v", got, err)
	}
}

func TestAzureKeepsOnlyStandardNeuralVoices(t *testing.T) {
	a := stub(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(mixedBody)) })
	got, err := a.ListVoices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, v := range got {
		ids = append(ids, v.ProviderVoiceID)
	}
	want := []string{"en-US-JennyNeural", "en-US-GuyNeural", "en-GB-SoniaNeural"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("kept %v, want %v", ids, want)
	}
}

// recordingCatalog is the part of the repository a sync needs: it stores what Sync receives, and, like the SQL,
// deactivates the provider's voices that are not offered anymore and never deletes a row.
type recordingCatalog struct {
	service.Repository
	active map[string]bool
	synced []string
}

func (c *recordingCatalog) Sync(_ context.Context, name string, voices []domain.ProviderVoice) (domain.VoiceSyncResult, error) {
	result := domain.VoiceSyncResult{Provider: name, Fetched: len(voices)}
	offered := map[string]bool{}
	c.synced = c.synced[:0]
	for _, v := range voices {
		offered[v.ProviderVoiceID] = true
		c.synced = append(c.synced, v.ProviderVoiceID)
		if _, known := c.active[v.ProviderVoiceID]; !known {
			c.active[v.ProviderVoiceID] = true
			result.Added++
		}
	}
	for id, active := range c.active {
		if active && !offered[id] {
			c.active[id] = false
			result.Deactivated++
		}
	}
	result.Unchanged = result.Fetched - result.Added
	return result, nil
}

// A voice that is now filtered out but was synced before (an HD or multilingual voice from an older catalog) goes
// through the normal "no longer offered" path: the row stays, inactive.
func TestSyncDeactivatesAFilteredOutVoiceAndNeverDeletesIt(t *testing.T) {
	a := stub(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(mixedBody)) })
	catalog := &recordingCatalog{active: map[string]bool{
		"en-US-JennyNeural":              true,
		"en-US-JennyMultilingualNeural":  true,
		"en-US-Ava:DragonHDLatestNeural": true,
	}}
	svc, err := service.New(catalog, a, []string{"en-US"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Sync(context.Background(), uuid.New(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(catalog.synced, ",") != "en-US-JennyNeural,en-US-GuyNeural" {
		t.Fatalf("the repository received %v; only en-US standard neural voices may reach it", catalog.synced)
	}
	if got.Fetched != 2 || got.Added != 1 || got.Deactivated != 2 || got.Unchanged != 1 {
		t.Fatalf("result = %+v, want fetched 2, added 1, deactivated 2, unchanged 1", got)
	}
	if len(catalog.active) != 4 {
		t.Fatalf("catalog has %d rows, a sync must never delete", len(catalog.active))
	}
	for _, id := range []string{"en-US-JennyMultilingualNeural", "en-US-Ava:DragonHDLatestNeural"} {
		if active, known := catalog.active[id]; !known || active {
			t.Fatalf("%s should stay as an inactive row (known=%v active=%v)", id, known, active)
		}
	}
}
