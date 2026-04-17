package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func AdminListReferralTemplates(c *gin.Context) {
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

	template := &model.ReferralTemplate{
		ReferralType:           req.ReferralType,
		Group:                  req.Group,
		Name:                   req.Name,
		LevelType:              req.LevelType,
		Enabled:                req.Enabled,
		DirectCapBps:           req.DirectCapBps,
		TeamCapBps:             req.TeamCapBps,
		InviteeShareDefaultBps: req.InviteeShareDefaultBps,
		CreatedBy:              c.GetInt("id"),
		UpdatedBy:              c.GetInt("id"),
	}

	if err := model.CreateReferralTemplate(template); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, template)
}

func AdminUpdateReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}

	existing, err := model.GetReferralTemplateByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req dto.ReferralTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	existing.ReferralType = req.ReferralType
	existing.Group = req.Group
	existing.Name = req.Name
	existing.LevelType = req.LevelType
	existing.Enabled = req.Enabled
	existing.DirectCapBps = req.DirectCapBps
	existing.TeamCapBps = req.TeamCapBps
	existing.InviteeShareDefaultBps = req.InviteeShareDefaultBps
	existing.UpdatedBy = c.GetInt("id")

	if err := model.UpdateReferralTemplate(existing); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, existing)
}

func AdminDeleteReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}

	if err := model.DeleteReferralTemplate(id); err != nil {
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
