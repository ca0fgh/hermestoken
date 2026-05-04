package token_verifier

import "testing"

func TestComputeIdentityVerdictCleanSubmodelMismatch(t *testing.T) {
	verdict := computeIdentityVerdict(IdentityVerdictInput{
		ClaimedFamily: "anthropic",
		ClaimedModel:  "claude-opus-4-6",
		Surface:       &IdentitySignal{Family: "anthropic", Score: 0.8},
		Behavior:      &IdentitySignal{Family: "anthropic", Score: 0.9},
		V3:            &IdentitySubmodelSignal{Family: "anthropic", ModelID: "claude-opus-4-7", DisplayName: "Claude Opus 4.7", Score: 0.88},
	})

	if verdict.Status != "clean_match_submodel_mismatch" {
		t.Fatalf("status = %q, want clean_match_submodel_mismatch: %+v", verdict.Status, verdict)
	}
	if verdict.TrueFamily != "anthropic" || verdict.TrueModel != "Claude Opus 4.7" {
		t.Fatalf("true identity = %q/%q, want anthropic/Claude Opus 4.7", verdict.TrueFamily, verdict.TrueModel)
	}
}

func TestComputeIdentityVerdictSpoofSelfclaimForged(t *testing.T) {
	verdict := computeIdentityVerdict(IdentityVerdictInput{
		ClaimedFamily: "openai",
		ClaimedModel:  "gpt-5.5",
		Surface:       &IdentitySignal{Family: "openai", Score: 0.8},
		Behavior:      &IdentitySignal{Family: "anthropic", Score: 0.82},
		V3:            &IdentitySubmodelSignal{Family: "openai", ModelID: "gpt-5.5", DisplayName: "GPT-5.5", Score: 0.7},
	})

	if verdict.Status != "spoof_selfclaim_forged" || verdict.SpoofMethod != "selfclaim_forged" {
		t.Fatalf("verdict = %+v, want selfclaim forged spoof", verdict)
	}
	if verdict.TrueFamily != "anthropic" {
		t.Fatalf("true family = %q, want anthropic", verdict.TrueFamily)
	}
}

func TestApplySubmodelAbstainGuardClearsWeakWrongFamilyMatch(t *testing.T) {
	submodels := []SubmodelAssessmentSummary{
		{Method: "v3", Family: "anthropic", ModelID: "claude-opus-4-7", DisplayName: "Claude Opus 4.7", Score: 0.7},
	}
	candidates := []IdentityCandidateSummary{
		{Family: "anthropic", Score: 0.7},
		{Family: "openai", Score: 0.45},
	}

	guarded := applySubmodelAbstainGuard(submodels, candidates, "openai")

	if len(guarded) != 1 || !guarded[0].Abstained {
		t.Fatalf("guarded submodels = %+v, want abstained", guarded)
	}
	if guarded[0].Family != "" || guarded[0].ModelID != "" || guarded[0].DisplayName != "" {
		t.Fatalf("abstained submodel leaked a concrete verdict: %+v", guarded[0])
	}
}
