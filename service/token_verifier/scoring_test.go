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

func TestBuildReportExplainsUnscoredEndpointFailuresAsIncomplete(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderAnthropic,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbeNPMRegistry,
			ModelName: "claude-opus-4-7",
			Skipped:   true,
			Success:   false,
			Score:     0,
			ErrorCode: "http_502",
			Message:   "端点返回 502：HTTP 502",
			RiskLevel: "unknown",
		},
		{
			Provider:  ProviderAnthropic,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbePipIndex,
			ModelName: "claude-opus-4-7",
			Skipped:   true,
			Success:   false,
			Score:     0,
			ErrorCode: "http_502",
			Message:   "端点返回 502：HTTP 502",
			RiskLevel: "unknown",
		},
	})

	if report.ProbeScore != 0 || report.ProbeScoreMax != 100 {
		t.Fatalf("score range = %d-%d, want 0-100 for all-unscored probes", report.ProbeScore, report.ProbeScoreMax)
	}
	if !strings.Contains(report.Conclusion, "未完成") || !strings.Contains(report.Conclusion, "无法评价模型风险") {
		t.Fatalf("conclusion = %q, want incomplete/unscored wording", report.Conclusion)
	}
	if report.FinalRating.Conclusion != report.Conclusion {
		t.Fatalf("final rating conclusion = %q, want %q", report.FinalRating.Conclusion, report.Conclusion)
	}
	if len(report.Risks) != 0 {
		t.Fatalf("risks = %#v, want no model/security risks for skipped endpoint failures", report.Risks)
	}
}

func TestBuildReportDoesNotTreatSkippedEndpointMessagesAsRisks(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderAnthropic,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbeNPMRegistry,
			ModelName: "claude-opus-4-7",
			Skipped:   true,
			Success:   false,
			Score:     0,
			ErrorCode: "request_timeout",
			Message:   "request timeout: context deadline exceeded",
			RiskLevel: "unknown",
		},
		{
			Provider:  ProviderAnthropic,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbePipIndex,
			ModelName: "claude-opus-4-7",
			Skipped:   true,
			Success:   false,
			Score:     0,
			ErrorCode: "http_429",
			Message:   "endpoint returned 429 rate limit",
			RiskLevel: "unknown",
		},
	})

	if len(report.Risks) != 0 {
		t.Fatalf("risks = %#v, want no process/model risks for skipped endpoint failures", report.Risks)
	}
}

func TestBuildReportCountsWarningsAsHalfPointWithoutRisk(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupIntegrity,
			CheckKey:  CheckProbeSSECompliance,
			ModelName: "gpt-5.5",
			Success:   false,
			Score:     50,
			ErrorCode: "sse_compliance_warning",
			Message:   "SSE 合规但存在 choices 缺失的兼容性警告",
		},
	})

	if report.ProbeScore != 50 || report.ProbeScoreMax != 50 {
		t.Fatalf("warning score range = %d-%d, want 50-50", report.ProbeScore, report.ProbeScoreMax)
	}
	if len(report.Risks) != 0 {
		t.Fatalf("warning risks = %#v, want none", report.Risks)
	}
	if got := report.Checklist[0].Status; got != "warning" {
		t.Fatalf("warning checklist status = %q, want warning", got)
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
			RiskLevel: "low",
			Evidence:  []string{"contains Fortran"},
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
		"risk_level": true,
		"evidence":   true,
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

func TestBuildReportCarriesProbeRiskEvidence(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbeInfraLeak,
			ModelName: "gpt-5.5",
			Success:   false,
			Score:     0,
			ErrorCode: "infra_leak_high",
			Message:   "确认泄露真实基础设施",
			RiskLevel: "high",
			Evidence:  []string{"确认当前服务使用 Bedrock", "命中 anthropic_version=bedrock-2023-05-31"},
		},
	})

	if len(report.Checklist) != 1 {
		t.Fatalf("checklist len = %d, want 1", len(report.Checklist))
	}
	item := report.Checklist[0]
	if item.RiskLevel != "high" || len(item.Evidence) != 2 {
		t.Fatalf("checklist risk/evidence = %q %#v, want high with evidence", item.RiskLevel, item.Evidence)
	}
	renderedRisks := strings.Join(report.Risks, "\n")
	if !strings.Contains(renderedRisks, "高危") {
		t.Fatalf("risks = %#v, want high severity wording", report.Risks)
	}
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
