package token_verifier

import (
	"testing"
	"time"
)

const aaSampleResponse = `{
  "status": 200,
  "data": [
    {
      "id": "uuid-1",
      "name": "GPT-4o",
      "slug": "gpt-4o",
      "median_time_to_first_token_seconds": 0.42,
      "median_output_tokens_per_second": 88.5
    },
    {
      "id": "uuid-2",
      "name": "GPT-4o mini",
      "slug": "gpt-4o-mini",
      "median_time_to_first_token_seconds": 0.31,
      "median_output_tokens_per_second": 142.7
    },
    {
      "id": "uuid-3",
      "name": "Claude 3.5 Haiku",
      "slug": "claude-3-5-haiku",
      "median_time_to_first_token_seconds": 0.55,
      "median_output_tokens_per_second": 65.2
    },
    {
      "id": "uuid-4",
      "name": "",
      "slug": "",
      "median_time_to_first_token_seconds": 9.99,
      "median_output_tokens_per_second": 99.99
    }
  ]
}`

func TestParseAABaselineResponse(t *testing.T) {
	snap, err := parseAABaselineResponse([]byte(aaSampleResponse))
	if err != nil {
		t.Fatalf("parseAABaselineResponse error: %v", err)
	}
	// Empty slug+name model must be dropped, leaving 3 distinct models.
	wantSlugs := []string{"gpt-4o", "gpt-4o-mini", "claude-3-5-haiku"}
	for _, slug := range wantSlugs {
		if _, ok := snap.Models[slug]; !ok {
			t.Errorf("missing baseline for slug=%s", slug)
		}
	}
	if got := snap.Models["gpt-4o"].TTFTSec; got != 0.42 {
		t.Errorf("gpt-4o TTFT: got %v, want 0.42", got)
	}
	if got := snap.Models["gpt-4o-mini"].OutputTPS; got != 142.7 {
		t.Errorf("gpt-4o-mini TPS: got %v, want 142.7", got)
	}
}

// LookupAABaseline must tolerate user-supplied model names with date suffixes,
// "-latest" tails, and underscore/case variants by reusing canonicalModelName.
func TestLookupAABaselineNormalization(t *testing.T) {
	t.Setenv("AA_API_KEY", "test-key")
	snap, err := parseAABaselineResponse([]byte(aaSampleResponse))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	SetAABaselineSnapshotForTest(snap)
	t.Cleanup(func() { SetAABaselineSnapshotForTest(nil) })

	cases := []struct {
		input string
		want  string // expected slug
	}{
		{"gpt-4o", "gpt-4o"},
		{"GPT-4o", "gpt-4o"},
		{"gpt-4o-2024-05-13", "gpt-4o"},
		{"gpt-4o-20240513", "gpt-4o"},
		{"gpt_4o_mini", "gpt-4o-mini"},
		{"claude-3-5-haiku-latest", "claude-3-5-haiku"},
		{"claude-3-5-haiku-20241022", "claude-3-5-haiku"},
		{"unknown-model-xyz", ""},
	}
	for _, tc := range cases {
		got := LookupAABaseline(tc.input)
		switch {
		case tc.want == "" && got != nil:
			t.Errorf("input=%q: expected miss, got slug=%s", tc.input, got.Slug)
		case tc.want != "" && got == nil:
			t.Errorf("input=%q: expected slug=%s, got nil", tc.input, tc.want)
		case tc.want != "" && got.Slug != tc.want:
			t.Errorf("input=%q: got slug=%s, want %s", tc.input, got.Slug, tc.want)
		}
	}
}

func TestAABaselineEnabledRespectsEnv(t *testing.T) {
	t.Setenv("AA_API_KEY", "")
	if AABaselineEnabled() {
		t.Errorf("expected disabled when AA_API_KEY empty")
	}

	t.Setenv("AA_API_KEY", "test-key")
	t.Setenv("AA_BASELINE_ENABLED", "")
	if !AABaselineEnabled() {
		t.Errorf("expected enabled by default when AA_API_KEY set")
	}

	t.Setenv("AA_BASELINE_ENABLED", "false")
	if AABaselineEnabled() {
		t.Errorf("expected disabled when AA_BASELINE_ENABLED=false")
	}
}

func TestAARefreshIntervalDefault(t *testing.T) {
	t.Setenv("AA_REFRESH_INTERVAL_HOURS", "")
	if got := AARefreshInterval(); got != 24*time.Hour {
		t.Errorf("default interval: got %v, want 24h", got)
	}
	t.Setenv("AA_REFRESH_INTERVAL_HOURS", "6")
	if got := AARefreshInterval(); got != 6*time.Hour {
		t.Errorf("custom interval: got %v, want 6h", got)
	}
	t.Setenv("AA_REFRESH_INTERVAL_HOURS", "abc")
	if got := AARefreshInterval(); got != 24*time.Hour {
		t.Errorf("invalid interval should fall back to default: got %v", got)
	}
}
