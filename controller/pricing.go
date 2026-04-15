package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"time"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func GetPricing(c *gin.Context) {
	totalStart := time.Now()

	modelStart := time.Now()
	pricing := model.GetPricing()
	modelDuration := time.Since(modelStart)

	contextStart := time.Now()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio, err := model.LoadEffectivePricingGroupRatios()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	currentUserID := 0
	var group string
	if exists {
		currentUserID = userId.(int)
		user, err := model.GetUserCache(currentUserID)
		if err == nil {
			group = user.Group
		}
	}

	displayGroup := group
	if !exists {
		displayGroup = "default"
	}
	for g := range groupRatio {
		groupRatio[g] = service.GetUserGroupRatio(displayGroup, g)
	}

	usableGroup = service.GetUserUsableGroupsForUser(currentUserID, displayGroup)
	contextDuration := time.Since(contextStart)

	filterStart := time.Now()
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	// check groupRatio contains usableGroup
	for group := range groupRatio {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}
	filterDuration := time.Since(filterStart)

	responseStart := time.Now()
	responsePayload := gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroupForUser(currentUserID, displayGroup),
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	}
	responseDuration := time.Since(responseStart)
	setServerTiming(c,
		serverTimingMetric{name: "pricing_model", dur: float64(modelDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_context", dur: float64(contextDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_filter", dur: float64(filterDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_response", dur: float64(responseDuration.Microseconds()) / 1000},
		serverTimingMetric{name: "pricing_total", dur: float64(time.Since(totalStart).Microseconds()) / 1000},
	)

	c.JSON(200, responsePayload)
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
