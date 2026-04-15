package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetPricingGroupConsistencyReport(c *gin.Context) {
	report := service.BuildPricingGroupConsistencyReport()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    report,
	})
}
