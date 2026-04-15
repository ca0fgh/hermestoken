package model

type PricingGroup struct {
	Id          int    `json:"id"`
	GroupKey    string `json:"group_key" gorm:"type:varchar(64);not null;uniqueIndex"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;default:''"`
	Description string `json:"description" gorm:"type:text"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
}

type PricingGroupAlias struct {
	Id        int    `json:"id"`
	GroupKey  string `json:"group_key" gorm:"type:varchar(64);not null;index"`
	AliasKey  string `json:"alias_key" gorm:"type:varchar(64);not null;uniqueIndex"`
	AliasName string `json:"alias_name" gorm:"type:varchar(128);not null;default:''"`
}

type PricingGroupRatioOverride struct {
	Id        int     `json:"id"`
	GroupKey  string  `json:"group_key" gorm:"type:varchar(64);not null;index"`
	ScopeType string  `json:"scope_type" gorm:"type:varchar(32);not null;default:'global';index"`
	ScopeKey  string  `json:"scope_key" gorm:"type:varchar(128);not null;default:'';index"`
	Ratio     float64 `json:"ratio" gorm:"type:decimal(12,6);not null;default:1"`
}

type PricingGroupVisibilityRule struct {
	Id          int    `json:"id"`
	GroupKey    string `json:"group_key" gorm:"type:varchar(64);not null;index"`
	SubjectType string `json:"subject_type" gorm:"type:varchar(32);not null;default:'user_group';index"`
	SubjectKey  string `json:"subject_key" gorm:"type:varchar(128);not null;default:'';index"`
	IsVisible   bool   `json:"is_visible" gorm:"not null;default:true"`
}

type PricingGroupAutoPriority struct {
	Id         int    `json:"id"`
	TargetType string `json:"target_type" gorm:"type:varchar(32);not null;default:'user';index"`
	TargetKey  string `json:"target_key" gorm:"type:varchar(128);not null;default:'';index"`
	GroupKey   string `json:"group_key" gorm:"type:varchar(64);not null;index"`
	Priority   int    `json:"priority" gorm:"type:int;not null;default:0"`
}
