package common

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func InitSessionCookieSettings() error {
	secureRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE"))
	trustedURLsRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_TRUSTED_URL"))

	SessionCookieSecure = false
	SessionCookieTrustedURLs = nil

	if secureRaw == "" || strings.EqualFold(secureRaw, "false") {
		if trustedURLsRaw != "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL requires SESSION_COOKIE_SECURE=true")
		}
		return nil
	}

	if !strings.EqualFold(secureRaw, "true") {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true or false")
	}

	if trustedURLsRaw == "" {
		return fmt.Errorf("SESSION_COOKIE_SECURE=true requires SESSION_COOKIE_TRUSTED_URL")
	}

	trustedURLs := strings.Split(trustedURLsRaw, ",")
	for _, trustedURL := range trustedURLs {
		trustedURL = strings.TrimSpace(trustedURL)
		if trustedURL == "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL contains an empty URL")
		}
		parsedURL, err := url.Parse(trustedURL)
		if err != nil {
			return fmt.Errorf("invalid SESSION_COOKIE_TRUSTED_URL: %w", err)
		}
		if parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL must contain only https URLs with hosts")
		}
		SessionCookieTrustedURLs = append(SessionCookieTrustedURLs, trustedURL)
	}

	SessionCookieSecure = true
	return nil
}

// ShouldUseSecureSessionCookie reports whether the session cookie must carry the
// Secure attribute. InitSessionCookieSettings only turns it on when the operator
// opts in explicitly, so an https ServerAddress alone still has to imply Secure —
// otherwise an https deployment that never set SESSION_COOKIE_SECURE would ship
// its session cookie over plain http.
func ShouldUseSecureSessionCookie(serverAddress string) bool {
	if SessionCookieSecure {
		return true
	}

	if override := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")); override != "" {
		return strings.EqualFold(override, "true") || override == "1"
	}

	parsed, err := url.Parse(strings.TrimSpace(serverAddress))
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Scheme, "https")
}
