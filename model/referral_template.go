package model

import (
	"errors"
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
	Name                   string  `json:"name" gorm:"type:varchar(128);not null;index:idx_referral_template_scope_name,priority:3;uniqueIndex:uk_referral_template_name"`
	LevelType              string  `json:"level_type" gorm:"type:varchar(32);not null;index"`
	Enabled                bool    `json:"enabled" gorm:"not null;default:false"`
	DirectCapBps           int     `json:"direct_cap_bps" gorm:"type:int;not null;default:0"`
	TeamCapBps             int     `json:"team_cap_bps" gorm:"type:int;not null;default:0"`
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
	if t.ReferralType == ReferralTypeSubscription {
		switch t.LevelType {
		case ReferralLevelTypeDirect:
			t.TeamCapBps = 0
		case ReferralLevelTypeTeam:
			t.DirectCapBps = 0
		}
	}
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
	switch t.LevelType {
	case ReferralLevelTypeDirect:
		if t.DirectCapBps < 0 || t.DirectCapBps > SubscriptionReferralMaxRateBps {
			return fmt.Errorf("invalid subscription cap bps")
		}
	case ReferralLevelTypeTeam:
		if t.TeamCapBps < 0 || t.TeamCapBps > SubscriptionReferralMaxRateBps {
			return fmt.Errorf("invalid subscription cap bps")
		}
	}
	return nil
}

func (t *ReferralTemplate) validateUniqueName(tx *gorm.DB) error {
	if tx == nil {
		tx = DB
	}
	if tx == nil {
		return errors.New("database is not initialized")
	}

	var existing ReferralTemplate
	err := tx.Where("name = ? AND id <> ?", t.Name, t.Id).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("template name already exists")
}

func (t *ReferralTemplate) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := t.Validate(); err != nil {
		return err
	}
	return t.validateUniqueName(tx)
}

func (t *ReferralTemplate) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = common.GetTimestamp()
	if err := t.Validate(); err != nil {
		return err
	}
	return t.validateUniqueName(tx)
}

func normalizeReferralTemplatePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	lowerError := strings.ToLower(err.Error())
	if strings.Contains(lowerError, "uk_referral_template_name") ||
		strings.Contains(lowerError, "referral_templates.name") {
		return fmt.Errorf("template name already exists")
	}
	return err
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

func CreateReferralTemplate(template *ReferralTemplate) error {
	if template == nil {
		return fmt.Errorf("template is required")
	}
	return normalizeReferralTemplatePersistenceError(DB.Create(template).Error)
}

func UpdateReferralTemplate(template *ReferralTemplate) error {
	if template == nil {
		return fmt.Errorf("template is required")
	}
	return normalizeReferralTemplatePersistenceError(DB.Save(template).Error)
}

func DeleteReferralTemplate(id int) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	return DB.Delete(&ReferralTemplate{}, id).Error
}

func ListReferralTemplates(referralType string, group string) ([]ReferralTemplate, error) {
	query := DB.Model(&ReferralTemplate{}).Order("referral_type ASC, " + commonGroupCol + " ASC, name ASC")
	if trimmedReferralType := strings.TrimSpace(referralType); trimmedReferralType != "" {
		query = query.Where("referral_type = ?", trimmedReferralType)
	}
	if trimmedGroup := strings.TrimSpace(group); trimmedGroup != "" {
		query = query.Where(commonGroupCol+" = ?", trimmedGroup)
	}

	var templates []ReferralTemplate
	if err := query.Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}
