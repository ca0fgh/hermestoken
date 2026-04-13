package controller

import (
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type UpdateSubscriptionReferralSelfRequest struct {
	Group          string `json:"group"`
	InviteeRateBps int    `json:"invitee_rate_bps"`
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

	groupViews := buildSubscriptionReferralSelfGroupViews(user)
	common.ApiSuccess(c, gin.H{
		"enabled":              len(groupViews) > 0,
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
	req.Group = strings.TrimSpace(req.Group)
	if req.Group == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}
	if !isValidSubscriptionReferralGroup(req.Group) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(userID, req.Group)
	if totalRateBps <= 0 {
		common.ApiErrorMsg(c, "该分组未启用订阅返佣")
		return
	}
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
	if req.Group == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}
	if !isValidSubscriptionReferralGroup(req.Group) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	if _, err := model.UpsertSubscriptionReferralOverride(userID, req.Group, req.TotalRateBps, c.GetInt("id")); err != nil {
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

	targetGroup, err := resolveSubscriptionReferralOverrideDeleteGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if targetGroup == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}
	if !canDeleteSubscriptionReferralOverrideGroup(userID, targetGroup) {
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

func buildSubscriptionReferralSelfGroupViews(user *model.User) []gin.H {
	setting := user.GetSetting()
	overrides, err := model.ListSubscriptionReferralOverridesByUserID(user.Id)
	if err != nil {
		return []gin.H{}
	}

	groupViews := make([]gin.H, 0, len(overrides))
	for _, override := range overrides {
		group := strings.TrimSpace(override.Group)
		if group == "" {
			continue
		}
		totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(user.Id, group)
		if totalRateBps <= 0 {
			continue
		}
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

	groups := collectSubscriptionReferralResponseGroups(model.ListSubscriptionReferralConfiguredGroups(), overrides)
	groupViews := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		override, hasOverride := overrideByGroup[group]
		overrideRateBps := interface{}(nil)
		if hasOverride {
			overrideRateBps = override.TotalRateBps
		}
		groupViews = append(groupViews, gin.H{
			"group":                    group,
			"effective_total_rate_bps": model.GetEffectiveSubscriptionReferralTotalRateBps(userID, group),
			"has_override":             hasOverride,
			"override_rate_bps":        overrideRateBps,
		})
	}

	return gin.H{
		"user_id": userID,
		"groups":  groupViews,
	}, nil
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
			continue
		}
		groupSet[trimmedGroup] = struct{}{}
	}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func resolveSubscriptionReferralOverrideDeleteGroup(c *gin.Context) (string, error) {
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
	return "", nil
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
		return false
	}
	if isValidSubscriptionReferralGroup(trimmedGroup) {
		return true
	}
	override, err := model.GetSubscriptionReferralOverrideByUserIDAndGroup(userID, trimmedGroup)
	return err == nil && override != nil
}
