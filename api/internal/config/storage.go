package config

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type StorageConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token"`
	// SSE selects server-side encryption for objects the API writes and for presigned uploads:
	// "AES256" (default when empty), "aws:kms", or "none" (loopback development only).
	SSE         string `mapstructure:"sse"`
	SSEKMSKeyID string `mapstructure:"sse_kms_key_id"`
}

const (
	SSEAES256 = "AES256"
	SSEKMS    = "aws:kms"
	SSENone   = "none"
)

// EffectiveSSE returns the configured mode with the default applied.
func (c StorageConfig) EffectiveSSE() string {
	if strings.TrimSpace(c.SSE) == "" {
		return SSEAES256
	}
	return strings.TrimSpace(c.SSE)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// Validate is called by the storage constructor, so loading unrelated application
// settings does not require credentials. Errors deliberately omit config values.
func (c StorageConfig) Validate() error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.New("storage endpoint must be an HTTP(S) origin without credentials, path, query or fragment")
	}
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return errors.New("storage endpoint must use https unless it is a loopback host")
	}
	switch c.EffectiveSSE() {
	case SSEAES256:
		if c.SSEKMSKeyID != "" {
			return errors.New("storage KMS key id requires STORAGE_SSE=aws:kms")
		}
	case SSEKMS:
		if strings.ContainsAny(c.SSEKMSKeyID, " \t\r\n") {
			return errors.New("storage KMS key id must not contain whitespace")
		}
	case SSENone:
		if !isLoopbackHost(u.Hostname()) {
			return errors.New("storage SSE none is only allowed for loopback development endpoints")
		}
	default:
		return errors.New("storage SSE must be AES256, aws:kms or none")
	}
	if strings.TrimSpace(c.Region) == "" || strings.ContainsAny(c.Region, " /\t\r\n") {
		return errors.New("storage region is required and must not contain whitespace or slashes")
	}
	if !bucketName.MatchString(c.Bucket) || strings.Contains(c.Bucket, "..") || net.ParseIP(c.Bucket) != nil {
		return errors.New("storage bucket must be a DNS-compatible name of 3 to 63 characters")
	}
	if strings.TrimSpace(c.AccessKeyID) == "" || strings.TrimSpace(c.SecretAccessKey) == "" || strings.ContainsAny(c.AccessKeyID+c.SecretAccessKey+c.SessionToken, "\r\n") {
		return errors.New("storage access key and secret key are required; credentials must not contain newlines")
	}
	return nil
}
