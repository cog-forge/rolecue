package config

import (
	"testing"
	"time"
)

func TestAvatarConfiguration(t *testing.T) {
	if err := (AvatarConversionConfig{}).Validate(); err != nil {
		t.Fatal(err)
	}
	c := AvatarConversionConfig{Enabled: true, BlenderBinary: "blender", Timeout: 120 * time.Second, MaxInputBytes: 50 << 20, MaxOutputBytes: 100 << 20}
	if c.Validate() == nil {
		t.Fatal("unconfigured usage policy accepted")
	}
	c.CommercialUsage = "personalNonProfit"
	c.LicenseURL = "https://example.com/source-license"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
