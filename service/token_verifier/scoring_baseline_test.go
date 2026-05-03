package token_verifier

import (
	"testing"
	"time"
)

// helper: install an in-memory AA snapshot containing the given (slug -> ttftSec, tps) pairs.
func installBaselineForTest(t *testing.T, models map[string][2]float64) {
	t.Helper()
	t.Setenv("AA_API_KEY", "test-key")
	snap := &AABaselineSnapshot{
		FetchedAt: time.Now(),
		Models:    make(map[string]*AABaselineModel, len(models)),
	}
	for slug, vals := range models {
		bm := &AABaselineModel{
			ID:        "uuid-" + slug,
			Slug:      slug,
			Name:      slug,
			TTFTSec:   vals[0],
			OutputTPS: vals[1],
		}
		// canonicalModelName(slug) is the lookup key used by LookupAABaseline.
		snap.Models[canonicalModelName(slug)] = bm
	}
	SetAABaselineSnapshotForTest(snap)
	t.Cleanup(func() { SetAABaselineSnapshotForTest(nil) })
}

// fastModelResults builds a minimal CheckResult slice for one provider/model where
// stream perf is significantly faster than baseline (should yield top score).
func fastModelResults(provider, model string, accessLatency, streamTTFT, streamLatency int64, tps float64) []CheckResult {
	return []CheckResult{
		{Provider: provider, CheckKey: CheckAvailability, ModelName: model, Success: true, LatencyMs: accessLatency},
		{Provider: provider, CheckKey: CheckModelAccess, ModelName: model, Success: true, LatencyMs: accessLatency},
		{Provider: provider, CheckKey: CheckModelIdentity, ModelName: model, ClaimedModel: model, ObservedModel: model, IdentityConfidence: 95, Success: true, Score: 95},
		{Provider: provider, CheckKey: CheckStream, ModelName: model, Success: true, LatencyMs: streamLatency, TTFTMs: streamTTFT, TokensPS: tps},
		{Provider: provider, CheckKey: CheckJSON, ModelName: model, Success: true, LatencyMs: accessLatency},
	}
}

func TestBuildReportRatioScoringFastEndpoint(t *testing.T) {
	// AA baseline: gpt-4o ttft 0.5s (500ms), tps 80
	installBaselineForTest(t, map[string][2]float64{
		"gpt-4o": {0.5, 80},
	})
	// Measured: ttft 300ms (ratio 0.6 → top), tps 120 (ratio 1.5 → top)
	results := fastModelResults("openai", "gpt-4o", 800, 300, 1200, 120)

	report := BuildReport(results)

	if report.ScoringVersion != ScoringVersionV2 {
		t.Errorf("scoring_version: got %q, want %q", report.ScoringVersion, ScoringVersionV2)
	}
	if report.BaselineSource != BaselineSourceAA {
		t.Errorf("baseline_source: got %q, want %q", report.BaselineSource, BaselineSourceAA)
	}
	if report.Dimensions["performance"] != 15 {
		t.Errorf("performance dim: got %d, want 15", report.Dimensions["performance"])
	}
	if len(report.Models) != 1 {
		t.Fatalf("models: got %d, want 1", len(report.Models))
	}
	m := report.Models[0]
	if m.Baseline == nil {
		t.Fatal("expected baseline to be attached")
	}
	if m.Baseline.Source != BaselineSourceAA {
		t.Errorf("baseline source on model: got %q, want %q", m.Baseline.Source, BaselineSourceAA)
	}
	if m.Baseline.TTFTRatio == 0 || m.Baseline.TPSRatio == 0 {
		t.Errorf("expected non-zero ratios, got ttft=%v tps=%v", m.Baseline.TTFTRatio, m.Baseline.TPSRatio)
	}
	if got := report.Metrics["aa_ttft_ratio_avg"]; got <= 0 {
		t.Errorf("aa_ttft_ratio_avg should be set, got %v", got)
	}
}

func TestBuildReportRatioScoringSlowEndpoint(t *testing.T) {
	installBaselineForTest(t, map[string][2]float64{
		"gpt-4o": {0.5, 80},
	})
	// Measured: ttft 1500ms (ratio 3.0 → score 2), tps 24 (ratio 0.3 → score 2)
	// Per-model perf score = 2 + 2 = 4
	results := fastModelResults("openai", "gpt-4o", 5000, 1500, 6000, 24)

	report := BuildReport(results)

	if report.BaselineSource != BaselineSourceAA {
		t.Errorf("baseline_source: got %q, want %q", report.BaselineSource, BaselineSourceAA)
	}
	if got := report.Dimensions["performance"]; got != 4 {
		t.Errorf("performance dim: got %d, want 4", got)
	}
}

func TestBuildReportFallbackToAbsoluteLadder(t *testing.T) {
	// No AA snapshot installed → fallback path.
	t.Setenv("AA_API_KEY", "")
	SetAABaselineSnapshotForTest(nil)

	// Avg latency = 800ms across access+stream+json checks → ladder gives 15.
	results := fastModelResults("openai", "gpt-4o", 800, 200, 800, 60)
	report := BuildReport(results)

	if report.BaselineSource != BaselineSourceFallback {
		t.Errorf("baseline_source: got %q, want %q", report.BaselineSource, BaselineSourceFallback)
	}
	if report.Dimensions["performance"] != 15 {
		t.Errorf("performance dim with avg<1500ms should be 15, got %d", report.Dimensions["performance"])
	}
	for _, m := range report.Models {
		if m.Baseline != nil {
			t.Errorf("expected nil baseline on model %s, got %+v", m.ModelName, m.Baseline)
		}
	}
}

func TestBuildReportMixedBaselineSource(t *testing.T) {
	// Only one of two tested models has a baseline.
	installBaselineForTest(t, map[string][2]float64{
		"gpt-4o": {0.5, 80},
	})

	results := append(
		fastModelResults("openai", "gpt-4o", 800, 300, 1200, 120),
		fastModelResults("openai", "gpt-some-rare-model", 1000, 400, 1500, 40)...,
	)
	report := BuildReport(results)

	if report.BaselineSource != BaselineSourceMixed {
		t.Errorf("baseline_source: got %q, want %q", report.BaselineSource, BaselineSourceMixed)
	}
	hasBaseline := 0
	for _, m := range report.Models {
		if m.Baseline != nil {
			hasBaseline++
		}
	}
	if hasBaseline != 1 {
		t.Errorf("models with baseline attached: got %d, want 1", hasBaseline)
	}
}

func TestTTFTAndTPSSubScoreEdges(t *testing.T) {
	// TTFT
	if got := ttftSubScore(0); got != 0 {
		t.Errorf("ttftSubScore(0)=%v want 0", got)
	}
	if got := ttftSubScore(0.5); got != 7.5 {
		t.Errorf("ttftSubScore(0.5)=%v want 7.5", got)
	}
	if got := ttftSubScore(1.0); got != 7.5 {
		t.Errorf("ttftSubScore(1.0)=%v want 7.5", got)
	}
	if got := ttftSubScore(1.15); got != 6.5 {
		t.Errorf("ttftSubScore(1.15)=%v want 6.5", got)
	}
	if got := ttftSubScore(5); got != 1.0 {
		t.Errorf("ttftSubScore(5)=%v want 1.0", got)
	}

	// TPS
	if got := tpsSubScore(0); got != 0 {
		t.Errorf("tpsSubScore(0)=%v want 0", got)
	}
	if got := tpsSubScore(2.0); got != 7.5 {
		t.Errorf("tpsSubScore(2.0)=%v want 7.5", got)
	}
	if got := tpsSubScore(0.85); got != 6.5 {
		t.Errorf("tpsSubScore(0.85)=%v want 6.5", got)
	}
	if got := tpsSubScore(0.1); got != 1.0 {
		t.Errorf("tpsSubScore(0.1)=%v want 1.0", got)
	}
}
