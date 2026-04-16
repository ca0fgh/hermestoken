package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ReferralInviteeShareOverride struct {
	Id              int    `json:"id"`
	InviterUserId   int    `json:"inviter_user_id" gorm:"type:int;not null;uniqueIndex:idx_referral_invitee_share_override_scope,priority:1"`
	InviteeUserId   int    `json:"invitee_user_id" gorm:"type:int;not null;uniqueIndex:idx_referral_invitee_share_override_scope,priority:2"`
	ReferralType    string `json:"referral_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_referral_invitee_share_override_scope,priority:3"`
	Group           string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_referral_invitee_share_override_scope,priority:4"`
	InviteeShareBps int    `json:"invitee_share_bps" gorm:"type:int;not null;default:0"`
	CreatedBy       int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy       int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

func (o *ReferralInviteeShareOverride) normalize() {
	o.ReferralType = strings.TrimSpace(o.ReferralType)
	o.Group = strings.TrimSpace(o.Group)
	o.InviteeShareBps = NormalizeSubscriptionReferralRateBps(o.InviteeShareBps)
}

func (o *ReferralInviteeShareOverride) Validate() error {
	o.normalize()
	if o.InviterUserId <= 0 {
		return errors.New("inviter user id is required")
	}
	if o.InviteeUserId <= 0 {
		return errors.New("invitee user id is required")
	}
	if o.ReferralType == "" {
		return errors.New("referral type is required")
	}
	if o.Group == "" {
		return errors.New("group is required")
	}
	return nil
}

func (o *ReferralInviteeShareOverride) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	o.CreatedAt = now
	o.UpdatedAt = now
	return o.Validate()
}

func (o *ReferralInviteeShareOverride) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = common.GetTimestamp()
	return o.Validate()
}
