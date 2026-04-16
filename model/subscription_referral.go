package model

import (
	"errors"
	"fmt"
	"sort"
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
	invitee := 0
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

func GetEffectiveSubscriptionReferralInviteeRateBpsForInvitee(inviterUserID int, inviteeUserID int, setting dto.UserSetting, group string, totalRateBps int) int {
	total := NormalizeSubscriptionReferralRateBps(totalRateBps)
	if total == 0 {
		return 0
	}

	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return 0
	}

	override, err := GetSubscriptionReferralInviteeOverrideByIDsAndGroup(inviterUserID, inviteeUserID, trimmedGroup)
	if err == nil && override != nil {
		inviteeRateBps := NormalizeSubscriptionReferralRateBps(override.InviteeRateBps)
		if inviteeRateBps > total {
			return total
		}
		return inviteeRateBps
	}

	return GetEffectiveSubscriptionReferralInviteeRateBps(setting, trimmedGroup, total)
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
	if err := DB.Where("user_id = ? AND "+commonGroupCol+" = ?", userID, "default").First(&override).Error; err == nil {
		return &override, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := DB.Where("user_id = ? AND "+commonGroupCol+" = ?", userID, "").First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func getLegacyUngroupedSubscriptionReferralOverrideByUserID(userID int) (*SubscriptionReferralOverride, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var override SubscriptionReferralOverride
	if err := DB.Where("user_id = ? AND "+commonGroupCol+" = ?", userID, "").First(&override).Error; err != nil {
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
	if err := DB.Where("user_id = ? AND "+commonGroupCol+" = ?", userID, trimmedGroup).First(&override).Error; err != nil {
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
	if group == "" {
		return nil, errors.New("subscription referral override group is required")
	}

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
		return tx.Where("user_id = ? AND "+commonGroupCol+" = ?", userID, group).First(override).Error
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
	return DB.Where("user_id = ? AND "+commonGroupCol+" = ?", userID, strings.TrimSpace(group)).Delete(&SubscriptionReferralOverride{}).Error
}

func ListSubscriptionReferralOverridesByUserID(userID int) ([]SubscriptionReferralOverride, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var overrides []SubscriptionReferralOverride
	if err := DB.Where("user_id = ?", userID).Order(commonGroupCol + " ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

func ListSubscriptionReferralConfiguredGroups() []string {
	return listSubscriptionPlanUpgradeGroups()
}

func listSubscriptionPlanUpgradeGroups() []string {
	return listSubscriptionPlanUpgradeGroupsWithDB(DB)
}

func listSubscriptionPlanUpgradeGroupsWithDB(tx *gorm.DB) []string {
	if tx == nil {
		return nil
	}

	var rawGroups []string
	if err := tx.Model(&SubscriptionPlan{}).
		Distinct("upgrade_group").
		Where("upgrade_group <> ''").
		Pluck("upgrade_group", &rawGroups).Error; err != nil {
		return nil
	}

	groupSet := make(map[string]struct{}, len(rawGroups))
	groups := make([]string, 0, len(rawGroups))
	for _, group := range rawGroups {
		trimmedGroup := strings.TrimSpace(group)
		if trimmedGroup == "" {
			continue
		}
		if _, exists := groupSet[trimmedGroup]; exists {
			continue
		}
		groupSet[trimmedGroup] = struct{}{}
		groups = append(groups, trimmedGroup)
	}
	sort.Strings(groups)
	return groups
}

func getSingleSubscriptionReferralGroupForMigration() (string, error) {
	return getSingleSubscriptionReferralGroupForMigrationWithDB(DB)
}

func getSingleSubscriptionReferralGroupForMigrationWithDB(tx *gorm.DB) (string, error) {
	groups := listSubscriptionPlanUpgradeGroupsWithDB(tx)
	switch len(groups) {
	case 1:
		return groups[0], nil
	case 0:
		return "", errors.New("no real subscription referral groups are configured")
	default:
		return "", fmt.Errorf("multiple real subscription referral groups are configured: %s", strings.Join(groups, ", "))
	}
}

func IsSubscriptionReferralPlanBackedGroup(group string) bool {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return false
	}

	for _, configuredGroup := range listSubscriptionPlanUpgradeGroups() {
		if configuredGroup == trimmedGroup {
			return true
		}
	}
	return false
}

func migrateLegacySubscriptionReferralOverrides() error {
	if DB == nil {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var legacyOverrides []SubscriptionReferralOverride
		if err := tx.Where(commonGroupCol+" = ?", "").Order("id ASC").Find(&legacyOverrides).Error; err != nil {
			return err
		}
		if len(legacyOverrides) == 0 {
			return nil
		}

		targetGroup, err := getSingleSubscriptionReferralGroupForMigrationWithDB(tx)
		if err != nil {
			return fmt.Errorf("legacy subscription referral override rows require exactly one real group for migration: %w", err)
		}

		for _, legacyOverride := range legacyOverrides {
			var existingOverride SubscriptionReferralOverride
			findErr := tx.Where("user_id = ? AND "+commonGroupCol+" = ?", legacyOverride.UserId, targetGroup).First(&existingOverride).Error
			switch {
			case findErr == nil:
				if err := tx.Delete(&SubscriptionReferralOverride{}, legacyOverride.Id).Error; err != nil {
					return err
				}
			case errors.Is(findErr, gorm.ErrRecordNotFound):
				if err := tx.Model(&SubscriptionReferralOverride{}).
					Where("id = ?", legacyOverride.Id).
					Updates(map[string]interface{}{
						"group":      targetGroup,
						"updated_at": common.GetTimestamp(),
					}).Error; err != nil {
					return err
				}
			default:
				return findErr
			}
		}

		return nil
	})
}

func migrateLegacySubscriptionReferralInviteeRates() error {
	if DB == nil {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var users []User
		if err := tx.Select("id", "setting").Where("setting <> ''").Find(&users).Error; err != nil {
			return err
		}

		legacyUsers := make([]User, 0, len(users))
		for _, user := range users {
			if user.GetSetting().SubscriptionReferralInviteeRateBps > 0 {
				legacyUsers = append(legacyUsers, user)
			}
		}
		if len(legacyUsers) == 0 {
			return nil
		}

		targetGroup, err := getSingleSubscriptionReferralGroupForMigrationWithDB(tx)
		if err != nil {
			return fmt.Errorf("legacy subscription referral invitee rate settings require exactly one real group for migration: %w", err)
		}

		for _, user := range legacyUsers {
			migratedSetting := user.GetSetting().WithMigratedSubscriptionReferralInviteeRate(targetGroup)
			user.SetSetting(migratedSetting)
			if err := tx.Model(&User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func validateNoLegacySubscriptionReferralData() error {
	if DB == nil {
		return nil
	}

	var emptyGroupOverrideCount int64
	if err := DB.Model(&SubscriptionReferralOverride{}).
		Where(commonGroupCol+" = ?", "").
		Count(&emptyGroupOverrideCount).Error; err != nil {
		return err
	}
	if emptyGroupOverrideCount > 0 {
		return fmt.Errorf("subscription referral startup validation failed: found %d leftover empty-group override rows", emptyGroupOverrideCount)
	}

	var users []User
	if err := DB.Select("id", "setting").Where("setting <> ''").Find(&users).Error; err != nil {
		return err
	}

	legacyScalarInviteeRateUsers := 0
	for _, user := range users {
		if user.GetSetting().SubscriptionReferralInviteeRateBps > 0 {
			legacyScalarInviteeRateUsers++
		}
	}
	if legacyScalarInviteeRateUsers > 0 {
		return fmt.Errorf("subscription referral startup validation failed: found %d users with legacy scalar SubscriptionReferralInviteeRateBps", legacyScalarInviteeRateUsers)
	}

	return nil
}

func GetEffectiveSubscriptionReferralTotalRateBps(userID int, group string) int {
	resolvedGroup := strings.TrimSpace(group)
	if resolvedGroup == "" || userID <= 0 {
		return 0
	}

	override, err := GetSubscriptionReferralOverrideByUserIDAndGroup(userID, resolvedGroup)
	if err == nil && override != nil {
		return NormalizeSubscriptionReferralRateBps(override.TotalRateBps)
	}
	return 0
}

func ApplySubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
	if tx == nil || order == nil || plan == nil || order.Money <= 0 {
		return nil
	}

	group := strings.TrimSpace(plan.UpgradeGroup)
	if group == "" {
		return nil
	}

	mode, err := ResolveReferralEngineMode(ReferralTypeSubscription, group)
	if err != nil {
		return err
	}
	if mode == ReferralEngineModeTemplate {
		return ApplyTemplateSubscriptionReferralOnOrderSuccessTx(tx, order, plan)
	}
	return applyLegacySubscriptionReferralOnOrderSuccessTx(tx, order, plan)
}

func applyLegacySubscriptionReferralOnOrderSuccessTx(tx *gorm.DB, order *SubscriptionOrder, plan *SubscriptionPlan) error {
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
	inviteeRateBps := GetEffectiveSubscriptionReferralInviteeRateBpsForInvitee(
		inviter.Id,
		invitee.Id,
		inviter.GetSetting(),
		group,
		totalRateBps,
	)
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

	mode, batch, err := findTemplateSettlementBatchByTradeNo(tradeNo)
	if err != nil {
		return err
	}
	if mode == ReferralEngineModeTemplate && batch != nil {
		return reverseReferralSettlementBatch(batch.Id)
	}
	return reverseLegacySubscriptionReferralByTradeNo(tradeNo, operatorId)
}

func reverseLegacySubscriptionReferralByTradeNo(tradeNo string, operatorId int) error {
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

func findTemplateSettlementBatchByTradeNo(tradeNo string) (string, *ReferralSettlementBatch, error) {
	var batch ReferralSettlementBatch
	err := DB.Where("referral_type = ? AND source_trade_no = ?", ReferralTypeSubscription, tradeNo).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReferralEngineModeLegacy, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return ReferralEngineModeTemplate, &batch, nil
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
