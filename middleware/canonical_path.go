package middleware

import (
	"github.com/ca0fgh/hermestoken/relay/constant"
	"github.com/gin-gonic/gin"
)

func CanonicalOpenAIPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil && c.Request.URL != nil {
			if canonicalPath, ok := constant.CanonicalOpenAIPath(c.Request.URL.Path); ok {
				c.Request.URL.Path = canonicalPath
			}
		}
		c.Next()
	}
}
