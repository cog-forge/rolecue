package config

import "testing"

func TestStorageEndpointAndSSEValidation(t *testing.T) {
	base := StorageConfig{Endpoint: "https://objects.example.test", Region: "auto", Bucket: "rolecue-prod", AccessKeyID: "k", SecretAccessKey: "s"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*StorageConfig)
		ok     bool
	}{
		{"http remote", func(c *StorageConfig) { c.Endpoint = "http://objects.example.test" }, false},
		{"http loopback ip", func(c *StorageConfig) { c.Endpoint = "http://127.0.0.1:9000" }, true},
		{"http localhost", func(c *StorageConfig) { c.Endpoint = "http://localhost:9000" }, true},
		{"http ipv6 loopback", func(c *StorageConfig) { c.Endpoint = "http://[::1]:9000" }, true},
		{"http lookalike", func(c *StorageConfig) { c.Endpoint = "http://localhost.evil.test" }, false},
		{"sse kms", func(c *StorageConfig) { c.SSE = "aws:kms"; c.SSEKMSKeyID = "alias/x" }, true},
		{"sse bad", func(c *StorageConfig) { c.SSE = "rot13" }, false},
		{"kms key without kms", func(c *StorageConfig) { c.SSEKMSKeyID = "alias/x" }, false},
		{"sse none remote", func(c *StorageConfig) { c.SSE = "none" }, false},
		{"sse none loopback", func(c *StorageConfig) { c.SSE = "none"; c.Endpoint = "http://localhost:9000" }, true},
	} {
		c := base
		tc.change(&c)
		if err := c.Validate(); (err == nil) != tc.ok {
			t.Errorf("%s: err=%v", tc.name, err)
		}
	}
}
