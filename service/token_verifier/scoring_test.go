package token_verifier

import "testing"

func TestBuildReportIncludesModelIdentity(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			CheckKey:  CheckAvailability,
			ModelName: "gpt-4o-mini",
			Success:   true,
			Score:     100,
			LatencyMs: 800,
		},
		{
			Provider:  ProviderOpenAI,
			CheckKey:  CheckModelAccess,
			ModelName: "gpt-4o-mini",
			Success:   true,
			Score:     100,
			LatencyMs: 800,
		},
		{
			Provider:           ProviderOpenAI,
			CheckKey:           CheckModelIdentity,
			ModelName:          "gpt-4o-mini",
			ClaimedModel:       "gpt-4o-mini",
			ObservedModel:      "gpt-4o-mini-2024-07-18",
			IdentityConfidence: 90,
			Success:            true,
			Score:              90,
		},
		{
			Provider:  ProviderOpenAI,
			CheckKey:  CheckStream,
			ModelName: "gpt-4o-mini",
			Success:   true,
			Score:     100,
			LatencyMs: 1200,
			TTFTMs:    300,
			TokensPS:  40,
		},
		{
			Provider:  ProviderOpenAI,
			CheckKey:  CheckJSON,
			ModelName: "gpt-4o-mini",
			Success:   true,
			Score:     100,
			LatencyMs: 900,
		},
	})

	if len(report.ModelIdentity) != 1 {
		t.Fatalf("len(ModelIdentity) = %d, want 1", len(report.ModelIdentity))
	}
	identity := report.ModelIdentity[0]
	if identity.ClaimedModel != "gpt-4o-mini" {
		t.Fatalf("ClaimedModel = %q", identity.ClaimedModel)
	}
	if identity.ObservedModel != "gpt-4o-mini-2024-07-18" {
		t.Fatalf("ObservedModel = %q", identity.ObservedModel)
	}
	if identity.IdentityConfidence != 90 {
		t.Fatalf("IdentityConfidence = %d, want 90", identity.IdentityConfidence)
	}
	if report.Dimensions["model_identity"] != 13 {
		t.Fatalf("model_identity dimension = %d, want 13", report.Dimensions["model_identity"])
	}
	if report.Score != 98 {
		t.Fatalf("Score = %d, want 98", report.Score)
	}
	if report.FinalRating.Score != report.Score {
		t.Fatalf("FinalRating.Score = %d, want %d", report.FinalRating.Score, report.Score)
	}
}

func TestBuildReportFlagsSuspectedDowngradeRisk(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			CheckKey:  CheckModelAccess,
			ModelName: "gpt-4o",
			Success:   true,
			Score:     100,
			LatencyMs: 1000,
		},
		{
			Provider:           ProviderOpenAI,
			CheckKey:           CheckModelIdentity,
			ModelName:          "gpt-4o",
			ClaimedModel:       "gpt-4o",
			ObservedModel:      "gpt-4o-mini",
			IdentityConfidence: 35,
			SuspectedDowngrade: true,
			Success:            false,
			Score:              35,
		},
	})

	if len(report.ModelIdentity) != 1 {
		t.Fatalf("len(ModelIdentity) = %d, want 1", len(report.ModelIdentity))
	}
	if !report.ModelIdentity[0].SuspectedDowngrade {
		t.Fatal("expected suspected downgrade in model identity summary")
	}
	if len(report.Risks) == 0 {
		t.Fatal("expected downgrade risk")
	}
}
