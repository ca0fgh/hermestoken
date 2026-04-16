package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetSubscriptionReferralInviteeOverrideByIDsAndGroup(inviterUserId int, inviteeUserId int, group string) (*SubscriptionReferralInviteeOverride, error) {
	if inviterUserId <= 0 {
		return nil, errors.New("invalid inviter user id")
	}
	if inviteeUserId <= 0 {
		return nil, errors.New("invalid invitee user id")
	}

	var override SubscriptionReferralInviteeOverride
	if err := DB.Where(
		"inviter_user_id = ? AND invitee_user_id = ? AND "+commonGroupCol+" = ?",
		inviterUserId,
		inviteeUserId,
		strings.TrimSpace(group),
	).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

type SubscriptionReferralInviteeOverride struct {
	Id             int    `json:"id"`
	InviterUserId  int    `json:"inviter_user_id" gorm:"uniqueIndex:idx_sub_referral_invitee_override_group"`
	InviteeUserId  int    `json:"invitee_user_id" gorm:"uniqueIndex:idx_sub_referral_invitee_override_group"`
	Group          string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_sub_referral_invitee_override_group"`
	InviteeRateBps int    `json:"invitee_rate_bps" gorm:"type:int;not null;default:0"`
	CreatedBy      int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy      int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

func (o *SubscriptionReferralInviteeOverride) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	o.CreatedAt = now
	o.UpdatedAt = now
	o.Group = strings.TrimSpace(o.Group)
	o.InviteeRateBps = NormalizeSubscriptionReferralRateBps(o.InviteeRateBps)
	return nil
}

func (o *SubscriptionReferralInviteeOverride) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = common.GetTimestamp()
	o.Group = strings.TrimSpace(o.Group)
	o.InviteeRateBps = NormalizeSubscriptionReferralRateBps(o.InviteeRateBps)
	return nil
}

type SubscriptionReferralInviteeContributionSummary struct {
	InviteeUserId     int    `json:"invitee_user_id" gorm:"column:invitee_user_id"`
	InviteeUsername   string `json:"invitee_username" gorm:"column:invitee_username"`
	InviteeGroup      string `json:"invitee_group" gorm:"column:invitee_group"`
	ContributionQuota int64  `json:"contribution_quota" gorm:"column:contribution_quota"`
	RewardQuota       int64  `json:"reward_quota" gorm:"column:reward_quota"`
	ReversedQuota     int64  `json:"reversed_quota" gorm:"column:reversed_quota"`
	DebtQuota         int64  `json:"debt_quota" gorm:"column:debt_quota"`
	OrderCount        int64  `json:"order_count" gorm:"column:order_count"`
}

func validateSubscriptionReferralInviteeOwnership(inviterUserId int, inviteeUserId int) error {
	if inviterUserId <= 0 {
		return errors.New("invalid inviter user id")
	}
	if inviteeUserId <= 0 {
		return errors.New("invalid invitee user id")
	}
	if inviterUserId == inviteeUserId {
		return errors.New("inviter user id and invitee user id must be different")
	}
	if _, err := GetUserById(inviterUserId, false); err != nil {
		return err
	}
	invitee, err := GetUserById(inviteeUserId, false)
	if err != nil {
		return err
	}
	if invitee.InviterId != inviterUserId {
		return errors.New("invitee does not belong to inviter")
	}
	return nil
}

func UpsertSubscriptionReferralInviteeOverride(inviterUserId int, inviteeUserId int, group string, inviteeRateBps int) (*SubscriptionReferralInviteeOverride, error) {
	if err := validateSubscriptionReferralInviteeOwnership(inviterUserId, inviteeUserId); err != nil {
		return nil, err
	}

	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return nil, errors.New("subscription referral invitee override group is required")
	}

	override := &SubscriptionReferralInviteeOverride{
		InviterUserId:  inviterUserId,
		InviteeUserId:  inviteeUserId,
		Group:          trimmedGroup,
		InviteeRateBps: NormalizeSubscriptionReferralRateBps(inviteeRateBps),
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "inviter_user_id"}, {Name: "invitee_user_id"}, {Name: "group"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"invitee_rate_bps": override.InviteeRateBps,
				"updated_at":       common.GetTimestamp(),
			}),
		}).Create(override).Error; err != nil {
			return err
		}
		return tx.Where("inviter_user_id = ? AND invitee_user_id = ? AND "+commonGroupCol+" = ?", inviterUserId, inviteeUserId, trimmedGroup).First(override).Error
	}); err != nil {
		return nil, err
	}
	return override, nil
}

func DeleteSubscriptionReferralInviteeOverride(inviterUserId int, inviteeUserId int, group string) error {
	if err := validateSubscriptionReferralInviteeOwnership(inviterUserId, inviteeUserId); err != nil {
		return err
	}
	return DB.Where("inviter_user_id = ? AND invitee_user_id = ? AND "+commonGroupCol+" = ?", inviterUserId, inviteeUserId, strings.TrimSpace(group)).Delete(&SubscriptionReferralInviteeOverride{}).Error
}

func ListSubscriptionReferralInviteeOverrides(inviterUserId int, inviteeUserId int) ([]SubscriptionReferralInviteeOverride, error) {
	if err := validateSubscriptionReferralInviteeOwnership(inviterUserId, inviteeUserId); err != nil {
		return nil, err
	}

	var overrides []SubscriptionReferralInviteeOverride
	if err := DB.Where("inviter_user_id = ? AND invitee_user_id = ?", inviterUserId, inviteeUserId).Order(commonGroupCol + " ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

func ListSubscriptionReferralInviteeOverrideCounts(inviterUserId int, inviteeUserIds []int) (map[int]int64, error) {
	if inviterUserId <= 0 {
		return nil, errors.New("invalid inviter user id")
	}
	if _, err := GetUserById(inviterUserId, false); err != nil {
		return nil, err
	}

	counts := make(map[int]int64, len(inviteeUserIds))
	filteredInviteeUserIds := make([]int, 0, len(inviteeUserIds))
	seenInviteeUserIDs := make(map[int]struct{}, len(inviteeUserIds))
	for _, inviteeUserId := range inviteeUserIds {
		if inviteeUserId <= 0 {
			continue
		}
		if _, exists := seenInviteeUserIDs[inviteeUserId]; exists {
			continue
		}
		seenInviteeUserIDs[inviteeUserId] = struct{}{}
		filteredInviteeUserIds = append(filteredInviteeUserIds, inviteeUserId)
	}
	if len(filteredInviteeUserIds) == 0 {
		return counts, nil
	}

	rows := make([]struct {
		InviteeUserId int   `gorm:"column:invitee_user_id"`
		Count         int64 `gorm:"column:override_group_count"`
	}, 0, len(filteredInviteeUserIds))
	if err := DB.Model(&SubscriptionReferralInviteeOverride{}).
		Select("invitee_user_id, COUNT(*) AS override_group_count").
		Where("inviter_user_id = ? AND invitee_user_id IN ?", inviterUserId, filteredInviteeUserIds).
		Group("invitee_user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.InviteeUserId] = row.Count
	}
	return counts, nil
}

func ListSubscriptionReferralInviteeContributionSummaries(inviterUserId int, keyword string, pageInfo *common.PageInfo) ([]*SubscriptionReferralInviteeContributionSummary, int64, int64, error) {
	if inviterUserId <= 0 {
		return nil, 0, 0, errors.New("invalid inviter user id")
	}
	if _, err := GetUserById(inviterUserId, false); err != nil {
		return nil, 0, 0, err
	}
	if pageInfo == nil {
		pageInfo = &common.PageInfo{}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}

	keyword = strings.TrimSpace(keyword)
	summaryQuery := DB.Table("users AS invitees").
		Select(strings.Join([]string{
			"invitees.id AS invitee_user_id",
			"COALESCE(invitees.username, '') AS invitee_username",
			"COALESCE(invitees." + commonGroupCol + ", '') AS invitee_group",
			"COALESCE(SUM(records.reward_quota - records.reversed_quota - records.debt_quota), 0) AS contribution_quota",
			"COALESCE(SUM(records.reward_quota), 0) AS reward_quota",
			"COALESCE(SUM(records.reversed_quota), 0) AS reversed_quota",
			"COALESCE(SUM(records.debt_quota), 0) AS debt_quota",
			"COUNT(DISTINCT records.order_id) AS order_count",
		}, ", ")).
		Joins("LEFT JOIN subscription_referral_records AS records ON records.payer_user_id = invitees.id AND records.inviter_user_id = ? AND records.beneficiary_role = ?", inviterUserId, SubscriptionReferralBeneficiaryRoleInviter).
		Where("invitees.inviter_id = ? AND invitees.deleted_at IS NULL", inviterUserId).
		Group("invitees.id, invitees.username, invitees." + commonGroupCol)

	if keyword != "" {
		if inviteeId, err := strconv.Atoi(keyword); err == nil {
			summaryQuery = summaryQuery.Where("invitees.id = ? OR invitees.username LIKE ?", inviteeId, "%"+keyword+"%")
		} else {
			summaryQuery = summaryQuery.Where("invitees.username LIKE ?", "%"+keyword+"%")
		}
	}

	countQuery := DB.Table("(?) AS subscription_referral_invitee_contribution_summaries", summaryQuery)
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	contributionQuery := DB.Table("(?) AS subscription_referral_invitee_contribution_summaries", summaryQuery).Select("COALESCE(SUM(contribution_quota), 0)")
	var contributionTotal int64
	if err := contributionQuery.Scan(&contributionTotal).Error; err != nil {
		return nil, 0, 0, err
	}

	summaries := make([]*SubscriptionReferralInviteeContributionSummary, 0)
	if err := summaryQuery.Order("invitee_user_id ASC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&summaries).Error; err != nil {
		return nil, 0, 0, err
	}
	return summaries, total, contributionTotal, nil
}
