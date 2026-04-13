package controller

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UpdateSubscriptionReferralSelfRequest struct {
	Group          string `json:"group"`
	InviteeRateBps int    `json:"invitee_rate_bps"`
}

type AdminUpdateSubscriptionReferralSettingsRequest struct {
	Enabled      *bool          `json:"enabled"`
	GroupRates   map[string]int `json:"group_rates"`
	TotalRateBps *int           `json:"total_rate_bps"`
}

type AdminUpsertSubscriptionReferralOverrideRequest struct {
	Group        string `json:"group"`
	TotalRateBps int    `json:"total_rate_bps"`
}

type adminDeleteSubscriptionReferralOverrideRequest struct {
	Group string `json:"group"`
}

func GetSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(userID, "")
	cfg := model.ResolveSubscriptionReferralConfig(totalRateBps, user.GetSetting().SubscriptionReferralInviteeRateBps)
	groupViews := buildSubscriptionReferralSelfGroupViews(user)
	common.ApiSuccess(c, gin.H{
		"enabled":              common.SubscriptionReferralEnabled,
		"total_rate_bps":       cfg.TotalRateBps,
		"invitee_rate_bps":     cfg.InviteeRateBps,
		"inviter_rate_bps":     cfg.InviterRateBps,
		"groups":               groupViews,
		"pending_reward_quota": user.AffQuota,
		"history_reward_quota": user.AffHistoryQuota,
		"inviter_count":        user.AffCount,
	})
}

func UpdateSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	var req UpdateSubscriptionReferralSelfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Group = normalizeSubscriptionReferralRequestGroup(req.Group)
	if !isValidSubscriptionReferralGroup(req.Group) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(userID, req.Group)
	if req.InviteeRateBps < 0 || req.InviteeRateBps > totalRateBps {
		common.ApiErrorMsg(c, "被邀请人比例不能超过总返佣率")
		return
	}

	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setting := user.GetSetting()
	nextByGroup := copySubscriptionReferralInviteeRatesByGroup(setting.SubscriptionReferralInviteeRateBpsByGroup)
	nextByGroup[req.Group] = req.InviteeRateBps
	setting.SubscriptionReferralInviteeRateBpsByGroup = nextByGroup
	user.SetSetting(setting)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}

	cfg := model.ResolveSubscriptionReferralConfig(totalRateBps, req.InviteeRateBps)
	common.ApiSuccess(c, gin.H{
		"group":            req.Group,
		"invitee_rate_bps": req.InviteeRateBps,
		"inviter_rate_bps": cfg.InviterRateBps,
		"total_rate_bps":   cfg.TotalRateBps,
	})
}

func AdminGetSubscriptionReferralSettings(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"enabled":        common.SubscriptionReferralEnabled,
		"groups":         model.ListSubscriptionReferralConfiguredGroups(),
		"group_rates":    common.GetSubscriptionReferralGroupRatesCopy(),
		"total_rate_bps": model.NormalizeSubscriptionReferralRateBps(common.SubscriptionReferralGlobalRateBps),
	})
}

func AdminUpdateSubscriptionReferralSettings(c *gin.Context) {
	var req AdminUpdateSubscriptionReferralSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	groupRates, err := resolveSubscriptionReferralSettingsGroupRates(req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	groupRatesJSON := ""
	if groupRates != nil {
		jsonBytes, err := json.Marshal(groupRates)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		groupRatesJSON = string(jsonBytes)
	}

	if req.Enabled != nil {
		if err := model.UpdateOption("SubscriptionReferralEnabled", strconv.FormatBool(*req.Enabled)); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if groupRates != nil {
		if err := model.UpdateOption("SubscriptionReferralGroupRates", groupRatesJSON); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	AdminGetSubscriptionReferralSettings(c)
}

func AdminGetSubscriptionReferralOverride(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	if userID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if _, err := model.GetUserById(userID, false); err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := buildAdminSubscriptionReferralOverrideResponse(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, response)
}

func AdminUpsertSubscriptionReferralOverride(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	if userID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if _, err := model.GetUserById(userID, false); err != nil {
		common.ApiError(c, err)
		return
	}

	var req AdminUpsertSubscriptionReferralOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Group = strings.TrimSpace(req.Group)
	if req.Group != "" && !isValidSubscriptionReferralGroup(req.Group) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	targetGroup := req.Group
	if targetGroup == "" {
		if override, err := model.GetLegacySubscriptionReferralOverrideByUserID(userID); err == nil && override != nil {
			targetGroup = override.Group
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, err)
			return
		}
	}

	override, err := model.UpsertSubscriptionReferralOverride(userID, targetGroup, req.TotalRateBps, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := buildAdminSubscriptionReferralOverrideResponse(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response["group"] = targetGroup
	response["has_override"] = true
	response["override_rate_bps"] = override.TotalRateBps
	response["effective_total_rate_bps"] = model.GetEffectiveSubscriptionReferralTotalRateBps(userID, targetGroup)
	common.ApiSuccess(c, response)
}

func AdminDeleteSubscriptionReferralOverride(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("id"))
	if userID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if _, err := model.GetUserById(userID, false); err != nil {
		common.ApiError(c, err)
		return
	}

	targetGroup, err := resolveSubscriptionReferralOverrideDeleteGroup(c, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if targetGroup != "" && !canDeleteSubscriptionReferralOverrideGroup(userID, targetGroup) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	if err := model.DeleteSubscriptionReferralOverrideByUserIDAndGroup(userID, targetGroup); err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := buildAdminSubscriptionReferralOverrideResponse(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response["group"] = targetGroup
	if targetGroup != "" {
		response["has_override"] = false
		response["override_rate_bps"] = nil
		response["effective_total_rate_bps"] = model.GetEffectiveSubscriptionReferralTotalRateBps(userID, targetGroup)
	}

	common.ApiSuccess(c, response)
}

func AdminReverseSubscriptionReferral(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	if tradeNo == "" {
		common.ApiErrorMsg(c, "无效的订单号")
		return
	}

	if err := model.ReverseSubscriptionReferralByTradeNo(tradeNo, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"trade_no": tradeNo})
}

func copySubscriptionReferralInviteeRatesByGroup(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for group, rate := range src {
		dst[group] = rate
	}
	return dst
}

func normalizeSubscriptionReferralGroupRates(groupRates map[string]int) map[string]int {
	normalized := make(map[string]int, len(groupRates))
	for group, rate := range groupRates {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			continue
		}
		if !isValidSubscriptionReferralGroup(trimmedGroup) {
			return nil
		}
		normalized[trimmedGroup] = model.NormalizeSubscriptionReferralRateBps(rate)
	}
	return normalized
}

func buildSubscriptionReferralSelfGroupViews(user *model.User) []gin.H {
	groups := model.ListSubscriptionReferralConfiguredGroups()
	setting := user.GetSetting()
	groupViews := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(user.Id, group)
		inviteeRateBps := model.GetEffectiveSubscriptionReferralInviteeRateBps(setting, group, totalRateBps)
		cfg := model.ResolveSubscriptionReferralConfig(totalRateBps, inviteeRateBps)
		groupViews = append(groupViews, gin.H{
			"group":            group,
			"total_rate_bps":   cfg.TotalRateBps,
			"invitee_rate_bps": cfg.InviteeRateBps,
			"inviter_rate_bps": cfg.InviterRateBps,
		})
	}
	return groupViews
}

func buildAdminSubscriptionReferralOverrideResponse(userID int) (gin.H, error) {
	overrides, err := model.ListSubscriptionReferralOverridesByUserID(userID)
	if err != nil {
		return nil, err
	}

	overrideByGroup := make(map[string]model.SubscriptionReferralOverride, len(overrides))
	for _, override := range overrides {
		trimmedGroup := strings.TrimSpace(override.Group)
		if trimmedGroup == "" {
			continue
		}
		overrideByGroup[trimmedGroup] = override
	}

	legacyOverride, err := model.GetLegacySubscriptionReferralOverrideByUserID(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	groups := collectSubscriptionReferralResponseGroups(model.ListSubscriptionReferralConfiguredGroups(), overrides)
	groupViews := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		override, hasOverride := overrideByGroup[group]
		overrideRateBps := interface{}(nil)
		if hasOverride {
			overrideRateBps = override.TotalRateBps
		} else if group == "default" && legacyOverride != nil && strings.TrimSpace(legacyOverride.Group) == "" {
			hasOverride = true
			overrideRateBps = legacyOverride.TotalRateBps
		}
		groupViews = append(groupViews, gin.H{
			"group":                    group,
			"effective_total_rate_bps": model.GetEffectiveSubscriptionReferralTotalRateBps(userID, group),
			"has_override":             hasOverride,
			"override_rate_bps":        overrideRateBps,
		})
	}

	response := gin.H{
		"user_id":                  userID,
		"groups":                   groupViews,
		"effective_total_rate_bps": model.GetEffectiveSubscriptionReferralTotalRateBps(userID, ""),
		"has_override":             legacyOverride != nil,
		"override_rate_bps":        nil,
	}
	if legacyOverride != nil {
		response["override_rate_bps"] = legacyOverride.TotalRateBps
	}
	return response, nil
}

func collectSubscriptionReferralResponseGroups(configuredGroups []string, overrides []model.SubscriptionReferralOverride) []string {
	groupSet := make(map[string]struct{}, len(configuredGroups)+len(overrides))
	for _, group := range configuredGroups {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			continue
		}
		groupSet[trimmedGroup] = struct{}{}
	}
	for _, override := range overrides {
		trimmedGroup := strings.TrimSpace(override.Group)
		if trimmedGroup == "" {
			groupSet["default"] = struct{}{}
			continue
		}
		groupSet[trimmedGroup] = struct{}{}
	}
	if len(groupSet) == 0 {
		groupSet["default"] = struct{}{}
	}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func resolveSubscriptionReferralOverrideDeleteGroup(c *gin.Context, userID int) (string, error) {
	group := strings.TrimSpace(c.Query("group"))
	if group != "" {
		return group, nil
	}
	if c.Request.ContentLength > 0 {
		var req adminDeleteSubscriptionReferralOverrideRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return "", err
		}
		return strings.TrimSpace(req.Group), nil
	}

	if override, err := model.GetLegacySubscriptionReferralOverrideByUserID(userID); err == nil && override != nil {
		return override.Group, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return "", nil
}

func normalizeSubscriptionReferralRequestGroup(group string) string {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return "default"
	}
	return trimmedGroup
}

func isValidSubscriptionReferralGroup(group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}
	if _, ok := ratio_setting.GetGroupRatioCopy()[trimmedGroup]; ok {
		return true
	}
	return model.IsSubscriptionReferralPlanBackedGroup(trimmedGroup)
}

func canDeleteSubscriptionReferralOverrideGroup(userID int, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return true
	}
	if isValidSubscriptionReferralGroup(trimmedGroup) {
		return true
	}
	override, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(userID, trimmedGroup)
	return err == nil && override != nil
}

func resolveSubscriptionReferralSettingsGroupRates(req AdminUpdateSubscriptionReferralSettingsRequest) (map[string]int, error) {
	if req.GroupRates == nil && req.TotalRateBps == nil {
		return nil, nil
	}

	if req.GroupRates != nil {
		normalized := make(map[string]int, len(req.GroupRates))
		for group, rate := range req.GroupRates {
			trimmedGroup := strings.TrimSpace(group)
			if trimmedGroup == "" {
				continue
			}
			normalizedRate := model.NormalizeSubscriptionReferralRateBps(rate)
			if !isValidSubscriptionReferralGroup(trimmedGroup) {
				if normalizedRate == 0 {
					continue
				}
				return nil, errors.New("分组不存在")
			}
			normalized[trimmedGroup] = normalizedRate
		}
		return normalized, nil
	}

	groupRates := common.GetSubscriptionReferralGroupRatesCopy()
	groupRates["default"] = model.NormalizeSubscriptionReferralRateBps(*req.TotalRateBps)
	return groupRates, nil
}
