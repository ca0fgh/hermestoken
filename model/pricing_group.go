package model

import "gorm.io/gorm"

type PricingGroup struct {
	Id             int            `json:"id"`
	GroupKey       string         `json:"group_key" gorm:"type:varchar(64);not null;uniqueIndex"`
	DisplayName    string         `json:"display_name" gorm:"type:varchar(128);not null;default:''"`
	Description    string         `json:"description" gorm:"type:text"`
	BillingRatio   float64        `json:"billing_ratio" gorm:"type:decimal(12,6);not null;default:1"`
	UserSelectable bool           `json:"user_selectable" gorm:"not null;default:false"`
	Status         string         `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	SortOrder      int            `json:"sort_order" gorm:"type:int;not null;default:0"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type PricingGroupAlias struct {
	Id       int    `json:"id"`
	AliasKey string `json:"alias_key" gorm:"type:varchar(64);not null;uniqueIndex"`
	GroupId  int    `json:"group_id" gorm:"not null;index"`
	Reason   string `json:"reason" gorm:"type:varchar(255);not null;default:''"`
}

type PricingGroupRatioOverride struct {
	Id            int     `json:"id"`
	SourceGroupId int     `json:"source_group_id" gorm:"not null;index"`
	TargetGroupId int     `json:"target_group_id" gorm:"not null;index"`
	Ratio         float64 `json:"ratio" gorm:"type:decimal(12,6);not null;default:1"`
}

type PricingGroupVisibilityRule struct {
	Id                  int    `json:"id"`
	SubjectGroupId      int    `json:"subject_group_id" gorm:"not null;index"`
	Action              string `json:"action" gorm:"type:varchar(32);not null;default:'show';index"`
	TargetGroupId       int    `json:"target_group_id" gorm:"not null;index"`
	DescriptionOverride string `json:"description_override" gorm:"type:varchar(255);not null;default:''"`
	SortOrder           int    `json:"sort_order" gorm:"type:int;not null;default:0"`
}

type PricingGroupAutoPriority struct {
	Id       int `json:"id"`
	GroupId  int `json:"group_id" gorm:"not null;index"`
	Priority int `json:"priority" gorm:"type:int;not null;default:0"`
}
