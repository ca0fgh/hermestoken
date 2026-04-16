package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ReferralEngineModeLegacy   = "legacy"
	ReferralEngineModeTemplate = "template"
)

type ReferralEngineRoute struct {
	Id           int    `json:"id"`
	ReferralType string `json:"referral_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_referral_engine_route_scope,priority:1"`
	Group        string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_referral_engine_route_scope,priority:2"`
	EngineMode   string `json:"engine_mode" gorm:"type:varchar(32);not null;default:'legacy'"`
	CreatedBy    int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy    int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

func (r *ReferralEngineRoute) normalize() {
	r.ReferralType = strings.TrimSpace(r.ReferralType)
	r.Group = strings.TrimSpace(r.Group)
	r.EngineMode = strings.TrimSpace(r.EngineMode)
	if r.EngineMode == "" {
		r.EngineMode = ReferralEngineModeLegacy
	}
}

func (r *ReferralEngineRoute) Validate() error {
	r.normalize()
	if r.ReferralType == "" {
		return errors.New("referral type is required")
	}
	if r.Group == "" {
		return errors.New("group is required")
	}
	if r.EngineMode != ReferralEngineModeLegacy && r.EngineMode != ReferralEngineModeTemplate {
		return errors.New("invalid engine mode")
	}
	return nil
}

func (r *ReferralEngineRoute) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return r.Validate()
}

func (r *ReferralEngineRoute) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return r.Validate()
}

func ResolveReferralEngineMode(referralType string, group string) (string, error) {
	var route ReferralEngineRoute
	err := DB.Where("referral_type = ? AND "+commonGroupCol+" = ?", strings.TrimSpace(referralType), strings.TrimSpace(group)).First(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReferralEngineModeLegacy, nil
	}
	if err != nil {
		return "", err
	}
	return route.EngineMode, nil
}
