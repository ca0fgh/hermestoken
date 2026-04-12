package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SubscriptionReferralMaxRateBps = 10000

	SubscriptionReferralStatusCredited      = "credited"
	SubscriptionReferralStatusReversed      = "reversed"
	SubscriptionReferralStatusPartialRevert = "partially_reversed"

	SubscriptionReferralBeneficiaryRoleInviter = "inviter"
	SubscriptionReferralBeneficiaryRoleInvitee = "invitee"
)

var ErrSubscriptionReferralRecordNotFound = errors.New("subscription referral record not found")

type SubscriptionReferralConfig struct {
	Enabled        bool `json:"enabled"`
	TotalRateBps   int  `json:"total_rate_bps"`
	InviteeRateBps int  `json:"invitee_rate_bps"`
	InviterRateBps int  `json:"inviter_rate_bps"`
}

type SubscriptionReferralOverride struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"uniqueIndex:idx_sub_referral_override_group"`
	Group        string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_sub_referral_override_group"`
	TotalRateBps int    `json:"total_rate_bps" gorm:"type:int;not null;default:0"`
	CreatedBy    int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy    int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

func (o *SubscriptionReferralOverride) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	o.CreatedAt = now
	o.UpdatedAt = now
	o.Group = strings.TrimSpace(o.Group)
	o.TotalRateBps = NormalizeSubscriptionReferralRateBps(o.TotalRateBps)
	return nil
}

func (o *SubscriptionReferralOverride) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = common.GetTimestamp()
	o.Group = strings.TrimSpace(o.Group)
	o.TotalRateBps = NormalizeSubscriptionReferralRateBps(o.TotalRateBps)
	return nil
}

type SubscriptionReferralRecord struct {
	Id                     int     `json:"id"`
	OrderId                int     `json:"order_id" gorm:"index;uniqueIndex:idx_sub_referral_once"`
	OrderTradeNo           string  `json:"order_trade_no" gorm:"type:varchar(255);index"`
	PlanId                 int     `json:"plan_id" gorm:"index"`
	ReferralGroup          string  `json:"referral_group" gorm:"type:varchar(64);not null;default:'';index"`
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
	r.ReferralGroup = strings.TrimSpace(r.ReferralGroup)
	r.TotalRateBpsSnapshot = NormalizeSubscriptionReferralRateBps(r.TotalRateBpsSnapshot)
	r.InviteeRateBpsSnapshot = NormalizeSubscriptionReferralRateBps(r.InviteeRateBpsSnapshot)
	r.AppliedRateBps = NormalizeSubscriptionReferralRateBps(r.AppliedRateBps)
	if r.Status == "" {
		r.Status = SubscriptionReferralStatusCredited
	}
	return nil
}

func (r *SubscriptionReferralRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	r.ReferralGroup = strings.TrimSpace(r.ReferralGroup)
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

func GetEffectiveSubscriptionReferralInviteeRateBps(setting dto.UserSetting, group string, totalRateBps int) int {
	total := NormalizeSubscriptionReferralRateBps(totalRateBps)
	invitee := setting.SubscriptionReferralInviteeRateBps
	if group != "" {
		if groupedRates := setting.SubscriptionReferralInviteeRateBpsByGroup; groupedRates != nil {
			if groupedRate, ok := groupedRates[group]; ok {
				invitee = groupedRate
			}
		}
	}
	invitee = NormalizeSubscriptionReferralRateBps(invitee)
	if invitee > total {
		invitee = total
	}
	return invitee
}

func ResolveSubscriptionReferralConfig(totalRateBps int, inviteeRateBps int) SubscriptionReferralConfig {
	total := NormalizeSubscriptionReferralRateBps(totalRateBps)
	invitee := NormalizeSubscriptionReferralRateBps(inviteeRateBps)
	if invitee > total {
		invitee = total
	}
	return SubscriptionReferralConfig{
		Enabled:        common.SubscriptionReferralEnabled && total > 0,
		TotalRateBps:   total,
		InviteeRateBps: invitee,
		InviterRateBps: total - invitee,
	}
}

func CalculateSubscriptionReferralQuota(orderMoney float64, rateBps int) int {
	normalizedRateBps := NormalizeSubscriptionReferralRateBps(rateBps)
	if orderMoney <= 0 || normalizedRateBps == 0 || common.QuotaPerUnit <= 0 {
		return 0
	}

	return int(
		decimal.NewFromFloat(orderMoney).
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
			Mul(decimal.NewFromInt(int64(normalizedRateBps))).
			Div(decimal.NewFromInt(SubscriptionReferralMaxRateBps)).
			IntPart(),
	)
}

func GetSubscriptionReferralOverrideByUserID(userID int) (*SubscriptionReferralOverride, error) {
	return GetLegacySubscriptionReferralOverrideByUserID(userID)
}

func GetLegacySubscriptionReferralOverrideByUserID(userID int) (*SubscriptionReferralOverride, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var override SubscriptionReferralOverride
	if err := DB.Where("user_id = ? AND `group` = ?", userID, "default").First(&override).Error; err == nil {
		return &override, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := DB.Where("user_id = ? AND `group` = ?", userID, "").First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func getLegacyUngroupedSubscriptionReferralOverrideByUserID(userID int) (*SubscriptionReferralOverride, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var override SubscriptionReferralOverride
	if err := DB.Where("user_id = ? AND `group` = ?", userID, "").First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func GetSubscriptionReferralOverrideByUserIDAndGroup(userID int, group string) (*SubscriptionReferralOverride, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var override SubscriptionReferralOverride
	trimmedGroup := strings.TrimSpace(group)
	if err := DB.Where("user_id = ? AND `group` = ?", userID, trimmedGroup).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func UpsertSubscriptionReferralOverride(userID int, group string, totalRateBps int, operatorID int) (*SubscriptionReferralOverride, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if _, err := GetUserById(userID, false); err != nil {
		return nil, err
	}

	group = strings.TrimSpace(group)

	override := &SubscriptionReferralOverride{
		UserId:       userID,
		Group:        group,
		TotalRateBps: NormalizeSubscriptionReferralRateBps(totalRateBps),
		CreatedBy:    operatorID,
		UpdatedBy:    operatorID,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "group"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"total_rate_bps": override.TotalRateBps,
				"updated_by":     operatorID,
				"updated_at":     common.GetTimestamp(),
			}),
		}).Create(override).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND `group` = ?", userID, group).First(override).Error
	})
	if err != nil {
		return nil, err
	}
	return override, nil
}

func DeleteSubscriptionReferralOverrideByUserID(userID int) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	return DB.Where("user_id = ?", userID).Delete(&SubscriptionReferralOverride{}).Error
}

func DeleteSubscriptionReferralOverrideByUserIDAndGroup(userID int, group string) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	return DB.Where("user_id = ? AND `group` = ?", userID, strings.TrimSpace(group)).Delete(&SubscriptionReferralOverride{}).Error
}

func GetEffectiveSubscriptionReferralTotalRateBps(userID int, group string) int {
	resolvedGroup := strings.TrimSpace(group)
	if resolvedGroup != "" {
		if userID > 0 {
			override, err := GetSubscriptionReferralOverrideByUserIDAndGroup(userID, resolvedGroup)
			if err == nil && override != nil {
				return NormalizeSubscriptionReferralRateBps(override.TotalRateBps)
			}

			override, err = getLegacyUngroupedSubscriptionReferralOverrideByUserID(userID)
			if err == nil && override != nil {
				return NormalizeSubscriptionReferralRateBps(override.TotalRateBps)
			}
		}
		if common.HasSubscriptionReferralGroupRatesConfigured() {
			return NormalizeSubscriptionReferralRateBps(common.GetSubscriptionReferralGroupRate(resolvedGroup))
		}
		return NormalizeSubscriptionReferralRateBps(common.SubscriptionReferralGlobalRateBps)
	}

	if userID > 0 {
		override, err := GetSubscriptionReferralOverrideByUserID(userID)
		if err == nil && override != nil {
			return NormalizeSubscriptionReferralRateBps(override.TotalRateBps)
		}
	}
	return NormalizeSubscriptionReferralRateBps(common.SubscriptionReferralGlobalRateBps)
}

func ApplySubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	if tx == nil || order == nil || plan == nil || !common.SubscriptionReferralEnabled || order.Money <= 0 {
		return nil
	}

	group := strings.TrimSpace(plan.UpgradeGroup)
	if group == "" {
		return nil
	}

	var invitee User
	if err := tx.First(&invitee, order.UserId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil
	}

	var inviter User
	if err := tx.First(&inviter, invitee.InviterId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	totalRateBps := GetEffectiveSubscriptionReferralTotalRateBps(inviter.Id, group)
	inviteeRateBps := GetEffectiveSubscriptionReferralInviteeRateBps(inviter.GetSetting(), group, totalRateBps)
	cfg := ResolveSubscriptionReferralConfig(totalRateBps, inviteeRateBps)
	if !cfg.Enabled {
		return nil
	}

	records := []SubscriptionReferralRecord{
		{
			OrderId:                order.Id,
			OrderTradeNo:           order.TradeNo,
			PlanId:                 order.PlanId,
			ReferralGroup:          group,
			PayerUserId:            order.UserId,
			InviterUserId:          invitee.InviterId,
			BeneficiaryUserId:      invitee.InviterId,
			BeneficiaryRole:        SubscriptionReferralBeneficiaryRoleInviter,
			OrderPaidAmount:        order.Money,
			QuotaPerUnitSnapshot:   common.QuotaPerUnit,
			TotalRateBpsSnapshot:   cfg.TotalRateBps,
			InviteeRateBpsSnapshot: cfg.InviteeRateBps,
			AppliedRateBps:         cfg.InviterRateBps,
			RewardQuota:            int64(CalculateSubscriptionReferralQuota(order.Money, cfg.InviterRateBps)),
			Status:                 SubscriptionReferralStatusCredited,
		},
		{
			OrderId:                order.Id,
			OrderTradeNo:           order.TradeNo,
			PlanId:                 order.PlanId,
			ReferralGroup:          group,
			PayerUserId:            order.UserId,
			InviterUserId:          invitee.InviterId,
			BeneficiaryUserId:      order.UserId,
			BeneficiaryRole:        SubscriptionReferralBeneficiaryRoleInvitee,
			OrderPaidAmount:        order.Money,
			QuotaPerUnitSnapshot:   common.QuotaPerUnit,
			TotalRateBpsSnapshot:   cfg.TotalRateBps,
			InviteeRateBpsSnapshot: cfg.InviteeRateBps,
			AppliedRateBps:         cfg.InviteeRateBps,
			RewardQuota:            int64(CalculateSubscriptionReferralQuota(order.Money, cfg.InviteeRateBps)),
			Status:                 SubscriptionReferralStatusCredited,
		},
	}

	for i := range records {
		record := records[i]
		if record.RewardQuota <= 0 {
			continue
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", record.BeneficiaryUserId).Updates(map[string]interface{}{
			"aff_quota":   gorm.Expr("aff_quota + ?", record.RewardQuota),
			"aff_history": gorm.Expr("aff_history + ?", record.RewardQuota),
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

func ReverseSubscriptionReferralByTradeNo(tradeNo string, operatorId int) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var records []SubscriptionReferralRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("order_trade_no = ?", tradeNo).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return ErrSubscriptionReferralRecordNotFound
		}

		for i := range records {
			record := &records[i]
			reversible := record.RewardQuota - record.ReversedQuota - record.DebtQuota
			if reversible <= 0 {
				continue
			}

			var user User
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, record.BeneficiaryUserId).Error; err != nil {
				return err
			}

			recovered := reversible
			if int64(user.AffQuota) < recovered {
				recovered = int64(user.AffQuota)
			}
			debt := reversible - recovered

			if recovered > 0 {
				if err := tx.Model(&User{}).Where("id = ?", user.Id).
					Update("aff_quota", gorm.Expr("aff_quota - ?", recovered)).Error; err != nil {
					return err
				}
			}

			record.ReversedQuota += recovered
			record.DebtQuota += debt
			if debt > 0 {
				record.Status = SubscriptionReferralStatusPartialRevert
			} else {
				record.Status = SubscriptionReferralStatusReversed
			}
			if err := tx.Save(record).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
