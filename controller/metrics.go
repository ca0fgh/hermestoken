package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// promHandler serves the default prometheus registry in exposition format.
// Initialised once because promhttp.Handler() returns a fully functional
// http.Handler that's safe for concurrent use.
var promHandler = promhttp.Handler()

// PrometheusMetrics exposes /metrics for Prometheus scraping.
// Mounted under /api/performance/metrics, which is already RootAuth-gated, so
// scraping requires an admin user access token + the HermesToken-User header
// (or a session cookie). Configure your Prometheus job accordingly.
func PrometheusMetrics(c *gin.Context) {
	promHandler.ServeHTTP(c.Writer, c.Request)
}
