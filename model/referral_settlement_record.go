package model

import (
	"errors"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"gorm.io/gorm"
)

type ReferralSettlementRecord struct {
	Id                       int      `json:"id"`
	BatchId                  int      `json:"batch_id" gorm:"type:int;not null;index"`
	ReferralType             string   `json:"referral_type" gorm:"type:varchar(64);not null;index"`
	Group                    string   `json:"group" gorm:"type:varchar(64);not null;default:'';index"`
	BeneficiaryUserId        int      `json:"beneficiary_user_id" gorm:"type:int;not null;index"`
	BeneficiaryLevelType     *string  `json:"beneficiary_level_type" gorm:"type:varchar(32)"`
	RewardComponent          string   `json:"reward_component" gorm:"type:varchar(64);not null;index"`
	SourceRewardComponent    *string  `json:"source_reward_component" gorm:"type:varchar(64)"`
	PathDistance             *int     `json:"path_distance"`
	MatchedTeamIndex         *int     `json:"matched_team_index"`
	WeightSnapshot           *float64 `json:"weight_snapshot" gorm:"type:decimal(18,10)"`
	ShareSnapshot            *float64 `json:"share_snapshot" gorm:"type:decimal(18,10)"`
	GrossRewardQuotaSnapshot *int64   `json:"gross_reward_quota_snapshot" gorm:"type:bigint"`
	InviteeShareBpsSnapshot  *int     `json:"invitee_share_bps_snapshot" gorm:"type:int"`
	PoolRateBpsSnapshot      *int     `json:"pool_rate_bps_snapshot" gorm:"type:int"`
	AppliedRateBps           *int     `json:"applied_rate_bps" gorm:"type:int"`
	RewardQuota              int64    `json:"reward_quota" gorm:"type:bigint;not null;default:0"`
	ReversedQuota            int64    `json:"reversed_quota" gorm:"type:bigint;not null;default:0"`
	DebtQuota                int64    `json:"debt_quota" gorm:"type:bigint;not null;default:0"`
	Status                   string   `json:"status" gorm:"type:varchar(32);not null;default:'credited';index"`
	CreatedAt                int64    `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64    `json:"updated_at" gorm:"bigint"`
}

func (r *ReferralSettlementRecord) normalize() {
	r.ReferralType = strings.TrimSpace(r.ReferralType)
	r.Group = strings.TrimSpace(r.Group)
	r.RewardComponent = strings.TrimSpace(r.RewardComponent)
	r.Status = strings.TrimSpace(r.Status)
	if r.BeneficiaryLevelType != nil {
		trimmed := strings.TrimSpace(*r.BeneficiaryLevelType)
		r.BeneficiaryLevelType = &trimmed
	}
	if r.SourceRewardComponent != nil {
		trimmed := strings.TrimSpace(*r.SourceRewardComponent)
		r.SourceRewardComponent = &trimmed
	}
}

func (r *ReferralSettlementRecord) Validate() error {
	r.normalize()
	if r.BatchId <= 0 {
		return errors.New("batch id is required")
	}
	if r.ReferralType == "" {
		return errors.New("referral type is required")
	}
	if r.Group == "" {
		return errors.New("group is required")
	}
	if r.RewardComponent == "" {
		return errors.New("reward component is required")
	}
	return nil
}

func (r *ReferralSettlementRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return r.Validate()
}

func (r *ReferralSettlementRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return r.Validate()
}
