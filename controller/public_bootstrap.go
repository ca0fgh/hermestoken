package controller

import (
	"bytes"
	"html"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
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

func BuildPublicBootstrapPayload() PublicBootstrapPayload {
	var (
		headerNavModules string
		homeMarkdown     string
		noticeMarkdown   string
	)

	common.OptionMapRWMutex.RLock()
	headerNavModules = strings.TrimSpace(common.OptionMap["HeaderNavModules"])
	homeMarkdown = strings.TrimSpace(common.OptionMap["HomePageContent"])
	noticeMarkdown = strings.TrimSpace(common.OptionMap["Notice"])
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
		Notice: PublicNoticeSnapshot{
			Markdown: noticeMarkdown,
		},
	}

	switch {
	case strings.HasPrefix(homeMarkdown, "https://"):
		payload.Home = PublicHomeSnapshot{
			Mode: PublicHomeModeIframe,
			URL:  homeMarkdown,
		}
	case homeMarkdown != "":
		var rendered bytes.Buffer
		if err := goldmark.Convert([]byte(homeMarkdown), &rendered); err == nil {
			payload.Home = PublicHomeSnapshot{
				Mode:     PublicHomeModeHTML,
				HTML:     rendered.String(),
				Markdown: homeMarkdown,
			}
			break
		}
		fallthrough
	default:
		payload.Home = PublicHomeSnapshot{
			Mode: PublicHomeModeDefault,
		}
	}

	return payload
}

func renderPublicHomeShell(payload PublicBootstrapPayload) string {
	switch payload.Home.Mode {
	case PublicHomeModeIframe:
		return `<iframe class="hermes-public-homeframe" src="` + html.EscapeString(payload.Home.URL) + `" title="Public homepage" loading="lazy" referrerpolicy="strict-origin-when-cross-origin"></iframe>`
	case PublicHomeModeHTML:
		return payload.Home.HTML
	case PublicHomeModeDefault:
		// Fall back only when the payload explicitly requests the default shell.
	}

	systemName := strings.TrimSpace(payload.Status.SystemName)
	if systemName == "" {
		systemName = "HermesToken"
	}
	systemName = html.EscapeString(systemName)

	return `<section class="hermes-public-fallback"><p class="eyebrow">` + systemName + `</p><h1>Fast, reliable AI gateway</h1><p>Configure HomePageContent to publish a custom landing page without waiting for the client app to boot.</p></section>`
}

func RenderPublicHomeIndex(baseIndex []byte, payload PublicBootstrapPayload) ([]byte, error) {
	bootstrapJSON, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	scriptTag := []byte(`<script id="hermes-public-bootstrap" type="application/json">` + string(bootstrapJSON) + `</script>`)
	rendered := bytes.Replace(baseIndex, []byte("</head>"), append(scriptTag, []byte("</head>")...), 1)
	if bytes.Equal(rendered, baseIndex) {
		rendered = append(scriptTag, rendered...)
	}

	rootShell := []byte(`<div id="root">` + renderPublicHomeShell(payload) + `</div>`)
	rendered = bytes.Replace(rendered, []byte(`<div id="root"></div>`), rootShell, 1)

	return rendered, nil
}

func GetPublicBootstrap(c *gin.Context) {
	payload := BuildPublicBootstrapPayload()
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payload,
	})
}

func PublicHomeIndexHandler(baseIndex []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload := BuildPublicBootstrapPayload()
		rendered, err := RenderPublicHomeIndex(baseIndex, payload)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to render public home")
			return
		}

		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", rendered)
	}
}
