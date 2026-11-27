package config

import (
	"strings"
	"testing"
)

func TestStorageEnvironmentOverrides(t *testing.T) {
	// config.Load requires PayOS settings, but this storage test never calls PayOS.
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	t.Setenv("S3_ENDPOINT", "https://objects.example.test")
	t.Setenv("S3_REGION", "auto")
	t.Setenv("S3_BUCKET", "new-bucket")
	t.Setenv("S3_ACCESS_KEY_ID", "test-key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("S3_SESSION_TOKEN", "")
	cfg, err := Load(writeConfig(t, "storage:\n  endpoint: http://localhost:9000\n  session_token: file-token\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := StorageConfig{Endpoint: "https://objects.example.test", Region: "auto", Bucket: "new-bucket", AccessKeyID: "test-key", SecretAccessKey: "test-secret"}
	if cfg.Storage != want {
		t.Fatal("storage environment did not override YAML, including empty values")
	}
}

func TestStorageCredentialModes(t *testing.T) {
	base := StorageConfig{Endpoint: "https://s3.ap-southeast-1.amazonaws.com", Region: "ap-southeast-1", Bucket: "rolecue-prod"}
	for _, tc := range []struct {
		name   string
		change func(*StorageConfig)
		static bool
	}{
		{"no keys uses the default AWS chain", func(*StorageConfig) {}, false},
		{"blank keys use the default AWS chain", func(c *StorageConfig) { c.AccessKeyID, c.SecretAccessKey = "  ", "\t" }, false},
		{"both keys are static", func(c *StorageConfig) { c.AccessKeyID, c.SecretAccessKey = "test-key", "test-secret" }, true},
		{"both keys with session token are static", func(c *StorageConfig) {
			c.AccessKeyID, c.SecretAccessKey, c.SessionToken = "test-key", "test-secret", "test-token"
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.change(&cfg)
			if err := cfg.Validate(); err != nil {
				t.Fatalf("valid storage config rejected: %v", err)
			}
			if got := cfg.UsesStaticCredentials(); got != tc.static {
				t.Fatalf("UsesStaticCredentials() = %v, want %v", got, tc.static)
			}
		})
	}
}

func TestStorageValidation(t *testing.T) {
	valid := StorageConfig{Endpoint: "http://localhost:9000", Region: "us-east-1", Bucket: "rolecue-local", AccessKeyID: "test-key", SecretAccessKey: "test-secret"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*StorageConfig)
	}{
		{"missing endpoint", func(c *StorageConfig) { c.Endpoint = "" }},
		{"endpoint scheme", func(c *StorageConfig) { c.Endpoint = "ftp://host" }},
		{"endpoint credentials", func(c *StorageConfig) { c.Endpoint = "http://secret:password@host" }},
		{"endpoint path", func(c *StorageConfig) { c.Endpoint = "http://host/other" }},
		{"endpoint query", func(c *StorageConfig) { c.Endpoint = "http://host?token=secret" }},
		{"endpoint fragment", func(c *StorageConfig) { c.Endpoint = "http://host#other" }},
		{"missing region", func(c *StorageConfig) { c.Region = " " }},
		{"invalid bucket", func(c *StorageConfig) { c.Bucket = "Upper_Case" }},
		{"short bucket", func(c *StorageConfig) { c.Bucket = "ab" }},
		{"IP bucket", func(c *StorageConfig) { c.Bucket = "127.0.0.1" }},
		{"missing key", func(c *StorageConfig) { c.AccessKeyID = "" }},
		{"missing secret", func(c *StorageConfig) { c.SecretAccessKey = "" }},
		{"only blank key", func(c *StorageConfig) { c.AccessKeyID = "  " }},
		{"only blank secret", func(c *StorageConfig) { c.SecretAccessKey = "\t" }},
		{"session token without keys", func(c *StorageConfig) { c.AccessKeyID, c.SecretAccessKey, c.SessionToken = "", "", "private-value" }},
		{"session token with blank keys", func(c *StorageConfig) { c.AccessKeyID, c.SecretAccessKey, c.SessionToken = " ", " ", "private-value" }},
		{"credential newline", func(c *StorageConfig) { c.SecretAccessKey = "private-value\n" }},
		{"session token newline", func(c *StorageConfig) { c.SessionToken = "private-value\n" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.change(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("invalid storage config accepted")
			}
			if strings.Contains(err.Error(), "private-value") || strings.Contains(err.Error(), "password@") {
				t.Fatal("validation error exposed a credential")
			}
		})
	}
}
