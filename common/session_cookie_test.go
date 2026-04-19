package common

import (
	"os"
	"testing"
)

func TestShouldUseSecureSessionCookie(t *testing.T) {
	previous := os.Getenv("SESSION_COOKIE_SECURE")
	t.Cleanup(func() {
		if previous == "" {
			_ = os.Unsetenv("SESSION_COOKIE_SECURE")
			return
		}
		_ = os.Setenv("SESSION_COOKIE_SECURE", previous)
	})

	_ = os.Unsetenv("SESSION_COOKIE_SECURE")
	if !ShouldUseSecureSessionCookie("https://example.com") {
		t.Fatal("expected https server address to enable secure cookie")
	}
	if ShouldUseSecureSessionCookie("http://example.com") {
		t.Fatal("expected http server address to disable secure cookie")
	}
	if ShouldUseSecureSessionCookie("://bad-url") {
		t.Fatal("expected invalid server address to default to insecure cookie")
	}

	_ = os.Setenv("SESSION_COOKIE_SECURE", "true")
	if !ShouldUseSecureSessionCookie("http://example.com") {
		t.Fatal("expected env override to force secure cookie")
	}

	_ = os.Setenv("SESSION_COOKIE_SECURE", "false")
	if ShouldUseSecureSessionCookie("https://example.com") {
		t.Fatal("expected env override to disable secure cookie")
	}
}
