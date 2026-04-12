package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UpdateSubscriptionReferralSelfRequest struct {
	InviteeRateBps int `json:"invitee_rate_bps"`
}

type AdminUpdateSubscriptionReferralSettingsRequest struct {
	Enabled      *bool `json:"enabled"`
	TotalRateBps *int  `json:"total_rate_bps"`
}

type AdminUpsertSubscriptionReferralOverrideRequest struct {
	TotalRateBps int `json:"total_rate_bps"`
}

func GetSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(userID)
	cfg := model.ResolveSubscriptionReferralConfig(totalRateBps, user.GetSetting())
	common.ApiSuccess(c, gin.H{
		"enabled":              cfg.Enabled,
		"total_rate_bps":       cfg.TotalRateBps,
		"invitee_rate_bps":     cfg.InviteeRateBps,
		"inviter_rate_bps":     cfg.InviterRateBps,
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

	totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(userID)
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
	setting.SubscriptionReferralInviteeRateBps = req.InviteeRateBps
	user.SetSetting(setting)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}

	cfg := model.ResolveSubscriptionReferralConfig(totalRateBps, setting)
	common.ApiSuccess(c, gin.H{
		"invitee_rate_bps": req.InviteeRateBps,
		"inviter_rate_bps": cfg.InviterRateBps,
		"total_rate_bps":   cfg.TotalRateBps,
	})
}

func AdminGetSubscriptionReferralSettings(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"enabled":        common.SubscriptionReferralEnabled,
		"total_rate_bps": model.NormalizeSubscriptionReferralRateBps(common.SubscriptionReferralGlobalRateBps),
	})
}

func AdminUpdateSubscriptionReferralSettings(c *gin.Context) {
	var req AdminUpdateSubscriptionReferralSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if req.Enabled != nil {
		if err := model.UpdateOption("SubscriptionReferralEnabled", strconv.FormatBool(*req.Enabled)); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if req.TotalRateBps != nil {
		rate := model.NormalizeSubscriptionReferralRateBps(*req.TotalRateBps)
		if err := model.UpdateOption("SubscriptionReferralGlobalRateBps", strconv.Itoa(rate)); err != nil {
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

	override, err := model.GetSubscriptionReferralOverrideByUserID(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}

	response := gin.H{
		"user_id":                  userID,
		"effective_total_rate_bps": model.GetEffectiveSubscriptionReferralTotalRateBps(userID),
		"has_override":             override != nil,
		"override_rate_bps":        nil,
	}
	if override != nil {
		response["override_rate_bps"] = override.TotalRateBps
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

	override, err := model.UpsertSubscriptionReferralOverride(userID, req.TotalRateBps, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"user_id":                  userID,
		"has_override":             true,
		"override_rate_bps":        override.TotalRateBps,
		"effective_total_rate_bps": model.GetEffectiveSubscriptionReferralTotalRateBps(userID),
	})
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

	if err := model.DeleteSubscriptionReferralOverrideByUserID(userID); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"user_id":                  userID,
		"has_override":             false,
		"effective_total_rate_bps": model.GetEffectiveSubscriptionReferralTotalRateBps(userID),
	})
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
