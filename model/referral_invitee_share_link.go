package model

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserInsertOptions struct {
	AffCode               string
	InviteeShareBps       int
	InviteeShareSignature string
}

func NewReferralInviteeShareLinkSignature(affCode string, inviteeShareBps int) string {
	trimmedAffCode := strings.TrimSpace(affCode)
	if trimmedAffCode == "" {
		return ""
	}
	normalizedBps := NormalizeSubscriptionReferralRateBps(inviteeShareBps)
	payload := fmt.Sprintf("subscription_referral_invitee_share:%s:%d", trimmedAffCode, normalizedBps)
	return common.HmacSha256(payload, common.SessionSecret)
}

func ValidateReferralInviteeShareLinkSignature(affCode string, inviteeShareBps int, signature string) bool {
	trimmedSignature := strings.TrimSpace(signature)
	if trimmedSignature == "" {
		return false
	}
	expectedSignature := NewReferralInviteeShareLinkSignature(affCode, inviteeShareBps)
	if expectedSignature == "" {
		return false
	}
	return hmac.Equal([]byte(expectedSignature), []byte(strings.ToLower(trimmedSignature)))
}

func ApplySignedReferralInviteeShareOverridesForNewInviteeTx(tx *gorm.DB, inviterUserID int, inviteeUserID int, options UserInsertOptions) error {
	if tx == nil {
		tx = DB
	}
	if tx == nil || inviterUserID <= 0 || inviteeUserID <= 0 {
		return nil
	}

	normalizedBps := NormalizeSubscriptionReferralRateBps(options.InviteeShareBps)
	if normalizedBps <= 0 {
		return nil
	}

	affCode := strings.TrimSpace(options.AffCode)
	if !ValidateReferralInviteeShareLinkSignature(affCode, normalizedBps, options.InviteeShareSignature) {
		return nil
	}

	var inviter User
	if err := tx.Select("id", "aff_code").First(&inviter, inviterUserID).Error; err != nil {
		return err
	}
	if strings.TrimSpace(inviter.AffCode) != affCode {
		return nil
	}

	var bindings []ReferralTemplateBinding
	if err := tx.Where(
		"user_id = ? AND referral_type = ?",
		inviterUserID,
		ReferralTypeSubscription,
	).Order(commonGroupCol + " ASC").Find(&bindings).Error; err != nil {
		return err
	}

	for _, binding := range bindings {
		group := strings.TrimSpace(binding.Group)
		if group == "" {
			continue
		}

		var template ReferralTemplate
		if err := tx.First(&template, binding.TemplateId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if !template.Enabled || strings.TrimSpace(template.ReferralType) != ReferralTypeSubscription || strings.TrimSpace(template.Group) != group {
			continue
		}

		totalRateBps := SubscriptionReferralTemplateVisibleTotalRateBps(template)
		if totalRateBps <= 0 {
			continue
		}
		inviteeShareBps := normalizedBps
		if inviteeShareBps > totalRateBps {
			inviteeShareBps = totalRateBps
		}

		override := ReferralInviteeShareOverride{
			InviterUserId:   inviterUserID,
			InviteeUserId:   inviteeUserID,
			ReferralType:    ReferralTypeSubscription,
			Group:           group,
			InviteeShareBps: inviteeShareBps,
			CreatedBy:       inviterUserID,
			UpdatedBy:       inviterUserID,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "inviter_user_id"},
				{Name: "invitee_user_id"},
				{Name: "referral_type"},
				{Name: "group"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"invitee_share_bps": override.InviteeShareBps,
				"updated_by":        inviterUserID,
				"updated_at":        common.GetTimestamp(),
			}),
		}).Create(&override).Error; err != nil {
			return err
		}
	}

	return nil
}

func SubscriptionReferralTemplateVisibleTotalRateBps(template ReferralTemplate) int {
	if template.LevelType == ReferralLevelTypeTeam {
		return NormalizeSubscriptionReferralRateBps(template.TeamCapBps)
	}
	return NormalizeSubscriptionReferralRateBps(template.DirectCapBps)
}
