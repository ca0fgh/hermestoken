package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ReferralTypeSubscription = "subscription_referral"

	ReferralLevelTypeDirect = "direct"
	ReferralLevelTypeTeam   = "team"
)

type ReferralTemplate struct {
	Id                     int     `json:"id"`
	ReferralType           string  `json:"referral_type" gorm:"type:varchar(64);not null;index:idx_referral_template_scope_name,priority:1"`
	Group                  string  `json:"group" gorm:"type:varchar(64);not null;default:'';index:idx_referral_template_scope_name,priority:2"`
	Name                   string  `json:"name" gorm:"type:varchar(128);not null;index:idx_referral_template_scope_name,priority:3"`
	LevelType              string  `json:"level_type" gorm:"type:varchar(32);not null;index"`
	Enabled                bool    `json:"enabled" gorm:"not null;default:false"`
	DirectCapBps           int     `json:"direct_cap_bps" gorm:"type:int;not null;default:0"`
	TeamCapBps             int     `json:"team_cap_bps" gorm:"type:int;not null;default:0"`
	TeamDecayRatio         float64 `json:"team_decay_ratio" gorm:"type:decimal(10,6);not null;default:0"`
	TeamMaxDepth           int     `json:"team_max_depth" gorm:"type:int;not null;default:0"`
	InviteeShareDefaultBps int     `json:"invitee_share_default_bps" gorm:"type:int;not null;default:0"`
	CreatedBy              int     `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy              int     `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint"`
}

func (t *ReferralTemplate) normalize() {
	t.ReferralType = strings.TrimSpace(t.ReferralType)
	t.Group = strings.TrimSpace(t.Group)
	t.Name = strings.TrimSpace(t.Name)
	t.LevelType = strings.TrimSpace(t.LevelType)
	t.InviteeShareDefaultBps = NormalizeSubscriptionReferralRateBps(t.InviteeShareDefaultBps)
}

func (t *ReferralTemplate) Validate() error {
	t.normalize()
	if t.ReferralType == "" {
		return fmt.Errorf("referral type is required")
	}
	if t.Group == "" {
		return fmt.Errorf("group is required")
	}
	if t.Name == "" {
		return fmt.Errorf("name is required")
	}

	if t.ReferralType != ReferralTypeSubscription {
		return nil
	}

	if t.LevelType != ReferralLevelTypeDirect && t.LevelType != ReferralLevelTypeTeam {
		return fmt.Errorf("invalid subscription level type: %s", t.LevelType)
	}
	if t.DirectCapBps < 0 || t.TeamCapBps < t.DirectCapBps || t.TeamCapBps > SubscriptionReferralMaxRateBps {
		return fmt.Errorf("invalid subscription cap bps")
	}
	if t.TeamDecayRatio <= 0 || t.TeamDecayRatio > 1 {
		return fmt.Errorf("invalid subscription decay ratio")
	}
	if t.TeamMaxDepth < 1 {
		return fmt.Errorf("invalid subscription max depth")
	}
	return nil
}

func (t *ReferralTemplate) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	t.CreatedAt = now
	t.UpdatedAt = now
	return t.Validate()
}

func (t *ReferralTemplate) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = common.GetTimestamp()
	return t.Validate()
}

func GetReferralTemplateByID(id int) (*ReferralTemplate, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var template ReferralTemplate
	if err := DB.First(&template, id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}
