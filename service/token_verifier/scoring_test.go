package token_verifier

import (
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

func TestBuildReportUsesLLMProbeScoreRange(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupQuality,
			CheckKey:  CheckProbeInstructionFollow,
			ModelName: "gpt-5.5",
			Success:   true,
			Score:     100,
		},
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbeInfraLeak,
			ModelName: "gpt-5.5",
			Success:   false,
			Score:     0,
		},
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupIntegrity,
			CheckKey:  CheckProbeSSECompliance,
			ModelName: "gpt-5.5",
			Skipped:   true,
			Success:   true,
			Score:     0,
		},
	})

	if report.Score != 33 || report.ProbeScore != 33 || report.ProbeScoreMax != 67 {
		t.Fatalf("score range = score:%d probe:%d-%d, want 33-67", report.Score, report.ProbeScore, report.ProbeScoreMax)
	}
	if report.FinalRating.Score != report.ProbeScore {
		t.Fatalf("FinalRating.Score = %d, want %d", report.FinalRating.Score, report.ProbeScore)
	}
	if report.FinalRating.ProbeScoreMax != report.ProbeScoreMax {
		t.Fatalf("FinalRating.ProbeScoreMax = %d, want %d", report.FinalRating.ProbeScoreMax, report.ProbeScoreMax)
	}
}

func TestBuildReportUsesLLMProbeReportShape(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupQuality,
			CheckKey:  CheckProbeInstructionFollow,
			ModelName: "gpt-5.5",
			Success:   true,
			Score:     100,
		},
	})

	payload, err := common.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var rendered map[string]any
	if err := common.Unmarshal(payload, &rendered); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	assertOnlyJSONKeys(t, rendered, map[string]bool{
		"score":                true,
		"grade":                true,
		"conclusion":           true,
		"checklist":            true,
		"identity_assessments": true,
		"risks":                true,
		"final_rating":         true,
		"scoring_version":      true,
		"probe_score":          true,
		"probe_score_max":      true,
	})
	checklist, ok := rendered["checklist"].([]any)
	if !ok || len(checklist) != 1 {
		t.Fatalf("checklist = %#v, want one item", rendered["checklist"])
	}
	item, ok := checklist[0].(map[string]any)
	if !ok {
		t.Fatalf("checklist item = %#v, want object", checklist[0])
	}
	assertOnlyJSONKeys(t, item, map[string]bool{
		"provider":   true,
		"group":      true,
		"check_key":  true,
		"check_name": true,
		"model_name": true,
		"passed":     true,
		"status":     true,
		"score":      true,
	})
	finalRating, ok := rendered["final_rating"].(map[string]any)
	if !ok {
		t.Fatalf("final_rating = %#v, want object", rendered["final_rating"])
	}
	assertOnlyJSONKeys(t, finalRating, map[string]bool{
		"score":           true,
		"grade":           true,
		"conclusion":      true,
		"risks":           true,
		"probe_score":     true,
		"probe_score_max": true,
	})
}

func assertOnlyJSONKeys(t *testing.T, object map[string]any, allowed map[string]bool) {
	t.Helper()
	for key := range object {
		if !allowed[key] {
			t.Fatalf("unexpected JSON key %q in %#v", key, object)
		}
	}
}

func TestBuildReportFlagsLLMProbeRisks(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbeInfraLeak,
			ModelName: "gpt-5.5",
			Success:   false,
			Score:     0,
		},
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupCanary,
			CheckKey:  CheckCanaryMathMul,
			ModelName: "gpt-5.5",
			Success:   false,
			Score:     0,
			Message:   "timeout",
		},
	})

	renderedRisks := strings.Join(report.Risks, "\n")
	for _, want := range []string{"基础设施探针", "金丝雀基准", "超时风险"} {
		if !strings.Contains(renderedRisks, want) {
			t.Fatalf("risks = %v, want %q", report.Risks, want)
		}
	}
}
