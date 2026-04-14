package controller

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type serverTimingMetric struct {
	name string
	dur  float64
}

func setServerTiming(c *gin.Context, metrics ...serverTimingMetric) {
	if len(metrics) == 0 {
		return
	}

	parts := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		if metric.name == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s;dur=%.2f", metric.name, metric.dur))
	}
	if len(parts) == 0 {
		return
	}

	c.Header("Server-Timing", strings.Join(parts, ", "))
}
