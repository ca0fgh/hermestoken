package model

import (
	"fmt"
	"strconv"
)

const (
	SubscriptionReferralTeamDecayRatioOptionKey            = "SubscriptionReferralTeamDecayRatio"
	SubscriptionReferralTeamMaxDepthOptionKey              = "SubscriptionReferralTeamMaxDepth"
	SubscriptionReferralAutoAssignInviteeTemplateOptionKey = "SubscriptionReferralAutoAssignInviteeTemplate"
	SubscriptionPlanOpenToAllUsersOptionKey                = "SubscriptionPlanOpenToAllUsers"

	DefaultSubscriptionReferralTeamDecayRatio            = 0.5
	DefaultSubscriptionReferralTeamMaxDepth              = 0
	DefaultSubscriptionReferralAutoAssignInviteeTemplate = true
	DefaultSubscriptionPlanOpenToAllUsers                = false
)

var (
	subscriptionReferralTeamDecayRatio            = DefaultSubscriptionReferralTeamDecayRatio
	subscriptionReferralTeamMaxDepth              = DefaultSubscriptionReferralTeamMaxDepth
	subscriptionReferralAutoAssignInviteeTemplate = DefaultSubscriptionReferralAutoAssignInviteeTemplate
	subscriptionPlanOpenToAllUsers                = DefaultSubscriptionPlanOpenToAllUsers
)

func resetSubscriptionReferralGlobalSettingDefaults() {
	subscriptionReferralTeamDecayRatio = DefaultSubscriptionReferralTeamDecayRatio
	subscriptionReferralTeamMaxDepth = DefaultSubscriptionReferralTeamMaxDepth
	subscriptionReferralAutoAssignInviteeTemplate = DefaultSubscriptionReferralAutoAssignInviteeTemplate
	subscriptionPlanOpenToAllUsers = DefaultSubscriptionPlanOpenToAllUsers
}

type SubscriptionReferralGlobalSetting struct {
	TeamDecayRatio            float64 `json:"team_decay_ratio"`
	TeamMaxDepth              int     `json:"team_max_depth"`
	AutoAssignInviteeTemplate bool    `json:"auto_assign_invitee_template"`
	PlanOpenToAllUsers        bool    `json:"plan_open_to_all_users"`
}

func GetSubscriptionReferralGlobalSetting() SubscriptionReferralGlobalSetting {
	return SubscriptionReferralGlobalSetting{
		TeamDecayRatio:            subscriptionReferralTeamDecayRatio,
		TeamMaxDepth:              subscriptionReferralTeamMaxDepth,
		AutoAssignInviteeTemplate: subscriptionReferralAutoAssignInviteeTemplate,
		PlanOpenToAllUsers:        subscriptionPlanOpenToAllUsers,
	}
}

func IsSubscriptionPlanOpenToAllUsersEnabled() bool {
	return subscriptionPlanOpenToAllUsers
}

func ValidateSubscriptionReferralGlobalSetting(setting SubscriptionReferralGlobalSetting) error {
	if setting.TeamDecayRatio <= 0 || setting.TeamDecayRatio > 1 {
		return fmt.Errorf("invalid subscription team decay ratio")
	}
	if setting.TeamMaxDepth < 0 {
		return fmt.Errorf("invalid subscription team max depth")
	}
	return nil
}

func UpdateSubscriptionReferralGlobalSetting(setting SubscriptionReferralGlobalSetting) error {
	if err := ValidateSubscriptionReferralGlobalSetting(setting); err != nil {
		return err
	}
	if err := UpdateOption(
		SubscriptionReferralTeamDecayRatioOptionKey,
		strconv.FormatFloat(setting.TeamDecayRatio, 'f', -1, 64),
	); err != nil {
		return err
	}
	if err := UpdateOption(
		SubscriptionReferralTeamMaxDepthOptionKey,
		strconv.Itoa(setting.TeamMaxDepth),
	); err != nil {
		return err
	}
	if err := UpdateOption(
		SubscriptionReferralAutoAssignInviteeTemplateOptionKey,
		strconv.FormatBool(setting.AutoAssignInviteeTemplate),
	); err != nil {
		return err
	}
	if err := UpdateOption(
		SubscriptionPlanOpenToAllUsersOptionKey,
		strconv.FormatBool(setting.PlanOpenToAllUsers),
	); err != nil {
		return err
	}
	return nil
}
