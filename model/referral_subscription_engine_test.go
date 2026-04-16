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

func TestApplySubscriptionReferralOnOrderSuccessTx_DispatchesByEngineRoute(t *testing.T) {
	order, plan, _ := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		Money:                 10,
	})

	if _, err := UpsertReferralEngineRoute(&ReferralEngineRoute{
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		EngineMode:   ReferralEngineModeTemplate,
		CreatedBy:    1,
		UpdatedBy:    1,
	}); err != nil {
		t.Fatalf("failed to upsert referral engine route: %v", err)
	}

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
