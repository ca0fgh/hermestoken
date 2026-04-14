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
	if !isValidSubscriptionReferralGroup(targetGroup) {
		common.ApiErrorMsg(c, "分组不存在")
		return
	}
	if model.GetEffectiveSubscriptionReferralTotalRateBps(userID, targetGroup) <= 0 {
		common.ApiErrorMsg(c, "该分组未启用订阅返佣")
		return
	}

	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
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

	summaries, total, contributionTotal, err := model.ListSubscriptionReferralInviteeContributionSummaries(userID, keyword, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	overrideCountByInviteeID := listSubscriptionReferralInviteeOverrideCounts(userID, summaries)
	items := make([]gin.H, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, gin.H{
			"id":                   summary.InviteeUserId,
			"username":             summary.InviteeUsername,
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

func getOwnedSubscriptionReferralInvitee(c *gin.Context, inviterUserID int) (*model.User, bool) {
	inviteeID, _ := strconv.Atoi(c.Param("invitee_id"))
	if inviteeID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return nil, false
	}

	invitee, err := model.GetUserById(inviteeID, false)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if invitee.InviterId != inviterUserID {
		common.ApiErrorMsg(c, "被邀请人不存在")
		return nil, false
	}
	return invitee, true
}

func listSubscriptionReferralInviteeOverrideCounts(inviterUserID int, summaries []*model.SubscriptionReferralInviteeContributionSummary) map[int]int64 {
	counts := make(map[int]int64, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.InviteeUserId <= 0 {
			continue
		}
		overrides, err := model.ListSubscriptionReferralInviteeOverrides(inviterUserID, summary.InviteeUserId)
		if err != nil {
			continue
		}
		counts[summary.InviteeUserId] = int64(len(overrides))
	}
	return counts
}

func buildSubscriptionReferralInviteeDetailResponse(inviterUserID int, invitee *model.User) (gin.H, error) {
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
	groups := collectSubscriptionReferralResponseGroups(model.ListSubscriptionReferralConfiguredGroups(), overrideGroups)
	availableGroups := make([]string, 0, len(groups))
	defaultInviteeRateBpsByGroup := make(map[string]int, len(groups))
	effectiveTotalRateBpsByGroup := make(map[string]int, len(groups))
	inviterSetting := inviter.GetSetting()
	for _, group := range groups {
		totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(inviterUserID, group)
		if totalRateBps > 0 {
			availableGroups = append(availableGroups, group)
		}
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

	return gin.H{
		"invitee": gin.H{
			"id":       invitee.Id,
			"username": invitee.Username,
		},
		"available_groups":                  availableGroups,
		"default_invitee_rate_bps_by_group": defaultInviteeRateBpsByGroup,
		"effective_total_rate_bps_by_group": effectiveTotalRateBpsByGroup,
		"overrides":                         overrideViews,
	}, nil
}

func canDeleteSubscriptionReferralInviteeOverrideGroup(inviterUserID int, inviteeUserID int, group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
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
