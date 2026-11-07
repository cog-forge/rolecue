package service

import (
	"net/url"
	"strings"
)

// allowedImageHost reports whether the host of rawURL is in allowed. An entry is an exact host name or
// "*.example.com", which matches any subdomain (not the bare domain). Matching is case-insensitive and
// ignores the port, so a look-alike such as lh3.googleusercontent.com.evil.test is refused.
func allowedImageHost(rawURL string, allowed []string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, entry := range allowed {
		entry = strings.ToLower(strings.TrimSpace(entry))
		switch {
		case entry == "":
		case strings.HasPrefix(entry, "*."):
			if strings.HasSuffix(host, entry[1:]) && len(host) > len(entry)-1 {
				return true
			}
		case host == entry:
			return true
		}
	}
	return false
}
