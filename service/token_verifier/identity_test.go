package token_verifier

import "testing"

func TestBuildModelIdentityResult(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		claimedModel   string
		observedModel  string
		wantConfidence int
		wantDowngrade  bool
		wantSuccess    bool
	}{
		{
			name:           "exact match",
			provider:       ProviderOpenAI,
			claimedModel:   "gpt-4o-mini",
			observedModel:  "gpt-4o-mini",
			wantConfidence: 95,
			wantSuccess:    true,
		},
		{
			name:           "dated model alias",
			provider:       ProviderOpenAI,
			claimedModel:   "gpt-4o-mini",
			observedModel:  "gpt-4o-mini-2024-07-18",
			wantConfidence: 90,
			wantSuccess:    true,
		},
		{
			name:           "anthropic latest alias",
			provider:       ProviderAnthropic,
			claimedModel:   "claude-3-5-haiku-latest",
			observedModel:  "claude-3-5-haiku-20241022",
			wantConfidence: 90,
			wantSuccess:    true,
		},
		{
			name:           "suspected downgrade",
			provider:       ProviderOpenAI,
			claimedModel:   "gpt-4o",
			observedModel:  "gpt-4o-mini",
			wantConfidence: 35,
			wantDowngrade:  true,
			wantSuccess:    false,
		},
		{
			name:           "gpt 5.5 exact match",
			provider:       ProviderOpenAI,
			claimedModel:   "gpt-5.5",
			observedModel:  "gpt-5.5",
			wantConfidence: 95,
			wantSuccess:    true,
		},
		{
			name:           "gpt 5.5 to 5.4 downgrade",
			provider:       ProviderOpenAI,
			claimedModel:   "gpt-5.5",
			observedModel:  "gpt-5.4",
			wantConfidence: 35,
			wantDowngrade:  true,
			wantSuccess:    false,
		},
		{
			name:           "claude opus 4.7 exact match",
			provider:       ProviderAnthropic,
			claimedModel:   "claude-opus-4-7",
			observedModel:  "claude-opus-4-7",
			wantConfidence: 95,
			wantSuccess:    true,
		},
		{
			name:           "claude opus 4.7 to 4.6 downgrade",
			provider:       ProviderAnthropic,
			claimedModel:   "claude-opus-4-7",
			observedModel:  "claude-opus-4-6",
			wantConfidence: 35,
			wantDowngrade:  true,
			wantSuccess:    false,
		},
		{
			name:           "missing observed model",
			provider:       ProviderOpenAI,
			claimedModel:   "gpt-4o-mini",
			observedModel:  "",
			wantConfidence: 50,
			wantSuccess:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildModelIdentityResult(tt.provider, tt.claimedModel, CheckResult{
				Provider:      tt.provider,
				CheckKey:      CheckModelAccess,
				ModelName:     tt.claimedModel,
				ClaimedModel:  tt.claimedModel,
				ObservedModel: tt.observedModel,
				Success:       true,
			})
			if result.CheckKey != CheckModelIdentity {
				t.Fatalf("CheckKey = %s, want %s", result.CheckKey, CheckModelIdentity)
			}
			if result.ClaimedModel != tt.claimedModel {
				t.Fatalf("ClaimedModel = %q, want %q", result.ClaimedModel, tt.claimedModel)
			}
			if result.ObservedModel != tt.observedModel {
				t.Fatalf("ObservedModel = %q, want %q", result.ObservedModel, tt.observedModel)
			}
			if result.IdentityConfidence != tt.wantConfidence {
				t.Fatalf("IdentityConfidence = %d, want %d", result.IdentityConfidence, tt.wantConfidence)
			}
			if result.SuspectedDowngrade != tt.wantDowngrade {
				t.Fatalf("SuspectedDowngrade = %v, want %v", result.SuspectedDowngrade, tt.wantDowngrade)
			}
			if result.Success != tt.wantSuccess {
				t.Fatalf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

func TestBuildModelIdentityResultFromRawModel(t *testing.T) {
	result := buildModelIdentityResult(ProviderOpenAI, "gpt-4o-mini", CheckResult{
		Provider: ProviderOpenAI,
		CheckKey: CheckModelAccess,
		Success:  true,
		Raw: map[string]any{
			"model": "gpt-4o-mini-2024-07-18",
		},
	})

	if result.ObservedModel != "gpt-4o-mini-2024-07-18" {
		t.Fatalf("ObservedModel = %q", result.ObservedModel)
	}
	if result.IdentityConfidence != 90 {
		t.Fatalf("IdentityConfidence = %d, want 90", result.IdentityConfidence)
	}
	if result.SuspectedDowngrade {
		t.Fatal("expected no suspected downgrade")
	}
}
