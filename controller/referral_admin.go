package controller

import (
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-gonic/gin"
)

func referralTemplateRequestGroups(req dto.ReferralTemplateUpsertRequest) []string {
	if len(req.Groups) > 0 {
		return req.Groups
	}
	if trimmed := strings.TrimSpace(req.Group); trimmed != "" {
		return []string{trimmed}
	}
	return nil
}

func referralTemplateRequestGroupRates(req dto.ReferralTemplateUpsertRequest) []model.ReferralTemplateGroupRate {
	if len(req.GroupRates) == 0 {
		return nil
	}
	groupRates := make([]model.ReferralTemplateGroupRate, 0, len(req.GroupRates))
	for _, rate := range req.GroupRates {
		groupRates = append(groupRates, model.ReferralTemplateGroupRate{
			Group:                  rate.Group,
			DirectCapBps:           rate.DirectCapBps,
			TeamCapBps:             rate.TeamCapBps,
			InviteeShareDefaultBps: rate.InviteeShareDefaultBps,
		})
	}
	return groupRates
}

func AdminListReferralTemplates(c *gin.Context) {
	if strings.EqualFold(strings.TrimSpace(c.Query("view")), "bundle") {
		bundles, err := model.ListReferralTemplateBundles(c.Query("referral_type"))
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{"items": bundles})
		return
	}

	templates, err := model.ListReferralTemplates(c.Query("referral_type"), c.Query("group"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": templates})
}

func AdminGetSubscriptionReferralGlobalSetting(c *gin.Context) {
	common.ApiSuccess(c, model.GetSubscriptionReferralGlobalSetting())
}

func AdminUpdateSubscriptionReferralGlobalSetting(c *gin.Context) {
	var req dto.SubscriptionReferralGlobalSettingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	currentSetting := model.GetSubscriptionReferralGlobalSetting()
	teamDecayRatio := currentSetting.TeamDecayRatio
	if req.TeamDecayRatio != nil {
		teamDecayRatio = *req.TeamDecayRatio
	}
	teamMaxDepth := currentSetting.TeamMaxDepth
	if req.TeamMaxDepth != nil {
		teamMaxDepth = *req.TeamMaxDepth
	}
	autoAssignInviteeTemplate := currentSetting.AutoAssignInviteeTemplate
	if req.AutoAssignInviteeTemplate != nil {
		autoAssignInviteeTemplate = *req.AutoAssignInviteeTemplate
	}
	planOpenToAllUsers := currentSetting.PlanOpenToAllUsers
	if req.PlanOpenToAllUsers != nil {
		planOpenToAllUsers = *req.PlanOpenToAllUsers
	}
	if err := model.UpdateSubscriptionReferralGlobalSetting(model.SubscriptionReferralGlobalSetting{
		TeamDecayRatio:            teamDecayRatio,
		TeamMaxDepth:              teamMaxDepth,
		AutoAssignInviteeTemplate: autoAssignInviteeTemplate,
		PlanOpenToAllUsers:        planOpenToAllUsers,
	}); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, model.GetSubscriptionReferralGlobalSetting())
}

func AdminCreateReferralTemplate(c *gin.Context) {
	var req dto.ReferralTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           req.ReferralType,
		Groups:                 referralTemplateRequestGroups(req),
		Name:                   req.Name,
		LevelType:              req.LevelType,
		Enabled:                req.Enabled,
		DirectCapBps:           req.DirectCapBps,
		TeamCapBps:             req.TeamCapBps,
		InviteeShareDefaultBps: req.InviteeShareDefaultBps,
		GroupRates:             referralTemplateRequestGroupRates(req),
	}, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": rows})
}

func AdminUpdateReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}

	var req dto.ReferralTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	rows, err := model.UpdateReferralTemplateBundleByTemplateID(id, model.ReferralTemplateBundleUpsertInput{
		ReferralType:           req.ReferralType,
		Groups:                 referralTemplateRequestGroups(req),
		Name:                   req.Name,
		LevelType:              req.LevelType,
		Enabled:                req.Enabled,
		DirectCapBps:           req.DirectCapBps,
		TeamCapBps:             req.TeamCapBps,
		InviteeShareDefaultBps: req.InviteeShareDefaultBps,
		GroupRates:             referralTemplateRequestGroupRates(req),
	}, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": rows})
}

func AdminDeleteReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}

	if err := model.DeleteReferralTemplateBundleByTemplateID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

func AdminListReferralTemplateBindingsByUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}

	if strings.EqualFold(strings.TrimSpace(c.Query("view")), "bundle") {
		items, err := model.ListReferralTemplateBindingBundlesByUser(userID, c.Query("referral_type"))
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{"items": items})
		return
	}

	items, err := model.ListReferralTemplateBindingsByUser(userID, c.Query("referral_type"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

func AdminUpsertReferralTemplateBindingForUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}

	var req dto.ReferralTemplateBindingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	saved, err := model.UpsertReferralTemplateBindingBundleForUser(
		userID,
		req.ReferralType,
		req.TemplateId,
		req.ReplaceBindingIds,
		c.GetInt("id"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": saved})
}
