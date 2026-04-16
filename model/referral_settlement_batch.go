package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ReferralSettlementBatch struct {
	Id                         int     `json:"id"`
	ReferralType               string  `json:"referral_type" gorm:"type:varchar(64);not null;index"`
	Group                      string  `json:"group" gorm:"type:varchar(64);not null;default:'';index"`
	SourceType                 string  `json:"source_type" gorm:"type:varchar(64);not null;index"`
	SourceId                   int     `json:"source_id" gorm:"type:int;not null;index"`
	SourceTradeNo              string  `json:"source_trade_no" gorm:"type:varchar(255);not null;index"`
	PayerUserId                int     `json:"payer_user_id" gorm:"type:int;not null;index"`
	ImmediateInviterUserId     int     `json:"immediate_inviter_user_id" gorm:"type:int;not null;index"`
	ActiveTemplateSnapshotJSON string  `json:"active_template_snapshot_json" gorm:"type:text"`
	TeamChainSnapshotJSON      string  `json:"team_chain_snapshot_json" gorm:"type:text"`
	SettlementMode             string  `json:"settlement_mode" gorm:"type:varchar(64);not null;index"`
	QuotaPerUnitSnapshot       float64 `json:"quota_per_unit_snapshot" gorm:"type:decimal(18,6);not null;default:0"`
	Status                     string  `json:"status" gorm:"type:varchar(32);not null;default:'credited';index"`
	SettledAt                  int64   `json:"settled_at" gorm:"bigint"`
	CreatedAt                  int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt                  int64   `json:"updated_at" gorm:"bigint"`
}

func (b *ReferralSettlementBatch) normalize() {
	b.ReferralType = strings.TrimSpace(b.ReferralType)
	b.Group = strings.TrimSpace(b.Group)
	b.SourceType = strings.TrimSpace(b.SourceType)
	b.SourceTradeNo = strings.TrimSpace(b.SourceTradeNo)
	b.SettlementMode = strings.TrimSpace(b.SettlementMode)
	b.Status = strings.TrimSpace(b.Status)
}

func (b *ReferralSettlementBatch) Validate() error {
	b.normalize()
	if b.ReferralType == "" {
		return errors.New("referral type is required")
	}
	if b.Group == "" {
		return errors.New("group is required")
	}
	if b.SourceType == "" {
		return errors.New("source type is required")
	}
	return nil
}

func (b *ReferralSettlementBatch) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	b.CreatedAt = now
	b.UpdatedAt = now
	if b.SettledAt == 0 {
		b.SettledAt = now
	}
	return b.Validate()
}

func (b *ReferralSettlementBatch) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = common.GetTimestamp()
	return b.Validate()
}
