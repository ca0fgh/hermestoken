package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
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

func NormalizeSubscriptionReferralRateBps(rateBps int) int {
	if rateBps < 0 {
		return 0
	}
	if rateBps > SubscriptionReferralMaxRateBps {
		return SubscriptionReferralMaxRateBps
	}
	return rateBps
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

func ResolveSubscriptionReferralConfig(totalRateBps int, inviteeRateBps int) SubscriptionReferralConfig {
	total := NormalizeSubscriptionReferralRateBps(totalRateBps)
	invitee := NormalizeSubscriptionReferralRateBps(inviteeRateBps)
	if invitee > total {
		invitee = total
	}
	return SubscriptionReferralConfig{
		Enabled:        total > 0,
		TotalRateBps:   total,
		InviteeRateBps: invitee,
		InviterRateBps: total - invitee,
	}
}

func ApplySubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	if tx == nil || order == nil || plan == nil || order.Money <= 0 {
		return nil
	}
	return ApplyTemplateSubscriptionReferralOnOrderSuccessTx(tx, order, plan)
}

func ReverseSubscriptionReferralByTradeNo(tradeNo string, operatorID int) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}

	batch, err := findSubscriptionReferralSettlementBatchByTradeNo(tradeNo)
	if err != nil {
		return err
	}
	if batch == nil {
		return ErrSubscriptionReferralRecordNotFound
	}
	return reverseReferralSettlementBatch(batch.Id)
}

func findSubscriptionReferralSettlementBatchByTradeNo(tradeNo string) (*ReferralSettlementBatch, error) {
	var batch ReferralSettlementBatch
	err := DB.Where("referral_type = ? AND source_trade_no = ?", ReferralTypeSubscription, tradeNo).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

func reverseReferralSettlementBatch(batchID int) error {
	if batchID <= 0 {
		return ErrSubscriptionReferralRecordNotFound
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var records []ReferralSettlementRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("batch_id = ?", batchID).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return ErrSubscriptionReferralRecordNotFound
		}

		for idx := range records {
			record := &records[idx]
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
