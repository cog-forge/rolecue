package config

import (
	"os"
	"testing"
	"time"
)

func TestLiveGuardValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(c *InterviewConfig)
		ok     bool
	}{
		{"defaults (zero)", func(c *InterviewConfig) {}, true},
		{"sixty seconds", func(c *InterviewConfig) { c.LiveRecheckInterval = time.Minute }, true},
		{"too frequent", func(c *InterviewConfig) { c.LiveRecheckInterval = time.Second }, false},
		{"too rare", func(c *InterviewConfig) { c.LiveRecheckInterval = time.Hour }, false},
		{"negative rate", func(c *InterviewConfig) { c.LiveMessageRate = -1 }, false},
		{"huge burst", func(c *InterviewConfig) { c.LiveMessageBurst = MaxLiveMessageBurst + 1 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c InterviewConfig
			tc.modify(&c)
			if err := c.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%t", err, tc.ok)
			}
		})
	}
}

func TestTrustedProxiesValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []string
		ok      bool
	}{
		{"empty trusts nobody", nil, true},
		{"loopback defaults", DefaultTrustedProxies, true},
		{"ipv4 and cidr", []string{"10.0.0.1", "172.16.0.0/12"}, true},
		{"ipv6 cidr", []string{"fd00::/8"}, true},
		{"hostname rejected", []string{"proxy.internal"}, false},
		{"bad prefix rejected", []string{"10.0.0.0/40"}, false},
		{"blank rejected", []string{""}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Server.TrustedProxies = tc.entries
			if err := cfg.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%t", err, tc.ok)
			}
		})
	}
}

func TestTrustedProxiesLoading(t *testing.T) {
	t.Setenv("PAYOS_CLIENT_ID", "id")
	t.Setenv("PAYOS_API_KEY", "key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "sum")

	t.Run("defaults to loopback", func(t *testing.T) {
		t.Setenv("SERVER_TRUSTED_PROXIES", "") // registers cleanup; an empty variable counts as set
		if err := os.Unsetenv("SERVER_TRUSTED_PROXIES"); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "127.0.0.1" {
			t.Fatalf("trusted proxies = %v, want loopback defaults", cfg.Server.TrustedProxies)
		}
	})
	t.Run("environment overrides", func(t *testing.T) {
		t.Setenv("SERVER_TRUSTED_PROXIES", "10.0.0.0/8,192.168.1.5")
		cfg, err := Load(writeConfig(t, minimalYAML))
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "10.0.0.0/8" {
			t.Fatalf("trusted proxies = %v", cfg.Server.TrustedProxies)
		}
	})
}
