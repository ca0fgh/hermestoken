package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/console", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/console", nil)
	router.ServeHTTP(recorder, request)

	headers := recorder.Header()
	if got := headers.Get("Strict-Transport-Security"); got != defaultHSTS {
		t.Fatalf("expected HSTS header, got %q", got)
	}
	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}
	if got := headers.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected DENY frame header, got %q", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Fatalf("expected strict referrer policy, got %q", got)
	}
	if got := headers.Get("Permissions-Policy"); got != "camera=(), microphone=(), geolocation=()" {
		t.Fatalf("expected permissions policy, got %q", got)
	}

	csp := headers.Get("Content-Security-Policy")
	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' https:",
		"frame-ancestors 'none'",
		"form-action 'self'",
	} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("expected CSP to contain %q, got %q", directive, csp)
		}
	}
}

func TestSecurityHeadersDoNotOverrideExistingResponseHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	})
	router.Use(SecurityHeaders())
	router.GET("/embedded", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/embedded", nil)
	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("expected existing X-Frame-Options to be preserved, got %q", got)
	}
}
