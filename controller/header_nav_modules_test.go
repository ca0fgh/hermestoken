package controller

import "testing"

func TestGetPricingHeaderNavConfigDefaultsWhenOptionIsEmpty(t *testing.T) {
	config := getPricingHeaderNavConfig("")
	if !config.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if config.RequireAuth {
		t.Fatal("RequireAuth = true, want false")
	}
}

func TestGetPricingHeaderNavConfigSupportsLegacyBooleanPricing(t *testing.T) {
	config := getPricingHeaderNavConfig(`{"pricing":false}`)
	if config.Enabled {
		t.Fatal("Enabled = true, want false for legacy boolean config")
	}
	if config.RequireAuth {
		t.Fatal("RequireAuth = true, want false for legacy boolean config")
	}
}

func TestGetPricingHeaderNavConfigSupportsStructuredPricingObject(t *testing.T) {
	config := getPricingHeaderNavConfig(`{"pricing":{"enabled":true,"requireAuth":true}}`)
	if !config.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if !config.RequireAuth {
		t.Fatal("RequireAuth = false, want true")
	}
}

func TestGetPricingHeaderNavConfigSupportsPartiallyMalformedStructuredPricingObject(t *testing.T) {
	config := getPricingHeaderNavConfig(`{"pricing":{"enabled":false,"requireAuth":"true"}}`)
	if config.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if config.RequireAuth {
		t.Fatal("RequireAuth = true, want false for malformed field")
	}
}

func TestGetPricingHeaderNavConfigSupportsLegacyBooleanPricingTrue(t *testing.T) {
	config := getPricingHeaderNavConfig(`{"pricing":true}`)
	if !config.Enabled {
		t.Fatal("Enabled = false, want true for legacy boolean config")
	}
	if config.RequireAuth {
		t.Fatal("RequireAuth = true, want false for legacy boolean config")
	}
}
