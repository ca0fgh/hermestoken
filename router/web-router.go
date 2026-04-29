package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/controller"
	"github.com/ca0fgh/hermestoken/middleware"
	relayconstant "github.com/ca0fgh/hermestoken/relay/constant"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded frontend assets for both themes.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
	ClassicBuildFS   embed.FS
	ClassicIndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")
	classicFS := common.EmbedFolder(assets.ClassicBuildFS, "web/classic/dist")
	themeFS := common.NewThemeAwareFS(defaultFS, classicFS)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.GET("/__internal/public-home", controller.PublicHomeIndexHandler(assets.DefaultIndexPage))
	router.Use(static.Serve("/", themeFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		if isRootOpenAIModelsAPIRequest(c) {
			c.Set(middleware.RouteTagKey, "relay")
			modelID := strings.TrimPrefix(c.Request.URL.Path, "/models/")
			if canonicalPath, ok := relayconstant.CanonicalOpenAIPath(c.Request.URL.Path); ok {
				c.Request.URL.Path = canonicalPath
			}
			middleware.TokenAuth()(c)
			if c.IsAborted() {
				return
			}
			if modelID == "" || modelID == c.Request.URL.Path {
				openAIModelsHandler(c)
			} else {
				c.Params = append(c.Params, gin.Param{Key: "model", Value: modelID})
				openAIModelHandler(c)
			}
			return
		}
		c.Header("Cache-Control", "no-cache")
		if common.GetTheme() == "classic" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.ClassicIndexPage)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
		}
	})
}

func isRootOpenAIModelsAPIRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	if c.Request.Method != http.MethodGet {
		return false
	}
	path := c.Request.URL.Path
	if path != "/models" && !strings.HasPrefix(path, "/models/") {
		return false
	}
	return c.GetHeader("Authorization") != "" ||
		c.GetHeader("x-api-key") != "" ||
		c.GetHeader("x-goog-api-key") != "" ||
		c.Query("key") != ""
}
