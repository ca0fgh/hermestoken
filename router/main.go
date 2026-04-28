package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

type frontendRouteMode int

const (
	frontendRouteModeDisabled frontendRouteMode = iota
	frontendRouteModeEmbedded
	frontendRouteModeRedirect
)

func resolveFrontendRouteMode(frontendBaseURL string, isMasterNode bool, hasEmbeddedAssets bool) (frontendRouteMode, string, bool) {
	ignoredFrontendBaseURL := isMasterNode && frontendBaseURL != ""
	if isMasterNode && frontendBaseURL != "" {
		frontendBaseURL = ""
	}
	if frontendBaseURL != "" {
		return frontendRouteModeRedirect, strings.TrimSuffix(frontendBaseURL, "/"), ignoredFrontendBaseURL
	}
	if hasEmbeddedAssets {
		return frontendRouteModeEmbedded, "", ignoredFrontendBaseURL
	}
	return frontendRouteModeDisabled, "", ignoredFrontendBaseURL
}

func SetRouter(router *gin.Engine, assets ThemeAssets) {
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	SetVideoRouter(router)
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	hasEmbeddedAssets := len(assets.DefaultIndexPage) > 0 || len(assets.ClassicIndexPage) > 0
	mode, resolvedFrontendBaseURL, ignoredFrontendBaseURL := resolveFrontendRouteMode(frontendBaseUrl, common.IsMasterNode, hasEmbeddedAssets)
	if ignoredFrontendBaseURL {
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	switch mode {
	case frontendRouteModeEmbedded:
		SetWebRouter(router, assets)
	case frontendRouteModeRedirect:
		router.NoRoute(func(c *gin.Context) {
			c.Set(middleware.RouteTagKey, "web")
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", resolvedFrontendBaseURL, c.Request.RequestURI))
		})
	default:
		common.SysLog("embedded frontend assets unavailable; web UI routes disabled. Build after generating `web/default/dist` and `web/classic/dist`, or set FRONTEND_BASE_URL")
	}
}
