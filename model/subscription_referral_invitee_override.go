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

type SubscriptionReferralInviteeContributionDetail struct {
	BatchId               int    `json:"batch_id"`
	TradeNo               string `json:"trade_no"`
	Group                 string `json:"group" gorm:"column:settlement_group"`
	RewardComponent       string `json:"reward_component"`
	SourceRewardComponent string `json:"source_reward_component"`
	RoleType              string `json:"role_type"`
	RewardQuota           int64  `json:"reward_quota"`
	ReversedQuota         int64  `json:"reversed_quota"`
	DebtQuota             int64  `json:"debt_quota"`
	EffectiveRewardQuota  int64  `json:"effective_reward_quota"`
	Status                string `json:"status"`
	SettledAt             int64  `json:"settled_at"`
	CreatedAt             int64  `json:"created_at"`
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
		likeKeyword := "%" + keyword + "%"
		searchClauses := []string{
			"invitees.username LIKE ?",
			"COALESCE(invitees.display_name, '') LIKE ?",
			"COALESCE(invitees." + commonGroupCol + ", '') = ?",
			"COALESCE(invitees." + commonGroupCol + ", '') LIKE ?",
		}
		searchArgs := []interface{}{
			likeKeyword,
			likeKeyword,
			keyword,
			likeKeyword,
		}
		if inviteeId, err := strconv.Atoi(keyword); err == nil {
			searchClauses = append([]string{"invitees.id = ?"}, searchClauses...)
			searchArgs = append([]interface{}{inviteeId}, searchArgs...)
		}
		summaryQuery = summaryQuery.Where("("+strings.Join(searchClauses, " OR ")+")", searchArgs...)
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
	if err := summaryQuery.
		Order("contribution_quota DESC").
		Order("order_count DESC").
		Order("invitee_user_id ASC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&summaries).Error; err != nil {
		return nil, 0, 0, err
	}
	return summaries, total, contributionTotal, nil
}

func ListSubscriptionReferralInviteeContributionDetails(inviterUserId int, inviteeUserId int) ([]*SubscriptionReferralInviteeContributionDetail, error) {
	if inviterUserId <= 0 {
		return nil, errors.New("invalid inviter user id")
	}
	if inviteeUserId <= 0 {
		return nil, errors.New("invalid invitee user id")
	}
	if _, err := GetUserById(inviterUserId, false); err != nil {
		return nil, err
	}
	if _, err := GetUserById(inviteeUserId, false); err != nil {
		return nil, err
	}

	roleTypeExpr := strings.Join([]string{
		"CASE",
		"WHEN records.reward_component = 'direct_reward' THEN 'direct'",
		"WHEN records.reward_component = 'team_direct_reward' THEN 'team'",
		"WHEN records.reward_component = 'team_reward' THEN 'team'",
		"WHEN records.reward_component = 'invitee_reward' AND COALESCE(records.source_reward_component, '') = 'team_direct_reward' THEN 'team'",
		"WHEN records.reward_component = 'invitee_reward' AND COALESCE(records.source_reward_component, '') = 'direct_reward' THEN 'direct'",
		"ELSE COALESCE(records.beneficiary_level_type, '')",
		"END",
	}, " ")

	selectColumns := strings.Join([]string{
		"records.batch_id AS batch_id",
		"batches.source_trade_no AS trade_no",
		"batches.`group` AS settlement_group",
		"records.reward_component AS reward_component",
		"COALESCE(records.source_reward_component, '') AS source_reward_component",
		roleTypeExpr + " AS role_type",
		"records.reward_quota AS reward_quota",
		"records.reversed_quota AS reversed_quota",
		"records.debt_quota AS debt_quota",
		"(records.reward_quota - records.reversed_quota - records.debt_quota) AS effective_reward_quota",
		"records.status AS status",
		"batches.settled_at AS settled_at",
		"records.created_at AS created_at",
	}, ", ")

	details := make([]*SubscriptionReferralInviteeContributionDetail, 0)
	err := DB.Table("referral_settlement_records AS records").
		Select(selectColumns).
		Joins("JOIN referral_settlement_batches AS batches ON batches.id = records.batch_id").
		Where(
			"records.referral_type = ? AND batches.immediate_inviter_user_id = ? AND batches.payer_user_id = ? AND ((records.beneficiary_user_id = ? AND records.reward_component IN ?) OR (records.beneficiary_user_id = ? AND records.reward_component = ?))",
			ReferralTypeSubscription,
			inviterUserId,
			inviteeUserId,
			inviterUserId,
			[]string{"direct_reward", "team_direct_reward", "team_reward"},
			inviteeUserId,
			"invitee_reward",
		).
		Order("batches.settled_at DESC").
		Order("records.batch_id DESC").
		Order("records.id ASC").
		Scan(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}
