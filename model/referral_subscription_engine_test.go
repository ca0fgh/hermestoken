package model

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestResolveSubscriptionTemplateSettlementContext_TeamDirect(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
	})

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}
	if context.Mode != ReferralSettlementModeTeamDirect {
		t.Fatalf("Mode = %q, want %q", context.Mode, ReferralSettlementModeTeamDirect)
	}
	if got := len(context.TeamChain); got != 0 {
		t.Fatalf("len(TeamChain) = %d, want 0", got)
	}
	if context.ImmediateInviter == nil || context.ImmediateInviter.Id != fixture.ImmediateInviter.Id {
		t.Fatalf("ImmediateInviter = %+v, want user id %d", context.ImmediateInviter, fixture.ImmediateInviter.Id)
	}
}

func TestResolveSubscriptionTemplateSettlementContext_DirectWithMixedAncestors(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
			ReferralLevelTypeDirect,
			ReferralLevelTypeTeam,
		},
	})

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}
	if context.Mode != ReferralSettlementModeDirectWithTeamChain {
		t.Fatalf("Mode = %q, want %q", context.Mode, ReferralSettlementModeDirectWithTeamChain)
	}
	if got := len(context.TeamChain); got != 2 {
		t.Fatalf("len(TeamChain) = %d, want 2", got)
	}

	firstTeam := context.TeamChain[0]
	if firstTeam.UserId != fixture.Ancestors[0].Id {
		t.Fatalf("first team user id = %d, want %d", firstTeam.UserId, fixture.Ancestors[0].Id)
	}
	if firstTeam.PathDistance != 1 {
		t.Fatalf("first team path distance = %d, want 1", firstTeam.PathDistance)
	}
	if firstTeam.MatchedTeamIndex != 1 {
		t.Fatalf("first team matched index = %d, want 1", firstTeam.MatchedTeamIndex)
	}
	if math.Abs(firstTeam.WeightSnapshot-1) > 1e-9 {
		t.Fatalf("first team weight = %f, want 1", firstTeam.WeightSnapshot)
	}

	secondTeam := context.TeamChain[1]
	if secondTeam.UserId != fixture.Ancestors[2].Id {
		t.Fatalf("second team user id = %d, want %d", secondTeam.UserId, fixture.Ancestors[2].Id)
	}
	if secondTeam.PathDistance != 3 {
		t.Fatalf("second team path distance = %d, want 3", secondTeam.PathDistance)
	}
	if secondTeam.MatchedTeamIndex != 2 {
		t.Fatalf("second team matched index = %d, want 2", secondTeam.MatchedTeamIndex)
	}
	if math.Abs(secondTeam.WeightSnapshot-0.25) > 1e-9 {
		t.Fatalf("second team weight = %f, want 0.25", secondTeam.WeightSnapshot)
	}
}

func TestResolveSubscriptionTemplateSettlementContext_ZeroMaxDepthMeansUnlimited(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
			ReferralLevelTypeTeam,
			ReferralLevelTypeTeam,
			ReferralLevelTypeTeam,
			ReferralLevelTypeTeam,
			ReferralLevelTypeTeam,
		},
	})

	if err := UpdateSubscriptionReferralGlobalSetting(SubscriptionReferralGlobalSetting{
		TeamDecayRatio: 0.5,
		TeamMaxDepth:   0,
	}); err != nil {
		t.Fatalf("failed to update subscription referral global setting: %v", err)
	}

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}
	if got := len(context.TeamChain); got != 6 {
		t.Fatalf("len(TeamChain) = %d, want 6", got)
	}
}

func TestResolveSubscriptionTemplateSettlementContext_UsesGlobalTeamDecayRatio(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
			ReferralLevelTypeDirect,
			ReferralLevelTypeTeam,
		},
	})

	if err := UpdateSubscriptionReferralGlobalSetting(SubscriptionReferralGlobalSetting{
		TeamDecayRatio: 0.25,
		TeamMaxDepth:   0,
	}); err != nil {
		t.Fatalf("failed to update subscription referral global setting: %v", err)
	}

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}
	if got := len(context.TeamChain); got != 2 {
		t.Fatalf("len(TeamChain) = %d, want 2", got)
	}
	secondTeam := context.TeamChain[1]
	if math.Abs(secondTeam.WeightSnapshot-0.0625) > 1e-9 {
		t.Fatalf("second team weight = %f, want 0.0625", secondTeam.WeightSnapshot)
	}
}

func TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesTeamDirectBatch(t *testing.T) {
	order, plan, fixture := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
		InviteeShareBps:       4000,
		Money:                 10,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	if batch.SettlementMode != ReferralSettlementModeTeamDirect {
		t.Fatalf("SettlementMode = %q, want %q", batch.SettlementMode, ReferralSettlementModeTeamDirect)
	}
	if batch.ImmediateInviterUserId != fixture.ImmediateInviter.Id {
		t.Fatalf("ImmediateInviterUserId = %d, want %d", batch.ImmediateInviterUserId, fixture.ImmediateInviter.Id)
	}
	assertRewardComponents(t, records, []string{"team_direct_reward", "invitee_reward"})
}

func TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesMixedTeamChainSnapshots(t *testing.T) {
	order, plan, _ := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
			ReferralLevelTypeDirect,
			ReferralLevelTypeTeam,
		},
		InviteeShareBps: 3000,
		Money:           10,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	if batch.SettlementMode != ReferralSettlementModeDirectWithTeamChain {
		t.Fatalf("SettlementMode = %q, want %q", batch.SettlementMode, ReferralSettlementModeDirectWithTeamChain)
	}
	assertRewardComponents(t, records, []string{"direct_reward", "invitee_reward", "team_reward", "team_reward"})
	assertTeamChainSnapshotDistances(t, batch.TeamChainSnapshotJSON, []int{1, 3})
}

func TestResolveTeamDifferentialActivationReturnsNilWithoutMatchedTeam(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		InviteeShareBps:       3000,
	})

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}

	order := &SubscriptionOrder{Money: 10}
	activation := resolveTeamDifferentialActivation(context, order)
	if activation != nil {
		t.Fatalf("expected nil team differential activation, got %+v", activation)
	}
}

func TestResolveTeamDifferentialActivationReturnsAllocationWhenMatchedTeamExists(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
		},
		InviteeShareBps: 3000,
	})

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}

	order := &SubscriptionOrder{Money: 10}
	activation := resolveTeamDifferentialActivation(context, order)
	if activation == nil {
		t.Fatal("expected non-nil team differential activation")
	}
	if activation.DifferentialRateBps != 1500 {
		t.Fatalf("DifferentialRateBps = %d, want 1500", activation.DifferentialRateBps)
	}
	if activation.DifferentialQuota <= 0 {
		t.Fatalf("DifferentialQuota = %d, want > 0", activation.DifferentialQuota)
	}
}

func TestResolveTeamDifferentialActivationUsesFirstMatchedTeamRateOnly(t *testing.T) {
	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		ImmediateDirectCapBps: 1000,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
			ReferralLevelTypeDirect,
			ReferralLevelTypeTeam,
		},
		AncestorTeamCapBps: []int{
			3000,
			0,
			4500,
		},
	})

	context, err := ResolveSubscriptionTemplateSettlementContext(DB, ReferralTypeSubscription, "vip", fixture.PayerUser.Id, 0)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if context == nil {
		t.Fatal("expected non-nil settlement context")
	}
	if got := len(context.TeamChain); got != 2 {
		t.Fatalf("len(TeamChain) = %d, want 2", got)
	}

	order := &SubscriptionOrder{Money: 10}
	activation := resolveTeamDifferentialActivation(context, order)
	if activation == nil {
		t.Fatal("expected non-nil team differential activation")
	}
	if activation.DifferentialRateBps != 2000 {
		t.Fatalf("DifferentialRateBps = %d, want 2000 (first matched team 3000 - direct 1000)", activation.DifferentialRateBps)
	}
}

func TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_DirectWithoutMatchedTeamWritesNoTeamReward(t *testing.T) {
	order, plan, _ := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		InviteeShareBps:       3000,
		Money:                 10,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	_, records := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	assertRewardComponents(t, records, []string{"direct_reward", "invitee_reward"})
}

func TestApplySubscriptionReferralOnOrderSuccessTxAlwaysUsesTemplateEngine(t *testing.T) {
	order, plan, _ := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		Money:                 10,
	})

	if err := ApplySubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplySubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, _ := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	if batch.ReferralType != ReferralTypeSubscription {
		t.Fatalf("ReferralType = %q, want %q", batch.ReferralType, ReferralTypeSubscription)
	}
}

func TestListSubscriptionReferralInviteeContributionSummariesIncludesTemplateLedger(t *testing.T) {
	fixture, _, _ := seedTemplateContributionLedger(t)

	summaries, total, contributionTotal, err := ListSubscriptionReferralInviteeContributionSummaries(fixture.ImmediateInviter.Id, "", &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries() error = %v", err)
	}
	if total <= 0 {
		t.Fatalf("total = %d, want > 0", total)
	}
	if contributionTotal <= 0 {
		t.Fatalf("contributionTotal = %d, want > 0", contributionTotal)
	}
	if len(summaries) == 0 {
		t.Fatal("expected at least one invitee summary")
	}
	if summaries[0].InviteeUserId != fixture.PayerUser.Id {
		t.Fatalf("InviteeUserId = %d, want %d", summaries[0].InviteeUserId, fixture.PayerUser.Id)
	}
}

func TestListSubscriptionReferralInviteeContributionDetailsForDirectInviter(t *testing.T) {
	fixture, order, _ := seedTemplateContributionLedger(t)

	details, err := ListSubscriptionReferralInviteeContributionDetails(
		fixture.ImmediateInviter.Id,
		fixture.PayerUser.Id,
	)
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionDetails() error = %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("len(details) = %d, want 2", len(details))
	}
	if details[0].TradeNo != order.TradeNo || details[1].TradeNo != order.TradeNo {
		t.Fatalf("unexpected trade numbers: %+v", details)
	}
	if details[0].RewardComponent != "direct_reward" {
		t.Fatalf("details[0].RewardComponent = %q, want direct_reward", details[0].RewardComponent)
	}
	if details[0].RoleType != ReferralLevelTypeDirect {
		t.Fatalf("details[0].RoleType = %q, want %q", details[0].RoleType, ReferralLevelTypeDirect)
	}
	if details[1].RewardComponent != "invitee_reward" {
		t.Fatalf("details[1].RewardComponent = %q, want invitee_reward", details[1].RewardComponent)
	}
	if details[1].RoleType != ReferralLevelTypeDirect {
		t.Fatalf("details[1].RoleType = %q, want %q", details[1].RoleType, ReferralLevelTypeDirect)
	}
	if details[0].EffectiveRewardQuota <= 0 {
		t.Fatalf("details[0].EffectiveRewardQuota = %d, want > 0", details[0].EffectiveRewardQuota)
	}
	if details[1].EffectiveRewardQuota <= 0 {
		t.Fatalf("details[1].EffectiveRewardQuota = %d, want > 0", details[1].EffectiveRewardQuota)
	}
}

func TestListSubscriptionReferralInviteeContributionDetailsForTeamInviter(t *testing.T) {
	order, plan, fixture := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
		InviteeShareBps:       3000,
		Money:                 10,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	details, err := ListSubscriptionReferralInviteeContributionDetails(
		fixture.ImmediateInviter.Id,
		fixture.PayerUser.Id,
	)
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionDetails() error = %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("len(details) = %d, want 2", len(details))
	}
	if details[0].RewardComponent != "team_direct_reward" {
		t.Fatalf("details[0].RewardComponent = %q, want team_direct_reward", details[0].RewardComponent)
	}
	if details[0].RoleType != ReferralLevelTypeTeam {
		t.Fatalf("details[0].RoleType = %q, want %q", details[0].RoleType, ReferralLevelTypeTeam)
	}
	if details[1].RewardComponent != "invitee_reward" {
		t.Fatalf("details[1].RewardComponent = %q, want invitee_reward", details[1].RewardComponent)
	}
	if details[1].RoleType != ReferralLevelTypeTeam {
		t.Fatalf("details[1].RoleType = %q, want %q", details[1].RoleType, ReferralLevelTypeTeam)
	}
}

func TestReverseSubscriptionReferralByTradeNoReversesTemplateBatch(t *testing.T) {
	fixture, order, _ := seedTemplateContributionLedger(t)

	if err := ReverseSubscriptionReferralByTradeNo(order.TradeNo, 1); err != nil {
		t.Fatalf("ReverseSubscriptionReferralByTradeNo() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, order.TradeNo)
	if batch == nil {
		t.Fatal("expected settlement batch to remain after reverse")
	}
	for _, record := range records {
		if record.Status != SubscriptionReferralStatusReversed {
			t.Fatalf("record %s status = %q, want %q", record.RewardComponent, record.Status, SubscriptionReferralStatusReversed)
		}
	}

	immediateInviter, err := GetUserById(fixture.ImmediateInviter.Id, false)
	if err != nil {
		t.Fatalf("GetUserById(immediate inviter) error = %v", err)
	}
	if immediateInviter.AffQuota != 0 {
		t.Fatalf("immediate inviter aff_quota = %d, want 0", immediateInviter.AffQuota)
	}
}
