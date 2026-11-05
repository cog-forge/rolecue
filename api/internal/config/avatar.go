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
	// UserDailyQuota is the conversions one user may start per rolling 24 hours (0 means unlimited);
	// UserMaxInflight is how many of one user's conversions may run at once (KAN-261).
	UserDailyQuota  int `mapstructure:"user_daily_quota"`
	UserMaxInflight int `mapstructure:"user_max_inflight"`
}

func (c AvatarConversionConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.BlenderBinary) == "" || c.Timeout < time.Second || c.Timeout > 5*time.Minute || c.MaxInputBytes < 28 || c.MaxInputBytes > 50<<20 || c.MaxOutputBytes < 28 || c.MaxOutputBytes > 100<<20 {
		return errors.New("invalid avatar conversion runtime limits")
	}
	if c.UserDailyQuota < 0 || c.UserMaxInflight < 0 || c.UserMaxInflight > 8 {
		return errors.New("invalid avatar conversion per-user limits")
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
