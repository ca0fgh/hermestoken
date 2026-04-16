package controller

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	inviterCount, err := model.CountInviteesByInviterID(userID)
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
		"inviter_count":        inviterCount,
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
	if !canMutateSubscriptionReferralSelfGroup(userID, req.Group) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	if updated, handled, err := updateSubscriptionReferralSelfViaTemplateBinding(userID, req.Group, req.InviteeRateBps); handled {
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, updated)
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

func DeleteSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	targetGroup, err := resolveSubscriptionReferralOverrideDeleteGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if targetGroup == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}
	if !canDeleteSubscriptionReferralSelfGroup(userID, targetGroup) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	if updated, handled, err := deleteSubscriptionReferralSelfViaTemplateBinding(userID, targetGroup); handled {
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, updated)
		return
	}

	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if model.GetEffectiveSubscriptionReferralTotalRateBps(userID, targetGroup) <= 0 {
		if !subscriptionReferralSelfSettingContainsGroup(user.GetSetting(), targetGroup) {
			common.ApiErrorMsg(c, "该分组未启用订阅返佣")
			return
		}
	}

	setting := user.GetSetting()
	nextByGroup := copySubscriptionReferralInviteeRatesByGroup(setting.SubscriptionReferralInviteeRateBpsByGroup)
	delete(nextByGroup, strings.TrimSpace(targetGroup))
	setting.SubscriptionReferralInviteeRateBpsByGroup = nextByGroup
	user.SetSetting(setting)
	if err := user.Update(false); err != nil {
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

func GetSubscriptionReferralInvitees(c *gin.Context) {
	userID := c.GetInt("id")
	keyword := strings.TrimSpace(c.Query("keyword"))
	pageInfo := common.GetPageQuery(c)
	if page, err := strconv.Atoi(strings.TrimSpace(c.Query("page"))); err == nil && page > 0 {
		pageInfo.Page = page
	}

	summaries, total, contributionTotal, err := model.ListSubscriptionReferralInviteeContributionSummaries(userID, keyword, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	overrideCountByInviteeID, err := listSubscriptionReferralInviteeOverrideCounts(userID, summaries)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, gin.H{
			"id":                   summary.InviteeUserId,
			"username":             summary.InviteeUsername,
			"group":                summary.InviteeGroup,
			"contribution_quota":   summary.ContributionQuota,
			"reward_quota":         summary.RewardQuota,
			"reversed_quota":       summary.ReversedQuota,
			"debt_quota":           summary.DebtQuota,
			"order_count":          summary.OrderCount,
			"override_group_count": overrideCountByInviteeID[summary.InviteeUserId],
		})
	}

	common.ApiSuccess(c, gin.H{
		"items":                    items,
		"total":                    total,
		"page":                     pageInfo.GetPage(),
		"page_size":                pageInfo.GetPageSize(),
		"invitee_count":            total,
		"total_contribution_quota": contributionTotal,
	})
}

func GetSubscriptionReferralInvitee(c *gin.Context) {
	userID := c.GetInt("id")
	invitee, ok := getOwnedSubscriptionReferralInvitee(c, userID)
	if !ok {
		return
	}

	response, err := buildSubscriptionReferralInviteeDetailResponse(userID, invitee)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func UpsertSubscriptionReferralInviteeOverride(c *gin.Context) {
	userID := c.GetInt("id")
	invitee, ok := getOwnedSubscriptionReferralInvitee(c, userID)
	if !ok {
		return
	}

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
	if !canMutateSubscriptionReferralInviteeGroup(userID, invitee.Id, req.Group) {
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

	if hasActiveTemplateBinding(userID, req.Group) {
		if _, err := model.UpsertReferralInviteeShareOverride(userID, invitee.Id, model.ReferralTypeSubscription, req.Group, req.InviteeRateBps, userID); err != nil {
			common.ApiError(c, err)
			return
		}

		response, err := buildSubscriptionReferralInviteeDetailResponse(userID, invitee)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, response)
		return
	}

	if _, err := model.UpsertSubscriptionReferralInviteeOverride(userID, invitee.Id, req.Group, req.InviteeRateBps); err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := buildSubscriptionReferralInviteeDetailResponse(userID, invitee)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func DeleteSubscriptionReferralInviteeOverride(c *gin.Context) {
	userID := c.GetInt("id")
	invitee, ok := getOwnedSubscriptionReferralInvitee(c, userID)
	if !ok {
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
	if !canDeleteSubscriptionReferralInviteeOverrideGroup(userID, invitee.Id, targetGroup) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}

	if hasActiveTemplateBinding(userID, targetGroup) {
		if err := model.DeleteReferralInviteeShareOverride(userID, invitee.Id, model.ReferralTypeSubscription, targetGroup); err != nil {
			common.ApiError(c, err)
			return
		}

		response, err := buildSubscriptionReferralInviteeDetailResponse(userID, invitee)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, response)
		return
	}

	if err := model.DeleteSubscriptionReferralInviteeOverride(userID, invitee.Id, targetGroup); err != nil {
		common.ApiError(c, err)
		return
	}

	response, err := buildSubscriptionReferralInviteeDetailResponse(userID, invitee)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
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
	templateViewsByGroup := make(map[string]gin.H)
	if bindingViews, err := model.ListReferralTemplateBindingsByUser(user.Id, model.ReferralTypeSubscription); err == nil && len(bindingViews) > 0 {
		for _, groupView := range buildSubscriptionReferralTemplateGroupViews(bindingViews) {
			group := strings.TrimSpace(common.Interface2String(groupView["group"]))
			if group == "" {
				continue
			}
			templateViewsByGroup[group] = groupView
		}
	}

	setting := user.GetSetting()
	overrides, err := model.ListSubscriptionReferralOverridesByUserID(user.Id)
	if err != nil {
		return collectSortedSubscriptionReferralGroupViews(templateViewsByGroup, nil)
	}

	legacyViews := make([]gin.H, 0, len(overrides))
	for _, override := range overrides {
		group := strings.TrimSpace(override.Group)
		if group == "" {
			continue
		}
		if !isValidSubscriptionReferralGroup(group) {
			continue
		}
		totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(user.Id, group)
		if totalRateBps <= 0 {
			continue
		}
		inviteeRateBps := model.GetEffectiveSubscriptionReferralInviteeRateBps(setting, group, totalRateBps)
		cfg := model.ResolveSubscriptionReferralConfig(totalRateBps, inviteeRateBps)
		legacyViews = append(legacyViews, gin.H{
			"group":            group,
			"total_rate_bps":   cfg.TotalRateBps,
			"invitee_rate_bps": cfg.InviteeRateBps,
			"inviter_rate_bps": cfg.InviterRateBps,
		})
	}
	return collectSortedSubscriptionReferralGroupViews(templateViewsByGroup, legacyViews)
}

func buildSubscriptionReferralTemplateGroupViews(bindingViews []model.ReferralTemplateBindingView) []gin.H {
	groupViews := make([]gin.H, 0, len(bindingViews))
	for _, view := range bindingViews {
		if !view.Template.Enabled {
			continue
		}
		totalRateBps := subscriptionTemplateVisibleTotalRateBps(view.Template)
		inviteeRateBps := model.ResolveBindingInviteeShareDefault(view)
		if inviteeRateBps > totalRateBps {
			inviteeRateBps = totalRateBps
		}
		groupViews = append(groupViews, gin.H{
			"group":            view.Binding.Group,
			"type":             "subscription",
			"template_id":      view.Template.Id,
			"template_name":    view.Template.Name,
			"level_type":       view.Template.LevelType,
			"total_rate_bps":   totalRateBps,
			"invitee_rate_bps": inviteeRateBps,
			"inviter_rate_bps": totalRateBps - inviteeRateBps,
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

func canMutateSubscriptionReferralSelfGroup(userID int, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}
	if hasActiveTemplateBinding(userID, trimmedGroup) {
		return true
	}
	if isValidSubscriptionReferralGroup(trimmedGroup) {
		return true
	}
	if model.GetEffectiveSubscriptionReferralTotalRateBps(userID, trimmedGroup) > 0 {
		return true
	}
	user, err := model.GetUserById(userID, true)
	if err != nil {
		return false
	}
	return subscriptionReferralSelfSettingContainsGroup(user.GetSetting(), trimmedGroup)
}

func canDeleteSubscriptionReferralSelfGroup(userID int, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}
	if hasActiveTemplateBinding(userID, trimmedGroup) {
		return true
	}
	return canMutateSubscriptionReferralSelfGroup(userID, trimmedGroup)
}

func subscriptionReferralSelfSettingContainsGroup(setting dto.UserSetting, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}
	_, exists := setting.SubscriptionReferralInviteeRateBpsByGroup[trimmedGroup]
	return exists
}

func canMutateSubscriptionReferralInviteeGroup(inviterUserID int, inviteeUserID int, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}
	if hasActiveTemplateBinding(inviterUserID, trimmedGroup) {
		return true
	}
	if isValidSubscriptionReferralGroup(trimmedGroup) {
		return true
	}
	if model.GetEffectiveSubscriptionReferralTotalRateBps(inviterUserID, trimmedGroup) > 0 {
		return true
	}
	overrides, err := model.ListSubscriptionReferralInviteeOverrides(inviterUserID, inviteeUserID)
	if err != nil {
		return false
	}
	for _, override := range overrides {
		if strings.TrimSpace(override.Group) == trimmedGroup {
			return true
		}
	}
	return false
}

func getOwnedSubscriptionReferralInvitee(c *gin.Context, inviterUserID int) (*model.User, bool) {
	inviteeID, _ := strconv.Atoi(c.Param("invitee_id"))
	if inviteeID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return nil, false
	}

	invitee, err := model.GetUserById(inviteeID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(strings.ToLower(err.Error()), "record not found") {
			common.ApiErrorMsg(c, "被邀请人不存在")
			return nil, false
		}
		common.ApiError(c, err)
		return nil, false
	}
	if invitee.InviterId != inviterUserID {
		common.ApiErrorMsg(c, "被邀请人不存在")
		return nil, false
	}
	return invitee, true
}

func listSubscriptionReferralInviteeOverrideCounts(inviterUserID int, summaries []*model.SubscriptionReferralInviteeContributionSummary) (map[int]int64, error) {
	counts := make(map[int]int64, len(summaries))
	inviteeUserIDs := make([]int, 0, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.InviteeUserId <= 0 {
			continue
		}
		inviteeUserIDs = append(inviteeUserIDs, summary.InviteeUserId)
	}
	batchCounts, err := model.ListSubscriptionReferralInviteeOverrideCounts(inviterUserID, inviteeUserIDs)
	if err != nil {
		return nil, err
	}
	for inviteeUserID, count := range batchCounts {
		counts[inviteeUserID] = count
	}
	templateCounts, err := model.ListReferralInviteeShareOverrideCounts(inviterUserID, inviteeUserIDs, model.ReferralTypeSubscription)
	if err != nil {
		return nil, err
	}
	for inviteeUserID, count := range templateCounts {
		counts[inviteeUserID] += count
	}
	return counts, nil
}

func buildSubscriptionReferralInviteeDetailResponse(inviterUserID int, invitee *model.User) (gin.H, error) {
	var templateResponse gin.H
	if bindingViews, err := model.ListReferralTemplateBindingsByUser(inviterUserID, model.ReferralTypeSubscription); err == nil && len(bindingViews) > 0 {
		response, handled, responseErr := buildSubscriptionReferralInviteeDetailResponseFromTemplateBindings(inviterUserID, invitee, bindingViews)
		if responseErr != nil {
			return nil, responseErr
		}
		if handled {
			templateResponse = response
		}
	}

	inviter, err := model.GetUserById(inviterUserID, true)
	if err != nil {
		return nil, err
	}
	overrides, err := model.ListSubscriptionReferralInviteeOverrides(inviterUserID, invitee.Id)
	if err != nil {
		return nil, err
	}

	overrideGroups := make([]model.SubscriptionReferralOverride, 0, len(overrides))
	for _, override := range overrides {
		overrideGroups = append(overrideGroups, model.SubscriptionReferralOverride{Group: override.Group})
	}
	groups := collectSubscriptionReferralResponseGroups(listAllSubscriptionReferralGroups(), overrideGroups)
	availableGroups := make([]string, 0, len(groups))
	defaultInviteeRateBpsByGroup := make(map[string]int, len(groups))
	effectiveTotalRateBpsByGroup := make(map[string]int, len(groups))
	inviterSetting := inviter.GetSetting()
	for _, group := range groups {
		totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(inviterUserID, group)
		if totalRateBps <= 0 {
			continue
		}
		availableGroups = append(availableGroups, group)
		effectiveTotalRateBpsByGroup[group] = totalRateBps
		defaultInviteeRateBpsByGroup[group] = model.GetEffectiveSubscriptionReferralInviteeRateBps(inviterSetting, group, totalRateBps)
	}

	overrideViews := make([]gin.H, 0, len(overrides))
	for _, override := range overrides {
		overrideViews = append(overrideViews, gin.H{
			"group":            override.Group,
			"invitee_rate_bps": override.InviteeRateBps,
		})
	}

	legacyResponse := gin.H{
		"invitee": gin.H{
			"id":       invitee.Id,
			"username": invitee.Username,
			"group":    invitee.Group,
		},
		"available_groups":                  availableGroups,
		"default_invitee_rate_bps_by_group": defaultInviteeRateBpsByGroup,
		"effective_total_rate_bps_by_group": effectiveTotalRateBpsByGroup,
		"overrides":                         overrideViews,
	}
	if templateResponse != nil {
		return mergeSubscriptionReferralInviteeDetailResponses(templateResponse, legacyResponse), nil
	}
	return legacyResponse, nil
}

func buildSubscriptionReferralInviteeDetailResponseFromTemplateBindings(inviterUserID int, invitee *model.User, bindingViews []model.ReferralTemplateBindingView) (gin.H, bool, error) {
	activeBindings := make([]model.ReferralTemplateBindingView, 0, len(bindingViews))
	for _, view := range bindingViews {
		if view.Template.Enabled {
			activeBindings = append(activeBindings, view)
		}
	}
	if len(activeBindings) == 0 {
		return nil, false, nil
	}

	availableGroups := make([]string, 0, len(activeBindings))
	defaultInviteeRateBpsByGroup := make(map[string]int, len(activeBindings))
	effectiveTotalRateBpsByGroup := make(map[string]int, len(activeBindings))
	overrideRows := make([]gin.H, 0)

	overrides, err := model.ListReferralInviteeShareOverrides(inviterUserID, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		return nil, true, err
	}
	overrideByGroup := make(map[string]model.ReferralInviteeShareOverride, len(overrides))
	for _, override := range overrides {
		overrideByGroup[strings.TrimSpace(override.Group)] = override
	}

	for _, view := range activeBindings {
		group := strings.TrimSpace(view.Binding.Group)
		totalRateBps := subscriptionTemplateVisibleTotalRateBps(view.Template)
		availableGroups = append(availableGroups, group)
		effectiveTotalRateBpsByGroup[group] = totalRateBps
		defaultInviteeRateBpsByGroup[group] = model.ResolveBindingInviteeShareDefault(view)
		if override, ok := overrideByGroup[group]; ok {
			overrideRows = append(overrideRows, gin.H{
				"group":            group,
				"invitee_rate_bps": override.InviteeShareBps,
			})
		}
	}

	return gin.H{
		"invitee": gin.H{
			"id":       invitee.Id,
			"username": invitee.Username,
			"group":    invitee.Group,
		},
		"available_groups":                  availableGroups,
		"default_invitee_rate_bps_by_group": defaultInviteeRateBpsByGroup,
		"effective_total_rate_bps_by_group": effectiveTotalRateBpsByGroup,
		"overrides":                         overrideRows,
	}, true, nil
}

func collectSortedSubscriptionReferralGroupViews(templateViewsByGroup map[string]gin.H, legacyViews []gin.H) []gin.H {
	groupViews := make([]gin.H, 0, len(templateViewsByGroup)+len(legacyViews))
	seenGroups := make(map[string]struct{}, len(templateViewsByGroup)+len(legacyViews))

	for group, view := range templateViewsByGroup {
		if group == "" {
			continue
		}
		seenGroups[group] = struct{}{}
		groupViews = append(groupViews, view)
	}
	for _, view := range legacyViews {
		group := strings.TrimSpace(common.Interface2String(view["group"]))
		if group == "" {
			continue
		}
		if _, exists := seenGroups[group]; exists {
			continue
		}
		seenGroups[group] = struct{}{}
		groupViews = append(groupViews, view)
	}

	sort.Slice(groupViews, func(i, j int) bool {
		return strings.TrimSpace(common.Interface2String(groupViews[i]["group"])) < strings.TrimSpace(common.Interface2String(groupViews[j]["group"]))
	})
	return groupViews
}

func mergeSubscriptionReferralInviteeDetailResponses(templateResponse gin.H, legacyResponse gin.H) gin.H {
	availableGroupSet := make(map[string]struct{})
	availableGroups := make([]string, 0)

	appendGroup := func(group string) {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			return
		}
		if _, exists := availableGroupSet[trimmedGroup]; exists {
			return
		}
		availableGroupSet[trimmedGroup] = struct{}{}
		availableGroups = append(availableGroups, trimmedGroup)
	}

	for _, rawValue := range toStringSlice(templateResponse["available_groups"]) {
		appendGroup(rawValue)
	}
	for _, rawValue := range toStringSlice(legacyResponse["available_groups"]) {
		appendGroup(rawValue)
	}
	sort.Strings(availableGroups)

	defaultInviteeRateBpsByGroup := make(map[string]int)
	effectiveTotalRateBpsByGroup := make(map[string]int)
	overrideByGroup := make(map[string]gin.H)

	mergeIntMap := func(rawMap interface{}, target map[string]int) {
		if rawMap == nil {
			return
		}
		if typedMap, ok := rawMap.(map[string]int); ok {
			for group, value := range typedMap {
				target[strings.TrimSpace(group)] = value
			}
			return
		}
		if typedMap, ok := rawMap.(map[string]interface{}); ok {
			for group, value := range typedMap {
				target[strings.TrimSpace(group)] = interfaceToInt(value)
			}
		}
	}

	mergeIntMap(legacyResponse["default_invitee_rate_bps_by_group"], defaultInviteeRateBpsByGroup)
	mergeIntMap(templateResponse["default_invitee_rate_bps_by_group"], defaultInviteeRateBpsByGroup)
	mergeIntMap(legacyResponse["effective_total_rate_bps_by_group"], effectiveTotalRateBpsByGroup)
	mergeIntMap(templateResponse["effective_total_rate_bps_by_group"], effectiveTotalRateBpsByGroup)

	if templateOverrides, ok := templateResponse["overrides"].([]gin.H); ok {
		for _, row := range templateOverrides {
			group := strings.TrimSpace(common.Interface2String(row["group"]))
			if group != "" {
				overrideByGroup[group] = row
			}
		}
	}
	if legacyOverrides, ok := legacyResponse["overrides"].([]gin.H); ok {
		for _, row := range legacyOverrides {
			group := strings.TrimSpace(common.Interface2String(row["group"]))
			if group != "" {
				if _, exists := overrideByGroup[group]; !exists {
					overrideByGroup[group] = row
				}
			}
		}
	}

	overrideRows := make([]gin.H, 0, len(overrideByGroup))
	for _, group := range availableGroups {
		if row, exists := overrideByGroup[group]; exists {
			overrideRows = append(overrideRows, row)
		}
	}

	inviteePayload := templateResponse["invitee"]
	if inviteePayload == nil {
		inviteePayload = legacyResponse["invitee"]
	}

	return gin.H{
		"invitee": inviteePayload,
		"available_groups": availableGroups,
		"default_invitee_rate_bps_by_group": defaultInviteeRateBpsByGroup,
		"effective_total_rate_bps_by_group": effectiveTotalRateBpsByGroup,
		"overrides": overrideRows,
	}
}

func toStringSlice(value interface{}) []string {
	if value == nil {
		return nil
	}
	if typed, ok := value.([]string); ok {
		return typed
	}
	if typed, ok := value.([]interface{}); ok {
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, common.Interface2String(item))
		}
		return values
	}
	return nil
}

func interfaceToInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func listAllSubscriptionReferralGroups() []string {
	configuredRatioGroups := ratio_setting.GetGroupRatioCopy()
	groups := make([]string, 0, len(configuredRatioGroups)+len(model.ListSubscriptionReferralConfiguredGroups()))
	for group := range configuredRatioGroups {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			continue
		}
		groups = append(groups, trimmedGroup)
	}
	for _, group := range model.ListSubscriptionReferralConfiguredGroups() {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			continue
		}
		groups = append(groups, trimmedGroup)
	}
	return collectSubscriptionReferralResponseGroups(groups, nil)
}

func canDeleteSubscriptionReferralInviteeOverrideGroup(inviterUserID int, inviteeUserID int, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}
	if hasActiveTemplateBinding(inviterUserID, trimmedGroup) {
		return true
	}
	if isValidSubscriptionReferralGroup(trimmedGroup) {
		return true
	}
	overrides, err := model.ListSubscriptionReferralInviteeOverrides(inviterUserID, inviteeUserID)
	if err != nil {
		return false
	}
	for _, override := range overrides {
		if strings.TrimSpace(override.Group) == trimmedGroup {
			return true
		}
	}
	return false
}

func getActiveSubscriptionTemplateBindingView(userID int, group string) (*model.ReferralTemplateBindingView, error) {
	view, err := model.GetReferralTemplateBindingViewByUserAndScope(userID, model.ReferralTypeSubscription, group)
	if err != nil || view == nil {
		return view, err
	}
	if !view.Template.Enabled {
		return nil, nil
	}
	return view, nil
}

func hasActiveTemplateBinding(userID int, group string) bool {
	view, err := getActiveSubscriptionTemplateBindingView(userID, group)
	return err == nil && view != nil
}

func subscriptionTemplateVisibleTotalRateBps(template model.ReferralTemplate) int {
	if template.LevelType == model.ReferralLevelTypeTeam {
		return model.NormalizeSubscriptionReferralRateBps(template.TeamCapBps)
	}
	return model.NormalizeSubscriptionReferralRateBps(template.DirectCapBps)
}

func updateSubscriptionReferralSelfViaTemplateBinding(userID int, group string, inviteeRateBps int) (gin.H, bool, error) {
	view, err := getActiveSubscriptionTemplateBindingView(userID, group)
	if err != nil {
		return nil, true, err
	}
	if view == nil {
		return nil, false, nil
	}

	totalRateBps := subscriptionTemplateVisibleTotalRateBps(view.Template)
	if inviteeRateBps < 0 || inviteeRateBps > totalRateBps {
		return nil, true, errors.New("被邀请人比例不能超过总返佣率")
	}

	override := inviteeRateBps
	if _, err := model.SetReferralTemplateBindingInviteeShareOverride(userID, model.ReferralTypeSubscription, group, &override, userID); err != nil {
		return nil, true, err
	}

	return gin.H{
		"group":            group,
		"invitee_rate_bps": inviteeRateBps,
		"inviter_rate_bps": totalRateBps - inviteeRateBps,
		"total_rate_bps":   totalRateBps,
	}, true, nil
}

func deleteSubscriptionReferralSelfViaTemplateBinding(userID int, group string) (gin.H, bool, error) {
	view, err := getActiveSubscriptionTemplateBindingView(userID, group)
	if err != nil {
		return nil, true, err
	}
	if view == nil {
		return nil, false, nil
	}

	if _, err := model.SetReferralTemplateBindingInviteeShareOverride(userID, model.ReferralTypeSubscription, group, nil, userID); err != nil {
		return nil, true, err
	}

	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, true, err
	}
	groupViews := buildSubscriptionReferralSelfGroupViews(user)
	return gin.H{
		"enabled":              len(groupViews) > 0,
		"groups":               groupViews,
		"pending_reward_quota": user.AffQuota,
		"history_reward_quota": user.AffHistoryQuota,
		"inviter_count":        user.AffCount,
	}, true, nil
}
