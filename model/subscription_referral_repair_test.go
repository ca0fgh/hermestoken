package model

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

func TestRepairSubscriptionReferralSettlementBatchByID_CorrectsLegacyTeamDirectSplit(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	order, plan, fixture := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
		ImmediateTeamCapBps:   5500,
		InviteeShareBps:       3000,
		Money:                 30,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	teamDirectRecord := findRewardRecordByComponent(t, records, "team_direct_reward")
	inviteeRecord := findRewardRecordByComponent(t, records, "invitee_reward")

	if err := DB.Model(&ReferralSettlementRecord{}).Where("id = ?", teamDirectRecord.Id).UpdateColumn("reward_quota", int64(1155)).Error; err != nil {
		t.Fatalf("failed to mutate team_direct_reward quota: %v", err)
	}
	if err := DB.Model(&ReferralSettlementRecord{}).Where("id = ?", inviteeRecord.Id).UpdateColumn("reward_quota", int64(495)).Error; err != nil {
		t.Fatalf("failed to mutate invitee_reward quota: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", fixture.ImmediateInviter.Id).Updates(map[string]any{
		"aff_quota":   1155,
		"aff_history": 1155,
	}).Error; err != nil {
		t.Fatalf("failed to mutate immediate inviter quotas: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", fixture.PayerUser.Id).Updates(map[string]any{
		"aff_quota":   495,
		"aff_history": 495,
	}).Error; err != nil {
		t.Fatalf("failed to mutate payer quotas: %v", err)
	}

	result, err := RepairSubscriptionReferralSettlementBatchByID(batch.Id)
	if err != nil {
		t.Fatalf("RepairSubscriptionReferralSettlementBatchByID() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("expected repair to change the batch")
	}

	_, repairedRecords := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	repairedTeamDirectRecord := findRewardRecordByComponent(t, repairedRecords, "team_direct_reward")
	repairedInviteeRecord := findRewardRecordByComponent(t, repairedRecords, "invitee_reward")

	if repairedTeamDirectRecord.RewardQuota != 750 {
		t.Fatalf("team_direct_reward quota = %d, want 750", repairedTeamDirectRecord.RewardQuota)
	}
	if repairedInviteeRecord.RewardQuota != 900 {
		t.Fatalf("invitee_reward quota = %d, want 900", repairedInviteeRecord.RewardQuota)
	}

	immediateInviter, err := GetUserById(fixture.ImmediateInviter.Id, false)
	if err != nil {
		t.Fatalf("GetUserById(immediate inviter) error = %v", err)
	}
	if immediateInviter.AffQuota != 750 || immediateInviter.AffHistoryQuota != 750 {
		t.Fatalf("immediate inviter quotas = (%d,%d), want (750,750)", immediateInviter.AffQuota, immediateInviter.AffHistoryQuota)
	}

	payerUser, err := GetUserById(fixture.PayerUser.Id, false)
	if err != nil {
		t.Fatalf("GetUserById(payer user) error = %v", err)
	}
	if payerUser.AffQuota != 900 || payerUser.AffHistoryQuota != 900 {
		t.Fatalf("payer quotas = (%d,%d), want (900,900)", payerUser.AffQuota, payerUser.AffHistoryQuota)
	}
}

func TestRepairSubscriptionReferralSettlementBatchByIDRejectsInsufficientAffQuota(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	order, plan, fixture := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
		ImmediateTeamCapBps:   5500,
		InviteeShareBps:       3000,
		Money:                 30,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	teamDirectRecord := findRewardRecordByComponent(t, records, "team_direct_reward")
	inviteeRecord := findRewardRecordByComponent(t, records, "invitee_reward")

	if err := DB.Model(&ReferralSettlementRecord{}).Where("id = ?", teamDirectRecord.Id).UpdateColumn("reward_quota", int64(1155)).Error; err != nil {
		t.Fatalf("failed to mutate team_direct_reward quota: %v", err)
	}
	if err := DB.Model(&ReferralSettlementRecord{}).Where("id = ?", inviteeRecord.Id).UpdateColumn("reward_quota", int64(495)).Error; err != nil {
		t.Fatalf("failed to mutate invitee_reward quota: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", fixture.ImmediateInviter.Id).Updates(map[string]any{
		"aff_quota":   100,
		"aff_history": 1155,
	}).Error; err != nil {
		t.Fatalf("failed to mutate immediate inviter quotas: %v", err)
	}

	if _, err := RepairSubscriptionReferralSettlementBatchByID(batch.Id); err == nil {
		t.Fatal("expected repair to reject insufficient aff_quota")
	}
}
