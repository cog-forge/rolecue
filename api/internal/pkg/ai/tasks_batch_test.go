package ai

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
)

// One 50-question reply does not fit the output throughput or the rate limits of the gpt-oss providers, so a
// chain that contains any of them asks for the bank in batches of at most 10 (a Cloudflare reply of about 900
// tokens took 16 s in the KAN-386 smoke test).
func TestBankBatchSize(t *testing.T) {
	entries := func(providers ...string) []routeEntry {
		var out []routeEntry
		for _, p := range providers {
			out = append(out, routeEntry{config: config.LLMConfig{Provider: p}})
		}
		return out
	}
	for _, tc := range []struct {
		name      string
		providers []string
		count     int
		want      int
	}{
		{"groq", []string{"groq"}, 50, 10},
		{"cloudflare", []string{"cloudflare"}, 50, 10},
		{"openrouter", []string{"openrouter"}, 50, 10},
		{"gemini only keeps one call", []string{"gemini"}, 50, 50},
		{"gemini first, cloudflare behind", []string{"gemini", "cloudflare"}, 50, 10},
		{"small bank", []string{"cloudflare"}, 8, 8},
		{"legacy entry without a provider", []string{""}, 50, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := bankBatchSize(entries(tc.providers...), tc.count); got != tc.want {
				t.Fatalf("bankBatchSize = %d, want %d", got, tc.want)
			}
		})
	}
}
