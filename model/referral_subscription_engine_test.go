package model

import (
	"math"
	"testing"
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
