package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func TestBuildPublicBootstrapPayloadReturnsPublicSubset(t *testing.T) {
	originalSystemName := common.SystemName
	originalFooter := common.Footer
	originalSetup := constant.Setup
	originalOptionMap := common.OptionMap

	t.Cleanup(func() {
		common.SystemName = originalSystemName
		common.Footer = originalFooter
		constant.Setup = originalSetup
		common.OptionMap = originalOptionMap
	})

	common.SystemName = "HermesToken"
	common.Footer = "<p>footer</p>"
	common.OptionMap = map[string]string{
		"HeaderNavModules": `{"home":true,"pricing":{"enabled":true,"requireAuth":false}}`,
		"HomePageContent":  "# Launch faster",
		"Notice":           "Scheduled maintenance tonight",
	}
	constant.Setup = true

	payload := BuildPublicBootstrapPayload()

	if payload.Status.SystemName != "HermesToken" {
		t.Fatalf("SystemName = %q, want %q", payload.Status.SystemName, "HermesToken")
	}
	if payload.Status.GitHubOAuth {
		t.Fatal("GitHubOAuth should remain false for the public payload")
	}
	if payload.Home.Mode != PublicHomeModeHTML {
		t.Fatalf("Home.Mode = %q, want %q", payload.Home.Mode, PublicHomeModeHTML)
	}
	if !strings.Contains(payload.Home.HTML, "<h1") {
		t.Fatalf("Home.HTML = %q, want rendered heading", payload.Home.HTML)
	}
	if payload.Notice.Markdown != "Scheduled maintenance tonight" {
		t.Fatalf("Notice.Markdown = %q, want %q", payload.Notice.Markdown, "Scheduled maintenance tonight")
	}
	if !strings.Contains(payload.Notice.HTML, "<p>Scheduled maintenance tonight</p>") {
		t.Fatalf("Notice.HTML = %q, want rendered notice html", payload.Notice.HTML)
	}
}

func TestRenderPublicHomeIndexEmbedsBootstrapAndShell(t *testing.T) {
	payload := PublicBootstrapPayload{
		Status: PublicStatusSnapshot{
			SystemName: "HermesToken",
			Setup:      true,
		},
		Home: PublicHomeSnapshot{
			Mode: PublicHomeModeHTML,
			HTML: `<section class="hero"><h1>Fast path</h1></section>`,
		},
	}

	output, err := RenderPublicHomeIndex(
		[]byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`),
		payload,
	)
	if err != nil {
		t.Fatalf("RenderPublicHomeIndex returned error: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, `id="hermes-public-bootstrap"`) {
		t.Fatalf("rendered output missing bootstrap script: %s", rendered)
	}
	if !strings.Contains(rendered, `<div id="root"><section class="hero"><h1>Fast path</h1></section></div>`) {
		t.Fatalf("rendered output missing injected root shell: %s", rendered)
	}
}

func TestRenderPublicHomeIndexReturnsErrorWhenHeadInjectionMissing(t *testing.T) {
	payload := PublicBootstrapPayload{
		Status: PublicStatusSnapshot{SystemName: "HermesToken"},
		Home: PublicHomeSnapshot{
			Mode: PublicHomeModeHTML,
			HTML: `<section class="hero"><h1>Fast path</h1></section>`,
		},
	}

	_, err := RenderPublicHomeIndex(
		[]byte(`<!doctype html><html><body><div id="root"></div></body></html>`),
		payload,
	)
	if err == nil {
		t.Fatal("RenderPublicHomeIndex should fail when </head> injection target is missing")
	}
}

func TestRenderPublicHomeIndexReturnsErrorWhenRootReplacementMissing(t *testing.T) {
	payload := PublicBootstrapPayload{
		Status: PublicStatusSnapshot{SystemName: "HermesToken"},
		Home: PublicHomeSnapshot{
			Mode: PublicHomeModeHTML,
			HTML: `<section class="hero"><h1>Fast path</h1></section>`,
		},
	}

	_, err := RenderPublicHomeIndex(
		[]byte(`<!doctype html><html><head></head><body><main id="root"></main></body></html>`),
		payload,
	)
	if err == nil {
		t.Fatal("RenderPublicHomeIndex should fail when the root replacement target is missing")
	}
}

func TestRenderPublicHomeShellDoesNotFallbackForEmptyExplicitModes(t *testing.T) {
	testCases := []struct {
		name    string
		payload PublicBootstrapPayload
		want    string
	}{
		{
			name: "iframe mode without url still returns iframe shell",
			payload: PublicBootstrapPayload{
				Status: PublicStatusSnapshot{SystemName: "HermesToken"},
				Home:   PublicHomeSnapshot{Mode: PublicHomeModeIframe},
			},
			want: `<iframe class="hermes-public-homeframe" src="" title="Public homepage" loading="lazy" referrerpolicy="strict-origin-when-cross-origin"></iframe>`,
		},
		{
			name: "html mode without html returns empty string",
			payload: PublicBootstrapPayload{
				Status: PublicStatusSnapshot{SystemName: "HermesToken"},
				Home:   PublicHomeSnapshot{Mode: PublicHomeModeHTML},
			},
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderPublicHomeShell(tc.payload)
			if got != tc.want {
				t.Fatalf("renderPublicHomeShell() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderPublicHomeShellDoesNotFallbackForUnknownMode(t *testing.T) {
	payload := PublicBootstrapPayload{
		Status: PublicStatusSnapshot{SystemName: "HermesToken"},
		Home: PublicHomeSnapshot{
			Mode: "mystery-mode",
		},
	}

	got := renderPublicHomeShell(payload)
	if got != "" {
		t.Fatalf("renderPublicHomeShell() = %q, want empty string for unknown mode", got)
	}
}

func TestGetPublicBootstrapReturnsJSONWithCaching(t *testing.T) {
	originalSystemName := common.SystemName
	originalFooter := common.Footer
	originalSetup := constant.Setup
	originalOptionMap := common.OptionMap

	t.Cleanup(func() {
		common.SystemName = originalSystemName
		common.Footer = originalFooter
		constant.Setup = originalSetup
		common.OptionMap = originalOptionMap
	})

	common.SystemName = "HermesToken"
	common.Footer = "<p>footer</p>"
	common.OptionMap = map[string]string{
		"Notice": "Scheduled maintenance tonight",
	}
	constant.Setup = true

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/public/bootstrap", nil)

	GetPublicBootstrap(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "public, max-age=60, stale-while-revalidate=300" {
		t.Fatalf("Cache-Control = %q, want public bootstrap cache policy", cacheControl)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want JSON", contentType)
	}

	var response struct {
		Success bool                   `json:"success"`
		Message string                 `json:"message"`
		Data    PublicBootstrapPayload `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON decode failed: %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Message != "" {
		t.Fatalf("message = %q, want empty string", response.Message)
	}
	if response.Data.Status.SystemName != "HermesToken" {
		t.Fatalf("data.status.system_name = %q, want HermesToken", response.Data.Status.SystemName)
	}
}

func TestPublicHomeIndexHandlerReturnsHTMLWithCaching(t *testing.T) {
	originalSystemName := common.SystemName
	originalFooter := common.Footer
	originalSetup := constant.Setup
	originalOptionMap := common.OptionMap

	t.Cleanup(func() {
		common.SystemName = originalSystemName
		common.Footer = originalFooter
		constant.Setup = originalSetup
		common.OptionMap = originalOptionMap
	})

	common.SystemName = "HermesToken"
	common.Footer = "<p>footer</p>"
	common.OptionMap = map[string]string{
		"HomePageContent": "# Launch faster",
	}
	constant.Setup = true

	recorder := httptest.NewRecorder()
	router := gin.New()
	router.GET("/", PublicHomeIndexHandler([]byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`)))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cacheControl)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want HTML", contentType)
	}
	if !strings.Contains(recorder.Body.String(), `id="hermes-public-bootstrap"`) {
		t.Fatalf("body missing bootstrap payload: %s", recorder.Body.String())
	}
}
