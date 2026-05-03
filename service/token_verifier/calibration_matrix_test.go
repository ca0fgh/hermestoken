package token_verifier

import (
	"strings"
	"testing"
	"time"
)

// Calibration matrix: deterministic, in-process integration tests that verify the
// scoring system produces the expected verdict on each of 10 archetypal scenarios
// drawn from the research §4.1 case list. No real upstream tokens needed; each
// scenario is fed into BuildReport via handcrafted CheckResult slices.
//
// Layer 2 (real-key end-to-end smoke) lives in scripts/, not here.

// ---------- helpers ----------

var gradeRank = map[string]int{"Fail": 0, "D": 1, "C": 2, "B": 3, "A": 4, "S": 5}

func gradeAtLeast(got, want string) bool {
	g, ok1 := gradeRank[got]
	w, ok2 := gradeRank[want]
	if !ok1 || !ok2 {
		return false
	}
	return g >= w
}

func gradeAtMost(got, want string) bool {
	g, ok1 := gradeRank[got]
	w, ok2 := gradeRank[want]
	if !ok1 || !ok2 {
		return false
	}
	return g <= w
}

func installCalibrationBaselines(t *testing.T) {
	t.Helper()
	t.Setenv("AA_API_KEY", "test-key")
	snap := &AABaselineSnapshot{
		FetchedAt: time.Now(),
		Models:    map[string]*AABaselineModel{},
	}
	add := func(slug string, ttftSec, tps float64) {
		bm := &AABaselineModel{ID: "uuid-" + slug, Slug: slug, Name: slug, TTFTSec: ttftSec, OutputTPS: tps}
		snap.Models[canonicalModelName(slug)] = bm
	}
	add("gpt-4o", 0.5, 80)
	add("gpt-4o-mini", 0.3, 140)
	add("gpt-5", 0.4, 110)
	add("claude-opus-4-5", 0.55, 60)
	add("claude-3-5-haiku", 0.45, 95)
	SetAABaselineSnapshotForTest(snap)
	t.Cleanup(func() { SetAABaselineSnapshotForTest(nil) })
}

// healthyResults builds the full check sequence that a successful run for one
// (provider, model) tuple would produce, with measured perf close to the AA baseline.
func healthyResults(provider, model string, accessLatencyMs, streamTTFTMs int64, tps float64) []CheckResult {
	access := CheckResult{
		Provider: provider, CheckKey: CheckModelAccess, ModelName: model,
		ClaimedModel: model, ObservedModel: model,
		Success: true, Score: 100, LatencyMs: accessLatencyMs,
	}
	identity := buildModelIdentityResult(provider, model, access)
	return []CheckResult{
		{Provider: provider, CheckKey: CheckModelsList, Success: true, Score: 100, LatencyMs: 200},
		{Provider: provider, CheckKey: CheckAvailability, ModelName: model, Success: true, Score: 100, LatencyMs: accessLatencyMs},
		access,
		identity,
		{Provider: provider, CheckKey: CheckStream, ModelName: model, Success: true, Score: 100, LatencyMs: 1500, TTFTMs: streamTTFTMs, TokensPS: tps},
		{Provider: provider, CheckKey: CheckJSON, ModelName: model, Success: true, Score: 100, LatencyMs: accessLatencyMs},
		{Provider: provider, CheckKey: CheckReproducibility, ModelName: model, Consistent: true, ConsistencyMethod: ConsistencyMethodSystemFingerprint, Success: true, Score: 100},
	}
}

// downgradeResults simulates a relay that requests `claimed` but the upstream returns `observed`.
func downgradeResults(provider, claimed, observed string) []CheckResult {
	access := CheckResult{
		Provider: provider, CheckKey: CheckModelAccess, ModelName: claimed,
		ClaimedModel: claimed, ObservedModel: observed,
		Success: true, Score: 100, LatencyMs: 1000,
	}
	identity := buildModelIdentityResult(provider, claimed, access)
	return []CheckResult{
		{Provider: provider, CheckKey: CheckModelsList, Success: true, Score: 100, LatencyMs: 200},
		{Provider: provider, CheckKey: CheckAvailability, ModelName: claimed, Success: true, Score: 100, LatencyMs: 1000},
		access,
		identity,
		{Provider: provider, CheckKey: CheckStream, ModelName: claimed, Success: true, Score: 100, LatencyMs: 1500, TTFTMs: 600, TokensPS: 70},
		{Provider: provider, CheckKey: CheckJSON, ModelName: claimed, Success: true, Score: 100, LatencyMs: 1000},
		{Provider: provider, CheckKey: CheckReproducibility, ModelName: claimed, Consistent: true, ConsistencyMethod: ConsistencyMethodSystemFingerprint, Success: true, Score: 100},
	}
}

// httpFailureResults simulates an upstream rejecting the request with a given status code.
func httpFailureResults(provider, model, errorCode, message string) []CheckResult {
	fail := CheckResult{
		Provider: provider, CheckKey: CheckModelAccess, ModelName: model,
		ClaimedModel: model,
		Success:   false, Score: 0,
		ErrorCode: errorCode, Message: message,
	}
	identity := buildModelIdentityResult(provider, model, fail)
	return []CheckResult{
		{Provider: provider, CheckKey: CheckModelsList, Success: true, Score: 100, LatencyMs: 200},
		{Provider: provider, CheckKey: CheckAvailability, ModelName: model, Success: false, Score: 0, ErrorCode: errorCode, Message: message},
		fail,
		identity,
		{Provider: provider, CheckKey: CheckStream, ModelName: model, Success: false, Score: 0, ErrorCode: errorCode, Message: message},
		{Provider: provider, CheckKey: CheckJSON, ModelName: model, Success: false, Score: 0, ErrorCode: errorCode, Message: message},
		{Provider: provider, CheckKey: CheckReproducibility, ModelName: model, Success: false, Score: 0, ErrorCode: errorCode, Message: message},
	}
}

// streamOffResults simulates an upstream where chat completion works but streaming does not.
func streamOffResults(provider, model string) []CheckResult {
	access := CheckResult{
		Provider: provider, CheckKey: CheckModelAccess, ModelName: model,
		ClaimedModel: model, ObservedModel: model,
		Success: true, Score: 100, LatencyMs: 800,
	}
	identity := buildModelIdentityResult(provider, model, access)
	return []CheckResult{
		{Provider: provider, CheckKey: CheckModelsList, Success: true, Score: 100, LatencyMs: 200},
		{Provider: provider, CheckKey: CheckAvailability, ModelName: model, Success: true, Score: 100, LatencyMs: 800},
		access,
		identity,
		{Provider: provider, CheckKey: CheckStream, ModelName: model, Success: false, Score: 0, ErrorCode: "empty_stream", Message: "stream response has no data chunks"},
		{Provider: provider, CheckKey: CheckJSON, ModelName: model, Success: true, Score: 100, LatencyMs: 800},
		{Provider: provider, CheckKey: CheckReproducibility, ModelName: model, Consistent: true, ConsistencyMethod: ConsistencyMethodSystemFingerprint, Success: true, Score: 100},
	}
}

// fingerprintChangeResults simulates a relay whose system_fingerprint flips between requests.
func fingerprintChangeResults(provider, model string) []CheckResult {
	access := CheckResult{
		Provider: provider, CheckKey: CheckModelAccess, ModelName: model,
		ClaimedModel: model, ObservedModel: model,
		Success: true, Score: 100, LatencyMs: 800,
	}
	identity := buildModelIdentityResult(provider, model, access)
	return []CheckResult{
		{Provider: provider, CheckKey: CheckModelsList, Success: true, Score: 100, LatencyMs: 200},
		{Provider: provider, CheckKey: CheckAvailability, ModelName: model, Success: true, Score: 100, LatencyMs: 800},
		access,
		identity,
		{Provider: provider, CheckKey: CheckStream, ModelName: model, Success: true, Score: 100, LatencyMs: 1500, TTFTMs: 500, TokensPS: 75},
		{Provider: provider, CheckKey: CheckJSON, ModelName: model, Success: true, Score: 100, LatencyMs: 800},
		{Provider: provider, CheckKey: CheckReproducibility, ModelName: model, Consistent: false, ConsistencyMethod: ConsistencyMethodSystemFingerprintChanged, Success: false, Score: 30},
	}
}

// ---------- assertion helpers ----------

func requireGradeAtLeast(t *testing.T, r ReportSummary, want string) {
	t.Helper()
	if !gradeAtLeast(r.Grade, want) {
		t.Errorf("grade=%s, want >= %s (score=%d, dims=%v)", r.Grade, want, r.Score, r.Dimensions)
	}
}

func requireGradeAtMost(t *testing.T, r ReportSummary, want string) {
	t.Helper()
	if !gradeAtMost(r.Grade, want) {
		t.Errorf("grade=%s, want <= %s (score=%d, dims=%v)", r.Grade, want, r.Score, r.Dimensions)
	}
}

func requireBaselineSource(t *testing.T, r ReportSummary, want string) {
	t.Helper()
	if r.BaselineSource != want {
		t.Errorf("baseline_source=%q, want %q", r.BaselineSource, want)
	}
}

func requireSuspectedDowngrade(t *testing.T, r ReportSummary) {
	t.Helper()
	for _, m := range r.ModelIdentity {
		if m.SuspectedDowngrade {
			return
		}
	}
	t.Errorf("expected at least one model_identity entry with suspected_downgrade=true, got %+v", r.ModelIdentity)
}

func requireNoSuspectedDowngrade(t *testing.T, r ReportSummary) {
	t.Helper()
	for _, m := range r.ModelIdentity {
		if m.SuspectedDowngrade {
			t.Errorf("expected no suspected_downgrade, got %+v", m)
		}
	}
}

func requireIdentityConfidenceAtMost(t *testing.T, r ReportSummary, max int) {
	t.Helper()
	if len(r.ModelIdentity) == 0 {
		t.Fatalf("no model_identity entries")
	}
	for _, m := range r.ModelIdentity {
		if m.IdentityConfidence > max {
			t.Errorf("identity_confidence=%d, want <= %d (model=%s)", m.IdentityConfidence, max, m.ClaimedModel)
		}
	}
}

func requireRiskMatching(t *testing.T, r ReportSummary, substr string) {
	t.Helper()
	for _, risk := range r.Risks {
		if strings.Contains(risk, substr) {
			return
		}
	}
	t.Errorf("expected risk containing %q, got %v", substr, r.Risks)
}

// ---------- the matrix ----------

type calibrationCase struct {
	id      string
	desc    string
	setup   func(t *testing.T)
	results []CheckResult
	assert  func(t *testing.T, r ReportSummary)
}

func TestCalibrationMatrix(t *testing.T) {
	cases := []calibrationCase{
		{
			id:    "OFFICIAL-OPENAI-01",
			desc:  "Direct OpenAI gpt-4o, healthy endpoint with measurements close to AA P50",
			setup: installCalibrationBaselines,
			// TTFT 480ms vs baseline 500ms (ratio 0.96), TPS 78 vs baseline 80 (ratio 0.975) → both top
			results: healthyResults("openai", "gpt-4o", 800, 480, 78),
			assert: func(t *testing.T, r ReportSummary) {
				requireGradeAtLeast(t, r, "A")
				requireBaselineSource(t, r, BaselineSourceAA)
				requireNoSuspectedDowngrade(t, r)
				if r.Dimensions["performance"] < 13 {
					t.Errorf("performance dim=%d, expected >= 13 (close-to-baseline)", r.Dimensions["performance"])
				}
			},
		},
		{
			id:      "OFFICIAL-OPENAI-02",
			desc:    "Direct OpenAI gpt-5, top-tier model, all checks green",
			setup:   installCalibrationBaselines,
			results: healthyResults("openai", "gpt-5", 600, 380, 105),
			assert: func(t *testing.T, r ReportSummary) {
				requireGradeAtLeast(t, r, "S")
				requireBaselineSource(t, r, BaselineSourceAA)
				requireNoSuspectedDowngrade(t, r)
			},
		},
		{
			id:    "OFFICIAL-ANTHROPIC-01",
			desc:  "Direct Anthropic claude-opus-4-5, healthy. Reproducibility skipped because anthropic doesn't expose seed.",
			setup: installCalibrationBaselines,
			results: func() []CheckResult {
				base := healthyResults("anthropic", "claude-opus-4-5", 900, 540, 58)
				// flip the reproducibility entry to skipped (anthropic path)
				for i := range base {
					if base[i].CheckKey == CheckReproducibility {
						base[i] = CheckResult{
							Provider: "anthropic", CheckKey: CheckReproducibility, ModelName: "claude-opus-4-5",
							Skipped: true, Success: true, ErrorCode: "skipped",
							Message: "Anthropic Messages API 不支持 seed 参数，跳过复现性检查",
						}
					}
				}
				return base
			}(),
			assert: func(t *testing.T, r ReportSummary) {
				requireGradeAtLeast(t, r, "A")
				requireBaselineSource(t, r, BaselineSourceAA)
				// stability must still be max because the only skipped check is excluded from the denominator
				if r.Dimensions["stability"] != 15 {
					t.Errorf("stability=%d, want 15 (skipped check must not penalize)", r.Dimensions["stability"])
				}
			},
		},
		{
			id:      "DOWNGRADE-01",
			desc:    "Gateway claims gpt-4o but upstream returns gpt-4o-mini (intra-family downgrade)",
			setup:   installCalibrationBaselines,
			results: downgradeResults("openai", "gpt-4o", "gpt-4o-mini"),
			assert: func(t *testing.T, r ReportSummary) {
				requireSuspectedDowngrade(t, r)
				requireIdentityConfidenceAtMost(t, r, 35)
				// Other checks (access/stream/json) all pass, so model_access stays high.
				if r.Dimensions["model_access"] < 18 {
					t.Errorf("model_access=%d, expected >= 18 (calls still succeed)", r.Dimensions["model_access"])
				}
			},
		},
		{
			id:      "DOWNGRADE-02",
			desc:    "Cross-family swap: claude-opus-4-5 routed to gpt-4o-mini",
			setup:   installCalibrationBaselines,
			results: downgradeResults("anthropic", "claude-opus-4-5", "gpt-4o-mini"),
			assert: func(t *testing.T, r ReportSummary) {
				requireSuspectedDowngrade(t, r)
				// Cross-family must be more aggressive than intra-family
				requireIdentityConfidenceAtMost(t, r, 25)
			},
		},
		{
			id:      "FAKE-MODEL-01",
			desc:    "Request a non-existent model (gpt-9-ultra) — must hard-fail",
			setup:   installCalibrationBaselines,
			results: httpFailureResults("openai", "gpt-9-ultra", "http_404", "model not found"),
			assert: func(t *testing.T, r ReportSummary) {
				requireGradeAtMost(t, r, "C")
				if r.Dimensions["availability"] != 0 {
					t.Errorf("availability=%d, want 0", r.Dimensions["availability"])
				}
			},
		},
		{
			id:      "EXPIRED-KEY-01",
			desc:    "Expired/invalid OpenAI key — must hard-fail with 401",
			setup:   installCalibrationBaselines,
			results: httpFailureResults("openai", "gpt-4o", "http_401", "invalid api key"),
			assert: func(t *testing.T, r ReportSummary) {
				requireGradeAtMost(t, r, "C")
				if r.Dimensions["availability"] != 0 {
					t.Errorf("availability=%d, want 0", r.Dimensions["availability"])
				}
			},
		},
		{
			id:      "QUOTA-EXHAUSTED-01",
			desc:    "Token has zero balance — quota error from upstream",
			setup:   installCalibrationBaselines,
			results: httpFailureResults("openai", "gpt-4o", "http_429", "insufficient_quota"),
			assert: func(t *testing.T, r ReportSummary) {
				requireGradeAtMost(t, r, "C")
				requireRiskMatching(t, r, "额度")
			},
		},
		{
			id:      "STREAM-OFF-01",
			desc:    "Upstream that supports chat but not streaming (some Azure deployments)",
			setup:   installCalibrationBaselines,
			results: streamOffResults("openai", "gpt-4o"),
			assert: func(t *testing.T, r ReportSummary) {
				if r.Dimensions["stream"] != 0 {
					t.Errorf("stream dim=%d, want 0", r.Dimensions["stream"])
				}
				// Other dimensions should remain healthy.
				requireGradeAtLeast(t, r, "B")
				if r.Dimensions["availability"] != 20 {
					t.Errorf("availability=%d, want 20", r.Dimensions["availability"])
				}
			},
		},
		{
			id:      "FINGERPRINT-CHANGED-01",
			desc:    "system_fingerprint differs between two seeded probes — relay swapping models behind the scenes",
			setup:   installCalibrationBaselines,
			results: fingerprintChangeResults("openai", "gpt-4o"),
			assert: func(t *testing.T, r ReportSummary) {
				if len(r.Reproducibility) == 0 || r.Reproducibility[0].Method != ConsistencyMethodSystemFingerprintChanged {
					t.Errorf("expected reproducibility entry with method=%s, got %+v", ConsistencyMethodSystemFingerprintChanged, r.Reproducibility)
				}
				requireRiskMatching(t, r, "system_fingerprint")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			report := BuildReport(tc.results)
			tc.assert(t, report)
		})
	}
}
