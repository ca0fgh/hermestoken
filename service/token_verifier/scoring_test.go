package token_verifier

import (
	"os"
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

func TestBuildReportGoldenReplays(t *testing.T) {
	for _, replay := range loadReportGoldenReplays(t) {
		replay := replay
		t.Run(replay.Name, func(t *testing.T) {
			results := make([]CheckResult, 0, len(replay.Results))
			for _, result := range replay.Results {
				results = append(results, result.CheckResult)
				results[len(results)-1].PrivateResponseText = result.PrivateResponseText
			}
			report := BuildReport(results)

			if report.ProbeScore != replay.WantProbeScore || report.ProbeScoreMax != replay.WantProbeScoreMax {
				t.Fatalf("score range = %d-%d, want %d-%d", report.ProbeScore, report.ProbeScoreMax, replay.WantProbeScore, replay.WantProbeScoreMax)
			}
			if replay.WantRisksCount != nil && len(report.Risks) != *replay.WantRisksCount {
				t.Fatalf("risks = %#v, want count %d", report.Risks, *replay.WantRisksCount)
			}
			joinedRisks := strings.Join(report.Risks, "\n")
			for _, want := range replay.WantRisksContains {
				if !strings.Contains(joinedRisks, want) {
					t.Fatalf("risks = %#v, want substring %q", report.Risks, want)
				}
			}
			if replay.WantConclusionContains != "" && !strings.Contains(report.Conclusion, replay.WantConclusionContains) {
				t.Fatalf("conclusion = %q, want substring %q", report.Conclusion, replay.WantConclusionContains)
			}
			for checkKey, wantStatus := range replay.WantStatusByCheck {
				item := findChecklistItemForTest(report.Checklist, CheckKey(checkKey))
				if item == nil {
					t.Fatalf("checklist missing %s in %+v", checkKey, report.Checklist)
				}
				if item.Status != wantStatus {
					t.Fatalf("%s status = %q, want %q", checkKey, item.Status, wantStatus)
				}
			}
			if replay.WantIdentityStatus != "" {
				if len(report.IdentityAssessments) != 1 {
					t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
				}
				assessment := report.IdentityAssessments[0]
				if assessment.Status != replay.WantIdentityStatus {
					t.Fatalf("identity status = %q, want %q: %+v", assessment.Status, replay.WantIdentityStatus, assessment)
				}
				if assessment.PredictedFamily != replay.WantPredictedFamily {
					t.Fatalf("predicted family = %q, want %q: %+v", assessment.PredictedFamily, replay.WantPredictedFamily, assessment)
				}
				if replay.WantIdentityVerdictStatus != "" {
					if assessment.Verdict == nil || assessment.Verdict.Status != replay.WantIdentityVerdictStatus {
						t.Fatalf("identity verdict = %+v, want status %q", assessment.Verdict, replay.WantIdentityVerdictStatus)
					}
				}
				if replay.WantTopCandidateFamily != "" {
					if len(assessment.Candidates) == 0 || assessment.Candidates[0].Family != replay.WantTopCandidateFamily {
						t.Fatalf("top candidates = %+v, want family %q", assessment.Candidates, replay.WantTopCandidateFamily)
					}
				}
			}
		})
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

func TestBuildReportAddsActionableMetadataToEveryChecklistItem(t *testing.T) {
	report := BuildReport([]CheckResult{
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupIntegrity,
			CheckKey:  CheckProbeToolCallIntegrity,
			ModelName: "gpt-test",
			Success:   false,
			Score:     0,
			ErrorCode: "tool_call_argument_mismatch",
			Message:   "工具调用参数被改写",
			RiskLevel: "high",
		},
		{
			Provider:  ProviderOpenAI,
			Group:     probeGroupSecurity,
			CheckKey:  CheckProbeURLExfiltration,
			ModelName: "gpt-test",
			Success:   true,
			Score:     100,
			RiskLevel: "low",
		},
	})

	if len(report.Checklist) != 2 {
		t.Fatalf("checklist length = %d, want 2", len(report.Checklist))
	}
	for _, item := range report.Checklist {
		if strings.TrimSpace(item.CheckDescription) == "" {
			t.Fatalf("%s missing description: %+v", item.CheckKey, item)
		}
		if strings.TrimSpace(item.Coverage) == "" {
			t.Fatalf("%s missing coverage metadata: %+v", item.CheckKey, item)
		}
		if strings.TrimSpace(item.Limitation) == "" {
			t.Fatalf("%s missing limitation metadata: %+v", item.CheckKey, item)
		}
		if strings.TrimSpace(item.RecommendedAction) == "" {
			t.Fatalf("%s missing recommended action metadata: %+v", item.CheckKey, item)
		}
	}

	toolCallItem := findChecklistItemForTest(report.Checklist, CheckProbeToolCallIntegrity)
	if toolCallItem == nil {
		t.Fatal("tool-call integrity checklist item missing")
	}
	if !strings.Contains(toolCallItem.Coverage, "tool-call") {
		t.Fatalf("tool-call coverage = %q, want tool-call scope", toolCallItem.Coverage)
	}
	if !strings.Contains(toolCallItem.Limitation, "provider-signed") {
		t.Fatalf("tool-call limitation = %q, want provider-signed limitation", toolCallItem.Limitation)
	}
	if !strings.Contains(toolCallItem.RecommendedAction, "fail-closed") {
		t.Fatalf("tool-call action = %q, want fail-closed guidance", toolCallItem.RecommendedAction)
	}
}

type reportGoldenReplay struct {
	Name    string `json:"name"`
	Results []struct {
		CheckResult
		PrivateResponseText string `json:"private_response_text"`
	} `json:"results"`
	WantProbeScore            int               `json:"want_probe_score"`
	WantProbeScoreMax         int               `json:"want_probe_score_max"`
	WantRisksCount            *int              `json:"want_risks_count"`
	WantRisksContains         []string          `json:"want_risks_contains"`
	WantConclusionContains    string            `json:"want_conclusion_contains"`
	WantStatusByCheck         map[string]string `json:"want_status_by_check"`
	WantIdentityStatus        string            `json:"want_identity_status"`
	WantIdentityVerdictStatus string            `json:"want_identity_verdict_status"`
	WantPredictedFamily       string            `json:"want_predicted_family"`
	WantTopCandidateFamily    string            `json:"want_top_candidate_family"`
}

func loadReportGoldenReplays(t *testing.T) []reportGoldenReplay {
	t.Helper()
	data, err := os.ReadFile("testdata/report_replay_golden.json")
	if err != nil {
		t.Fatalf("read report replay golden samples: %v", err)
	}
	var replays []reportGoldenReplay
	if err := common.Unmarshal(data, &replays); err != nil {
		t.Fatalf("parse report replay golden samples: %v", err)
	}
	if len(replays) == 0 {
		t.Fatal("report replay golden samples must not be empty")
	}
	for _, replay := range replays {
		if strings.TrimSpace(replay.Name) == "" || len(replay.Results) == 0 {
			t.Fatalf("invalid report replay sample: %+v", replay)
		}
	}
	return replays
}

func findChecklistItemForTest(items []ChecklistItem, checkKey CheckKey) *ChecklistItem {
	for i := range items {
		if items[i].CheckKey == string(checkKey) {
			return &items[i]
		}
	}
	return nil
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
		"provider":           true,
		"group":              true,
		"check_key":          true,
		"check_name":         true,
		"check_description":  true,
		"coverage":           true,
		"limitation":         true,
		"recommended_action": true,
		"model_name":         true,
		"passed":             true,
		"status":             true,
		"score":              true,
		"risk_level":         true,
		"evidence":           true,
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

func TestBuildReportAddsUsableDescriptionsForRuntimeFullProbeItems(t *testing.T) {
	results := make([]CheckResult, 0)
	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		results = append(results, CheckResult{
			Provider:  ProviderOpenAI,
			Group:     probe.Group,
			CheckKey:  probe.Key,
			ModelName: "gpt-5.5",
			Success:   true,
			Score:     100,
		})
	}

	report := BuildReport(results)
	if len(report.Checklist) != len(results) {
		t.Fatalf("checklist count = %d, want %d", len(report.Checklist), len(results))
	}
	for _, item := range report.Checklist {
		if strings.TrimSpace(item.CheckName) == "" {
			t.Fatalf("%s check name is empty", item.CheckKey)
		}
		if strings.TrimSpace(item.CheckDescription) == "" {
			t.Fatalf("%s check description is empty", item.CheckKey)
		}
		if item.CheckDescription == item.CheckName {
			t.Fatalf("%s description duplicates name %q", item.CheckKey, item.CheckName)
		}
	}
}

func TestAllFullProbeDefinitionsHaveSpecificDescriptions(t *testing.T) {
	genericDescription := checkDescription("")
	seen := make(map[CheckKey]bool)
	probes := append(probeSuiteDefinitions(ProbeProfileFull), verifierCanaryProbeSuite()...)
	for _, probe := range probes {
		if seen[probe.Key] {
			continue
		}
		seen[probe.Key] = true
		description := strings.TrimSpace(checkDescription(probe.Key))
		if description == "" {
			t.Fatalf("%s description is empty", probe.Key)
		}
		if description == genericDescription {
			t.Fatalf("%s uses generic description %q", probe.Key, description)
		}
		if description == checkDisplayName(probe.Key) {
			t.Fatalf("%s description duplicates display name %q", probe.Key, description)
		}
	}
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
