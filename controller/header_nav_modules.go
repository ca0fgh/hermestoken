package controller

import (
	"encoding/json"
	"strings"
)

type pricingHeaderNavConfig struct {
	Enabled     bool
	RequireAuth bool
}

func getPricingHeaderNavConfig(raw string) pricingHeaderNavConfig {
	config := pricingHeaderNavConfig{
		Enabled:     true,
		RequireAuth: false,
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return config
	}

	pricingRaw, ok := root["pricing"]
	if !ok {
		return config
	}

	var legacy bool
	if err := json.Unmarshal(pricingRaw, &legacy); err == nil {
		config.Enabled = legacy
		return config
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(pricingRaw, &payload); err != nil {
		return config
	}

	if enabledRaw, ok := payload["enabled"]; ok {
		var enabled bool
		if err := json.Unmarshal(enabledRaw, &enabled); err == nil {
			config.Enabled = enabled
		}
	}
	if requireAuthRaw, ok := payload["requireAuth"]; ok {
		var requireAuth bool
		if err := json.Unmarshal(requireAuthRaw, &requireAuth); err == nil {
			config.RequireAuth = requireAuth
		}
	}
	return config
}
