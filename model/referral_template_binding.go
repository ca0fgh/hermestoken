package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReferralTemplateBinding struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"type:int;not null;uniqueIndex:idx_referral_template_binding_scope"`
	ReferralType string `json:"referral_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_referral_template_binding_scope"`
	Group        string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_referral_template_binding_scope"`
	TemplateId   int    `json:"template_id" gorm:"type:int;not null;index"`
	CreatedBy    int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy    int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

type ReferralTemplateBindingView struct {
	Binding  ReferralTemplateBinding `json:"binding"`
	Template ReferralTemplate        `json:"template"`
}

func (b *ReferralTemplateBinding) normalize() {
	b.ReferralType = strings.TrimSpace(b.ReferralType)
	b.Group = strings.TrimSpace(b.Group)
}

func (b *ReferralTemplateBinding) ValidateAgainstTemplate(template *ReferralTemplate) error {
	b.normalize()
	if template == nil {
		return errors.New("template is required")
	}

	templateReferralType := strings.TrimSpace(template.ReferralType)
	templateGroup := strings.TrimSpace(template.Group)
	if templateReferralType == "" {
		return fmt.Errorf("template %d referral type is required", template.Id)
	}
	if templateGroup == "" {
		return fmt.Errorf("template %d group is required", template.Id)
	}
	b.ReferralType = templateReferralType
	b.Group = templateGroup
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

func HasActiveReferralTemplateBinding(userID int, referralType string, group string) (bool, *ReferralTemplateBinding, error) {
	if userID <= 0 {
		return false, nil, errors.New("invalid user id")
	}

	var binding ReferralTemplateBinding
	err := DB.Where(
		"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		userID,
		strings.TrimSpace(referralType),
		strings.TrimSpace(group),
	).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	template, err := GetReferralTemplateByID(binding.TemplateId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, &binding, nil
		}
		return false, nil, err
	}
	if !template.Enabled {
		return false, &binding, nil
	}
	return true, &binding, nil
}

func ListReferralTemplateBindingsByUser(userID int, referralType string) ([]ReferralTemplateBindingView, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var bindings []ReferralTemplateBinding
	query := DB.Where("user_id = ?", userID).Order(commonGroupCol + " ASC")
	if trimmedReferralType := strings.TrimSpace(referralType); trimmedReferralType != "" {
		query = query.Where("referral_type = ?", trimmedReferralType)
	}
	if err := query.Find(&bindings).Error; err != nil {
		return nil, err
	}

	views := make([]ReferralTemplateBindingView, 0, len(bindings))
	for _, binding := range bindings {
		template, err := GetReferralTemplateByID(binding.TemplateId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		views = append(views, ReferralTemplateBindingView{
			Binding:  binding,
			Template: *template,
		})
	}
	return views, nil
}

func ResolveBindingInviteeShareDefault(view ReferralTemplateBindingView) int {
	return NormalizeSubscriptionReferralRateBps(view.Template.InviteeShareDefaultBps)
}

func GetReferralTemplateBindingViewByUserAndScope(userID int, referralType string, group string) (*ReferralTemplateBindingView, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var binding ReferralTemplateBinding
	err := DB.Where(
		"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		userID,
		strings.TrimSpace(referralType),
		strings.TrimSpace(group),
	).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	template, err := GetReferralTemplateByID(binding.TemplateId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	view := &ReferralTemplateBindingView{
		Binding:  binding,
		Template: *template,
	}
	return view, nil
}

func UpsertReferralTemplateBinding(binding *ReferralTemplateBinding) (*ReferralTemplateBinding, error) {
	if binding == nil {
		return nil, errors.New("binding is required")
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := binding.validateWithTemplateID(tx); err != nil {
			return err
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "referral_type"},
				{Name: "group"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"template_id": binding.TemplateId,
				"updated_by":  binding.UpdatedBy,
				"updated_at":  common.GetTimestamp(),
			}),
		}).Create(binding).Error; err != nil {
			return err
		}

		return tx.Where(
			"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
			binding.UserId,
			binding.ReferralType,
			binding.Group,
		).First(binding).Error
	})
	if err != nil {
		return nil, err
	}
	return binding, nil
}
