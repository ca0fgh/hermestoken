package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
			"COALESCE(SUM(records.reward_quota - records.reversed_quota - records.debt_quota), 0) AS contribution_quota",
			"COALESCE(SUM(records.reward_quota), 0) AS reward_quota",
			"COALESCE(SUM(records.reversed_quota), 0) AS reversed_quota",
			"COALESCE(SUM(records.debt_quota), 0) AS debt_quota",
			"COUNT(DISTINCT records.order_id) AS order_count",
		}, ", ")).
		Joins("LEFT JOIN subscription_referral_records AS records ON records.payer_user_id = invitees.id AND records.inviter_user_id = ? AND records.beneficiary_role = ?", inviterUserId, SubscriptionReferralBeneficiaryRoleInviter).
		Where("invitees.inviter_id = ?", inviterUserId).
		Group("invitees.id, invitees.username")

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
