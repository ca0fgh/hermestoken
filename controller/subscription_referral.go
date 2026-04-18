package controller

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type upsertSubscriptionReferralInviteeOverrideRequest struct {
	Group          string `json:"group"`
	InviteeRateBps int    `json:"invitee_rate_bps"`
}

type deleteSubscriptionReferralOverrideRequest struct {
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

	groupViews, err := buildSubscriptionReferralSelfGroupViews(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enabled":              len(groupViews) > 0,
		"groups":               groupViews,
		"pending_reward_quota": user.AffQuota,
		"history_reward_quota": user.AffHistoryQuota,
		"inviter_count":        inviterCount,
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

	var req upsertSubscriptionReferralInviteeOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	group := strings.TrimSpace(req.Group)
	if group == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}

	view, err := getActiveSubscriptionTemplateBindingView(userID, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if view == nil {
		common.ApiErrorMsg(c, "该分组未启用订阅返佣")
		return
	}

	totalRateBps := subscriptionTemplateVisibleTotalRateBps(view.Template)
	if req.InviteeRateBps < 0 || req.InviteeRateBps > totalRateBps {
		common.ApiErrorMsg(c, "被邀请人比例不能超过总返佣率")
		return
	}

	if _, err := model.UpsertReferralInviteeShareOverride(userID, invitee.Id, model.ReferralTypeSubscription, group, req.InviteeRateBps, userID); err != nil {
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

	view, err := getActiveSubscriptionTemplateBindingView(userID, targetGroup)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if view == nil {
		common.ApiErrorMsg(c, "该分组未启用订阅返佣")
		return
	}

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

func buildSubscriptionReferralSelfGroupViews(userID int) ([]gin.H, error) {
	bindingViews, err := model.ListReferralTemplateBindingsByUser(userID, model.ReferralTypeSubscription)
	if err != nil {
		return nil, err
	}

	groupViews := make([]gin.H, 0, len(bindingViews))
	for _, view := range bindingViews {
		if !view.Template.Enabled {
			continue
		}
		scopePayload := buildSubscriptionReferralScopePayload(view, nil)
		scopePayload["invitee_rate_bps"] = scopePayload["default_invitee_rate_bps"]
		scopePayload["inviter_rate_bps"] = scopePayload["effective_inviter_rate_bps"]
		groupViews = append(groupViews, scopePayload)
	}

	sort.Slice(groupViews, func(i, j int) bool {
		return strings.TrimSpace(common.Interface2String(groupViews[i]["group"])) < strings.TrimSpace(common.Interface2String(groupViews[j]["group"]))
	})
	return groupViews, nil
}

func listSubscriptionReferralInviteeOverrideCounts(inviterUserID int, summaries []*model.SubscriptionReferralInviteeContributionSummary) (map[int]int64, error) {
	inviteeUserIDs := make([]int, 0, len(summaries))
	for _, summary := range summaries {
		if summary == nil || summary.InviteeUserId <= 0 {
			continue
		}
		inviteeUserIDs = append(inviteeUserIDs, summary.InviteeUserId)
	}
	return model.ListReferralInviteeShareOverrideCounts(inviterUserID, inviteeUserIDs, model.ReferralTypeSubscription)
}

func buildSubscriptionReferralScopePayload(view model.ReferralTemplateBindingView, override *model.ReferralInviteeShareOverride) gin.H {
	group := strings.TrimSpace(view.Binding.Group)
	totalRateBps := subscriptionTemplateVisibleTotalRateBps(view.Template)
	defaultInviteeRateBps := model.ResolveBindingInviteeShareDefault(view)
	if defaultInviteeRateBps > totalRateBps {
		defaultInviteeRateBps = totalRateBps
	}

	overrideInviteeRateBps := 0
	effectiveInviteeRateBps := defaultInviteeRateBps
	hasOverride := false
	if override != nil {
		hasOverride = true
		overrideInviteeRateBps = model.NormalizeSubscriptionReferralRateBps(override.InviteeShareBps)
		if overrideInviteeRateBps > totalRateBps {
			overrideInviteeRateBps = totalRateBps
		}
		effectiveInviteeRateBps = overrideInviteeRateBps
	}

	return gin.H{
		"group":                      group,
		"type":                       "subscription",
		"template_id":                view.Template.Id,
		"template_name":              view.Template.Name,
		"level_type":                 view.Template.LevelType,
		"total_rate_bps":             totalRateBps,
		"default_invitee_rate_bps":   defaultInviteeRateBps,
		"override_invitee_rate_bps":  overrideInviteeRateBps,
		"effective_invitee_rate_bps": effectiveInviteeRateBps,
		"effective_inviter_rate_bps": totalRateBps - effectiveInviteeRateBps,
		"has_override":               hasOverride,
	}
}

func buildSubscriptionReferralInviteeDetailResponse(inviterUserID int, invitee *model.User) (gin.H, error) {
	bindingViews, err := model.ListReferralTemplateBindingsByUser(inviterUserID, model.ReferralTypeSubscription)
	if err != nil {
		return nil, err
	}

	activeBindings := make([]model.ReferralTemplateBindingView, 0, len(bindingViews))
	for _, view := range bindingViews {
		if view.Template.Enabled {
			activeBindings = append(activeBindings, view)
		}
	}

	scopeRows := make([]gin.H, 0, len(activeBindings))

	overrides, err := model.ListReferralInviteeShareOverrides(inviterUserID, invitee.Id, model.ReferralTypeSubscription)
	if err != nil {
		return nil, err
	}
	overrideByGroup := make(map[string]model.ReferralInviteeShareOverride, len(overrides))
	for _, override := range overrides {
		overrideByGroup[strings.TrimSpace(override.Group)] = override
	}

	for _, view := range activeBindings {
		group := strings.TrimSpace(view.Binding.Group)
		if override, ok := overrideByGroup[group]; ok {
			scopeRows = append(scopeRows, buildSubscriptionReferralScopePayload(view, &override))
			continue
		}
		scopeRows = append(scopeRows, buildSubscriptionReferralScopePayload(view, nil))
	}

	sort.Slice(scopeRows, func(i, j int) bool {
		return strings.TrimSpace(common.Interface2String(scopeRows[i]["group"])) < strings.TrimSpace(common.Interface2String(scopeRows[j]["group"]))
	})

	contributionDetails, err := model.ListSubscriptionReferralInviteeContributionDetails(inviterUserID, invitee.Id)
	if err != nil {
		return nil, err
	}
	detailRows := make([]gin.H, 0, len(contributionDetails))
	for _, detail := range contributionDetails {
		if detail == nil {
			continue
		}
		detailRows = append(detailRows, gin.H{
			"batch_id":                detail.BatchId,
			"trade_no":                detail.TradeNo,
			"group":                   detail.Group,
			"reward_component":        detail.RewardComponent,
			"source_reward_component": detail.SourceRewardComponent,
			"role_type":               detail.RoleType,
			"reward_quota":            detail.RewardQuota,
			"reversed_quota":          detail.ReversedQuota,
			"debt_quota":              detail.DebtQuota,
			"effective_reward_quota":  detail.EffectiveRewardQuota,
			"status":                  detail.Status,
			"settled_at":              detail.SettledAt,
			"created_at":              detail.CreatedAt,
		})
	}

	return gin.H{
		"invitee": gin.H{
			"id":       invitee.Id,
			"username": invitee.Username,
			"group":    invitee.Group,
		},
		"scopes":               scopeRows,
		"contribution_details": detailRows,
	}, nil
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

func resolveSubscriptionReferralOverrideDeleteGroup(c *gin.Context) (string, error) {
	group := strings.TrimSpace(c.Query("group"))
	if group != "" {
		return group, nil
	}
	if c.Request.ContentLength > 0 {
		var req deleteSubscriptionReferralOverrideRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return "", err
		}
		return strings.TrimSpace(req.Group), nil
	}
	return "", nil
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
