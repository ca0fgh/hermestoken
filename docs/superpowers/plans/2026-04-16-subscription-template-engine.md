# Subscription Template Settlement Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `subscription_referral` template-mode settlement, including `team_direct` / `direct_with_team_chain`, immutable ledger snapshots, engine-route dispatch, and reverse/read-side compatibility.

**Architecture:** Keep the old `subscription_referral_records` flow untouched for `legacy` groups and add a new template settlement engine behind `referral_engine_routes`. The new engine resolves the immediate inviter’s active template, computes the team chain and invitee-share split once per order, writes generic settlement batches/records, updates user quota balances, and exposes compatible read/reverse behavior.

**Tech Stack:** Go 1.26, Gorm transactions, existing subscription order flow in `model/subscription.go`, current referral helper/test fixtures.

---

### Task 1: Build Active Template Resolution and Team-Chain Traversal

**Files:**
- Create: `model/referral_subscription_engine.go`
- Create: `model/referral_subscription_engine_test.go`
- Create: `model/referral_subscription_engine_test_helpers.go`
- Modify: `model/referral_template.go`
- Test: `model/referral_subscription_engine_test.go`

- [ ] **Step 1: Write the failing resolver tests**

```go
func TestResolveSubscriptionTemplateSettlementContext_TeamDirect(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	seedTemplateEngineFixture(t, db, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
	})

	ctx, err := ResolveSubscriptionTemplateSettlementContext(db, ReferralTypeSubscription, "vip", 5, 2)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if ctx.Mode != ReferralSettlementModeTeamDirect {
		t.Fatalf("Mode = %q, want %q", ctx.Mode, ReferralSettlementModeTeamDirect)
	}
	if len(ctx.TeamChain) != 0 {
		t.Fatalf("TeamChain length = %d, want 0", len(ctx.TeamChain))
	}
}

func TestResolveSubscriptionTemplateSettlementContext_DirectWithMixedAncestors(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	seedTemplateEngineFixture(t, db, seedTemplateEngineFixtureInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels:        []string{ReferralLevelTypeTeam, ReferralLevelTypeDirect, ReferralLevelTypeTeam},
	})

	ctx, err := ResolveSubscriptionTemplateSettlementContext(db, ReferralTypeSubscription, "vip", 5, 2)
	if err != nil {
		t.Fatalf("ResolveSubscriptionTemplateSettlementContext() error = %v", err)
	}
	if ctx.Mode != ReferralSettlementModeDirectWithTeamChain {
		t.Fatalf("Mode = %q, want %q", ctx.Mode, ReferralSettlementModeDirectWithTeamChain)
	}
	if got, want := len(ctx.TeamChain), 2; got != want {
		t.Fatalf("len(TeamChain) = %d, want %d", got, want)
	}
	if ctx.TeamChain[0].PathDistance != 1 || ctx.TeamChain[1].PathDistance != 3 {
		t.Fatalf("unexpected team chain distances: %+v", ctx.TeamChain)
	}
}
```

- [ ] **Step 2: Run the resolver tests to verify they fail**

Run: `go test ./model -run 'TestResolveSubscriptionTemplateSettlementContext_TeamDirect|TestResolveSubscriptionTemplateSettlementContext_DirectWithMixedAncestors'`
Expected: FAIL with `undefined: ResolveSubscriptionTemplateSettlementContext` and missing settlement-mode constants.

- [ ] **Step 3: Implement the settlement context resolver**

```go
const (
	ReferralSettlementModeTeamDirect          = "team_direct"
	ReferralSettlementModeDirectWithTeamChain = "direct_with_team_chain"
)

type ReferralSettlementContext struct {
	ReferralType      string
	Group             string
	PayerUser         *User
	ImmediateInviter  *User
	ActiveTemplate    *ReferralTemplate
	ActiveBinding     *ReferralTemplateBinding
	Mode              string
	TeamChain         []ResolvedTeamNode
}

type ResolvedTeamNode struct {
	UserId            int
	BindingId         int
	TemplateId        int
	PathDistance      int
	MatchedTeamIndex  int
	WeightSnapshot    float64
	ShareSnapshot     float64
}

func ResolveSubscriptionTemplateSettlementContext(tx *gorm.DB, referralType string, group string, payerUserID int, orderID int) (*ReferralSettlementContext, error) {
	payer, inviter, activeBinding, activeTemplate, err := loadImmediateInviterTemplateScope(tx, referralType, group, payerUserID)
	if err != nil || inviter == nil || activeTemplate == nil {
		return nil, err
	}

	ctx := &ReferralSettlementContext{
		ReferralType:     referralType,
		Group:            strings.TrimSpace(group),
		PayerUser:        payer,
		ImmediateInviter: inviter,
		ActiveBinding:    activeBinding,
		ActiveTemplate:   activeTemplate,
	}

	if activeTemplate.LevelType == ReferralLevelTypeTeam {
		ctx.Mode = ReferralSettlementModeTeamDirect
		ctx.TeamChain = []ResolvedTeamNode{}
		return ctx, nil
	}

	ctx.Mode = ReferralSettlementModeDirectWithTeamChain
	ctx.TeamChain, err = resolveSubscriptionTeamChain(tx, inviter.InviterId, referralType, group, activeTemplate.TeamDecayRatio, activeTemplate.TeamMaxDepth)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func loadImmediateInviterTemplateScope(tx *gorm.DB, referralType string, group string, payerUserID int) (*User, *User, *ReferralTemplateBinding, *ReferralTemplate, error) {
	var payer User
	if err := tx.First(&payer, payerUserID).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	if payer.InviterId <= 0 || payer.InviterId == payer.Id {
		return &payer, nil, nil, nil, nil
	}

	var inviter User
	if err := tx.First(&inviter, payer.InviterId).Error; err != nil {
		return &payer, nil, nil, nil, err
	}
	active, binding, err := HasActiveReferralTemplateBinding(inviter.Id, referralType, group)
	if err != nil || !active || binding == nil {
		return &payer, &inviter, nil, nil, err
	}
	tpl, err := GetReferralTemplateByID(binding.TemplateId)
	if err != nil {
		return &payer, &inviter, nil, nil, err
	}
	return &payer, &inviter, binding, tpl, nil
}

func resolveSubscriptionTeamChain(tx *gorm.DB, parentUserID int, referralType string, group string, decayRatio float64, maxDepth int) ([]ResolvedTeamNode, error) {
	chain := make([]ResolvedTeamNode, 0)
	currentUserID := parentUserID
	pathDistance := 1
	matchedIndex := 0
	for currentUserID > 0 {
		var user User
		if err := tx.First(&user, currentUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return nil, err
		}
		active, binding, err := HasActiveReferralTemplateBinding(user.Id, referralType, group)
		if err != nil {
			return nil, err
		}
		if active && binding != nil {
			tpl, err := GetReferralTemplateByID(binding.TemplateId)
			if err != nil {
				return nil, err
			}
			if tpl.LevelType == ReferralLevelTypeTeam && pathDistance <= maxDepth {
				matchedIndex++
				chain = append(chain, ResolvedTeamNode{
					UserId:           user.Id,
					BindingId:        binding.Id,
					TemplateId:       tpl.Id,
					PathDistance:     pathDistance,
					MatchedTeamIndex: matchedIndex,
					WeightSnapshot:   math.Pow(decayRatio, float64(pathDistance-1)),
				})
			}
		}
		currentUserID = user.InviterId
		pathDistance++
	}
	return chain, nil
}
```

```go
// model/referral_subscription_engine_test_helpers.go
type seedTemplateEngineFixtureInput struct {
	Group                 string
	ImmediateInviterLevel string
	AncestorLevels        []string
}

func seedTemplateEngineFixture(t *testing.T, db *gorm.DB, input seedTemplateEngineFixtureInput) {
	t.Helper()
	payer := &User{Id: 5, Username: "payer", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	immediateInviter := &User{Id: 2, Username: "immediate", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	if err := db.Create(immediateInviter).Error; err != nil {
		t.Fatalf("create immediate inviter: %v", err)
	}
	payer.InviterId = immediateInviter.Id
	if err := db.Create(payer).Error; err != nil {
		t.Fatalf("create payer: %v", err)
	}

	createTemplateBinding := func(userID int, levelType string) {
		tpl := &ReferralTemplate{
			ReferralType:           ReferralTypeSubscription,
			Group:                  input.Group,
			Name:                   fmt.Sprintf("%s-%d", levelType, userID),
			LevelType:              levelType,
			Enabled:                true,
			DirectCapBps:           1000,
			TeamCapBps:             2500,
			TeamDecayRatio:         0.5,
			TeamMaxDepth:           5,
			InviteeShareDefaultBps: 0,
		}
		if err := db.Create(tpl).Error; err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := db.Create(&ReferralTemplateBinding{
			UserId:       userID,
			ReferralType: ReferralTypeSubscription,
			Group:        input.Group,
			TemplateId:   tpl.Id,
		}).Error; err != nil {
			t.Fatalf("create binding: %v", err)
		}
	}

	createTemplateBinding(immediateInviter.Id, input.ImmediateInviterLevel)
	currentParentID := immediateInviter.Id
	nextUserID := 3
	for _, levelType := range input.AncestorLevels {
		ancestor := &User{
			Id:        nextUserID,
			Username:  fmt.Sprintf("ancestor-%d", nextUserID),
			Password:  "password",
			Role:      common.RoleCommonUser,
			Status:    common.UserStatusEnabled,
			InviterId: nextUserID + 1,
		}
		if err := db.Create(ancestor).Error; err != nil {
			t.Fatalf("create ancestor: %v", err)
		}
		if err := db.Model(&User{}).Where("id = ?", currentParentID).Update("inviter_id", ancestor.Id).Error; err != nil {
			t.Fatalf("chain ancestor onto parent: %v", err)
		}
		createTemplateBinding(ancestor.Id, levelType)
		currentParentID = ancestor.Id
		nextUserID++
	}
}

type seedTemplateSettlementOrderInput struct {
	Group                 string
	ImmediateInviterLevel string
	AncestorLevels        []string
	InviteeShareBps       int
	Money                 float64
}

func seedTemplateSettlementOrder(t *testing.T, db *gorm.DB, input seedTemplateSettlementOrderInput) (*SubscriptionOrder, *SubscriptionPlan) {
	t.Helper()
	seedTemplateEngineFixture(t, db, seedTemplateEngineFixtureInput{
		Group:                 input.Group,
		ImmediateInviterLevel: input.ImmediateInviterLevel,
		AncestorLevels:        input.AncestorLevels,
	})
	plan := seedSubscriptionPlan(t, db, "template-engine-plan")
	plan.UpgradeGroup = input.Group
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", input.Group).Error; err != nil {
		t.Fatalf("set upgrade_group: %v", err)
	}
	order := &SubscriptionOrder{
		UserId:        5,
		PlanId:        plan.Id,
		Money:         input.Money,
		TradeNo:       fmt.Sprintf("template-order-%d", common.GetTimestamp()),
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	return order, plan
}
```

- [ ] **Step 4: Run the resolver tests to verify the context logic passes**

Run: `go test ./model -run 'TestResolveSubscriptionTemplateSettlementContext_TeamDirect|TestResolveSubscriptionTemplateSettlementContext_DirectWithMixedAncestors'`
Expected: PASS

- [ ] **Step 5: Commit the resolver**

```bash
git add model/referral_subscription_engine.go model/referral_subscription_engine_test.go model/referral_template.go
git commit -m "feat: add subscription referral template resolver"
```

### Task 2: Implement Formula Evaluation and Ledger Snapshot Writes

**Files:**
- Modify: `model/referral_subscription_engine.go`
- Modify: `model/referral_settlement_batch.go`
- Modify: `model/referral_settlement_record.go`
- Modify: `model/referral_subscription_engine_test_helpers.go`
- Modify: `model/referral_subscription_engine_test.go`
- Test: `model/referral_subscription_engine_test.go`

- [ ] **Step 1: Write the failing settlement math tests**

```go
func TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesTeamDirectBatch(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	order, plan := seedTemplateSettlementOrder(t, db, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeTeam,
		InviteeShareBps:       4000,
		Money:                 10,
	})

	err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(db, order, plan)
	if err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, db, order.TradeNo)
	if batch.SettlementMode != ReferralSettlementModeTeamDirect {
		t.Fatalf("SettlementMode = %q, want %q", batch.SettlementMode, ReferralSettlementModeTeamDirect)
	}
	assertRewardComponents(t, records, []string{"team_direct_reward", "invitee_reward"})
}

func TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesMixedTeamChainSnapshots(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	order, plan := seedTemplateSettlementOrder(t, db, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		AncestorLevels:        []string{ReferralLevelTypeTeam, ReferralLevelTypeDirect, ReferralLevelTypeTeam},
		InviteeShareBps:       3000,
		Money:                 10,
	})

	err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(db, order, plan)
	if err != nil {
		t.Fatalf("ApplyTemplateSubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	batch, records := loadReferralSettlementBatchByTradeNo(t, db, order.TradeNo)
	if batch.SettlementMode != ReferralSettlementModeDirectWithTeamChain {
		t.Fatalf("SettlementMode = %q, want %q", batch.SettlementMode, ReferralSettlementModeDirectWithTeamChain)
	}
	assertRewardComponents(t, records, []string{"direct_reward", "invitee_reward", "team_reward", "team_reward"})
	assertTeamChainSnapshotDistances(t, batch.TeamChainSnapshotJSON, []int{1, 3})
}
```

- [ ] **Step 2: Run the settlement tests to verify they fail**

Run: `go test ./model -run 'TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesTeamDirectBatch|TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesMixedTeamChainSnapshots'`
Expected: FAIL with `undefined: ApplyTemplateSubscriptionReferralOnOrderSuccessTx` and missing batch/record helpers.

- [ ] **Step 3: Implement formula evaluation and snapshot persistence**

```go
func ApplyTemplateSubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	ctx, err := ResolveSubscriptionTemplateSettlementContext(tx, ReferralTypeSubscription, strings.TrimSpace(plan.UpgradeGroup), order.UserId, order.Id)
	if err != nil || ctx == nil {
		return err
	}

	basisQuota := decimal.NewFromFloat(order.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	components := buildSubscriptionSettlementComponents(ctx, basisQuota)
	if len(components) == 0 {
		return nil
	}

	batch := &ReferralSettlementBatch{
		ReferralType:            ReferralTypeSubscription,
		Group:                   ctx.Group,
		SourceType:              "subscription_order",
		SourceId:                order.Id,
		SourceTradeNo:           order.TradeNo,
		PayerUserId:             order.UserId,
		ImmediateInviterUserId:  ctx.ImmediateInviter.Id,
		SettlementMode:          ctx.Mode,
		QuotaPerUnitSnapshot:    common.QuotaPerUnit,
		ActiveTemplateSnapshotJSON: mustMarshalJSON(activeTemplateSnapshotFromContext(ctx)),
		TeamChainSnapshotJSON:      mustMarshalJSON(teamChainSnapshotFromContext(ctx)),
		Status:                 SubscriptionReferralStatusCredited,
		SettledAt:              common.GetTimestamp(),
	}
	if err := tx.Create(batch).Error; err != nil {
		return err
	}

	for _, component := range components {
		if component.RewardQuota <= 0 {
			continue
		}
		component.BatchId = batch.Id
		if err := tx.Create(&component).Error; err != nil {
			return err
		}
		if err := creditReferralBeneficiary(tx, component.BeneficiaryUserId, component.RewardQuota); err != nil {
			return err
		}
	}
	return nil
}

func buildSubscriptionSettlementComponents(ctx *ReferralSettlementContext, basisQuota int64) []ReferralSettlementRecord {
	if ctx == nil || ctx.ActiveTemplate == nil {
		return nil
	}
	records := make([]ReferralSettlementRecord, 0, 2+len(ctx.TeamChain))
	directGross := basisQuota * int64(ctx.ActiveTemplate.DirectCapBps) / 10000
	teamDirectGross := basisQuota * int64(ctx.ActiveTemplate.TeamCapBps) / 10000
	teamPool := basisQuota * int64(ctx.ActiveTemplate.TeamCapBps-ctx.ActiveTemplate.DirectCapBps) / 10000

	switch ctx.Mode {
	case ReferralSettlementModeTeamDirect:
		inviteeReward := teamDirectGross * int64(ctx.ActiveTemplate.InviteeShareDefaultBps) / 10000
		records = append(records, ReferralSettlementRecord{
			ReferralType:         ctx.ReferralType,
			Group:                ctx.Group,
			BeneficiaryUserId:    ctx.ImmediateInviter.Id,
			RewardComponent:      "team_direct_reward",
			GrossRewardQuotaSnapshot: &teamDirectGross,
			RewardQuota:          teamDirectGross - inviteeReward,
			Status:               SubscriptionReferralStatusCredited,
		})
		if inviteeReward > 0 {
			sourceComponent := "team_direct_reward"
			records = append(records, ReferralSettlementRecord{
				ReferralType:          ctx.ReferralType,
				Group:                 ctx.Group,
				BeneficiaryUserId:     ctx.PayerUser.Id,
				RewardComponent:       "invitee_reward",
				SourceRewardComponent: &sourceComponent,
				GrossRewardQuotaSnapshot: &teamDirectGross,
				RewardQuota:           inviteeReward,
				Status:                SubscriptionReferralStatusCredited,
			})
		}
	case ReferralSettlementModeDirectWithTeamChain:
		inviteeReward := directGross * int64(ctx.ActiveTemplate.InviteeShareDefaultBps) / 10000
		records = append(records, ReferralSettlementRecord{
			ReferralType:         ctx.ReferralType,
			Group:                ctx.Group,
			BeneficiaryUserId:    ctx.ImmediateInviter.Id,
			RewardComponent:      "direct_reward",
			GrossRewardQuotaSnapshot: &directGross,
			RewardQuota:          directGross - inviteeReward,
			Status:               SubscriptionReferralStatusCredited,
		})
		if inviteeReward > 0 {
			sourceComponent := "direct_reward"
			records = append(records, ReferralSettlementRecord{
				ReferralType:          ctx.ReferralType,
				Group:                 ctx.Group,
				BeneficiaryUserId:     ctx.PayerUser.Id,
				RewardComponent:       "invitee_reward",
				SourceRewardComponent: &sourceComponent,
				GrossRewardQuotaSnapshot: &directGross,
				RewardQuota:           inviteeReward,
				Status:                SubscriptionReferralStatusCredited,
			})
		}
		totalWeight := 0.0
		for _, node := range ctx.TeamChain {
			totalWeight += node.WeightSnapshot
		}
		for idx := range ctx.TeamChain {
			node := ctx.TeamChain[idx]
			share := node.WeightSnapshot / totalWeight
			ctx.TeamChain[idx].ShareSnapshot = share
			reward := int64(math.Floor(float64(teamPool) * share))
			if reward <= 0 {
				continue
			}
			pathDistance := node.PathDistance
			matchedIndex := node.MatchedTeamIndex
			weightSnapshot := node.WeightSnapshot
			shareSnapshot := share
			records = append(records, ReferralSettlementRecord{
				ReferralType:      ctx.ReferralType,
				Group:             ctx.Group,
				BeneficiaryUserId: node.UserId,
				RewardComponent:   "team_reward",
				PathDistance:      &pathDistance,
				MatchedTeamIndex:  &matchedIndex,
				WeightSnapshot:    &weightSnapshot,
				ShareSnapshot:     &shareSnapshot,
				RewardQuota:       reward,
				Status:            SubscriptionReferralStatusCredited,
			})
		}
	}
	return records
}

func activeTemplateSnapshotFromContext(ctx *ReferralSettlementContext) map[string]any {
	return map[string]any{
		"template_id":                 ctx.ActiveTemplate.Id,
		"level_type":                  ctx.ActiveTemplate.LevelType,
		"direct_cap_bps":              ctx.ActiveTemplate.DirectCapBps,
		"team_cap_bps":                ctx.ActiveTemplate.TeamCapBps,
		"team_decay_ratio":            ctx.ActiveTemplate.TeamDecayRatio,
		"team_max_depth":              ctx.ActiveTemplate.TeamMaxDepth,
		"invitee_share_default_bps":   ctx.ActiveTemplate.InviteeShareDefaultBps,
	}
}

func teamChainSnapshotFromContext(ctx *ReferralSettlementContext) []map[string]any {
	items := make([]map[string]any, 0, len(ctx.TeamChain))
	for _, node := range ctx.TeamChain {
		items = append(items, map[string]any{
			"user_id":            node.UserId,
			"path_distance":      node.PathDistance,
			"matched_team_index": node.MatchedTeamIndex,
			"weight_snapshot":    node.WeightSnapshot,
			"share_snapshot":     node.ShareSnapshot,
		})
	}
	return items
}

func mustMarshalJSON(value any) string {
	raw, _ := common.Marshal(value)
	return string(raw)
}

func creditReferralBeneficiary(tx *gorm.DB, userID int, rewardQuota int64) error {
	return tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"aff_quota":   gorm.Expr("aff_quota + ?", rewardQuota),
		"aff_history": gorm.Expr("aff_history + ?", rewardQuota),
	}).Error
}

func debitReferralBeneficiary(tx *gorm.DB, userID int, rewardQuota int64) error {
	return tx.Model(&User{}).Where("id = ?", userID).Update("aff_quota", gorm.Expr("aff_quota - ?", rewardQuota)).Error
}
```

```go
// model/referral_subscription_engine_test_helpers.go
func loadReferralSettlementBatchByTradeNo(t *testing.T, db *gorm.DB, tradeNo string) (*ReferralSettlementBatch, []ReferralSettlementRecord) {
	t.Helper()
	var batch ReferralSettlementBatch
	if err := db.Where("source_trade_no = ?", tradeNo).First(&batch).Error; err != nil {
		t.Fatalf("load batch: %v", err)
	}
	var records []ReferralSettlementRecord
	if err := db.Where("batch_id = ?", batch.Id).Order("id ASC").Find(&records).Error; err != nil {
		t.Fatalf("load records: %v", err)
	}
	return &batch, records
}

func assertRewardComponents(t *testing.T, records []ReferralSettlementRecord, expected []string) {
	t.Helper()
	actual := make([]string, 0, len(records))
	for _, record := range records {
		actual = append(actual, record.RewardComponent)
	}
	require.Equal(t, expected, actual)
}

func assertTeamChainSnapshotDistances(t *testing.T, raw string, expected []int) {
	t.Helper()
	var payload []struct {
		PathDistance int `json:"path_distance"`
	}
	if err := common.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode team chain snapshot: %v", err)
	}
	actual := make([]int, 0, len(payload))
	for _, item := range payload {
		actual = append(actual, item.PathDistance)
	}
	require.Equal(t, expected, actual)
}

func seedTemplateContributionLedger(t *testing.T, db *gorm.DB) {
	t.Helper()
	order, plan := seedTemplateSettlementOrder(t, db, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		InviteeShareBps:       3000,
		Money:                 10,
	})
	if err := ApplyTemplateSubscriptionReferralOnOrderSuccessTx(db, order, plan); err != nil {
		t.Fatalf("apply template settlement: %v", err)
	}
}
```

- [ ] **Step 4: Run the settlement tests to verify the new engine writes the expected ledger**

Run: `go test ./model -run 'TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesTeamDirectBatch|TestApplyTemplateSubscriptionReferralOnOrderSuccessTx_WritesMixedTeamChainSnapshots'`
Expected: PASS

- [ ] **Step 5: Commit the settlement writer**

```bash
git add model/referral_subscription_engine.go model/referral_settlement_batch.go model/referral_settlement_record.go model/referral_subscription_engine_test.go
git commit -m "feat: add subscription referral template settlement writer"
```

### Task 3: Dispatch by Engine Route, Support Reverse, and Preserve Read-Side Compatibility

**Files:**
- Modify: `model/subscription.go`
- Modify: `model/subscription_referral.go`
- Modify: `model/subscription_referral_invitee_override.go`
- Modify: `controller/subscription_referral.go`
- Modify: `model/referral_subscription_engine_test.go`
- Modify: `controller/subscription_referral_test.go`
- Test: `model/referral_subscription_engine_test.go`
- Test: `controller/subscription_referral_test.go`

- [ ] **Step 1: Write the failing dispatch/compatibility tests**

```go
func TestApplySubscriptionReferralOnOrderSuccessTx_DispatchesByEngineRoute(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	order, plan := seedTemplateSettlementOrder(t, db, seedTemplateSettlementOrderInput{
		Group:                 "vip",
		ImmediateInviterLevel: ReferralLevelTypeDirect,
		Money:                 10,
	})
	if err := db.Create(&ReferralEngineRoute{
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		EngineMode:   ReferralEngineModeTemplate,
	}).Error; err != nil {
		t.Fatalf("create engine route: %v", err)
	}

	err := ApplySubscriptionReferralOnOrderSuccessTx(db, order, plan)
	if err != nil {
		t.Fatalf("ApplySubscriptionReferralOnOrderSuccessTx() error = %v", err)
	}

	var batch ReferralSettlementBatch
	if err := db.Where("source_trade_no = ?", order.TradeNo).First(&batch).Error; err != nil {
		t.Fatalf("expected template settlement batch for %s: %v", order.TradeNo, err)
	}
}

func TestListSubscriptionReferralInviteeContributionSummariesIncludesTemplateLedger(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	seedTemplateContributionLedger(t, db)

	summaries, _, totalContribution, err := ListSubscriptionReferralInviteeContributionSummaries(1, "", &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries() error = %v", err)
	}
	if len(summaries) == 0 || totalContribution <= 0 {
		t.Fatalf("expected template ledger contribution summary, got len=%d total=%d", len(summaries), totalContribution)
	}
}
```

- [ ] **Step 2: Run the compatibility tests to verify they fail**

Run: `go test ./model -run 'TestApplySubscriptionReferralOnOrderSuccessTx_DispatchesByEngineRoute|TestListSubscriptionReferralInviteeContributionSummariesIncludesTemplateLedger'`
Expected: FAIL because `ApplySubscriptionReferralOnOrderSuccessTx` still writes only legacy records and invitee contribution summaries only query `subscription_referral_records`.

- [ ] **Step 3: Implement engine dispatch, reverse support, and read-side union**

```go
func ApplySubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	if tx == nil || order == nil || plan == nil || order.Money <= 0 {
		return nil
	}

	group := strings.TrimSpace(plan.UpgradeGroup)
	if group == "" {
		return nil
	}

	mode, err := ResolveReferralEngineMode(ReferralTypeSubscription, group)
	if err != nil {
		return err
	}
	if mode == ReferralEngineModeTemplate {
		return ApplyTemplateSubscriptionReferralOnOrderSuccessTx(tx, order, plan)
	}
	return applyLegacySubscriptionReferralOnOrderSuccessTx(tx, order, plan)
}

func ReverseSubscriptionReferralByTradeNo(tradeNo string, operatorId int) error {
	mode, batch, err := findTemplateSettlementBatchByTradeNo(tradeNo)
	if err == nil && batch != nil && mode == ReferralEngineModeTemplate {
		return reverseReferralSettlementBatch(batch.Id, operatorId)
	}
	return reverseLegacySubscriptionReferralByTradeNo(tradeNo, operatorId)
}

func applyLegacySubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	group := strings.TrimSpace(plan.UpgradeGroup)
	if group == "" {
		return nil
	}

	var invitee User
	if err := tx.First(&invitee, order.UserId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil
	}

	var inviter User
	if err := tx.First(&inviter, invitee.InviterId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	totalRateBps := GetEffectiveSubscriptionReferralTotalRateBps(inviter.Id, group)
	inviteeRateBps := GetEffectiveSubscriptionReferralInviteeRateBpsForInvitee(inviter.Id, invitee.Id, inviter.GetSetting(), group, totalRateBps)
	cfg := ResolveSubscriptionReferralConfig(totalRateBps, inviteeRateBps)
	if !cfg.Enabled {
		return nil
	}

	records := []SubscriptionReferralRecord{
		{
			OrderId:                order.Id,
			OrderTradeNo:           order.TradeNo,
			PlanId:                 order.PlanId,
			ReferralGroup:          group,
			PayerUserId:            order.UserId,
			InviterUserId:          invitee.InviterId,
			BeneficiaryUserId:      invitee.InviterId,
			BeneficiaryRole:        SubscriptionReferralBeneficiaryRoleInviter,
			OrderPaidAmount:        order.Money,
			QuotaPerUnitSnapshot:   common.QuotaPerUnit,
			TotalRateBpsSnapshot:   cfg.TotalRateBps,
			InviteeRateBpsSnapshot: cfg.InviteeRateBps,
			AppliedRateBps:         cfg.InviterRateBps,
			RewardQuota:            int64(CalculateSubscriptionReferralQuota(order.Money, cfg.InviterRateBps)),
			Status:                 SubscriptionReferralStatusCredited,
		},
		{
			OrderId:                order.Id,
			OrderTradeNo:           order.TradeNo,
			PlanId:                 order.PlanId,
			ReferralGroup:          group,
			PayerUserId:            order.UserId,
			InviterUserId:          invitee.InviterId,
			BeneficiaryUserId:      order.UserId,
			BeneficiaryRole:        SubscriptionReferralBeneficiaryRoleInvitee,
			OrderPaidAmount:        order.Money,
			QuotaPerUnitSnapshot:   common.QuotaPerUnit,
			TotalRateBpsSnapshot:   cfg.TotalRateBps,
			InviteeRateBpsSnapshot: cfg.InviteeRateBps,
			AppliedRateBps:         cfg.InviteeRateBps,
			RewardQuota:            int64(CalculateSubscriptionReferralQuota(order.Money, cfg.InviteeRateBps)),
			Status:                 SubscriptionReferralStatusCredited,
		},
	}

	for _, record := range records {
		if record.RewardQuota <= 0 {
			continue
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := creditReferralBeneficiary(tx, record.BeneficiaryUserId, record.RewardQuota); err != nil {
			return err
		}
	}
	return nil
}

func reverseLegacySubscriptionReferralByTradeNo(tradeNo string, operatorId int) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var records []SubscriptionReferralRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("order_trade_no = ?", tradeNo).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return ErrSubscriptionReferralRecordNotFound
		}

		for i := range records {
			record := &records[i]
			reversible := record.RewardQuota - record.ReversedQuota - record.DebtQuota
			if reversible <= 0 {
				continue
			}
			if err := debitReferralBeneficiary(tx, record.BeneficiaryUserId, reversible); err != nil {
				return err
			}
			record.ReversedQuota += reversible
			record.Status = SubscriptionReferralStatusReversed
			if err := tx.Save(record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func findTemplateSettlementBatchByTradeNo(tradeNo string) (string, *ReferralSettlementBatch, error) {
	var batch ReferralSettlementBatch
	err := DB.Where("source_trade_no = ? AND referral_type = ?", tradeNo, ReferralTypeSubscription).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReferralEngineModeLegacy, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return ReferralEngineModeTemplate, &batch, nil
}

func reverseReferralSettlementBatch(batchID int, operatorID int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var records []ReferralSettlementRecord
		if err := tx.Where("batch_id = ?", batchID).Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			reversible := record.RewardQuota - record.ReversedQuota - record.DebtQuota
			if reversible <= 0 {
				continue
			}
			if err := debitReferralBeneficiary(tx, record.BeneficiaryUserId, reversible); err != nil {
				return err
			}
			record.ReversedQuota += reversible
			record.Status = SubscriptionReferralStatusReversed
			if err := tx.Save(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
```

```go
// model/subscription_referral_invitee_override.go
templateSummaryQuery := DB.Table("referral_settlement_records AS records").
	Select(strings.Join([]string{
		"records.beneficiary_user_id AS inviter_user_id",
		"batches.payer_user_id AS invitee_user_id",
		"COALESCE(SUM(records.reward_quota - records.reversed_quota - records.debt_quota), 0) AS contribution_quota",
	}, ", ")).
	Joins("JOIN referral_settlement_batches AS batches ON batches.id = records.batch_id").
	Where("records.referral_type = ? AND records.reward_component IN ?", ReferralTypeSubscription, []string{"direct_reward", "team_direct_reward"}).
	Group("records.beneficiary_user_id, batches.payer_user_id")
```

- [ ] **Step 4: Run the model/controller regression tests**

Run: `go test ./model -run 'TestApplySubscriptionReferralOnOrderSuccessTx_DispatchesByEngineRoute|TestListSubscriptionReferralInviteeContributionSummariesIncludesTemplateLedger|TestReverseSubscriptionReferralByTradeNo'`

Run: `go test ./controller -run 'TestGetSubscriptionReferralInvitees|TestGetSubscriptionReferralInvitee|TestAdminReverseSubscriptionReferral'`

Expected: PASS

- [ ] **Step 5: Commit the engine dispatch and compatibility layer**

```bash
git add model/subscription.go model/subscription_referral.go model/subscription_referral_invitee_override.go controller/subscription_referral.go model/referral_subscription_engine_test.go controller/subscription_referral_test.go
git commit -m "feat: wire template subscription referral engine"
```
