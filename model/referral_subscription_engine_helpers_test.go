package model

import (
	"fmt"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

type seedTemplateEngineFixtureInput struct {
	Group                 string
	ImmediateInviterLevel string
	AncestorLevels        []string
	InviteeShareBps       int
	ImmediateDirectCapBps int
	ImmediateTeamCapBps   int
	AncestorDirectCapBps  []int
	AncestorTeamCapBps    []int
}

type templateEngineFixture struct {
	PayerUser        *User
	ImmediateInviter *User
	Ancestors        []*User
}

func seedTemplateEngineFixture(t *testing.T, input seedTemplateEngineFixtureInput) *templateEngineFixture {
	t.Helper()

	db := setupReferralTemplateDB(t)
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("failed to migrate users: %v", err)
	}

	payer := &User{
		Id:       501,
		Username: "template_payer",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "template_payer_code",
	}
	if err := db.Create(payer).Error; err != nil {
		t.Fatalf("failed to create payer: %v", err)
	}

	immediateInviter := &User{
		Id:       401,
		Username: "template_immediate_inviter",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "template_immediate_inviter_code",
	}
	if err := db.Create(immediateInviter).Error; err != nil {
		t.Fatalf("failed to create immediate inviter: %v", err)
	}

	payer.InviterId = immediateInviter.Id
	if err := db.Model(&User{}).Where("id = ?", payer.Id).Update("inviter_id", payer.InviterId).Error; err != nil {
		t.Fatalf("failed to update payer inviter id: %v", err)
	}

	createTemplateBindingForUser(
		t,
		immediateInviter.Id,
		input.Group,
		input.ImmediateInviterLevel,
		input.InviteeShareBps,
		resolveTemplateDirectCapBps(input.ImmediateInviterLevel, input.ImmediateDirectCapBps),
		resolveTemplateTeamCapBps(input.ImmediateInviterLevel, input.ImmediateTeamCapBps),
	)

	fixture := &templateEngineFixture{
		PayerUser:        payer,
		ImmediateInviter: immediateInviter,
		Ancestors:        make([]*User, 0, len(input.AncestorLevels)),
	}

	parentID := immediateInviter.Id
	nextID := 301
	for idx, levelType := range input.AncestorLevels {
		ancestor := &User{
			Id:       nextID,
			Username: fmt.Sprintf("template_ancestor_%d", idx+1),
			Password: "password",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			AffCode:  fmt.Sprintf("template_ancestor_%d_code", idx+1),
		}
		if err := db.Create(ancestor).Error; err != nil {
			t.Fatalf("failed to create ancestor: %v", err)
		}

		if err := db.Model(&User{}).Where("id = ?", parentID).Update("inviter_id", ancestor.Id).Error; err != nil {
			t.Fatalf("failed to update inviter chain: %v", err)
		}

		createTemplateBindingForUser(
			t,
			ancestor.Id,
			input.Group,
			levelType,
			0,
			resolveTemplateDirectCapBps(levelType, rateAt(input.AncestorDirectCapBps, idx)),
			resolveTemplateTeamCapBps(levelType, rateAt(input.AncestorTeamCapBps, idx)),
		)
		fixture.Ancestors = append(fixture.Ancestors, ancestor)
		parentID = ancestor.Id
		nextID--
	}

	return fixture
}

func createTemplateBindingForUser(t *testing.T, userID int, group string, levelType string, inviteeShareDefaultBps int, directCapBps int, teamCapBps int) {
	t.Helper()

	template := &ReferralTemplate{
		Name:                   fmt.Sprintf("%s_template_%d", levelType, userID),
		ReferralType:           ReferralTypeSubscription,
		Group:                  group,
		LevelType:              levelType,
		Enabled:                true,
		DirectCapBps:           directCapBps,
		TeamCapBps:             teamCapBps,
		InviteeShareDefaultBps: inviteeShareDefaultBps,
	}
	if err := CreateReferralTemplate(template); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	binding := &ReferralTemplateBinding{
		UserId:       userID,
		ReferralType: ReferralTypeSubscription,
		Group:        group,
		TemplateId:   template.Id,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}
	if _, err := UpsertReferralTemplateBinding(binding); err != nil {
		t.Fatalf("failed to create template binding: %v", err)
	}
}

func rateAt(items []int, idx int) int {
	if idx < 0 || idx >= len(items) {
		return 0
	}
	return items[idx]
}

func resolveTemplateDirectCapBps(levelType string, configured int) int {
	if configured > 0 {
		return configured
	}
	if levelType == ReferralLevelTypeDirect {
		return 1000
	}
	return 0
}

func resolveTemplateTeamCapBps(levelType string, configured int) int {
	if configured > 0 {
		return configured
	}
	if levelType == ReferralLevelTypeTeam {
		return 2500
	}
	return 0
}

type seedTemplateSettlementOrderInput struct {
	Group                 string
	ImmediateInviterLevel string
	AncestorLevels        []string
	InviteeShareBps       int
	ImmediateDirectCapBps int
	ImmediateTeamCapBps   int
	AncestorDirectCapBps  []int
	AncestorTeamCapBps    []int
	Money                 float64
}

func seedTemplateSettlementOrder(t *testing.T, input seedTemplateSettlementOrderInput) (*SubscriptionOrder, *SubscriptionPlan, *templateEngineFixture) {
	t.Helper()

	fixture := seedTemplateEngineFixture(t, seedTemplateEngineFixtureInput{
		Group:                 input.Group,
		ImmediateInviterLevel: input.ImmediateInviterLevel,
		AncestorLevels:        input.AncestorLevels,
		InviteeShareBps:       input.InviteeShareBps,
		ImmediateDirectCapBps: input.ImmediateDirectCapBps,
		ImmediateTeamCapBps:   input.ImmediateTeamCapBps,
		AncestorDirectCapBps:  input.AncestorDirectCapBps,
		AncestorTeamCapBps:    input.AncestorTeamCapBps,
	})

	plan := &SubscriptionPlan{
		Title:         "template_engine_plan",
		PriceAmount:   input.Money,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		UpgradeGroup:  input.Group,
	}
	if err := DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create subscription plan: %v", err)
	}

	order := &SubscriptionOrder{
		UserId:        fixture.PayerUser.Id,
		PlanId:        plan.Id,
		Money:         input.Money,
		TradeNo:       fmt.Sprintf("template_order_%d", common.GetTimestamp()),
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := DB.Create(order).Error; err != nil {
		t.Fatalf("failed to create subscription order: %v", err)
	}

	return order, plan, fixture
}

func loadReferralSettlementBatchByTradeNo(t *testing.T, tradeNo string) (*ReferralSettlementBatch, []ReferralSettlementRecord) {
	t.Helper()

	var batch ReferralSettlementBatch
	if err := DB.Where("source_trade_no = ?", tradeNo).First(&batch).Error; err != nil {
		t.Fatalf("failed to load settlement batch: %v", err)
	}

	var records []ReferralSettlementRecord
	if err := DB.Where("batch_id = ?", batch.Id).Order("id ASC").Find(&records).Error; err != nil {
		t.Fatalf("failed to load settlement records: %v", err)
	}
	return &batch, records
}

func assertRewardComponents(t *testing.T, records []ReferralSettlementRecord, expected []string) {
	t.Helper()

	if len(records) != len(expected) {
		t.Fatalf("record length = %d, want %d", len(records), len(expected))
	}
	for idx, record := range records {
		if record.RewardComponent != expected[idx] {
			t.Fatalf("records[%d].RewardComponent = %q, want %q", idx, record.RewardComponent, expected[idx])
		}
	}
}

func findRewardRecordByComponent(t *testing.T, records []ReferralSettlementRecord, component string) ReferralSettlementRecord {
	t.Helper()

	for _, record := range records {
		if record.RewardComponent == component {
			return record
		}
	}

	t.Fatalf("missing reward component %q in %+v", component, records)
	return ReferralSettlementRecord{}
}

func assertTeamChainSnapshotDistances(t *testing.T, raw string, expected []int) {
	t.Helper()

	var snapshot []struct {
		PathDistance int `json:"path_distance"`
	}
	if err := common.Unmarshal([]byte(raw), &snapshot); err != nil {
		t.Fatalf("failed to decode team chain snapshot: %v", err)
	}
	if len(snapshot) != len(expected) {
		t.Fatalf("snapshot length = %d, want %d", len(snapshot), len(expected))
	}
	for idx, item := range snapshot {
		if item.PathDistance != expected[idx] {
			t.Fatalf("snapshot[%d].PathDistance = %d, want %d", idx, item.PathDistance, expected[idx])
		}
	}
}

func seedTemplateContributionLedger(t *testing.T) (*templateEngineFixture, *SubscriptionOrder, *SubscriptionPlan) {
	t.Helper()

	order, plan, fixture := seedTemplateSettlementOrder(t, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels: []string{
			ReferralLevelTypeTeam,
			ReferralLevelTypeDirect,
			ReferralLevelTypeTeam,
		},
		InviteeShareBps: 300,
		Money:           10,
	})

	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(DB, order, plan); err != nil {
		t.Fatalf("failed to apply template settlement: %v", err)
	}
	return fixture, order, plan
}
