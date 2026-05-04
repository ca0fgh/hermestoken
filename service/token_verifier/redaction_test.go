package token_verifier

import (
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

func TestRedactDirectProbeResponseRedactsNestedSecretFields(t *testing.T) {
	const secret = "sk-test-secret-value"
	response := &DirectProbeResponse{
		BaseURL:  "https://api.example.com",
		Provider: ProviderOpenAI,
		Model:    "gpt-5.5",
		Results: []CheckResult{
			{
				Provider:  ProviderOpenAI,
				Group:     probeGroupQuality,
				CheckKey:  CheckProbeInstructionFollow,
				ModelName: "gpt-5.5",
				Message:   "upstream echoed " + secret,
				Raw: map[string]any{
					"authorization": "Bearer " + secret,
					"nested": []any{
						map[string]any{"value": secret},
					},
				},
			},
		},
		Report: ReportSummary{
			Conclusion: "report " + secret,
			Checklist: []ChecklistItem{
				{
					Provider:  ProviderOpenAI,
					CheckKey:  string(CheckProbeInstructionFollow),
					CheckName: "probe " + secret,
					ModelName: "gpt-5.5",
					Message:   "check " + secret,
				},
			},
			IdentityAssessments: []IdentityAssessmentSummary{
				{
					Provider:        ProviderOpenAI,
					ModelName:       "gpt-5.5",
					ClaimedModel:    "claim " + secret,
					Status:          "status " + secret,
					PredictedFamily: "family " + secret,
					PredictedModel:  "model " + secret,
					Method:          "method " + secret,
					Candidates: []IdentityCandidateSummary{
						{
							Family:  "candidate-family " + secret,
							Model:   "candidate-model " + secret,
							Reasons: []string{"reason " + secret},
						},
					},
					SubmodelAssessments: []SubmodelAssessmentSummary{
						{
							Method:      "submethod " + secret,
							Family:      "subfamily " + secret,
							ModelID:     "submodel " + secret,
							DisplayName: "display " + secret,
							Evidence:    []string{"sub evidence " + secret},
						},
					},
					Verdict: &IdentityVerdictSummary{
						Status:         "verdict " + secret,
						TrueFamily:     "true family " + secret,
						TrueModel:      "true model " + secret,
						SpoofMethod:    "spoof " + secret,
						ConfidenceBand: "band " + secret,
						Reasoning:      []string{"verdict reason " + secret},
					},
					RiskFlags: []string{"flag " + secret},
					Evidence:  []string{"identity evidence " + secret},
				},
			},
			Risks: []string{"risk " + secret},
			FinalRating: FinalRating{
				Conclusion: "final " + secret,
				Risks:      []string{"final risk " + secret},
			},
		},
	}

	redacted := RedactDirectProbeResponse(response, secret)
	if redacted == response {
		t.Fatal("expected redaction to return a copied response")
	}

	payload, err := common.Marshal(redacted)
	if err != nil {
		t.Fatalf("failed to marshal redacted response: %v", err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatalf("redacted response still contains secret: %s", string(payload))
	}
	if !strings.Contains(string(payload), "[REDACTED]") {
		t.Fatalf("expected redaction marker in response: %s", string(payload))
	}

	originalPayload, err := common.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal original response: %v", err)
	}
	if !strings.Contains(string(originalPayload), secret) {
		t.Fatal("redaction should not mutate the original response")
	}
}
