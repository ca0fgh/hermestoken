package dto

type ReferralTemplateUpsertRequest struct {
	ReferralType           string  `json:"referral_type"`
	Group                  string  `json:"group"`
	Name                   string  `json:"name"`
	LevelType              string  `json:"level_type"`
	Enabled                bool    `json:"enabled"`
	DirectCapBps           int     `json:"direct_cap_bps"`
	TeamCapBps             int     `json:"team_cap_bps"`
	TeamDecayRatio         float64 `json:"team_decay_ratio"`
	TeamMaxDepth           int     `json:"team_max_depth"`
	InviteeShareDefaultBps int     `json:"invitee_share_default_bps"`
}

type ReferralTemplateBindingUpsertRequest struct {
	ReferralType            string `json:"referral_type"`
	Group                   string `json:"group"`
	TemplateId              int    `json:"template_id"`
	InviteeShareOverrideBps *int   `json:"invitee_share_override_bps"`
}

type ReferralEngineRouteUpsertRequest struct {
	ReferralType string `json:"referral_type"`
	Group        string `json:"group"`
	EngineMode   string `json:"engine_mode"`
}
