package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func performCacheRequest(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Cache())
	router.GET("/*path", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	router.ServeHTTP(recorder, request)

	return recorder
}

func TestCacheRootUsesNoCache(t *testing.T) {
	t.Parallel()

	recorder := performCacheRequest(t, "/")

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("expected root cache-control to be no-cache, got %q", cacheControl)
	}
}

func TestCacheVersionedAssetsAreImmutable(t *testing.T) {
	t.Parallel()

	recorder := performCacheRequest(t, "/assets/index-Y-fqA6pr.js")

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("expected versioned assets to be immutable, got %q", cacheControl)
	}
}

func TestCacheNonVersionedAssetsStayShortLived(t *testing.T) {
	t.Parallel()

	recorder := performCacheRequest(t, "/assets/logo.png")

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "max-age=604800" {
		t.Fatalf("expected non-versioned assets to keep weekly cache, got %q", cacheControl)
	}
}
