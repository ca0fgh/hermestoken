package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublicBootstrapEndpointReturnsCacheableJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	SetApiRouter(r)

	req := httptest.NewRequest(http.MethodGet, "/api/public/bootstrap", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60, stale-while-revalidate=300" {
		t.Fatalf("Cache-Control = %q, want cacheable public bootstrap response", got)
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("body = %s, want success payload", rec.Body.String())
	}
}

func TestInternalPublicHomeEndpointReturnsNoCacheHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	SetWebRouter(r, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`),
	})

	req := httptest.NewRequest(http.MethodGet, "/__internal/public-home", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if !strings.Contains(rec.Body.String(), `id="hermes-public-bootstrap"`) {
		t.Fatalf("body = %s, want public bootstrap script tag", rec.Body.String())
	}
}
