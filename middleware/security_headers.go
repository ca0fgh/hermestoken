package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	defaultHSTS = "max-age=31536000"
	defaultCSP  = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https:; style-src 'self' 'unsafe-inline' https:; img-src 'self' data: blob: https:; font-src 'self' data: https:; connect-src 'self' https: wss:; frame-src https: blob: data:; child-src https: blob: data:; media-src 'self' data: blob: https:; worker-src 'self' blob:; form-action 'self'"
)

// SecurityHeaders sets a conservative baseline for browser-facing responses.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		setIfMissing(header, "Strict-Transport-Security", defaultHSTS)
		setIfMissing(header, "Content-Security-Policy", defaultCSP)
		setIfMissing(header, "X-Content-Type-Options", "nosniff")
		setIfMissing(header, "X-Frame-Options", "DENY")
		setIfMissing(header, "Referrer-Policy", "strict-origin-when-cross-origin")
		setIfMissing(header, "Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

func setIfMissing(header http.Header, key, value string) {
	if existing := header.Get(key); existing != "" {
		return
	}
	header.Set(key, value)
}
