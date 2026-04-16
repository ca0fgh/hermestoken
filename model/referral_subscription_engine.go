package model

import (
	"errors"
	"math"
	"strings"

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
