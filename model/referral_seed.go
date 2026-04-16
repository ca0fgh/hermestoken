package model

import (
	"strings"
)

type LegacySubscriptionReferralSeedRows struct {
	Group                string                                `json:"group"`
	OverrideSeeds        []SubscriptionReferralOverride        `json:"override_seeds"`
	InviteeRateSeeds     map[int]int                           `json:"invitee_rate_seeds"`
	InviteeOverrideSeeds []SubscriptionReferralInviteeOverride `json:"invitee_override_seeds"`
}

func ListLegacySubscriptionReferralSeedRows(group string) (*LegacySubscriptionReferralSeedRows, error) {
	trimmedGroup := strings.TrimSpace(group)
	seeds := &LegacySubscriptionReferralSeedRows{
		Group:                trimmedGroup,
		InviteeRateSeeds:     make(map[int]int),
		InviteeOverrideSeeds: make([]SubscriptionReferralInviteeOverride, 0),
	}

	if err := DB.Where(commonGroupCol+" = ?", trimmedGroup).Order("id ASC").Find(&seeds.OverrideSeeds).Error; err != nil {
		return nil, err
	}
	if err := DB.Where(commonGroupCol+" = ?", trimmedGroup).Order("id ASC").Find(&seeds.InviteeOverrideSeeds).Error; err != nil {
		return nil, err
	}

	var users []User
	if err := DB.Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		if rate := user.GetSetting().SubscriptionReferralInviteeRateBpsByGroup[trimmedGroup]; rate > 0 {
			seeds.InviteeRateSeeds[user.Id] = rate
		}
	}

	return seeds, nil
}
