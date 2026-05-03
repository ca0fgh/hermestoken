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
				CheckKey:  CheckModelAccess,
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
				{Message: "check " + secret},
			},
			Models: []ModelSummary{
				{Message: "model " + secret},
			},
			ModelIdentity: []ModelIdentitySummary{
				{Message: "identity " + secret},
			},
			Reproducibility: []ReproducibilitySummary{
				{Message: "repro " + secret},
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
