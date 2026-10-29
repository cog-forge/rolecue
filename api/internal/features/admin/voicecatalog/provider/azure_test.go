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
)

const azureBody = `[
  {"Name":"Microsoft Server Speech Text to Speech Voice (en-US, AvaNeural)","DisplayName":"Ava","LocalName":"Ava","ShortName":"en-US-AvaNeural","Gender":"Female","Locale":"en-US","LocaleName":"English (United States)","StyleList":["cheerful"],"SampleRateHertz":"48000","VoiceType":"Neural","Status":"GA","WordsPerMinute":"152"},
  {"Name":"Microsoft Server Speech Text to Speech Voice (zh-CN, YunxiNeural)","DisplayName":"","LocalName":"云希","ShortName":"zh-CN-YunxiNeural","Gender":"Male","Locale":"zh-CN","Status":"GA"}
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
