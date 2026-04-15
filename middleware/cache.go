package middleware

import (
	"regexp"

	"github.com/gin-gonic/gin"
)

var versionedAssetPattern = regexp.MustCompile(`^/assets/.+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$`)

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.Request.RequestURI == "/" {
			c.Header("Cache-Control", "no-cache")
		} else if versionedAssetPattern.MatchString(c.Request.URL.Path) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "max-age=604800") // one week
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}
