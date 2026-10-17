package authcookie

// Matches reports whether name is a Better Auth session cookie for this configuration.
func Matches(name, cookiePrefix string, secure bool) bool {
	if cookiePrefix == "" {
		return false
	}
	prefix := cookiePrefix + "."
	if secure {
		prefix = "__Secure-" + prefix
	}
	return name == prefix+"session_token" || name == prefix+"dont_remember" || name == prefix+"session_data"
}
