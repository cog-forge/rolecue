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
}

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// Validate is called by the storage constructor, so loading unrelated application
// settings does not require credentials. Errors deliberately omit config values.
func (c StorageConfig) Validate() error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.New("storage endpoint must be an HTTP(S) origin without credentials, path, query or fragment")
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
