package common

import (
	"net/url"
	"os"
	"strings"
)

func ShouldUseSecureSessionCookie(serverAddress string) bool {
	if override := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")); override != "" {
		return strings.EqualFold(override, "true") || override == "1"
	}

	parsed, err := url.Parse(strings.TrimSpace(serverAddress))
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Scheme, "https")
}
