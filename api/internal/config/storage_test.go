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
		{"credential newline", func(c *StorageConfig) { c.SecretAccessKey = "private-value\n" }},
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
