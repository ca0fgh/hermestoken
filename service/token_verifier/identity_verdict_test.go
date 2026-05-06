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

func TestSurfaceIdentitySignalNormalizesClaudeToAnthropicFamily(t *testing.T) {
	signal := surfaceIdentitySignal(map[CheckKey]string{
		CheckProbeIdentitySelfKnowledge: "I am Claude, created by Anthropic.",
	})

	if signal == nil {
		t.Fatal("surface signal missing for explicit Claude self-claim")
	}
	if signal.Family != "anthropic" {
		t.Fatalf("surface family = %q, want anthropic", signal.Family)
	}
}
