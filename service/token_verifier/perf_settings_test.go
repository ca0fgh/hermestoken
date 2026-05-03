package token_verifier

import (
	"testing"
	"time"
)

func TestModelCheckConcurrencyParsing(t *testing.T) {
	cases := []struct {
		env  string
		want int
	}{
		{"", defaultModelCheckConcurrency},
		{"  ", defaultModelCheckConcurrency},
		{"1", 1},
		{"5", 5},
		{"16", 16},
		{"100", maxModelCheckConcurrency}, // clamp upper bound
		{"0", defaultModelCheckConcurrency},
		{"-1", defaultModelCheckConcurrency},
		{"abc", defaultModelCheckConcurrency},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("TOKEN_VERIFIER_MODEL_CONCURRENCY", tc.env)
			if got := modelCheckConcurrency(); got != tc.want {
				t.Errorf("env=%q: got %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}

func TestBuildModelJobsPreservesOrderAndIsFirst(t *testing.T) {
	cases := []struct {
		name      string
		providers []string
		models    []string
		want      []modelJob
	}{
		{
			name:      "single provider, multiple models — only model 0 is first",
			providers: []string{"openai"},
			models:    []string{"gpt-4o", "gpt-4o-mini"},
			want: []modelJob{
				{provider: "openai", model: "gpt-4o", isFirst: true},
				{provider: "openai", model: "gpt-4o-mini", isFirst: false},
			},
		},
		{
			name:      "two providers each get their own first model",
			providers: []string{"openai", "anthropic"},
			models:    []string{"gpt-4o", "gpt-4o-mini"},
			want: []modelJob{
				{provider: "openai", model: "gpt-4o", isFirst: true},
				{provider: "openai", model: "gpt-4o-mini", isFirst: false},
				{provider: "anthropic", model: "gpt-4o", isFirst: true},
				{provider: "anthropic", model: "gpt-4o-mini", isFirst: false},
			},
		},
		{
			name:      "anthropic + default OpenAI singleton triggers Anthropic-default substitution",
			providers: []string{"anthropic"},
			models:    []string{defaultVerifierModel},
			want: []modelJob{
				{provider: "anthropic", model: defaultAnthropicModel, isFirst: true},
			},
		},
		{
			name:      "substitution does NOT fire when user explicitly listed the OpenAI default twice",
			providers: []string{"anthropic"},
			models:    []string{defaultVerifierModel, "another"},
			want: []modelJob{
				{provider: "anthropic", model: defaultVerifierModel, isFirst: true},
				{provider: "anthropic", model: "another", isFirst: false},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildModelJobs(tc.providers, tc.models)
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %d, want %d (jobs=%+v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("job[%d]: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestTaskTimeoutParsing(t *testing.T) {
	cases := []struct {
		env  string
		want time.Duration
	}{
		{"", defaultTaskTimeout},
		{"300", 300 * time.Second},
		{"60", minTaskTimeout},
		{"30", minTaskTimeout},      // clamp lower
		{"5", minTaskTimeout},       // clamp lower
		{"1800", maxTaskTimeout},    // exact upper
		{"3600", maxTaskTimeout},    // clamp upper
		{"0", defaultTaskTimeout},   // invalid → default
		{"abc", defaultTaskTimeout}, // invalid → default
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("TOKEN_VERIFIER_TASK_TIMEOUT_SEC", tc.env)
			if got := TaskTimeout(); got != tc.want {
				t.Errorf("env=%q: got %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
