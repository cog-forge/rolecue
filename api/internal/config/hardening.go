package config

import (
	"fmt"
	"net/netip"
	"strings"
)

// DefaultTrustedProxies trusts only a reverse proxy on the same host. Gin otherwise trusts every
// peer, so any client could choose its own logged address with a forged X-Forwarded-For.
var DefaultTrustedProxies = []string{"127.0.0.1", "::1"}

// validateTrustedProxies checks that every entry is an IP address or a CIDR range.
func validateTrustedProxies(entries []string) error {
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if _, err := netip.ParsePrefix(e); err == nil {
			continue
		}
		if _, err := netip.ParseAddr(e); err == nil {
			continue
		}
		return fmt.Errorf("server.trusted_proxies entry %q is not an IP address or CIDR", e)
	}
	return nil
}
