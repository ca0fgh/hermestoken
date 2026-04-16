package model

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	ReferralSettlementModeTeamDirect          = "team_direct"
	ReferralSettlementModeDirectWithTeamChain = "direct_with_team_chain"
)

type ReferralSettlementContext struct {
	ReferralType     string
	Group            string
	PayerUser        *User
	ImmediateInviter *User
	ActiveBinding    *ReferralTemplateBinding
	ActiveTemplate   *ReferralTemplate
	Mode             string
	TeamChain        []ResolvedTeamNode
}

type ResolvedTeamNode struct {
	UserId           int
	BindingId        int
	TemplateId       int
	PathDistance     int
	MatchedTeamIndex int
	WeightSnapshot   float64
	ShareSnapshot    float64
}

func ResolveSubscriptionTemplateSettlementContext(tx *gorm.DB, referralType string, group string, payerUserID int, orderID int) (*ReferralSettlementContext, error) {
	if tx == nil {
		tx = DB
	}

	trimmedReferralType := strings.TrimSpace(referralType)
	trimmedGroup := strings.TrimSpace(group)
	if trimmedReferralType == "" || trimmedGroup == "" || payerUserID <= 0 {
		return nil, nil
	}

	payer, inviter, binding, template, err := loadImmediateInviterTemplateScope(tx, trimmedReferralType, trimmedGroup, payerUserID)
	if err != nil {
		return nil, err
	}
	if payer == nil || inviter == nil || binding == nil || template == nil {
		return nil, nil
	}

	context := &ReferralSettlementContext{
		ReferralType:     trimmedReferralType,
		Group:            trimmedGroup,
		PayerUser:        payer,
		ImmediateInviter: inviter,
		ActiveBinding:    binding,
		ActiveTemplate:   template,
		TeamChain:        make([]ResolvedTeamNode, 0),
	}

	switch template.LevelType {
	case ReferralLevelTypeTeam:
		context.Mode = ReferralSettlementModeTeamDirect
		return context, nil
	case ReferralLevelTypeDirect:
		context.Mode = ReferralSettlementModeDirectWithTeamChain
		teamChain, err := resolveSubscriptionTeamChain(tx, inviter.InviterId, trimmedReferralType, trimmedGroup, template.TeamDecayRatio, template.TeamMaxDepth)
		if err != nil {
			return nil, err
		}
		context.TeamChain = teamChain
		return context, nil
	default:
		return nil, nil
	}
}

func loadImmediateInviterTemplateScope(tx *gorm.DB, referralType string, group string, payerUserID int) (*User, *User, *ReferralTemplateBinding, *ReferralTemplate, error) {
	var payer User
	if err := tx.First(&payer, payerUserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, nil, nil
		}
		return nil, nil, nil, nil, err
	}

	if payer.InviterId <= 0 || payer.InviterId == payer.Id {
		return &payer, nil, nil, nil, nil
	}

	var inviter User
	if err := tx.First(&inviter, payer.InviterId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &payer, nil, nil, nil, nil
		}
		return nil, nil, nil, nil, err
	}

	binding, template, active, err := resolveActiveReferralTemplateBindingTx(tx, inviter.Id, referralType, group)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !active {
		return &payer, &inviter, nil, nil, nil
	}

	return &payer, &inviter, binding, template, nil
}

func resolveSubscriptionTeamChain(tx *gorm.DB, ancestorUserID int, referralType string, group string, decayRatio float64, maxDepth int) ([]ResolvedTeamNode, error) {
	teamChain := make([]ResolvedTeamNode, 0)
	if ancestorUserID <= 0 {
		return teamChain, nil
	}

	pathDistance := 1
	matchedTeamIndex := 0
	currentUserID := ancestorUserID

	for currentUserID > 0 {
		var user User
		if err := tx.First(&user, currentUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return teamChain, nil
			}
			return nil, err
		}

		binding, template, active, err := resolveActiveReferralTemplateBindingTx(tx, user.Id, referralType, group)
		if err != nil {
			return nil, err
		}
		if active && binding != nil && template != nil && template.LevelType == ReferralLevelTypeTeam && pathDistance <= maxDepth {
			matchedTeamIndex++
			teamChain = append(teamChain, ResolvedTeamNode{
				UserId:           user.Id,
				BindingId:        binding.Id,
				TemplateId:       template.Id,
				PathDistance:     pathDistance,
				MatchedTeamIndex: matchedTeamIndex,
				WeightSnapshot:   math.Pow(decayRatio, float64(pathDistance-1)),
			})
		}

		currentUserID = user.InviterId
		pathDistance++
	}

	return teamChain, nil
}

func resolveActiveReferralTemplateBindingTx(tx *gorm.DB, userID int, referralType string, group string) (*ReferralTemplateBinding, *ReferralTemplate, bool, error) {
	if userID <= 0 {
		return nil, nil, false, nil
	}

	var binding ReferralTemplateBinding
	err := tx.Where(
		"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		userID,
		strings.TrimSpace(referralType),
		strings.TrimSpace(group),
	).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}

	var template ReferralTemplate
	if err := tx.First(&template, binding.TemplateId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	if !template.Enabled {
		return &binding, &template, false, nil
	}
	return &binding, &template, true, nil
}

func ApplyTemplateSubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	if tx == nil {
		tx = DB
	}
	if order == nil || plan == nil || order.Money <= 0 {
		return nil
	}

	context, err := ResolveSubscriptionTemplateSettlementContext(tx, ReferralTypeSubscription, strings.TrimSpace(plan.UpgradeGroup), order.UserId, order.Id)
	if err != nil {
		return err
	}
	if context == nil {
		return nil
	}

	components, err := buildSubscriptionSettlementComponents(tx, context, order)
	if err != nil {
		return err
	}
	if len(components) == 0 {
		return nil
	}

	activeTemplateSnapshotJSON, err := mustMarshalJSON(activeTemplateSnapshotFromContext(context))
	if err != nil {
		return err
	}
	teamChainSnapshotJSON, err := mustMarshalJSON(teamChainSnapshotFromContext(context))
	if err != nil {
		return err
	}

	batch := &ReferralSettlementBatch{
		ReferralType:               ReferralTypeSubscription,
		Group:                      context.Group,
		SourceType:                 "subscription_order",
		SourceId:                   order.Id,
		SourceTradeNo:              order.TradeNo,
		PayerUserId:                order.UserId,
		ImmediateInviterUserId:     context.ImmediateInviter.Id,
		ActiveTemplateSnapshotJSON: activeTemplateSnapshotJSON,
		TeamChainSnapshotJSON:      teamChainSnapshotJSON,
		SettlementMode:             context.Mode,
		QuotaPerUnitSnapshot:       common.QuotaPerUnit,
		Status:                     SubscriptionReferralStatusCredited,
	}
	if err := tx.Create(batch).Error; err != nil {
		return err
	}

	for idx := range components {
		components[idx].BatchId = batch.Id
		if components[idx].RewardQuota <= 0 {
			continue
		}
		if err := tx.Create(&components[idx]).Error; err != nil {
			return err
		}
		if err := creditReferralBeneficiary(tx, components[idx].BeneficiaryUserId, components[idx].RewardQuota); err != nil {
			return err
		}
	}

	return nil
}

func buildSubscriptionSettlementComponents(tx *gorm.DB, context *ReferralSettlementContext, order *SubscriptionOrder) ([]ReferralSettlementRecord, error) {
	if context == nil || context.ActiveTemplate == nil || context.ImmediateInviter == nil || context.PayerUser == nil {
		return nil, nil
	}

	effectiveInviteeShareBps, err := resolveTemplateInviteeShareBps(tx, context)
	if err != nil {
		return nil, err
	}

	directGross := int64(CalculateSubscriptionReferralQuota(order.Money, context.ActiveTemplate.DirectCapBps))
	teamDirectGross := int64(CalculateSubscriptionReferralQuota(order.Money, context.ActiveTemplate.TeamCapBps))
	teamPoolRateBps := context.ActiveTemplate.TeamCapBps - context.ActiveTemplate.DirectCapBps
	teamPoolQuota := int64(CalculateSubscriptionReferralQuota(order.Money, teamPoolRateBps))

	records := make([]ReferralSettlementRecord, 0, 2+len(context.TeamChain))

	switch context.Mode {
	case ReferralSettlementModeTeamDirect:
		records = append(records, buildImmediateRewardRecord(context, "team_direct_reward", ReferralLevelTypeTeam, teamDirectGross, context.ActiveTemplate.TeamCapBps, effectiveInviteeShareBps))
		if inviteeRecord := buildInviteeRewardRecord(context, "team_direct_reward", teamDirectGross, effectiveInviteeShareBps); inviteeRecord != nil {
			records = append(records, *inviteeRecord)
		}
	case ReferralSettlementModeDirectWithTeamChain:
		records = append(records, buildImmediateRewardRecord(context, "direct_reward", ReferralLevelTypeDirect, directGross, context.ActiveTemplate.DirectCapBps, effectiveInviteeShareBps))
		if inviteeRecord := buildInviteeRewardRecord(context, "direct_reward", directGross, effectiveInviteeShareBps); inviteeRecord != nil {
			records = append(records, *inviteeRecord)
		}
		records = append(records, buildTeamRewardRecords(context, teamPoolQuota, teamPoolRateBps)...)
	}

	filtered := make([]ReferralSettlementRecord, 0, len(records))
	for _, record := range records {
		if record.RewardQuota <= 0 {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered, nil
}

func buildImmediateRewardRecord(context *ReferralSettlementContext, component string, levelType string, grossQuota int64, appliedRateBps int, inviteeShareBps int) ReferralSettlementRecord {
	beneficiaryLevelType := levelType
	appliedRate := appliedRateBps
	grossSnapshot := grossQuota
	inviteeShareSnapshot := inviteeShareBps
	netQuota := grossQuota - calculateInviteeRewardQuota(grossQuota, inviteeShareBps)

	return ReferralSettlementRecord{
		ReferralType:             context.ReferralType,
		Group:                    context.Group,
		BeneficiaryUserId:        context.ImmediateInviter.Id,
		BeneficiaryLevelType:     &beneficiaryLevelType,
		RewardComponent:          component,
		GrossRewardQuotaSnapshot: &grossSnapshot,
		InviteeShareBpsSnapshot:  &inviteeShareSnapshot,
		AppliedRateBps:           &appliedRate,
		RewardQuota:              netQuota,
		Status:                   SubscriptionReferralStatusCredited,
	}
}

func buildInviteeRewardRecord(context *ReferralSettlementContext, sourceComponent string, grossQuota int64, inviteeShareBps int) *ReferralSettlementRecord {
	inviteeRewardQuota := calculateInviteeRewardQuota(grossQuota, inviteeShareBps)
	if inviteeRewardQuota <= 0 {
		return nil
	}

	source := sourceComponent
	grossSnapshot := grossQuota
	inviteeShareSnapshot := inviteeShareBps
	return &ReferralSettlementRecord{
		ReferralType:             context.ReferralType,
		Group:                    context.Group,
		BeneficiaryUserId:        context.PayerUser.Id,
		RewardComponent:          "invitee_reward",
		SourceRewardComponent:    &source,
		GrossRewardQuotaSnapshot: &grossSnapshot,
		InviteeShareBpsSnapshot:  &inviteeShareSnapshot,
		RewardQuota:              inviteeRewardQuota,
		Status:                   SubscriptionReferralStatusCredited,
	}
}

func buildTeamRewardRecords(context *ReferralSettlementContext, teamPoolQuota int64, teamPoolRateBps int) []ReferralSettlementRecord {
	if teamPoolQuota <= 0 || len(context.TeamChain) == 0 {
		return nil
	}

	totalWeight := 0.0
	for _, node := range context.TeamChain {
		totalWeight += node.WeightSnapshot
	}
	if totalWeight <= 0 {
		return nil
	}

	poolRateSnapshot := teamPoolRateBps
	beneficiaryLevelType := ReferralLevelTypeTeam
	rewards := make([]ReferralSettlementRecord, 0, len(context.TeamChain))
	allocated := int64(0)

	for idx, node := range context.TeamChain {
		share := node.WeightSnapshot / totalWeight
		context.TeamChain[idx].ShareSnapshot = share
		rewardQuota := int64(math.Floor(float64(teamPoolQuota) * share))
		allocated += rewardQuota

		pathDistance := node.PathDistance
		matchedTeamIndex := node.MatchedTeamIndex
		weightSnapshot := node.WeightSnapshot
		shareSnapshot := share
		rewards = append(rewards, ReferralSettlementRecord{
			ReferralType:         context.ReferralType,
			Group:                context.Group,
			BeneficiaryUserId:    node.UserId,
			BeneficiaryLevelType: &beneficiaryLevelType,
			RewardComponent:      "team_reward",
			PathDistance:         &pathDistance,
			MatchedTeamIndex:     &matchedTeamIndex,
			WeightSnapshot:       &weightSnapshot,
			ShareSnapshot:        &shareSnapshot,
			PoolRateBpsSnapshot:  &poolRateSnapshot,
			RewardQuota:          rewardQuota,
			Status:               SubscriptionReferralStatusCredited,
		})
	}

	remainder := teamPoolQuota - allocated
	if remainder > 0 && len(rewards) > 0 {
		rewards[0].RewardQuota += remainder
	}

	return rewards
}

func resolveTemplateInviteeShareBps(tx *gorm.DB, context *ReferralSettlementContext) (int, error) {
	if context == nil || context.ActiveTemplate == nil || context.ImmediateInviter == nil || context.PayerUser == nil {
		return 0, nil
	}

	var override ReferralInviteeShareOverride
	err := tx.Where(
		"inviter_user_id = ? AND invitee_user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		context.ImmediateInviter.Id,
		context.PayerUser.Id,
		context.ReferralType,
		context.Group,
	).First(&override).Error
	if err == nil {
		return NormalizeSubscriptionReferralRateBps(override.InviteeShareBps), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	if context.ActiveBinding.InviteeShareOverrideBps != nil {
		return NormalizeSubscriptionReferralRateBps(*context.ActiveBinding.InviteeShareOverrideBps), nil
	}

	return NormalizeSubscriptionReferralRateBps(context.ActiveTemplate.InviteeShareDefaultBps), nil
}

func calculateInviteeRewardQuota(grossQuota int64, inviteeShareBps int) int64 {
	if grossQuota <= 0 || inviteeShareBps <= 0 {
		return 0
	}
	return decimal.NewFromInt(grossQuota).
		Mul(decimal.NewFromInt(int64(NormalizeSubscriptionReferralRateBps(inviteeShareBps)))).
		Div(decimal.NewFromInt(SubscriptionReferralMaxRateBps)).
		IntPart()
}

func activeTemplateSnapshotFromContext(context *ReferralSettlementContext) map[string]interface{} {
	if context == nil || context.ActiveTemplate == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"template_id":               context.ActiveTemplate.Id,
		"name":                      context.ActiveTemplate.Name,
		"level_type":                context.ActiveTemplate.LevelType,
		"direct_cap_bps":            context.ActiveTemplate.DirectCapBps,
		"team_cap_bps":              context.ActiveTemplate.TeamCapBps,
		"team_decay_ratio":          context.ActiveTemplate.TeamDecayRatio,
		"team_max_depth":            context.ActiveTemplate.TeamMaxDepth,
		"invitee_share_default_bps": context.ActiveTemplate.InviteeShareDefaultBps,
	}
}

func teamChainSnapshotFromContext(context *ReferralSettlementContext) []map[string]interface{} {
	if context == nil {
		return []map[string]interface{}{}
	}

	snapshot := make([]map[string]interface{}, 0, len(context.TeamChain))
	for _, node := range context.TeamChain {
		snapshot = append(snapshot, map[string]interface{}{
			"user_id":            node.UserId,
			"binding_id":         node.BindingId,
			"template_id":        node.TemplateId,
			"path_distance":      node.PathDistance,
			"matched_team_index": node.MatchedTeamIndex,
			"weight_snapshot":    node.WeightSnapshot,
			"share_snapshot":     node.ShareSnapshot,
		})
	}
	return snapshot
}

func mustMarshalJSON(value interface{}) (string, error) {
	raw, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func creditReferralBeneficiary(tx *gorm.DB, userID int, rewardQuota int64) error {
	if userID <= 0 || rewardQuota <= 0 {
		return nil
	}
	return tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"aff_quota":   gorm.Expr("aff_quota + ?", rewardQuota),
		"aff_history": gorm.Expr("aff_history + ?", rewardQuota),
	}).Error
}
