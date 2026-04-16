package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ReferralTemplateBinding struct {
	Id                      int    `json:"id"`
	UserId                  int    `json:"user_id" gorm:"type:int;not null;uniqueIndex:idx_referral_template_binding_scope"`
	ReferralType            string `json:"referral_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_referral_template_binding_scope"`
	Group                   string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_referral_template_binding_scope"`
	TemplateId              int    `json:"template_id" gorm:"type:int;not null;index"`
	InviteeShareOverrideBps *int   `json:"invitee_share_override_bps" gorm:"type:int"`
	CreatedBy               int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy               int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt               int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt               int64  `json:"updated_at" gorm:"bigint"`
}

func (b *ReferralTemplateBinding) normalize() {
	b.ReferralType = strings.TrimSpace(b.ReferralType)
	b.Group = strings.TrimSpace(b.Group)
	if b.InviteeShareOverrideBps != nil {
		normalized := NormalizeSubscriptionReferralRateBps(*b.InviteeShareOverrideBps)
		b.InviteeShareOverrideBps = &normalized
	}
}

func (b *ReferralTemplateBinding) ValidateAgainstTemplate(template *ReferralTemplate) error {
	b.normalize()
	if template == nil {
		return errors.New("template is required")
	}
	if b.ReferralType == "" {
		return errors.New("binding referral type is required")
	}
	if b.Group == "" {
		return errors.New("binding group is required")
	}

	templateReferralType := strings.TrimSpace(template.ReferralType)
	templateGroup := strings.TrimSpace(template.Group)
	if b.ReferralType != templateReferralType {
		return fmt.Errorf("binding referral type %q does not match template referral type %q", b.ReferralType, templateReferralType)
	}
	if b.Group != templateGroup {
		return fmt.Errorf("binding group %q does not match template group %q", b.Group, templateGroup)
	}
	return nil
}

func (b *ReferralTemplateBinding) validateWithTemplateID(tx *gorm.DB) error {
	b.normalize()
	if b.TemplateId <= 0 {
		return errors.New("template_id is required")
	}
	var template ReferralTemplate
	if err := tx.First(&template, b.TemplateId).Error; err != nil {
		return err
	}
	return b.ValidateAgainstTemplate(&template)
}

func (b *ReferralTemplateBinding) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	b.CreatedAt = now
	b.UpdatedAt = now
	return b.validateWithTemplateID(tx)
}

func (b *ReferralTemplateBinding) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = common.GetTimestamp()
	return b.validateWithTemplateID(tx)
}
