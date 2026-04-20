package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
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

	if err := model.UpdateSubscriptionReferralGlobalSetting(model.SubscriptionReferralGlobalSetting{
		TeamDecayRatio: req.TeamDecayRatio,
		TeamMaxDepth:   req.TeamMaxDepth,
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

	binding := &model.ReferralTemplateBinding{
		UserId:       userID,
		ReferralType: req.ReferralType,
		TemplateId:   req.TemplateId,
		CreatedBy:    c.GetInt("id"),
		UpdatedBy:    c.GetInt("id"),
	}

	saved, err := model.UpsertReferralTemplateBinding(binding)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, saved)
}
