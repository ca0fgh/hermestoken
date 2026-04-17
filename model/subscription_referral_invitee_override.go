package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type SubscriptionReferralInviteeContributionSummary struct {
	InviteeUserId       int    `json:"invitee_user_id"`
	InviteeUsername     string `json:"invitee_username"`
	InviteeGroup        string `json:"invitee_group"`
	ContributionQuota   int64  `json:"contribution_quota"`
	RewardQuota         int64  `json:"reward_quota"`
	ReversedQuota       int64  `json:"reversed_quota"`
	DebtQuota           int64  `json:"debt_quota"`
	OrderCount          int64  `json:"order_count"`
	OverrideGroupCount  int64  `json:"override_group_count"`
	EffectiveInviteeBps int    `json:"effective_invitee_bps"`
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
	templateSummaryQuery := DB.Table("referral_settlement_records AS records").
		Select(strings.Join([]string{
			"batches.payer_user_id AS invitee_user_id",
			"COALESCE(SUM(records.reward_quota - records.reversed_quota - records.debt_quota), 0) AS contribution_quota",
			"COALESCE(SUM(records.reward_quota), 0) AS reward_quota",
			"COALESCE(SUM(records.reversed_quota), 0) AS reversed_quota",
			"COALESCE(SUM(records.debt_quota), 0) AS debt_quota",
			"COUNT(DISTINCT batches.source_id) AS order_count",
		}, ", ")).
		Joins("JOIN referral_settlement_batches AS batches ON batches.id = records.batch_id").
		Where(
			"records.referral_type = ? AND batches.immediate_inviter_user_id = ? AND records.beneficiary_user_id = ? AND records.reward_component IN ?",
			ReferralTypeSubscription,
			inviterUserId,
			inviterUserId,
			[]string{"direct_reward", "team_direct_reward"},
		).
		Group("batches.payer_user_id")

	summaryQuery := DB.Table("users AS invitees").
		Select(strings.Join([]string{
			"invitees.id AS invitee_user_id",
			"COALESCE(invitees.username, '') AS invitee_username",
			"COALESCE(invitees." + commonGroupCol + ", '') AS invitee_group",
			"COALESCE(template_records.contribution_quota, 0) AS contribution_quota",
			"COALESCE(template_records.reward_quota, 0) AS reward_quota",
			"COALESCE(template_records.reversed_quota, 0) AS reversed_quota",
			"COALESCE(template_records.debt_quota, 0) AS debt_quota",
			"COALESCE(template_records.order_count, 0) AS order_count",
		}, ", ")).
		Joins("LEFT JOIN (?) AS template_records ON template_records.invitee_user_id = invitees.id", templateSummaryQuery).
		Where("invitees.inviter_id = ? AND invitees.deleted_at IS NULL", inviterUserId).
		Group(strings.Join([]string{
			"invitees.id",
			"invitees.username",
			"invitees." + commonGroupCol,
			"template_records.contribution_quota",
			"template_records.reward_quota",
			"template_records.reversed_quota",
			"template_records.debt_quota",
			"template_records.order_count",
		}, ", "))

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
