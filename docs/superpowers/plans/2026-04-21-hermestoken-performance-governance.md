# HermesToken Performance Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild HermesToken startup so `/` paints meaningful content from HTML immediately, `/console` loads from a separate lighter startup path, and production delivery enforces cache/compression/bundle budgets that stop the current 9-second white-screen behavior from coming back.

**Architecture:** Add a backend-generated public bootstrap that powers a dynamic root document and a minimal `/api/public/bootstrap` refresh path, while splitting public and console bootstraps in the React app. Route `/` through a dedicated internal backend HTML endpoint, keep versioned assets static and immutable, and tighten Vite chunking plus post-build budget checks so public startup cannot silently absorb console-only dependencies again.

**Tech Stack:** Go, Gin, GORM, goldmark, React 18, Vite 5, node:test, Python deployment scripts, Nginx.

---

## File Structure / Responsibility Map

- `controller/public_bootstrap.go`
  - Build the public-safe status snapshot, render the public bootstrap payload, render the root HTML shell, and expose handlers for `/api/public/bootstrap` and the internal dynamic root HTML endpoint.
- `controller/public_bootstrap_test.go`
  - TDD coverage for bootstrap payload shape, markdown/iframe handling, root-shell HTML injection, and cache headers.
- `router/api-router.go`
  - Register `/api/public/bootstrap` as the public refresh endpoint.
- `router/web-router.go`
  - Register the internal dynamic root HTML endpoint that Nginx proxies `/` to.
- `router/web_router_test.go`
  - Verify the internal dynamic root endpoint and the public bootstrap API return the expected content types and cache headers.
- `web/src/helpers/bootstrapData.js`
  - Read injected bootstrap JSON, normalize cached bootstrap content, and expose client preference helpers for theme/language startup.
- `web/src/helpers/publicStartupCache.js`
  - Cache and load public bootstrap payloads with TTL-based fallback.
- `web/src/helpers/idleTask.js`
  - Schedule non-critical follow-up work with `requestIdleCallback` and a `setTimeout` fallback.
- `web/src/i18n/i18n.js`
  - Let i18n initialize with an optional preferred-language override without blocking the first render.
- `web/src/index.jsx`
  - Render immediately, choose public vs console bootstrap path, and stop waiting for `initializeI18n()` before mounting.
- `web/src/context/Status/index.jsx`
  - Accept an initial status payload so public routes can start from injected data instead of a post-render fetch.
- `web/src/bootstrap/publicApp.jsx`
  - Minimal public startup root using injected/cached bootstrap data.
- `web/src/bootstrap/consoleApp.jsx`
  - Full console startup root with the existing authenticated shell behavior.
- `web/src/components/layout/PageLayout.jsx`
  - Stop eagerly loading `/api/status` for the public startup path and only fetch the full status when a console route or a missing bootstrap requires it.
- `web/src/pages/Home/index.jsx`
  - Prefer injected HTML/bootstrap content, move notice/home refresh off the critical path, and stop dynamically importing `marked` on startup.
- `web/index.html`
  - Add a tiny inline client-preference script that applies saved theme and language hints before the module graph boots.
- `web/vite.config.js`
  - Create explicit startup/public/console/heavy-runtime chunk boundaries, emit a manifest, and filter non-critical preloads out of the public entry graph.
- `web/scripts/check-public-startup-budget.mjs`
  - Read the build manifest and fail when the public startup entry breaks request-count or gzip-size budgets.
- `web/tests/bootstrap-data.test.mjs`
  - Unit tests for bootstrap parsing, caching, and startup preference normalization.
- `web/tests/public-startup-entry.test.mjs`
  - Source/assertion tests that protect the non-blocking root render and the public-vs-console bootstrap split.
- `web/tests/home-bootstrap.test.mjs`
  - Protect the home page bootstrap-first flow and the removal of startup-critical markdown parsing.
- `web/tests/public-startup-budget.test.mjs`
  - Unit tests for the budget evaluator script.
- `web/package.json`
  - Add scripts for the new startup tests and budget checks.
- `scripts/prod.py`
  - Detect optional Brotli support, proxy `/` to the internal dynamic root endpoint, and keep generated Nginx config in sync with the new delivery strategy.
- `scripts/tests/test_prod.py`
  - Verify the generated Nginx config uses the root proxy, serves assets immutably, and only enables Brotli when supported.
- `hermestoken.top.nginx.conf`
  - Mirror the generated production config so the checked-in sample matches the deployed behavior.

### Task 1: Add the Backend Public Bootstrap Builder and Root HTML Renderer

**Files:**
- Create: `controller/public_bootstrap.go`
- Create: `controller/public_bootstrap_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Test: `controller/public_bootstrap_test.go`

- [ ] **Step 1: Write the failing backend bootstrap tests**

```go
package controller

import (
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
)

func TestBuildPublicBootstrapPayloadReturnsPublicSubset(t *testing.T) {
	common.SystemName = "HermesToken"
	common.Footer = "<p>footer</p>"
	common.OptionMap = map[string]string{
		"HeaderNavModules": `{"home":true,"pricing":{"enabled":true,"requireAuth":false}}`,
		"HomePageContent": "# Launch faster",
		"Notice":          "Scheduled maintenance tonight",
	}
	constant.Setup = true

	payload, err := BuildPublicBootstrapPayload()
	if err != nil {
		t.Fatalf("BuildPublicBootstrapPayload() error = %v", err)
	}
	if payload.Status.SystemName != "HermesToken" {
		t.Fatalf("SystemName = %q, want HermesToken", payload.Status.SystemName)
	}
	if payload.Status.GitHubOAuth {
		t.Fatal("public bootstrap must not leak console-only OAuth toggles")
	}
	if payload.Home.Mode != PublicHomeModeHTML {
		t.Fatalf("Home.Mode = %q, want %q", payload.Home.Mode, PublicHomeModeHTML)
	}
	if !strings.Contains(payload.Home.HTML, "<h1") {
		t.Fatalf("expected markdown HTML, got %q", payload.Home.HTML)
	}
	if payload.Notice.Markdown != "Scheduled maintenance tonight" {
		t.Fatalf("Notice.Markdown = %q", payload.Notice.Markdown)
	}
}

func TestRenderPublicHomeIndexEmbedsBootstrapAndShell(t *testing.T) {
	payload := PublicBootstrapPayload{
		Status: PublicStatusSnapshot{SystemName: "HermesToken", Setup: true},
		Home: PublicHomeSnapshot{
			Mode: PublicHomeModeHTML,
			HTML: `<section class="hero"><h1>Fast path</h1></section>`,
		},
	}

	page, err := RenderPublicHomeIndex([]byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`), payload)
	if err != nil {
		t.Fatalf("RenderPublicHomeIndex() error = %v", err)
	}
	if !strings.Contains(string(page), `id="hermes-public-bootstrap"`) {
		t.Fatalf("expected bootstrap script tag in %s", string(page))
	}
	if !strings.Contains(string(page), `<div id="root"><section class="hero"><h1>Fast path</h1></section></div>`) {
		t.Fatalf("expected prerendered shell in %s", string(page))
	}
}
```

- [ ] **Step 2: Run the backend bootstrap tests to verify they fail**

Run: `go test ./controller -run 'TestBuildPublicBootstrapPayloadReturnsPublicSubset|TestRenderPublicHomeIndexEmbedsBootstrapAndShell'`

Expected: FAIL because `BuildPublicBootstrapPayload` and `RenderPublicHomeIndex` do not exist yet.

- [ ] **Step 3: Implement the backend public bootstrap builder and renderer**

```go
// controller/public_bootstrap.go
package controller

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/ca0fgh/hermestoken/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
)

const (
	PublicHomeModeDefault = "default"
	PublicHomeModeHTML    = "html"
	PublicHomeModeIframe  = "iframe"
)

type PublicStatusSnapshot struct {
	SystemName       string `json:"system_name"`
	Logo             string `json:"logo"`
	FooterHTML       string `json:"footer_html,omitempty"`
	DocsLink         string `json:"docs_link,omitempty"`
	HeaderNavModules string `json:"HeaderNavModules,omitempty"`
	ServerAddress    string `json:"server_address"`
	Setup            bool   `json:"setup"`
	Version          string `json:"version"`
	GitHubOAuth      bool   `json:"-"`
}

type PublicHomeSnapshot struct {
	Mode     string `json:"mode"`
	HTML     string `json:"html,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	URL      string `json:"url,omitempty"`
}

type PublicNoticeSnapshot struct {
	Markdown string `json:"markdown,omitempty"`
}

type PublicBootstrapPayload struct {
	Status PublicStatusSnapshot `json:"status"`
	Home   PublicHomeSnapshot   `json:"home"`
	Notice PublicNoticeSnapshot `json:"notice"`
}

func BuildPublicBootstrapPayload() (PublicBootstrapPayload, error) {
	common.OptionMapRWMutex.RLock()
	homeMarkdown := common.OptionMap["HomePageContent"]
	noticeMarkdown := common.OptionMap["Notice"]
	headerNavModules := common.OptionMap["HeaderNavModules"]
	common.OptionMapRWMutex.RUnlock()

	payload := PublicBootstrapPayload{
		Status: PublicStatusSnapshot{
			SystemName:       common.SystemName,
			Logo:             resolveLogoOptionValue(),
			FooterHTML:       common.Footer,
			DocsLink:         operation_setting.GetGeneralSetting().DocsLink,
			HeaderNavModules: headerNavModules,
			ServerAddress:    system_setting.ServerAddress,
			Setup:            constant.Setup,
			Version:          common.Version,
		},
		Notice: PublicNoticeSnapshot{Markdown: noticeMarkdown},
	}

	switch {
	case strings.HasPrefix(strings.TrimSpace(homeMarkdown), "https://"):
		payload.Home = PublicHomeSnapshot{Mode: PublicHomeModeIframe, URL: strings.TrimSpace(homeMarkdown)}
	case strings.TrimSpace(homeMarkdown) != "":
		var rendered bytes.Buffer
		if err := goldmark.Convert([]byte(homeMarkdown), &rendered); err != nil {
			return PublicBootstrapPayload{}, err
		}
		payload.Home = PublicHomeSnapshot{
			Mode:     PublicHomeModeHTML,
			HTML:     rendered.String(),
			Markdown: homeMarkdown,
		}
	default:
		payload.Home = PublicHomeSnapshot{Mode: PublicHomeModeDefault}
	}

	return payload, nil
}

func renderPublicHomeShell(payload PublicBootstrapPayload) string {
	switch payload.Home.Mode {
	case PublicHomeModeIframe:
		return fmt.Sprintf(`<iframe class="w-full h-screen border-none" src="%s"></iframe>`, template.HTMLEscapeString(payload.Home.URL))
	case PublicHomeModeHTML:
		return payload.Home.HTML
	default:
		return `<section class="hermes-home-fallback"><h1>HERMESTOKEN</h1><p>LLM Token usage-rights infrastructure</p></section>`
	}
}

func RenderPublicHomeIndex(baseIndex []byte, payload PublicBootstrapPayload) ([]byte, error) {
	payloadJSON, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	page := append([]byte(nil), baseIndex...)
	bootstrapScript := []byte(`<script id="hermes-public-bootstrap" type="application/json">` + string(payloadJSON) + `</script>`)
	page = bytes.Replace(page, []byte("</head>"), append(bootstrapScript, []byte("</head>")...), 1)
	page = bytes.Replace(page, []byte(`<div id="root"></div>`), []byte(`<div id="root">`+renderPublicHomeShell(payload)+`</div>`), 1)
	return page, nil
}

func GetPublicBootstrap(c *gin.Context) {
	payload, err := BuildPublicBootstrapPayload()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": payload})
}

func PublicHomeIndexHandler(baseIndex []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload, err := BuildPublicBootstrapPayload()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		page, err := RenderPublicHomeIndex(baseIndex, payload)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	}
}
```

Run: `go get github.com/yuin/goldmark@v1.7.8`

- [ ] **Step 4: Run the backend bootstrap tests to verify they pass**

Run: `go test ./controller -run 'TestBuildPublicBootstrapPayloadReturnsPublicSubset|TestRenderPublicHomeIndexEmbedsBootstrapAndShell'`

Expected: PASS.

- [ ] **Step 5: Commit the backend bootstrap builder**

```bash
git add controller/public_bootstrap.go controller/public_bootstrap_test.go go.mod go.sum
git commit -m "feat: add public bootstrap renderer"
```

### Task 2: Expose the Public Bootstrap API and Internal Dynamic Root Endpoint

**Files:**
- Modify: `router/api-router.go`
- Modify: `router/web-router.go`
- Create: `router/web_router_test.go`
- Test: `router/web_router_test.go`

- [ ] **Step 1: Write the failing router tests**

```go
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

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/bootstrap", nil)
	r.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "public, max-age=60, stale-while-revalidate=300" {
		t.Fatalf("Cache-Control = %q", cacheControl)
	}
	if !strings.Contains(recorder.Body.String(), `"success":true`) {
		t.Fatalf("expected success payload, got %s", recorder.Body.String())
	}
}

func TestInternalPublicHomeEndpointReturnsNoCacheHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetWebRouter(r, embed.FS{}, []byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/__internal/public-home", nil)
	r.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("Cache-Control = %q", cacheControl)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), `id="hermes-public-bootstrap"`) {
		t.Fatalf("expected injected bootstrap HTML, got %s", recorder.Body.String())
	}
}
```

- [ ] **Step 2: Run the router tests to verify they fail**

Run: `go test ./router -run 'TestPublicBootstrapEndpointReturnsCacheableJSON|TestInternalPublicHomeEndpointReturnsNoCacheHTML'`

Expected: FAIL because `/api/public/bootstrap` and `/__internal/public-home` are not registered yet.

- [ ] **Step 3: Register the new public bootstrap routes**

```go
// router/api-router.go
func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup())
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/public/bootstrap", controller.GetPublicBootstrap)
		apiRouter.GET("/logo", controller.GetLogo)
	}
}

// router/web-router.go
func SetWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.GET("/__internal/public-home", controller.PublicHomeIndexHandler(indexPage))
	router.Use(static.Serve("/", common.EmbedFolder(buildFS, "web/dist")))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})
}
```

- [ ] **Step 4: Run the router tests to verify they pass**

Run: `go test ./router -run 'TestPublicBootstrapEndpointReturnsCacheableJSON|TestInternalPublicHomeEndpointReturnsNoCacheHTML'`

Expected: PASS.

- [ ] **Step 5: Commit the new public bootstrap routes**

```bash
git add router/api-router.go router/web-router.go router/web_router_test.go
git commit -m "feat: add public bootstrap routes"
```

### Task 3: Add Frontend Bootstrap Parsing, Caching, and Non-Blocking Startup

**Files:**
- Create: `web/src/helpers/bootstrapData.js`
- Create: `web/src/helpers/publicStartupCache.js`
- Modify: `web/src/i18n/i18n.js`
- Modify: `web/src/index.jsx`
- Modify: `web/index.html`
- Create: `web/tests/bootstrap-data.test.mjs`
- Create: `web/tests/public-startup-entry.test.mjs`
- Test: `web/tests/bootstrap-data.test.mjs`
- Test: `web/tests/public-startup-entry.test.mjs`

- [ ] **Step 1: Write the failing frontend startup helper tests**

```js
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  parsePublicBootstrapJson,
  readClientStartupSettings,
} from '../src/helpers/bootstrapData.js';

const indexEntryPath = new URL('../src/index.jsx', import.meta.url);
const templatePath = new URL('../index.html', import.meta.url);

test('parsePublicBootstrapJson returns null for empty or malformed payloads', () => {
  assert.equal(parsePublicBootstrapJson(''), null);
  assert.equal(parsePublicBootstrapJson('{not-json'), null);
});

test('readClientStartupSettings normalizes saved theme and language', () => {
  const fakeStorage = {
    getItem(key) {
      return {
        'theme-mode': 'dark',
        i18nextLng: 'en_US',
      }[key] ?? null;
    },
  };

  assert.deepEqual(readClientStartupSettings(fakeStorage), {
    themeMode: 'dark',
    language: 'en',
  });
});

test('index entry renders immediately instead of waiting for initializeI18n finally', async () => {
  const source = await readFile(indexEntryPath, 'utf8');

  assert.doesNotMatch(source, /initializeI18n\(\)\.catch\(console\.error\)\.finally\(renderApp\)/);
  assert.match(source, /renderPublicApp\(/);
  assert.match(source, /renderConsoleApp\(/);
});

test('index template primes startup preferences before module boot', async () => {
  const source = await readFile(templatePath, 'utf8');

  assert.match(source, /window\.__HERMES_CLIENT_PREFS__/);
  assert.match(source, /theme-mode/);
});
```

- [ ] **Step 2: Run the frontend startup helper tests to verify they fail**

Run: `cd web && node --test tests/bootstrap-data.test.mjs tests/public-startup-entry.test.mjs`

Expected: FAIL because the helper module, immediate render path, and inline preference primer do not exist yet.

- [ ] **Step 3: Implement bootstrap parsing, caching, and non-blocking entry startup**

```js
// web/src/helpers/bootstrapData.js
import { normalizeLanguage } from '../i18n/language';

export const PUBLIC_BOOTSTRAP_SCRIPT_ID = 'hermes-public-bootstrap';

export function parsePublicBootstrapJson(rawValue) {
  if (!rawValue || !rawValue.trim()) {
    return null;
  }

  try {
    return JSON.parse(rawValue);
  } catch {
    return null;
  }
}

export function readInjectedBootstrap(doc = globalThis.document) {
  const element = doc?.getElementById?.(PUBLIC_BOOTSTRAP_SCRIPT_ID);
  return parsePublicBootstrapJson(element?.textContent || '');
}

export function readClientStartupSettings(storage = globalThis.localStorage) {
  const themeMode = storage?.getItem?.('theme-mode') || 'auto';
  const language = normalizeLanguage(storage?.getItem?.('i18nextLng')) || 'zh-CN';
  return { themeMode, language };
}

// web/src/helpers/publicStartupCache.js
import { parsePublicBootstrapJson } from './bootstrapData';

const PUBLIC_BOOTSTRAP_CACHE_KEY = 'hermes-public-bootstrap-v1';
const PUBLIC_BOOTSTRAP_MAX_AGE_MS = 15 * 60 * 1000;

export function cachePublicBootstrap(payload, storage = globalThis.localStorage) {
  if (!payload) return;
  storage?.setItem?.(
    PUBLIC_BOOTSTRAP_CACHE_KEY,
    JSON.stringify({ savedAt: Date.now(), payload }),
  );
}

export function readCachedPublicBootstrap(storage = globalThis.localStorage) {
  const rawValue = storage?.getItem?.(PUBLIC_BOOTSTRAP_CACHE_KEY);
  const parsed = parsePublicBootstrapJson(rawValue);
  if (!parsed?.savedAt || !parsed?.payload) {
    return null;
  }
  if (Date.now() - parsed.savedAt > PUBLIC_BOOTSTRAP_MAX_AGE_MS) {
    return null;
  }
  return parsed.payload;
}

// web/src/i18n/i18n.js
export async function initializeI18n(preferredLanguageOverride) {
  const preferredLanguage =
    normalizeLanguage(preferredLanguageOverride) ||
    getPreferredLanguage() ||
    normalizeLanguage(i18n.resolvedLanguage || i18n.language) ||
    DEFAULT_LANGUAGE;

  await ensureLanguageResources(preferredLanguage);
  if (preferredLanguage !== i18n.language) {
    await i18n.changeLanguage(preferredLanguage);
  }
  return i18n;
}

// web/src/index.jsx
import { renderConsoleApp } from './bootstrap/consoleApp';
import { renderPublicApp } from './bootstrap/publicApp';
import { readInjectedBootstrap, readClientStartupSettings } from './helpers/bootstrapData';
import { cachePublicBootstrap } from './helpers/publicStartupCache';

const rootElement = document.getElementById('root');
const injectedBootstrap = readInjectedBootstrap();
const startupSettings = readClientStartupSettings();

cachePublicBootstrap(injectedBootstrap);

if (window.location.pathname.startsWith('/console')) {
  renderConsoleApp(rootElement);
} else {
  renderPublicApp(rootElement, injectedBootstrap);
}

void initializeI18n(startupSettings.language).catch(console.error);
```

```html
<!-- web/index.html -->
<script>
  (function () {
    try {
      const themeMode = localStorage.getItem('theme-mode') || 'auto';
      const language = localStorage.getItem('i18nextLng') || 'zh-CN';
      window.__HERMES_CLIENT_PREFS__ = { themeMode, language };
      if (themeMode === 'dark') {
        document.documentElement.classList.add('dark');
      }
    } catch {
      window.__HERMES_CLIENT_PREFS__ = { themeMode: 'auto', language: 'zh-CN' };
    }
  })();
</script>
```

- [ ] **Step 4: Run the frontend startup helper tests to verify they pass**

Run: `cd web && node --test tests/bootstrap-data.test.mjs tests/public-startup-entry.test.mjs`

Expected: PASS.

- [ ] **Step 5: Commit the frontend startup helper foundation**

```bash
git add web/src/helpers/bootstrapData.js web/src/helpers/publicStartupCache.js web/src/i18n/i18n.js web/src/index.jsx web/index.html web/tests/bootstrap-data.test.mjs web/tests/public-startup-entry.test.mjs
git commit -m "feat: add non-blocking public startup helpers"
```

### Task 4: Split Public and Console Bootstraps and Stop Public Eager Status Fetches

**Files:**
- Create: `web/src/bootstrap/publicApp.jsx`
- Create: `web/src/bootstrap/consoleApp.jsx`
- Modify: `web/src/context/Status/index.jsx`
- Modify: `web/src/components/layout/PageLayout.jsx`
- Create: `web/tests/public-layout-mode.test.mjs`
- Test: `web/tests/public-layout-mode.test.mjs`

- [ ] **Step 1: Write the failing bootstrap-split tests**

```js
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const publicAppPath = new URL('../src/bootstrap/publicApp.jsx', import.meta.url);
const consoleAppPath = new URL('../src/bootstrap/consoleApp.jsx', import.meta.url);
const pageLayoutPath = new URL('../src/components/layout/PageLayout.jsx', import.meta.url);
const statusProviderPath = new URL('../src/context/Status/index.jsx', import.meta.url);

test('status provider accepts an initial status payload', async () => {
  const source = await readFile(statusProviderPath, 'utf8');

  assert.match(source, /export const StatusProvider = \(\{ children, initialStatus/);
});

test('page layout only fetches full status for console startup or missing bootstrap', async () => {
  const source = await readFile(pageLayoutPath, 'utf8');

  assert.match(source, /startupMode === 'console'/);
  assert.match(source, /if \(!shouldFetchFullStatus\) \{\s*return;/);
});

test('public and console bootstraps are separate files', async () => {
  const [publicSource, consoleSource] = await Promise.all([
    readFile(publicAppPath, 'utf8'),
    readFile(consoleAppPath, 'utf8'),
  ]);

  assert.match(publicSource, /startupMode='public'/);
  assert.match(consoleSource, /startupMode='console'/);
});
```

- [ ] **Step 2: Run the bootstrap-split tests to verify they fail**

Run: `cd web && node --test tests/public-layout-mode.test.mjs`

Expected: FAIL because the split bootstraps and `initialStatus` support do not exist yet.

- [ ] **Step 3: Implement separate public/console app roots and conditional full-status loading**

```jsx
// web/src/context/Status/index.jsx
export const StatusProvider = ({ children, initialStatus }) => {
  const [state, dispatch] = React.useReducer(reducer, {
    ...initialState,
    status: initialStatus ?? initialState.status,
  });

  return (
    <StatusContext.Provider value={[state, dispatch]}>
      {children}
    </StatusContext.Provider>
  );
};

// web/src/bootstrap/publicApp.jsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { StatusProvider } from '../context/Status';
import { ThemeProvider } from '../context/Theme';
import { UserProvider } from '../context/User';
import { readCachedPublicBootstrap } from '../helpers/publicStartupCache';
import PageLayout from '../components/layout/PageLayout';

export function renderPublicApp(rootElement, injectedBootstrap) {
  const bootstrap = injectedBootstrap || readCachedPublicBootstrap();
  const root = ReactDOM.createRoot(rootElement);
  root.render(
    <React.StrictMode>
      <StatusProvider initialStatus={bootstrap?.status}>
        <UserProvider>
          <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
            <ThemeProvider>
              <PageLayout startupMode='public' />
            </ThemeProvider>
          </BrowserRouter>
        </UserProvider>
      </StatusProvider>
    </React.StrictMode>,
  );
}

// web/src/bootstrap/consoleApp.jsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { StatusProvider } from '../context/Status';
import { ThemeProvider } from '../context/Theme';
import { UserProvider } from '../context/User';
import PageLayout from '../components/layout/PageLayout';

export function renderConsoleApp(rootElement) {
  const root = ReactDOM.createRoot(rootElement);
  root.render(
    <React.StrictMode>
      <StatusProvider>
        <UserProvider>
          <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
            <ThemeProvider>
              <PageLayout startupMode='console' />
            </ThemeProvider>
          </BrowserRouter>
        </UserProvider>
      </StatusProvider>
    </React.StrictMode>,
  );
}

// web/src/components/layout/PageLayout.jsx
const PageLayout = ({ startupMode = 'console' }) => {
  const [statusState, statusDispatch] = useContext(StatusContext);
  const isConsoleRoute = location.pathname.startsWith('/console');
  const shouldFetchFullStatus = startupMode === 'console' || isConsoleRoute || !statusState?.status;

  useEffect(() => {
    if (!shouldFetchFullStatus) {
      return;
    }

    const loadStatus = async () => {
      try {
        const { success, data } = await fetchStatusPayload();
        if (success) {
          statusDispatch({ type: 'set', payload: data });
          setStatusData(data);
          return;
        }
        showError('Unable to connect to server');
      } catch {
        showError('Failed to load status');
      }
    };

    void loadStatus();
  }, [shouldFetchFullStatus, statusDispatch]);
```

- [ ] **Step 4: Run the bootstrap-split tests to verify they pass**

Run: `cd web && node --test tests/public-layout-mode.test.mjs`

Expected: PASS.

- [ ] **Step 5: Commit the startup split**

```bash
git add web/src/bootstrap/publicApp.jsx web/src/bootstrap/consoleApp.jsx web/src/context/Status/index.jsx web/src/components/layout/PageLayout.jsx web/tests/public-layout-mode.test.mjs
git commit -m "feat: split public and console startup"
```

### Task 5: Rewrite the Home Page to Prefer Injected HTML and Defer Non-Critical Refreshes

**Files:**
- Create: `web/src/helpers/idleTask.js`
- Modify: `web/src/pages/Home/index.jsx`
- Create: `web/tests/home-bootstrap.test.mjs`
- Test: `web/tests/home-bootstrap.test.mjs`

- [ ] **Step 1: Write the failing home bootstrap tests**

```js
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const homePath = new URL('../src/pages/Home/index.jsx', import.meta.url);

test('home page prefers public bootstrap content before network fetches', async () => {
  const source = await readFile(homePath, 'utf8');

  assert.match(source, /readInjectedBootstrap\(\)/);
  assert.match(source, /readCachedPublicBootstrap\(\)/);
  assert.match(source, /scheduleNonCriticalWork\(/);
});

test('home page no longer parses markdown in the startup path', async () => {
  const source = await readFile(homePath, 'utf8');

  assert.doesNotMatch(source, /import\('marked'\)/);
  assert.match(source, /\/api\/public\/bootstrap/);
});
```

- [ ] **Step 2: Run the home bootstrap tests to verify they fail**

Run: `cd web && node --test tests/home-bootstrap.test.mjs`

Expected: FAIL because the current home page still imports `marked` on demand and fetches `/api/home_page_content` plus `/api/notice` directly in startup effects.

- [ ] **Step 3: Implement bootstrap-first home rendering and idle refreshes**

```js
// web/src/helpers/idleTask.js
export function scheduleNonCriticalWork(task, timeout = 250) {
  if (typeof window !== 'undefined' && typeof window.requestIdleCallback === 'function') {
    const idleId = window.requestIdleCallback(() => {
      void task();
    }, { timeout: 1500 });

    return () => window.cancelIdleCallback?.(idleId);
  }

  const timerId = window.setTimeout(() => {
    void task();
  }, timeout);

  return () => window.clearTimeout(timerId);
}

// web/src/pages/Home/index.jsx
import { readInjectedBootstrap } from '../../helpers/bootstrapData';
import { readCachedPublicBootstrap, cachePublicBootstrap } from '../../helpers/publicStartupCache';
import { scheduleNonCriticalWork } from '../../helpers/idleTask';

const startupBootstrap =
  readInjectedBootstrap() || readCachedPublicBootstrap() || null;

const Home = () => {
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(Boolean(startupBootstrap?.home));
  const [homePageContent, setHomePageContent] = useState(startupBootstrap?.home?.html || '');
  const [homePageFrameUrl, setHomePageFrameUrl] = useState(startupBootstrap?.home?.url || '');
  const [noticeVisible, setNoticeVisible] = useState(Boolean(startupBootstrap?.notice?.markdown));

  const applyBootstrap = useCallback((payload) => {
    if (!payload) return;
    cachePublicBootstrap(payload);
    if (payload.home?.mode === 'iframe') {
      setHomePageFrameUrl(payload.home.url || '');
      setHomePageContent('');
    } else {
      setHomePageFrameUrl('');
      setHomePageContent(payload.home?.html || '');
    }
    setNoticeVisible(Boolean(payload.notice?.markdown));
    setHomePageContentLoaded(true);
  }, []);

  useEffect(() => {
    if (startupBootstrap) {
      applyBootstrap(startupBootstrap);
    }
  }, [applyBootstrap]);

  useEffect(() => {
    return scheduleNonCriticalWork(async () => {
      try {
        const response = await fetch('/api/public/bootstrap', {
          headers: { 'Cache-Control': 'no-store' },
        });
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }
        const payload = await response.json();
        if (payload?.success && payload?.data) {
          applyBootstrap(payload.data);
        }
      } catch (error) {
        console.error('failed to refresh public bootstrap', error);
      }
    });
  }, [applyBootstrap]);

  const renderDefaultNarrative = () => (
    <div className='w-full border-b border-semi-color-border min-h-[500px] md:min-h-[600px] lg:min-h-[700px] relative overflow-x-hidden'>
      <div className='blur-ball blur-ball-indigo' />
      <div className='blur-ball blur-ball-teal' />
      <div className='flex items-center justify-center h-full px-4 py-20 md:py-24 lg:py-32 mt-10'>
        <div className='w-full max-w-5xl mx-auto'>
          <section className='text-center'>
            <p className='text-sm md:text-base text-semi-color-text-2 mb-4'>
              {t('LLM Token 使用权共享基础设施')}
            </p>
            <h1 className='text-3xl md:text-5xl lg:text-6xl font-bold text-semi-color-text-0 leading-tight'>
              {t('面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施')}
            </h1>
          </section>
        </div>
      </div>
    </div>
  );

  return (
    <div className='w-full overflow-x-hidden'>
      {homePageFrameUrl ? (
        <iframe ref={homepageIframeRef} src={homePageFrameUrl} onLoad={syncIframeThemeAndLanguage} className='w-full h-screen border-none' />
      ) : homePageContent ? (
        <div className='overflow-x-hidden w-full'>
          <div className='mt-[60px]' dangerouslySetInnerHTML={{ __html: homePageContent }} />
        </div>
      ) : !homePageContentLoaded ? (
        <div className='w-full border-b border-semi-color-border min-h-[500px] md:min-h-[600px] lg:min-h-[700px] relative overflow-x-hidden'>
          <div className='flex items-center justify-center h-full px-4 py-20 md:py-24 lg:py-32 mt-10'>
            <p className='text-base md:text-lg text-semi-color-text-2'>{t('加载中...')}</p>
          </div>
        </div>
      ) : (
        renderDefaultNarrative()
      )}
    </div>
  );
};
```

- [ ] **Step 4: Run the home bootstrap tests to verify they pass**

Run: `cd web && node --test tests/home-bootstrap.test.mjs`

Expected: PASS.

- [ ] **Step 5: Commit the bootstrap-first home page**

```bash
git add web/src/helpers/idleTask.js web/src/pages/Home/index.jsx web/tests/home-bootstrap.test.mjs
git commit -m "feat: make home page bootstrap-first"
```

### Task 6: Tighten Vite Chunking and Enforce Public Startup Budgets

**Files:**
- Modify: `web/vite.config.js`
- Create: `web/scripts/check-public-startup-budget.mjs`
- Create: `web/tests/public-startup-budget.test.mjs`
- Modify: `web/package.json`
- Test: `web/tests/public-startup-budget.test.mjs`

- [ ] **Step 1: Write the failing public startup budget tests**

```js
import assert from 'node:assert/strict';
import test from 'node:test';

import {
  evaluatePublicStartupBudget,
} from '../scripts/check-public-startup-budget.mjs';

test('evaluatePublicStartupBudget accepts a healthy startup graph', () => {
  const result = evaluatePublicStartupBudget({
    requestCount: 8,
    totalStartupGzipBytes: 105 * 1024,
    jsStartupGzipBytes: 95 * 1024,
  });

  assert.deepEqual(result.errors, []);
});

test('evaluatePublicStartupBudget flags oversized startup graphs', () => {
  const result = evaluatePublicStartupBudget({
    requestCount: 16,
    totalStartupGzipBytes: 210 * 1024,
    jsStartupGzipBytes: 180 * 1024,
  });

  assert.match(result.errors.join('\n'), /public startup request budget/);
  assert.match(result.errors.join('\n'), /public startup JavaScript budget/);
});
```

- [ ] **Step 2: Run the public startup budget tests to verify they fail**

Run: `cd web && node --test tests/public-startup-budget.test.mjs`

Expected: FAIL because the budget script does not exist yet.

- [ ] **Step 3: Implement the build manifest budget checker and stricter chunk rules**

```js
// web/scripts/check-public-startup-budget.mjs
import { gzipSync } from 'node:zlib';
import { readFileSync } from 'node:fs';
import path from 'node:path';

export function evaluatePublicStartupBudget({
  requestCount,
  totalStartupGzipBytes,
  jsStartupGzipBytes,
}) {
  const errors = [];

  if (requestCount > 10) {
    errors.push(`public startup request budget exceeded: ${requestCount} > 10`);
  }
  if (jsStartupGzipBytes > 120 * 1024) {
    errors.push(`public startup JavaScript budget exceeded: ${jsStartupGzipBytes} > ${120 * 1024}`);
  }
  if (totalStartupGzipBytes > 150 * 1024) {
    errors.push(`public startup total startup budget exceeded: ${totalStartupGzipBytes} > ${150 * 1024}`);
  }

  return { errors };
}

function gzipSizeOf(filePath) {
  return gzipSync(readFileSync(filePath)).length;
}

function loadManifest(distDir) {
  return JSON.parse(readFileSync(path.join(distDir, '.vite', 'manifest.json'), 'utf8'));
}

function collectPublicStartupBudget(distDir) {
  const manifest = loadManifest(distDir);
  const html = readFileSync(path.join(distDir, 'index.html'), 'utf8');
  const entryMatch = html.match(/src="\/assets\/([^"]+)"/);
  if (!entryMatch) {
    throw new Error('failed to locate public entry asset in dist/index.html');
  }

  const entryFile = `assets/${entryMatch[1]}`;
  const visited = new Set([entryFile]);
  const queue = [entryFile];
  let totalStartupGzipBytes = 0;
  let jsStartupGzipBytes = 0;

  while (queue.length > 0) {
    const fileName = queue.shift();
    const filePath = path.join(distDir, fileName);
    const gzipSize = gzipSizeOf(filePath);
    totalStartupGzipBytes += gzipSize;
    if (fileName.endsWith('.js')) {
      jsStartupGzipBytes += gzipSize;
    }

    const manifestEntry = Object.values(manifest).find((item) => item.file === fileName);
    for (const imported of manifestEntry?.imports || []) {
      const nextFile = manifest[imported]?.file;
      if (nextFile && !visited.has(nextFile)) {
        visited.add(nextFile);
        queue.push(nextFile);
      }
    }
    for (const cssFile of manifestEntry?.css || []) {
      if (!visited.has(cssFile)) {
        visited.add(cssFile);
        queue.push(cssFile);
      }
    }
  }

  return {
    requestCount: visited.size,
    totalStartupGzipBytes,
    jsStartupGzipBytes,
  };
}

if (process.argv[1] && import.meta.url.endsWith(process.argv[1])) {
  const distDir = path.resolve(process.cwd(), 'dist');
  const stats = collectPublicStartupBudget(distDir);
  const result = evaluatePublicStartupBudget(stats);
  if (result.errors.length > 0) {
    console.error(result.errors.join('\n'));
    process.exit(1);
  }
  console.log(JSON.stringify(stats, null, 2));
}

// web/vite.config.js
const NON_STARTUP_PRELOAD_PATTERNS = [
  /console-/,
  /markdown-runtime/,
  /diagram-runtime/,
  /math-runtime/,
  /chart-runtime/,
  /semi-runtime/,
  /tooltip-/,
  /baseForm-/,
];

function buildManualChunkName(id) {
  if (!id.includes('node_modules')) {
    if (id.includes('/pages/Home') || id.includes('/components/layout/Marketing')) {
      return 'public-marketing';
    }
    if (id.includes('/pages/Setting') || id.includes('/pages/Dashboard') || id.includes('/pages/Token')) {
      return 'console-shell';
    }
    return undefined;
  }

  if (id.includes('@douyinfe/semi-ui') || id.includes('@douyinfe/semi-icons')) {
    return 'semi-runtime';
  }
  if (id.includes('@visactor')) {
    return 'chart-runtime';
  }
  if (id.includes('mermaid') || id.includes('cytoscape')) {
    return 'diagram-runtime';
  }
  if (id.includes('katex')) {
    return 'math-runtime';
  }
  if (
    id.includes('/marked/') ||
    id.includes('react-markdown') ||
    id.includes('remark-') ||
    id.includes('rehype-')
  ) {
    return 'markdown-runtime';
  }
  if (id.includes('react-router-dom') || id.includes('/react/') || id.includes('scheduler') || id.includes('i18next')) {
    return 'startup-runtime';
  }
  return undefined;
}

export default defineConfig({
  build: {
    manifest: true,
    modulePreload: {
      resolveDependencies(_filename, deps) {
        return deps.filter((dependency) => !NON_STARTUP_PRELOAD_PATTERNS.some((pattern) => pattern.test(dependency)));
      },
    },
    rollupOptions: {
      output: {
        manualChunks: buildManualChunkName,
      },
    },
  },
});

// web/package.json
{
  "scripts": {
    "build": "BROWSERSLIST_IGNORE_OLD_DATA=true vite build",
    "test:public-startup": "node --test tests/bootstrap-data.test.mjs tests/public-startup-entry.test.mjs tests/public-layout-mode.test.mjs tests/home-bootstrap.test.mjs tests/public-startup-budget.test.mjs",
    "perf:budget": "node scripts/check-public-startup-budget.mjs"
  }
}
```

- [ ] **Step 4: Run the budget tests and real build budget check to verify they pass**

Run: `cd web && node --test tests/public-startup-budget.test.mjs && npm run build && npm run perf:budget`

Expected: PASS. The unit tests pass, the build succeeds, and the budget script prints the startup stats instead of exiting with an error.

- [ ] **Step 5: Commit the bundle governance changes**

```bash
git add web/vite.config.js web/scripts/check-public-startup-budget.mjs web/tests/public-startup-budget.test.mjs web/package.json
git commit -m "feat: enforce public startup budgets"
```

### Task 7: Update Production Delivery to Proxy `/` Through the Dynamic Root and Enable Optional Brotli

**Files:**
- Modify: `scripts/prod.py`
- Modify: `scripts/tests/test_prod.py`
- Modify: `hermestoken.top.nginx.conf`
- Test: `scripts/tests/test_prod.py`

- [ ] **Step 1: Write the failing production-config tests**

```python
class ProdLauncherTests(unittest.TestCase):
    def test_build_nginx_site_config_proxies_root_to_internal_public_home(self):
        config = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="3000",
            frontend_dist_path=Path("/opt/hermestoken/web/dist"),
        )

        self.assertIn("location = / {", config)
        self.assertIn("proxy_pass http://127.0.0.1:3000/__internal/public-home;", config)
        self.assertIn('add_header Cache-Control "no-cache" always;', config)

    def test_build_nginx_site_config_only_emits_brotli_when_supported(self):
        config_without_brotli = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="3000",
            frontend_dist_path=Path("/opt/hermestoken/web/dist"),
            enable_brotli=False,
        )
        self.assertNotIn("brotli on;", config_without_brotli)

        config_with_brotli = prod.build_nginx_site_config(
            public_url="https://hermestoken.top",
            app_port="3000",
            frontend_dist_path=Path("/opt/hermestoken/web/dist"),
            enable_brotli=True,
        )
        self.assertIn("brotli on;", config_with_brotli)
        self.assertIn("brotli_types", config_with_brotli)
```

- [ ] **Step 2: Run the production-config tests to verify they fail**

Run: `python3 -m unittest scripts.tests.test_prod.ProdLauncherTests.test_build_nginx_site_config_proxies_root_to_internal_public_home scripts.tests.test_prod.ProdLauncherTests.test_build_nginx_site_config_only_emits_brotli_when_supported`

Expected: FAIL because the generated config still serves `/` from static `index.html` and has no Brotli toggle.

- [ ] **Step 3: Implement the root proxy and optional Brotli support in generated and checked-in Nginx config**

```python
# scripts/prod.py
import subprocess


def detect_nginx_supports_brotli() -> bool:
    nginx_binary = shutil.which("nginx")
    if nginx_binary is None:
        return False

    result = subprocess.run([nginx_binary, "-V"], capture_output=True, text=True, check=False)
    combined_output = f"{result.stdout}\n{result.stderr}"
    return "brotli" in combined_output.lower()


def build_nginx_site_config(
    *,
    public_url: str,
    app_port: str = "3000",
    include_real_ip_directives: bool = True,
    frontend_dist_path: Optional[Path] = None,
    enable_brotli: bool = False,
) -> str:
    compression_block = """
    gzip on;
    gzip_comp_level 5;
    gzip_min_length 1024;
    gzip_proxied any;
    gzip_vary on;
    gzip_types
        text/plain
        text/css
        text/javascript
        application/javascript
        application/json
        application/manifest+json
        application/xml
        image/svg+xml;
"""
    if enable_brotli:
        compression_block = """
    brotli on;
    brotli_comp_level 5;
    brotli_static on;
    brotli_types
        text/plain
        text/css
        text/javascript
        application/javascript
        application/json
        application/manifest+json
        application/xml
        image/svg+xml;
""" + compression_block

    static_assets_block = f"""    location ^~ /assets/ {{
        root {dist_root};
        access_log off;
        etag on;
        try_files $uri =404;
        add_header Access-Control-Allow-Origin "*" always;
        add_header Cache-Control "public, max-age=31536000, immutable" always;
    }}

    location = / {{
        proxy_pass http://127.0.0.1:{app_port}/__internal/public-home;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        add_header Cache-Control "no-cache" always;
    }}

    location = /index.html {{
        root {dist_root};
        etag on;
        try_files $uri =404;
        add_header Cache-Control "no-cache" always;
    }}

    location / {{
        root {dist_root};
        index index.html;
        etag on;
        try_files $uri $uri/ /index.html;
    }}
"""

    return f"""server {{
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name {canonical_host};

    ssl_certificate /etc/letsencrypt/live/{canonical_host}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/{canonical_host}/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    client_max_body_size 100m;
{compression_block}
{static_assets_block}}}
"""


def sync_nginx_site_config(
    *,
    public_url: str,
    env_values: Mapping[str, str],
    output: Optional[TextIO] = None,
    site_path: Optional[Path] = None,
    conf_d_path: Optional[Path] = None,
    frontend_dist_path: Optional[Path] = None,
) -> bool:
    app_port = env_values.get("APP_PORT", "3000").strip()
    if not app_port.isdigit():
        app_port = "3000"

    rendered = build_nginx_site_config(
        public_url=public_url,
        app_port=app_port,
        include_real_ip_directives=include_real_ip_directives,
        frontend_dist_path=frontend_dist_path,
        enable_brotli=detect_nginx_supports_brotli(),
    )
```

```nginx
# hermestoken.top.nginx.conf
location = / {
    proxy_pass http://127.0.0.1:3000/__internal/public-home;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    add_header Cache-Control "no-cache" always;
}

location ^~ /assets/ {
    root /opt/hermestoken/web/dist;
    access_log off;
    etag on;
    try_files $uri =404;
    add_header Access-Control-Allow-Origin "*" always;
    add_header Cache-Control "public, max-age=31536000, immutable" always;
}
```

- [ ] **Step 4: Run the production-config tests to verify they pass**

Run: `python3 -m unittest scripts.tests.test_prod.ProdLauncherTests.test_build_nginx_site_config_proxies_root_to_internal_public_home scripts.tests.test_prod.ProdLauncherTests.test_build_nginx_site_config_only_emits_brotli_when_supported`

Expected: PASS.

- [ ] **Step 5: Commit the delivery-layer updates**

```bash
git add scripts/prod.py scripts/tests/test_prod.py hermestoken.top.nginx.conf
git commit -m "feat: serve dynamic public root through nginx"
```

### Task 8: Run the Cross-Stack Verification Pass Before Shipping

**Files:**
- Modify: `docs/superpowers/specs/2026-04-21-hermestoken-performance-governance-design.md`
- Test: `controller/public_bootstrap_test.go`
- Test: `router/web_router_test.go`
- Test: `web/tests/bootstrap-data.test.mjs`
- Test: `web/tests/public-startup-entry.test.mjs`
- Test: `web/tests/public-layout-mode.test.mjs`
- Test: `web/tests/home-bootstrap.test.mjs`
- Test: `web/tests/public-startup-budget.test.mjs`
- Test: `scripts/tests/test_prod.py`

- [ ] **Step 1: Add the final verification checklist to the design spec so the rollout stays measurable**

```md
## Verification Checklist

- [ ] `go test ./controller -run 'TestBuildPublicBootstrapPayloadReturnsPublicSubset|TestRenderPublicHomeIndexEmbedsBootstrapAndShell'`
- [ ] `go test ./router -run 'TestPublicBootstrapEndpointReturnsCacheableJSON|TestInternalPublicHomeEndpointReturnsNoCacheHTML'`
- [ ] `cd web && node --test tests/bootstrap-data.test.mjs tests/public-startup-entry.test.mjs tests/public-layout-mode.test.mjs tests/home-bootstrap.test.mjs tests/public-startup-budget.test.mjs`
- [ ] `cd web && npm run build && npm run perf:budget`
- [ ] `python3 -m unittest scripts.tests.test_prod`
- [ ] Browser benchmark on `https://hermestoken.top/` confirms meaningful HTML before React enhancement and materially lower startup timings than the 2026-04-21 baseline.
```

- [ ] **Step 2: Run the backend verification suite**

Run: `go test ./controller ./router`

Expected: PASS.

- [ ] **Step 3: Run the frontend verification suite**

Run: `cd web && node --test tests/bootstrap-data.test.mjs tests/public-startup-entry.test.mjs tests/public-layout-mode.test.mjs tests/home-bootstrap.test.mjs tests/public-startup-budget.test.mjs && npm run build && npm run perf:budget`

Expected: PASS.

- [ ] **Step 4: Run the deployment/config verification suite**

Run: `python3 -m unittest scripts.tests.test_prod`

Expected: PASS.

- [ ] **Step 5: Commit the rollout checklist updates**

```bash
git add docs/superpowers/specs/2026-04-21-hermestoken-performance-governance-design.md
git commit -m "docs: add performance rollout checklist"
```

## Self-Review

- Spec coverage: The tasks cover the public bootstrap builder, the public refresh endpoint, the root HTML route, the public/console startup split, the home-page startup rewrite, bundle budgets, production Nginx generation, and a measurable final verification pass.
- Placeholder scan: No `TODO`, `TBD`, or “similar to Task N” shorthand remains in the task steps.
- Type consistency: The plan uses one shared `PublicBootstrapPayload` shape across backend rendering, JSON refresh, frontend parsing, caching, and home-page application so the handoff remains consistent.
