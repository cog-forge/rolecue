package config

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

type AvatarConversionConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	BlenderBinary   string        `mapstructure:"blender_binary"`
	Timeout         time.Duration `mapstructure:"timeout"`
	MaxInputBytes   int64         `mapstructure:"max_input_bytes"`
	MaxOutputBytes  int64         `mapstructure:"max_output_bytes"`
	CommercialUsage string        `mapstructure:"commercial_usage"`
	LicenseURL      string        `mapstructure:"license_url"`
}

func (c AvatarConversionConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.BlenderBinary) == "" || c.Timeout < time.Second || c.Timeout > 5*time.Minute || c.MaxInputBytes < 28 || c.MaxInputBytes > 50<<20 || c.MaxOutputBytes < 28 || c.MaxOutputBytes > 100<<20 {
		return errors.New("invalid avatar conversion runtime limits")
	}
	if c.CommercialUsage != "personalNonProfit" && c.CommercialUsage != "personalProfit" && c.CommercialUsage != "corporation" {
		return errors.New("avatar conversion requires an explicit VRM commercial usage policy")
	}
	u, err := url.Parse(c.LicenseURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return errors.New("avatar conversion requires an HTTPS source license URL")
	}
	return nil
}
