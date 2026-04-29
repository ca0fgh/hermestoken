package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

func TestMarketplaceUIAPIRoutesAreRegisteredBeforeAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-session-secret"))))
	SetApiRouter(r)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/marketplace/pricing"},
		{http.MethodGet, "/api/marketplace/order-filter-ranges"},
		{http.MethodGet, "/api/marketplace/orders"},
		{http.MethodPost, "/api/marketplace/fixed-orders"},
		{http.MethodGet, "/api/marketplace/fixed-orders"},
		{http.MethodPost, "/api/marketplace/fixed-orders/bind-token"},
		{http.MethodGet, "/api/marketplace/fixed-orders/1"},
		{http.MethodPost, "/api/marketplace/fixed-orders/1/bind-token"},
		{http.MethodPost, "/api/marketplace/fixed-orders/1/bind-tokens"},
		{http.MethodGet, "/api/marketplace/pool/models"},
		{http.MethodGet, "/api/marketplace/pool/candidates"},
		{http.MethodPost, "/api/marketplace/seller/credentials"},
		{http.MethodGet, "/api/marketplace/seller/credentials"},
		{http.MethodPost, "/api/marketplace/seller/credentials/fetch-models"},
		{http.MethodGet, "/api/marketplace/seller/credentials/1"},
		{http.MethodPut, "/api/marketplace/seller/credentials/1"},
		{http.MethodPost, "/api/marketplace/seller/credentials/1/test"},
		{http.MethodPost, "/api/marketplace/seller/credentials/1/list"},
		{http.MethodPost, "/api/marketplace/seller/credentials/1/unlist"},
		{http.MethodPost, "/api/marketplace/seller/credentials/1/enable"},
		{http.MethodPost, "/api/marketplace/seller/credentials/1/disable"},
		{http.MethodGet, "/api/marketplace/seller/priced-models"},
		{http.MethodGet, "/api/marketplace/seller/income"},
		{http.MethodGet, "/api/marketplace/seller/settlements"},
		{http.MethodPost, "/api/marketplace/seller/settlements/release"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s %s returned 404; route is not registered", route.method, route.path)
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d auth gate for registered route; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
		})
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

func TestRootOpenAIRelayPathsAreRegisteredBeforeWebFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	SetRelayRouter(r)
	SetWebRouter(r, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte(`<!doctype html><html><body><div id="root"></div></body></html>`),
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5.5","input":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want relay auth gate; body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want relay JSON error instead of web fallback", got)
	}
	if strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("body = %s, want relay JSON error instead of web fallback", rec.Body.String())
	}
}

func TestRootModelsWithoutAPIKeyKeepsWebFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	SetRelayRouter(r)
	SetWebRouter(r, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte(`<!doctype html><html><body><div id="root"></div></body></html>`),
	})

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want web fallback", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "token.invalid") {
		t.Fatalf("body = %s, want web fallback without relay auth gate", rec.Body.String())
	}
}

func TestRootModelsWithAPIKeyUsesRelayAuthGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupWebRouterTokenAuthTestDB(t)

	r := gin.New()
	SetRelayRouter(r)
	SetWebRouter(r, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte(`<!doctype html><html><body><div id="root"></div></body></html>`),
	})

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	req.Header.Set("Authorization", "Bearer sk-invalid")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want relay auth gate; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("body = %s, want relay JSON error instead of web fallback", rec.Body.String())
	}
}

func TestRootModelDetailWithAPIKeyUsesRelayAuthGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupWebRouterTokenAuthTestDB(t)

	r := gin.New()
	SetRelayRouter(r)
	SetWebRouter(r, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte(`<!doctype html><html><body><div id="root"></div></body></html>`),
	})

	req := httptest.NewRequest(http.MethodGet, "/models/gpt-5.5", nil)
	req.Header.Set("Authorization", "Bearer sk-invalid")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want relay auth gate; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("body = %s, want relay JSON error instead of web fallback", rec.Body.String())
	}
}

func setupWebRouterTokenAuthTestDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedis := common.RedisEnabled
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	model.InitColumnMetadata()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.Token{}, &model.User{}); err != nil {
		t.Fatalf("failed to migrate sqlite db: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedis
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
	})
}
