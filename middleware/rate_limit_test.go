package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestShouldBypassGlobalWebRateLimit(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		requestPath string
		expected    bool
	}{
		{
			name:        "root html request stays rate limited",
			requestPath: "/",
			expected:    false,
		},
		{
			name:        "assets directory is bypassed",
			requestPath: "/assets/index.js",
			expected:    true,
		},
		{
			name:        "favicon is bypassed",
			requestPath: "/favicon.ico",
			expected:    true,
		},
		{
			name:        "nested html route stays rate limited",
			requestPath: "/console/setting",
			expected:    false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if actual := shouldBypassGlobalWebRateLimit(testCase.requestPath); actual != testCase.expected {
				t.Fatalf("expected bypass=%t, got %t for %q", testCase.expected, actual, testCase.requestPath)
			}
		})
	}
}

func TestGlobalWebRateLimitBypassesStaticAssetsButStillLimitsPages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldEnable := common.GlobalWebRateLimitEnable
	oldNum := common.GlobalWebRateLimitNum
	oldDuration := common.GlobalWebRateLimitDuration
	oldRedisEnabled := common.RedisEnabled
	oldLimiter := inMemoryRateLimiter
	defer func() {
		common.GlobalWebRateLimitEnable = oldEnable
		common.GlobalWebRateLimitNum = oldNum
		common.GlobalWebRateLimitDuration = oldDuration
		common.RedisEnabled = oldRedisEnabled
		inMemoryRateLimiter = oldLimiter
	}()

	common.GlobalWebRateLimitEnable = true
	common.GlobalWebRateLimitNum = 1
	common.GlobalWebRateLimitDuration = 60
	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	router := gin.New()
	router.Use(GlobalWebRateLimit())
	router.GET("/assets/index.js", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/console/setting", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	staticRequest1 := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	staticRequest1.RemoteAddr = "203.0.113.10:1234"
	staticRecorder1 := httptest.NewRecorder()
	router.ServeHTTP(staticRecorder1, staticRequest1)
	if staticRecorder1.Code != http.StatusOK {
		t.Fatalf("expected first static asset request to pass, got %d", staticRecorder1.Code)
	}

	staticRequest2 := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	staticRequest2.RemoteAddr = "203.0.113.10:1234"
	staticRecorder2 := httptest.NewRecorder()
	router.ServeHTTP(staticRecorder2, staticRequest2)
	if staticRecorder2.Code != http.StatusOK {
		t.Fatalf("expected repeated static asset request to bypass rate limit, got %d", staticRecorder2.Code)
	}

	pageRequest1 := httptest.NewRequest(http.MethodGet, "/console/setting", nil)
	pageRequest1.RemoteAddr = "198.51.100.25:5678"
	pageRecorder1 := httptest.NewRecorder()
	router.ServeHTTP(pageRecorder1, pageRequest1)
	if pageRecorder1.Code != http.StatusOK {
		t.Fatalf("expected first page request to pass, got %d", pageRecorder1.Code)
	}

	pageRequest2 := httptest.NewRequest(http.MethodGet, "/console/setting", nil)
	pageRequest2.RemoteAddr = "198.51.100.25:5678"
	pageRecorder2 := httptest.NewRecorder()
	router.ServeHTTP(pageRecorder2, pageRequest2)
	if pageRecorder2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated page request to be rate limited, got %d", pageRecorder2.Code)
	}
}

func TestGlobalAPIRateLimitUsesAuthenticatedUserIdentityBeforeIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldEnable := common.GlobalApiRateLimitEnable
	oldNum := common.GlobalApiRateLimitNum
	oldDuration := common.GlobalApiRateLimitDuration
	oldRedisEnabled := common.RedisEnabled
	oldLimiter := inMemoryRateLimiter
	defer func() {
		common.GlobalApiRateLimitEnable = oldEnable
		common.GlobalApiRateLimitNum = oldNum
		common.GlobalApiRateLimitDuration = oldDuration
		common.RedisEnabled = oldRedisEnabled
		inMemoryRateLimiter = oldLimiter
	}()

	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	router := gin.New()
	store := cookie.NewStore([]byte("test-session-secret"))
	router.Use(sessions.Sessions("session", store))
	router.Use(func(c *gin.Context) {
		if raw := c.GetHeader("X-Debug-User-ID"); raw != "" {
			if id, err := strconv.Atoi(raw); err == nil {
				session := sessions.Default(c)
				session.Set("id", id)
				_ = session.Save()
			}
		}
		c.Next()
	})
	router.Use(GlobalAPIRateLimit())
	router.GET("/api/demo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/api/demo", nil)
	req1.RemoteAddr = "198.51.100.25:5678"
	req1.Header.Set("X-Debug-User-ID", "101")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first authenticated request to pass, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/demo", nil)
	req2.RemoteAddr = "198.51.100.25:5678"
	req2.Header.Set("X-Debug-User-ID", "202")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected second authenticated user from same IP to use separate bucket, got %d", rec2.Code)
	}
}

func TestGlobalAPIRateLimitStillFallsBackToIPForAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldEnable := common.GlobalApiRateLimitEnable
	oldNum := common.GlobalApiRateLimitNum
	oldDuration := common.GlobalApiRateLimitDuration
	oldRedisEnabled := common.RedisEnabled
	oldLimiter := inMemoryRateLimiter
	defer func() {
		common.GlobalApiRateLimitEnable = oldEnable
		common.GlobalApiRateLimitNum = oldNum
		common.GlobalApiRateLimitDuration = oldDuration
		common.RedisEnabled = oldRedisEnabled
		inMemoryRateLimiter = oldLimiter
	}()

	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}

	router := gin.New()
	store := cookie.NewStore([]byte("test-session-secret"))
	router.Use(sessions.Sessions("session", store))
	router.Use(GlobalAPIRateLimit())
	router.GET("/api/demo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/api/demo", nil)
	req1.RemoteAddr = "203.0.113.10:1234"
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first anonymous request to pass, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/demo", nil)
	req2.RemoteAddr = "203.0.113.10:1234"
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected anonymous requests from same IP to still share limit bucket, got %d", rec2.Code)
	}
}
