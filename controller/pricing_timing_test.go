package controller

import (
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/model"
)

func TestGetPricingSetsServerTimingHeader(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withPricingGuestSettings(
		t,
		`{}`,
		`{"default":1}`,
		`{"default":{"default":1}}`,
	)
	seedPricingAbility(t, db, "default", "gpt-default")
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	serverTiming := recorder.Header().Get("Server-Timing")
	if serverTiming == "" {
		t.Fatalf("expected Server-Timing header to be set")
	}
	for _, metric := range []string{
		"pricing_model",
		"pricing_context",
		"pricing_filter",
		"pricing_response",
		"pricing_total",
	} {
		if !strings.Contains(serverTiming, metric) {
			t.Fatalf("expected Server-Timing header %q to contain metric %q", serverTiming, metric)
		}
	}
}
