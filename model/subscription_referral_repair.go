package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type SubscriptionReferralSettlementBatchRepairResult struct {
	BatchID                    int   `json:"batch_id"`
	SourceTradeNo              string `json:"source_trade_no"`
	ImmediateRecordID          int   `json:"immediate_record_id"`
	InviteeRecordID            int   `json:"invitee_record_id"`
	ImmediateBeneficiaryUserID int   `json:"immediate_beneficiary_user_id"`
	InviteeBeneficiaryUserID   int   `json:"invitee_beneficiary_user_id"`
	ImmediateOldQuota          int64 `json:"immediate_old_quota"`
	ImmediateNewQuota          int64 `json:"immediate_new_quota"`
	InviteeOldQuota            int64 `json:"invitee_old_quota"`
	InviteeNewQuota            int64 `json:"invitee_new_quota"`
	Changed                    bool  `json:"changed"`
}

func RepairSubscriptionReferralSettlementBatchByID(batchID int) (*SubscriptionReferralSettlementBatchRepairResult, error) {
	if batchID <= 0 {
		return nil, errors.New("invalid batch id")
	}

	result := &SubscriptionReferralSettlementBatchRepairResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		batch, records, err := loadRepairableSubscriptionReferralBatchTx(tx, batchID)
		if err != nil {
			return err
		}

		preview, err := previewSubscriptionReferralSettlementBatchRepair(tx, batch, records)
		if err != nil {
			return err
		}
		if preview == nil {
			return nil
		}

		if !preview.Changed {
			*result = *preview
			return nil
		}

		immediateDelta := preview.ImmediateNewQuota - preview.ImmediateOldQuota
		if err := adjustReferralRepairUserQuotaTx(tx, preview.ImmediateBeneficiaryUserID, immediateDelta); err != nil {
			return err
		}
		if preview.InviteeRecordID > 0 {
			inviteeDelta := preview.InviteeNewQuota - preview.InviteeOldQuota
			if err := adjustReferralRepairUserQuotaTx(tx, preview.InviteeBeneficiaryUserID, inviteeDelta); err != nil {
				return err
			}
		}

		if err := tx.Model(&ReferralSettlementRecord{}).
			Where("id = ?", preview.ImmediateRecordID).
			UpdateColumn("reward_quota", preview.ImmediateNewQuota).Error; err != nil {
			return err
		}
		if preview.InviteeRecordID > 0 {
			if err := tx.Model(&ReferralSettlementRecord{}).
				Where("id = ?", preview.InviteeRecordID).
				UpdateColumn("reward_quota", preview.InviteeNewQuota).Error; err != nil {
				return err
			}
		}

		*result = *preview
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func PreviewSubscriptionReferralSettlementBatchRepair(batchID int) (*SubscriptionReferralSettlementBatchRepairResult, error) {
	if batchID <= 0 {
		return nil, errors.New("invalid batch id")
	}

	var result *SubscriptionReferralSettlementBatchRepairResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		batch, records, err := loadRepairableSubscriptionReferralBatchTx(tx, batchID)
		if err != nil {
			return err
		}

		preview, err := previewSubscriptionReferralSettlementBatchRepair(tx, batch, records)
		if err != nil {
			return err
		}
		result = preview
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func loadRepairableSubscriptionReferralBatchTx(tx *gorm.DB, batchID int) (*ReferralSettlementBatch, []ReferralSettlementRecord, error) {
	var batch ReferralSettlementBatch
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&batch, batchID).Error; err != nil {
		return nil, nil, err
	}
	if batch.ReferralType != ReferralTypeSubscription {
		return nil, nil, fmt.Errorf("batch %d is not a subscription referral batch", batchID)
	}
	if batch.Status != SubscriptionReferralStatusCredited {
		return nil, nil, fmt.Errorf("batch %d status %q is not repairable", batchID, batch.Status)
	}

	records := make([]ReferralSettlementRecord, 0)
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("batch_id = ?", batch.Id).
		Order("id ASC").
		Find(&records).Error; err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("batch %d has no settlement records", batchID)
	}
	for _, record := range records {
		if record.Status != SubscriptionReferralStatusCredited || record.ReversedQuota != 0 || record.DebtQuota != 0 {
			return nil, nil, fmt.Errorf("batch %d contains non-credited settlement state", batchID)
		}
	}

	return &batch, records, nil
}

func previewSubscriptionReferralSettlementBatchRepair(tx *gorm.DB, batch *ReferralSettlementBatch, records []ReferralSettlementRecord) (*SubscriptionReferralSettlementBatchRepairResult, error) {
	if batch == nil {
		return nil, errors.New("batch is required")
	}
	if len(records) == 0 {
		return nil, errors.New("records are required")
	}

	immediateComponent := ""
	switch batch.SettlementMode {
	case ReferralSettlementModeTeamDirect:
		immediateComponent = "team_direct_reward"
	case ReferralSettlementModeDirectWithTeamChain:
		immediateComponent = "direct_reward"
	default:
		return nil, fmt.Errorf("unsupported settlement mode %q", batch.SettlementMode)
	}

	var immediateRecord *ReferralSettlementRecord
	var inviteeRecord *ReferralSettlementRecord
	for idx := range records {
		record := &records[idx]
		switch record.RewardComponent {
		case immediateComponent:
			immediateRecord = record
		case "invitee_reward":
			if record.SourceRewardComponent != nil && strings.TrimSpace(*record.SourceRewardComponent) == immediateComponent {
				inviteeRecord = record
			}
		}
	}
	if immediateRecord == nil {
		return nil, fmt.Errorf("batch %d missing %s record", batch.Id, immediateComponent)
	}

	var order SubscriptionOrder
	if err := tx.First(&order, batch.SourceId).Error; err != nil {
		return nil, err
	}

	inviteeRateBps := 0
	if immediateRecord.InviteeShareBpsSnapshot != nil {
		inviteeRateBps = *immediateRecord.InviteeShareBpsSnapshot
	} else if inviteeRecord != nil && inviteeRecord.InviteeShareBpsSnapshot != nil {
		inviteeRateBps = *inviteeRecord.InviteeShareBpsSnapshot
	}

	appliedRateBps := 0
	if immediateRecord.AppliedRateBps != nil {
		appliedRateBps = *immediateRecord.AppliedRateBps
	}
	if appliedRateBps <= 0 {
		return nil, fmt.Errorf("batch %d missing applied_rate_bps on %s record", batch.Id, immediateComponent)
	}

	config := ResolveSubscriptionReferralConfig(appliedRateBps, inviteeRateBps)
	targetImmediateQuota := CalculateSubscriptionReferralQuotaWithQuotaPerUnit(order.Money, config.InviterRateBps, batch.QuotaPerUnitSnapshot)
	targetInviteeQuota := CalculateSubscriptionReferralQuotaWithQuotaPerUnit(order.Money, config.InviteeRateBps, batch.QuotaPerUnitSnapshot)

	result := &SubscriptionReferralSettlementBatchRepairResult{
		BatchID:                    batch.Id,
		SourceTradeNo:              batch.SourceTradeNo,
		ImmediateRecordID:          immediateRecord.Id,
		ImmediateBeneficiaryUserID: immediateRecord.BeneficiaryUserId,
		ImmediateOldQuota:          immediateRecord.RewardQuota,
		ImmediateNewQuota:          targetImmediateQuota,
		Changed:                    immediateRecord.RewardQuota != targetImmediateQuota,
	}

	if inviteeRecord != nil {
		result.InviteeRecordID = inviteeRecord.Id
		result.InviteeBeneficiaryUserID = inviteeRecord.BeneficiaryUserId
		result.InviteeOldQuota = inviteeRecord.RewardQuota
		result.InviteeNewQuota = targetInviteeQuota
		result.Changed = result.Changed || inviteeRecord.RewardQuota != targetInviteeQuota
	} else if targetInviteeQuota > 0 {
		return nil, fmt.Errorf("batch %d missing invitee_reward record", batch.Id)
	}

	return result, nil
}

func adjustReferralRepairUserQuotaTx(tx *gorm.DB, userID int, delta int64) error {
	if tx == nil || userID <= 0 || delta == 0 {
		return nil
	}

	var user User
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, userID).Error; err != nil {
		return err
	}

	if delta > 0 {
		return tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
			"aff_quota":   gorm.Expr("aff_quota + ?", delta),
			"aff_history": gorm.Expr("aff_history + ?", delta),
		}).Error
	}

	reduction := -delta
	if int64(user.AffQuota) < reduction {
		return fmt.Errorf("user %d aff_quota %d is insufficient for repair reduction %d", userID, user.AffQuota, reduction)
	}
	if int64(user.AffHistoryQuota) < reduction {
		return fmt.Errorf("user %d aff_history %d is insufficient for repair reduction %d", userID, user.AffHistoryQuota, reduction)
	}

	return tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"aff_quota":   gorm.Expr("aff_quota - ?", reduction),
		"aff_history": gorm.Expr("aff_history - ?", reduction),
	}).Error
}
