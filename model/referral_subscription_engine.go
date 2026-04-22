package model

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	GlobalSetting    SubscriptionReferralGlobalSetting
	Mode             string
	TeamChain        []ResolvedTeamNode
}

type ResolvedTeamNode struct {
	UserId           int
	BindingId        int
	TemplateId       int
	TeamRateBps      int
	PathDistance     int
	MatchedTeamIndex int
	WeightSnapshot   float64
	ShareSnapshot    float64
}

type TeamDifferentialActivation struct {
	DifferentialRateBps int
	DifferentialQuota   int64
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
		GlobalSetting:    GetSubscriptionReferralGlobalSetting(),
		TeamChain:        make([]ResolvedTeamNode, 0),
	}

	switch template.LevelType {
	case ReferralLevelTypeTeam:
		context.Mode = ReferralSettlementModeTeamDirect
		return context, nil
	case ReferralLevelTypeDirect:
		context.Mode = ReferralSettlementModeDirectWithTeamChain
		teamChain, err := resolveSubscriptionTeamChain(
			tx,
			inviter.InviterId,
			trimmedReferralType,
			trimmedGroup,
			context.GlobalSetting.TeamDecayRatio,
			context.GlobalSetting.TeamMaxDepth,
		)
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
		withinMaxDepth := maxDepth <= 0 || pathDistance <= maxDepth
		if active && binding != nil && template != nil && template.LevelType == ReferralLevelTypeTeam && withinMaxDepth {
			matchedTeamIndex++
			teamChain = append(teamChain, ResolvedTeamNode{
				UserId:           user.Id,
				BindingId:        binding.Id,
				TemplateId:       template.Id,
				TeamRateBps:      NormalizeSubscriptionReferralRateBps(template.TeamCapBps),
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
	globalSettingSnapshotJSON, err := mustMarshalJSON(globalSettingSnapshotFromContext(context))
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
		GlobalSettingSnapshotJSON:  globalSettingSnapshotJSON,
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

	records := make([]ReferralSettlementRecord, 0, 2+len(context.TeamChain))

	switch context.Mode {
	case ReferralSettlementModeTeamDirect:
		teamDirectConfig := ResolveSubscriptionReferralConfig(context.ActiveTemplate.TeamCapBps, effectiveInviteeShareBps)
		teamDirectGross := int64(CalculateSubscriptionReferralQuota(order.Money, teamDirectConfig.TotalRateBps))
		teamDirectInviteeQuota := int64(CalculateSubscriptionReferralQuota(order.Money, teamDirectConfig.InviteeRateBps))
		records = append(records, buildImmediateRewardRecord(context, "team_direct_reward", ReferralLevelTypeTeam, teamDirectGross, teamDirectInviteeQuota, teamDirectConfig.TotalRateBps, teamDirectConfig.InviteeRateBps))
		if inviteeRecord := buildInviteeRewardRecord(context, "team_direct_reward", teamDirectGross, teamDirectInviteeQuota, teamDirectConfig.InviteeRateBps); inviteeRecord != nil {
			records = append(records, *inviteeRecord)
		}
	case ReferralSettlementModeDirectWithTeamChain:
		directConfig := ResolveSubscriptionReferralConfig(context.ActiveTemplate.DirectCapBps, effectiveInviteeShareBps)
		directGross := int64(CalculateSubscriptionReferralQuota(order.Money, directConfig.TotalRateBps))
		directInviteeQuota := int64(CalculateSubscriptionReferralQuota(order.Money, directConfig.InviteeRateBps))
		records = append(records, buildImmediateRewardRecord(context, "direct_reward", ReferralLevelTypeDirect, directGross, directInviteeQuota, directConfig.TotalRateBps, directConfig.InviteeRateBps))
		if inviteeRecord := buildInviteeRewardRecord(context, "direct_reward", directGross, directInviteeQuota, directConfig.InviteeRateBps); inviteeRecord != nil {
			records = append(records, *inviteeRecord)
		}
		activation := resolveTeamDifferentialActivation(context, order)
		records = append(records, buildTeamDifferentialRewardRecords(context, activation)...)
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

func resolveTeamDifferentialActivation(context *ReferralSettlementContext, order *SubscriptionOrder) *TeamDifferentialActivation {
	if context == nil || context.ActiveTemplate == nil || order == nil {
		return nil
	}
	if context.Mode != ReferralSettlementModeDirectWithTeamChain {
		return nil
	}
	if len(context.TeamChain) == 0 {
		return nil
	}

	firstMatchedTeamRateBps := NormalizeSubscriptionReferralRateBps(context.TeamChain[0].TeamRateBps)
	differentialRateBps := firstMatchedTeamRateBps - context.ActiveTemplate.DirectCapBps
	if differentialRateBps <= 0 {
		return nil
	}

	differentialQuota := int64(CalculateSubscriptionReferralQuota(order.Money, differentialRateBps))
	if differentialQuota <= 0 {
		return nil
	}

	return &TeamDifferentialActivation{
		DifferentialRateBps: differentialRateBps,
		DifferentialQuota:   differentialQuota,
	}
}

func buildImmediateRewardRecord(context *ReferralSettlementContext, component string, levelType string, grossQuota int64, inviteeRewardQuota int64, appliedRateBps int, inviteeShareBps int) ReferralSettlementRecord {
	beneficiaryLevelType := levelType
	appliedRate := appliedRateBps
	grossSnapshot := grossQuota
	inviteeShareSnapshot := inviteeShareBps
	netQuota := grossQuota - inviteeRewardQuota

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

func buildInviteeRewardRecord(context *ReferralSettlementContext, sourceComponent string, grossQuota int64, inviteeRewardQuota int64, inviteeShareBps int) *ReferralSettlementRecord {
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

func buildTeamDifferentialRewardRecords(context *ReferralSettlementContext, activation *TeamDifferentialActivation) []ReferralSettlementRecord {
	if activation == nil || activation.DifferentialQuota <= 0 || len(context.TeamChain) == 0 {
		return nil
	}

	totalWeight := 0.0
	for _, node := range context.TeamChain {
		totalWeight += node.WeightSnapshot
	}
	if totalWeight <= 0 {
		return nil
	}

	differentialRateSnapshot := activation.DifferentialRateBps
	beneficiaryLevelType := ReferralLevelTypeTeam
	rewards := make([]ReferralSettlementRecord, 0, len(context.TeamChain))
	allocated := int64(0)

	for idx, node := range context.TeamChain {
		share := node.WeightSnapshot / totalWeight
		context.TeamChain[idx].ShareSnapshot = share
		rewardQuota := int64(math.Floor(float64(activation.DifferentialQuota) * share))
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
			PoolRateBpsSnapshot:  &differentialRateSnapshot,
			RewardQuota:          rewardQuota,
			Status:               SubscriptionReferralStatusCredited,
		})
	}

	remainder := activation.DifferentialQuota - allocated
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
	result := tx.Where(
		"inviter_user_id = ? AND invitee_user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		context.ImmediateInviter.Id,
		context.PayerUser.Id,
		context.ReferralType,
		context.Group,
	).Limit(1).Find(&override)
	if result.Error == nil && result.RowsAffected > 0 {
		return NormalizeSubscriptionReferralRateBps(override.InviteeShareBps), nil
	}
	if result.Error != nil {
		return 0, result.Error
	}

	return NormalizeSubscriptionReferralRateBps(context.ActiveTemplate.InviteeShareDefaultBps), nil
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
		"invitee_share_default_bps": context.ActiveTemplate.InviteeShareDefaultBps,
	}
}

func globalSettingSnapshotFromContext(context *ReferralSettlementContext) map[string]interface{} {
	if context == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"team_decay_ratio": context.GlobalSetting.TeamDecayRatio,
		"team_max_depth":   context.GlobalSetting.TeamMaxDepth,
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
				"team_rate_bps":      node.TeamRateBps,
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
