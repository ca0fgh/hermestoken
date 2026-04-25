package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
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

func filterPricingForMarketplaceDisplay(pricing []model.Pricing, isGuest bool) []model.Pricing {
	if len(pricing) == 0 || !isGuest {
		return pricing
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "default") || common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func buildMarketplaceDisplayGroups(pricing []model.Pricing, isGuest bool) map[string]string {
	displayGroups := make(map[string]string)
	for _, item := range pricing {
		for _, group := range item.EnableGroup {
			if isGuest && group != "default" && group != "all" {
				continue
			}
			if _, ok := displayGroups[group]; !ok {
				displayGroups[group] = setting.GetUsableGroupDescription(group)
			}
		}
	}
	return displayGroups
}

func filterGroupRatioByDisplayGroups(groupRatio map[string]float64, displayGroups map[string]string) map[string]float64 {
	filtered := make(map[string]float64, len(displayGroups))
	for group := range displayGroups {
		if ratio, ok := groupRatio[group]; ok {
			filtered[group] = ratio
		}
	}
	return filtered
}

func emptyPricingResponse() gin.H {
	return gin.H{
		"success":            true,
		"data":               []model.Pricing{},
		"vendors":            []model.PricingVendor{},
		"group_ratio":        map[string]float64{},
		"display_groups":     map[string]string{},
		"usable_group":       map[string]string{},
		"supported_endpoint": map[string]common.EndpointInfo{},
		"auto_groups":        []string{},
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	}
}

func GetPricing(c *gin.Context) {
	pricingConfig := getPricingHeaderNavConfig(common.OptionMap["HeaderNavModules"])
	if !pricingConfig.Enabled {
		disabledStart := time.Now()
		setServerTiming(c,
			serverTimingMetric{name: "pricing_model", dur: 0},
			serverTimingMetric{name: "pricing_context", dur: 0},
			serverTimingMetric{name: "pricing_filter", dur: 0},
			serverTimingMetric{name: "pricing_response", dur: 0},
			serverTimingMetric{name: "pricing_total", dur: float64(time.Since(disabledStart).Microseconds()) / 1000},
		)
		c.JSON(200, emptyPricingResponse())
		return
	}

	totalStart := time.Now()

	modelStart := time.Now()
	pricing := model.GetPricing()
	modelDuration := time.Since(modelStart)

	contextStart := time.Now()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
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
		ratio, ok := ratio_setting.GetGroupGroupRatio(displayGroup, g)
		if ok {
			groupRatio[g] = ratio
		}
	}

	usableGroup = service.GetUserUsableGroupsForUser(currentUserID, displayGroup)
	contextDuration := time.Since(contextStart)

	filterStart := time.Now()
	isGuest := !exists
	pricing = filterPricingForMarketplaceDisplay(pricing, isGuest)
	displayGroups := buildMarketplaceDisplayGroups(pricing, isGuest)
	groupRatio = filterGroupRatioByDisplayGroups(groupRatio, displayGroups)
	filterDuration := time.Since(filterStart)

	responseStart := time.Now()
	responsePayload := gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"display_groups":     displayGroups,
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
