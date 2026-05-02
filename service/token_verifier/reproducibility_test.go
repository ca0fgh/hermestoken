package token_verifier

import "testing"

func TestDecideReproducibility(t *testing.T) {
	cases := []struct {
		name       string
		fp1        string
		fp2        string
		hash1      string
		hash2      string
		wantOK     bool
		wantMethod string
	}{
		{
			name:       "system_fingerprint match wins over hash mismatch",
			fp1:        "fp_abc",
			fp2:        "fp_abc",
			hash1:      "h1",
			hash2:      "h2",
			wantOK:     true,
			wantMethod: ConsistencyMethodSystemFingerprint,
		},
		{
			name:       "system_fingerprint diverged is a strong negative signal",
			fp1:        "fp_abc",
			fp2:        "fp_def",
			hash1:      "h1",
			hash2:      "h1",
			wantOK:     false,
			wantMethod: ConsistencyMethodSystemFingerprintChanged,
		},
		{
			name:       "fall back to content hash match when fingerprints absent",
			fp1:        "",
			fp2:        "",
			hash1:      "h1",
			hash2:      "h1",
			wantOK:     true,
			wantMethod: ConsistencyMethodContentHash,
		},
		{
			name:       "content hash diverged when fingerprints absent",
			fp1:        "",
			fp2:        "",
			hash1:      "h1",
			hash2:      "h2",
			wantOK:     false,
			wantMethod: ConsistencyMethodContentDiverged,
		},
		{
			name:       "insufficient data when no fingerprint and content empty",
			fp1:        "",
			fp2:        "",
			hash1:      "",
			hash2:      "",
			wantOK:     false,
			wantMethod: ConsistencyMethodInsufficientData,
		},
		{
			name:       "one fingerprint missing falls back to hash",
			fp1:        "fp_abc",
			fp2:        "",
			hash1:      "h1",
			hash2:      "h1",
			wantOK:     true,
			wantMethod: ConsistencyMethodContentHash,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, method := decideReproducibility(tc.fp1, tc.fp2, tc.hash1, tc.hash2)
			if ok != tc.wantOK || method != tc.wantMethod {
				t.Errorf("got (%v, %q), want (%v, %q)", ok, method, tc.wantOK, tc.wantMethod)
			}
		})
	}
}

func TestExtractAssistantContent(t *testing.T) {
	openAILike := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{"content": "STABLE_PING_9F3"},
			},
		},
	}
	if got := extractAssistantContent(openAILike); got != "STABLE_PING_9F3" {
		t.Errorf("openai-like: got %q", got)
	}

	textLike := map[string]any{
		"choices": []any{
			map[string]any{"text": "alt"},
		},
	}
	if got := extractAssistantContent(textLike); got != "alt" {
		t.Errorf("text-like: got %q", got)
	}

	if got := extractAssistantContent(nil); got != "" {
		t.Errorf("nil: got %q", got)
	}
	if got := extractAssistantContent(map[string]any{}); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestSha256HexStable(t *testing.T) {
	a := sha256Hex("abc")
	b := sha256Hex("abc")
	if a != b {
		t.Errorf("sha256Hex not deterministic")
	}
	if a == sha256Hex("abd") {
		t.Errorf("sha256Hex collided on different inputs")
	}
}

func TestBuildReportReproducibilitySection(t *testing.T) {
	results := []CheckResult{
		// One model with consistent reproducibility
		{Provider: "openai", CheckKey: CheckModelAccess, ModelName: "gpt-4o", Success: true, LatencyMs: 800},
		{
			Provider:          "openai",
			CheckKey:          CheckReproducibility,
			ModelName:         "gpt-4o",
			Consistent:        true,
			ConsistencyMethod: ConsistencyMethodSystemFingerprint,
			Success:           true,
			Score:             100,
			Message:           "两次请求 system_fingerprint 一致",
		},
		// Anthropic skipped — must NOT pull stability down
		{Provider: "anthropic", CheckKey: CheckModelAccess, ModelName: "claude-3-5-haiku-latest", Success: true, LatencyMs: 900},
		{
			Provider:  "anthropic",
			CheckKey:  CheckReproducibility,
			ModelName: "claude-3-5-haiku-latest",
			Skipped:   true,
			Success:   true,
			ErrorCode: "skipped",
			Message:   "Anthropic Messages API 不支持 seed 参数，跳过复现性检查",
		},
	}

	report := BuildReport(results)

	if len(report.Reproducibility) != 2 {
		t.Fatalf("reproducibility entries: got %d, want 2", len(report.Reproducibility))
	}

	var openaiEntry, anthropicEntry *ReproducibilitySummary
	for i := range report.Reproducibility {
		switch report.Reproducibility[i].Provider {
		case "openai":
			openaiEntry = &report.Reproducibility[i]
		case "anthropic":
			anthropicEntry = &report.Reproducibility[i]
		}
	}
	if openaiEntry == nil || anthropicEntry == nil {
		t.Fatalf("missing one of the reproducibility entries")
	}
	if !openaiEntry.Consistent || openaiEntry.Method != ConsistencyMethodSystemFingerprint {
		t.Errorf("openai: got %+v", openaiEntry)
	}
	if !anthropicEntry.Skipped {
		t.Errorf("anthropic: expected skipped, got %+v", anthropicEntry)
	}

	// Stability must reach the maximum (15) because the only failure-eligible checks
	// (model_access x2) both passed; the skipped reproducibility check must not appear
	// in either numerator or denominator.
	if got := report.Dimensions["stability"]; got != 15 {
		t.Errorf("stability dimension: got %d, want 15 (skipped must not be counted)", got)
	}

	// The skipped item must show status="skipped" in the checklist.
	foundSkipped := false
	for _, item := range report.Checklist {
		if item.CheckKey == string(CheckReproducibility) && item.Provider == "anthropic" {
			foundSkipped = item.Status == "skipped" && item.Skipped
			break
		}
	}
	if !foundSkipped {
		t.Errorf("anthropic reproducibility checklist item should have status=skipped and Skipped=true")
	}
}

func TestBuildReportFingerprintChangeRaisesRisk(t *testing.T) {
	results := []CheckResult{
		{Provider: "openai", CheckKey: CheckModelAccess, ModelName: "gpt-4o", Success: true, LatencyMs: 800},
		{
			Provider:          "openai",
			CheckKey:          CheckReproducibility,
			ModelName:         "gpt-4o",
			Consistent:        false,
			ConsistencyMethod: ConsistencyMethodSystemFingerprintChanged,
			Success:           false,
			Score:             30,
		},
	}
	report := BuildReport(results)

	hasRisk := false
	for _, r := range report.Risks {
		if containsAll(r, "system_fingerprint", "路由抖动") {
			hasRisk = true
			break
		}
	}
	if !hasRisk {
		t.Errorf("expected system_fingerprint-change risk in report.Risks, got %v", report.Risks)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
