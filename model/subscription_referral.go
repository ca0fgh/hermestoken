package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	SubscriptionReferralMaxRateBps = 10000
)

type SubscriptionReferralConfig struct {
	Enabled        bool `json:"enabled"`
	TotalRateBps   int  `json:"total_rate_bps"`
	InviteeRateBps int  `json:"invitee_rate_bps"`
	InviterRateBps int  `json:"inviter_rate_bps"`
}

type SubscriptionReferralOverride struct {
	Id           int   `json:"id"`
	UserId       int   `json:"user_id" gorm:"uniqueIndex"`
	TotalRateBps int   `json:"total_rate_bps" gorm:"type:int;not null;default:0"`
	CreatedBy    int   `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy    int   `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt    int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64 `json:"updated_at" gorm:"bigint"`
}

func (o *SubscriptionReferralOverride) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	o.CreatedAt = now
	o.UpdatedAt = now
	o.TotalRateBps = NormalizeSubscriptionReferralRateBps(o.TotalRateBps)
	return nil
}

func (o *SubscriptionReferralOverride) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = common.GetTimestamp()
	o.TotalRateBps = NormalizeSubscriptionReferralRateBps(o.TotalRateBps)
	return nil
}

type SubscriptionReferralRecord struct {
	Id                     int     `json:"id"`
	OrderId                int     `json:"order_id" gorm:"index;uniqueIndex:idx_sub_referral_once"`
	OrderTradeNo           string  `json:"order_trade_no" gorm:"type:varchar(255);index"`
	PlanId                 int     `json:"plan_id" gorm:"index"`
	PayerUserId            int     `json:"payer_user_id" gorm:"index"`
	InviterUserId          int     `json:"inviter_user_id" gorm:"index"`
	BeneficiaryUserId      int     `json:"beneficiary_user_id" gorm:"index;uniqueIndex:idx_sub_referral_once"`
	BeneficiaryRole        string  `json:"beneficiary_role" gorm:"type:varchar(16);uniqueIndex:idx_sub_referral_once"`
	OrderPaidAmount        float64 `json:"order_paid_amount" gorm:"type:decimal(10,6);not null;default:0"`
	QuotaPerUnitSnapshot   float64 `json:"quota_per_unit_snapshot" gorm:"type:decimal(18,6);not null;default:0"`
	TotalRateBpsSnapshot   int     `json:"total_rate_bps_snapshot" gorm:"type:int;not null;default:0"`
	InviteeRateBpsSnapshot int     `json:"invitee_rate_bps_snapshot" gorm:"type:int;not null;default:0"`
	AppliedRateBps         int     `json:"applied_rate_bps" gorm:"type:int;not null;default:0"`
	RewardQuota            int64   `json:"reward_quota" gorm:"type:bigint;not null;default:0"`
	ReversedQuota          int64   `json:"reversed_quota" gorm:"type:bigint;not null;default:0"`
	DebtQuota              int64   `json:"debt_quota" gorm:"type:bigint;not null;default:0"`
	Status                 string  `json:"status" gorm:"type:varchar(32);not null;default:'credited';index"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionReferralRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	r.TotalRateBpsSnapshot = NormalizeSubscriptionReferralRateBps(r.TotalRateBpsSnapshot)
	r.InviteeRateBpsSnapshot = NormalizeSubscriptionReferralRateBps(r.InviteeRateBpsSnapshot)
	r.AppliedRateBps = NormalizeSubscriptionReferralRateBps(r.AppliedRateBps)
	return nil
}

func (r *SubscriptionReferralRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func NormalizeSubscriptionReferralRateBps(rateBps int) int {
	if rateBps < 0 {
		return 0
	}
	if rateBps > SubscriptionReferralMaxRateBps {
		return SubscriptionReferralMaxRateBps
	}
	return rateBps
}

func ResolveSubscriptionReferralConfig(setting dto.UserSetting, override *SubscriptionReferralOverride) SubscriptionReferralConfig {
	config := SubscriptionReferralConfig{
		Enabled: common.SubscriptionReferralEnabled,
	}
	if !config.Enabled {
		return config
	}

	totalRateBps := NormalizeSubscriptionReferralRateBps(common.SubscriptionReferralGlobalRateBps)
	if override != nil {
		totalRateBps = NormalizeSubscriptionReferralRateBps(override.TotalRateBps)
	}

	inviteeRateBps := NormalizeSubscriptionReferralRateBps(setting.SubscriptionReferralInviteeRateBps)
	if inviteeRateBps > totalRateBps {
		inviteeRateBps = totalRateBps
	}

	config.TotalRateBps = totalRateBps
	config.InviteeRateBps = inviteeRateBps
	config.InviterRateBps = totalRateBps - inviteeRateBps
	return config
}

func CalculateSubscriptionReferralQuota(orderMoney float64, rateBps int, quotaPerUnit float64) int {
	normalizedRateBps := NormalizeSubscriptionReferralRateBps(rateBps)
	if orderMoney <= 0 || normalizedRateBps == 0 || quotaPerUnit <= 0 {
		return 0
	}

	return int(
		decimal.NewFromFloat(orderMoney).
			Mul(decimal.NewFromFloat(quotaPerUnit)).
			Mul(decimal.NewFromInt(int64(normalizedRateBps))).
			Div(decimal.NewFromInt(SubscriptionReferralMaxRateBps)).
			IntPart(),
	)
}
