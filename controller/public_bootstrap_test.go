package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
