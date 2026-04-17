package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func seedSubscriptionReferralControllerUser(t *testing.T, username string, inviterID int, setting dto.UserSetting) *model.User {
	t.Helper()

	user := &model.User{
		Username:  username,
		Password:  "password",
		AffCode:   username + "_code",
		Group:     "default",
		InviterId: inviterID,
	}
	user.SetSetting(setting)
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func seedActiveSubscriptionReferralBinding(t *testing.T, userID int, group string, levelType string, inviteeShareDefaultBps int) *model.ReferralTemplate {
	t.Helper()

	template := &model.ReferralTemplate{
		ReferralType:           model.ReferralTypeSubscription,
		Group:                  group,
		Name:                   group + "-" + levelType,
		LevelType:              levelType,
		Enabled:                true,
		DirectCapBps:           1200,
		TeamCapBps:             2500,
		InviteeShareDefaultBps: inviteeShareDefaultBps,
		CreatedBy:              1,
		UpdatedBy:              1,
	}
	if err := model.CreateReferralTemplate(template); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	if _, err := model.UpsertReferralTemplateBinding(&model.ReferralTemplateBinding{
		UserId:       userID,
		ReferralType: model.ReferralTypeSubscription,
		Group:        group,
		TemplateId:   template.Id,
		CreatedBy:    1,
		UpdatedBy:    1,
	}); err != nil {
		t.Fatalf("failed to create template binding: %v", err)
	}

	return template
}

func seedSubscriptionReferralControllerTradeNo(t *testing.T) string {
	t.Helper()

	common.QuotaPerUnit = 100

	inviter := seedSubscriptionReferralControllerUser(t, "admin-inviter", 0, dto.UserSetting{})
	invitee := seedSubscriptionReferralControllerUser(t, "admin-invitee", inviter.Id, dto.UserSetting{})
	seedActiveSubscriptionReferralBinding(t, inviter.Id, "default", model.ReferralLevelTypeDirect, 500)

	plan := seedSubscriptionPlan(t, model.DB, "referral-admin-plan")
	plan.UpgradeGroup = "default"
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error; err != nil {
		t.Fatalf("failed to update referral admin plan upgrade group: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)

	order := &model.SubscriptionOrder{
		UserId:        invitee.Id,
		PlanId:        plan.Id,
		Money:         10,
		TradeNo:       "trade-ref-admin",
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := model.DB.Create(order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	if err := model.CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("failed to complete referral order: %v", err)
	}
	return order.TradeNo
}
