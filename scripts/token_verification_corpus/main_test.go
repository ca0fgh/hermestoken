package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
)

func TestDecodeDirectProbeResponseAcceptsAPIEnvelope(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"success": true,
		"message": "",
		"data": map[string]any{
			"provider":      tokenverifier.ProviderOpenAI,
			"model":         "gpt-test",
			"probe_profile": tokenverifier.ProbeProfileFull,
			"results": []map[string]any{
				{
					"provider":   tokenverifier.ProviderOpenAI,
					"check_key":  tokenverifier.CheckProbeInstructionFollow,
					"model_name": "gpt-test",
					"success":    true,
					"score":      100,
					"raw": map[string]any{
						"response_sample": "Fortran",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	response, err := decodeDirectProbeResponse(payload)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].CheckKey != tokenverifier.CheckProbeInstructionFollow {
		t.Fatalf("response = %+v, want one instruction-follow result", response)
	}

	corpus := tokenverifier.BuildLabeledProbeCorpusDraftFromResults("capture", response.Results)
	if len(corpus.Cases) != 1 || corpus.Cases[0].ResponseText != "Fortran" {
		t.Fatalf("corpus = %+v, want response sample case", corpus)
	}
}

func TestDecodeDirectProbeResponseAcceptsTaskDetailEnvelope(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"success": true,
		"data": map[string]any{
			"task": map[string]any{
				"id":         42,
				"models":     []string{"gpt-task"},
				"providers":  []string{tokenverifier.ProviderOpenAI},
				"created_at": 1788566400,
			},
			"results": []map[string]any{
				{
					"provider":   tokenverifier.ProviderOpenAI,
					"group":      "quality",
					"check_key":  tokenverifier.CheckProbeInstructionFollow,
					"model_name": "gpt-task",
					"success":    true,
					"score":      100,
					"raw":        `{"response_sample":"Fortran\nLisp"}`,
				},
			},
			"report": map[string]any{
				"score":           100,
				"grade":           "A",
				"conclusion":      "ok",
				"checklist":       []map[string]any{},
				"risks":           []string{},
				"scoring_version": tokenverifier.ScoringVersionV4,
				"probe_score":     100,
				"probe_score_max": 100,
				"final_rating":    map[string]any{"score": 100, "grade": "A", "conclusion": "ok", "risks": []string{}, "probe_score": 100, "probe_score_max": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal task detail: %v", err)
	}

	response, err := decodeDirectProbeResponse(payload)
	if err != nil {
		t.Fatalf("decode task detail: %v", err)
	}
	if response.Provider != tokenverifier.ProviderOpenAI || response.Model != "gpt-task" {
		t.Fatalf("response target = provider:%q model:%q, want task provider/model", response.Provider, response.Model)
	}
	if response.SourceTaskID != "42" || response.CapturedAt != "2026-09-05T00:00:00Z" {
		t.Fatalf("response source = task:%q captured:%q, want task detail metadata", response.SourceTaskID, response.CapturedAt)
	}
	if len(response.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(response.Results))
	}
	if got := response.Results[0].Raw["response_sample"]; got != "Fortran\nLisp" {
		t.Fatalf("raw response_sample = %#v, want decoded JSON raw", got)
	}
}

func TestTaskDetailEnvelopeCanBuildProbeCorpusDraft(t *testing.T) {
	payload := []byte(`{
		"success": true,
		"data": {
			"task": {"id":42,"models":["gpt-task"],"providers":["openai"],"created_at":1788566400},
			"results": [
				{
					"provider":"openai",
					"group":"quality",
					"check_key":"probe_instruction_follow",
					"model_name":"gpt-task",
					"success":true,
					"score":100,
					"risk_level":"low",
					"raw":"{\"response_sample\":\"Fortran\\nLisp\"}"
				},
				{
					"provider":"openai",
					"group":"security",
					"check_key":"probe_channel_signature",
					"model_name":"gpt-task",
					"neutral":true,
					"success":true,
					"score":100,
					"raw":"{\"channel\":\"openrouter\",\"headers\":{\"x-generation-id\":\"gen-test\"}}"
				}
			]
		}
	}`)

	response, err := decodeDirectProbeResponse(payload)
	if err != nil {
		t.Fatalf("decode task detail: %v", err)
	}
	probeOutput, err := buildCorpusDraftOutput(*response, "probe", "task capture", nil)
	if err != nil {
		t.Fatalf("build probe corpus: %v", err)
	}
	if !strings.Contains(string(probeOutput), `"response_text": "Fortran\nLisp"`) {
		t.Fatalf("probe output = %s, want response text from task raw", string(probeOutput))
	}
	if !strings.Contains(string(probeOutput), `"task_id": "42"`) || !strings.Contains(string(probeOutput), `"captured_at": "2026-09-05T00:00:00Z"`) {
		t.Fatalf("probe output = %s, want task source metadata", string(probeOutput))
	}
	infoOutput, err := buildCorpusDraftOutput(*response, "informational", "task capture", nil)
	if err != nil {
		t.Fatalf("build informational corpus: %v", err)
	}
	if !strings.Contains(string(infoOutput), `"want_channel": "openrouter"`) {
		t.Fatalf("informational output = %s, want channel signature from task raw", string(infoOutput))
	}
}

func TestSourceMetadataFlagsCanBuildProbeCorpusDraft(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider: tokenverifier.ProviderOpenAI,
		Model:    "gpt-manual",
		Results: []tokenverifier.CheckResult{
			{
				CheckKey: tokenverifier.CheckProbeInstructionFollow,
				Success:  true,
				Score:    100,
				Raw: map[string]any{
					"response_sample": "Fortran",
				},
			},
		},
	}
	source, err := parseSourceMetadataFlags("manual-task-17", "2026-05-06T12:34:56Z")
	if err != nil {
		t.Fatalf("parse source flags: %v", err)
	}
	response = directProbeResponseWithSourceFallback(response, source)

	output, err := buildCorpusDraftOutput(response, "probe", "manual capture", nil)
	if err != nil {
		t.Fatalf("build probe corpus: %v", err)
	}
	text := string(output)
	for _, want := range []string{
		`"provider": "openai"`,
		`"model": "gpt-manual"`,
		`"task_id": "manual-task-17"`,
		`"captured_at": "2026-05-06T12:34:56Z"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("probe output = %s, want %q", text, want)
		}
	}
}

func TestSourceMetadataFlagsDoNotOverrideTaskDetailSource(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider:     tokenverifier.ProviderOpenAI,
		Model:        "gpt-task",
		SourceTaskID: "42",
		CapturedAt:   "2026-09-05T00:00:00Z",
	}
	source, err := parseSourceMetadataFlags("manual-task-17", "2026-05-06T12:34:56Z")
	if err != nil {
		t.Fatalf("parse source flags: %v", err)
	}
	response = directProbeResponseWithSourceFallback(response, source)
	if response.SourceTaskID != "42" || response.CapturedAt != "2026-09-05T00:00:00Z" {
		t.Fatalf("response source = task:%q captured:%q, want original task detail source", response.SourceTaskID, response.CapturedAt)
	}
}

func TestSourceMetadataFlagsRejectInvalidCapturedAt(t *testing.T) {
	_, err := parseSourceMetadataFlags("manual-task-17", "2026-05-06 12:34:56")
	if err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Fatalf("captured-at error = %v, want RFC3339 validation error", err)
	}
}

func TestCorpusBundleWritesThreeDraftFiles(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider:     tokenverifier.ProviderOpenAI,
		Model:        "gpt-task",
		SourceTaskID: "manual-task-17",
		CapturedAt:   "2026-05-06T12:34:56Z",
		Results: []tokenverifier.CheckResult{
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "quality",
				CheckKey:            tokenverifier.CheckProbeInstructionFollow,
				ModelName:           "gpt-task",
				Success:             true,
				PrivateResponseText: "Fortran",
			},
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "identity",
				CheckKey:            tokenverifier.CheckProbeIdentitySelfKnowledge,
				ModelName:           "gpt-task",
				Neutral:             true,
				Success:             true,
				PrivateResponseText: "I am ChatGPT, a model created by OpenAI.",
			},
			{
				Provider:  tokenverifier.ProviderOpenAI,
				Group:     "security",
				CheckKey:  tokenverifier.CheckProbeChannelSignature,
				ModelName: "gpt-task",
				Neutral:   true,
				Success:   true,
				Raw: map[string]any{
					"channel": "openrouter",
				},
			},
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "quality",
				CheckKey:            tokenverifier.CheckProbeZHReasoning,
				ModelName:           "gpt-task",
				Success:             true,
				PrivateResponseText: "繁中推理参考答案",
			},
		},
	}
	response.Report = tokenverifier.BuildReport(response.Results)

	bundle := buildCorpusBundleDrafts(response, "task capture", nil)
	outputDir := t.TempDir()
	if err := writeCorpusBundleFiles(outputDir, bundle); err != nil {
		t.Fatalf("write bundle files: %v", err)
	}

	for _, name := range []string{
		labeledProbeCorpusDraftFilename,
		identityAssessmentCorpusDraftFilename,
		informationalProbeCorpusDraftFilename,
		probeBaselineDraftFilename,
	} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(data), "task capture") {
			t.Fatalf("%s = %s, want bundle description", name, string(data))
		}
	}
	for _, name := range []string{
		labeledProbeCorpusDraftFilename,
		identityAssessmentCorpusDraftFilename,
		informationalProbeCorpusDraftFilename,
	} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(data), `"captured_at": "2026-05-06T12:34:56Z"`) {
			t.Fatalf("%s = %s, want direct probe captured_at inherited by corpus cases", name, string(data))
		}
	}
	baseline, err := os.ReadFile(filepath.Join(outputDir, probeBaselineDraftFilename))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !strings.Contains(string(baseline), `"probeId": "zh_reasoning"`) {
		t.Fatalf("baseline = %s, want zh_reasoning baseline", string(baseline))
	}
}

func TestBuildCorpusEvidenceSummaryReportsDraftCountsAndSourceReadiness(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider:     tokenverifier.ProviderOpenAI,
		Model:        "gpt-task",
		ProbeProfile: tokenverifier.ProbeProfileFull,
		SourceTaskID: "manual-task-17",
		CapturedAt:   "2026-05-06T12:34:56Z",
		Results: []tokenverifier.CheckResult{
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "quality",
				CheckKey:            tokenverifier.CheckProbeInstructionFollow,
				ModelName:           "gpt-task",
				Success:             true,
				PrivateResponseText: "Fortran",
			},
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "identity",
				CheckKey:            tokenverifier.CheckProbeIdentitySelfKnowledge,
				ModelName:           "gpt-task",
				Neutral:             true,
				Success:             true,
				PrivateResponseText: "I am ChatGPT, a model created by OpenAI.",
			},
			{
				Provider:  tokenverifier.ProviderOpenAI,
				Group:     "security",
				CheckKey:  tokenverifier.CheckProbeChannelSignature,
				ModelName: "gpt-task",
				Neutral:   true,
				Success:   true,
				Raw: map[string]any{
					"channel": "openrouter",
				},
			},
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "quality",
				CheckKey:            tokenverifier.CheckProbeZHReasoning,
				ModelName:           "gpt-task",
				Success:             true,
				PrivateResponseText: "繁中推理参考答案",
			},
		},
	}
	response.Report = tokenverifier.BuildReport(response.Results)

	output, err := buildCorpusEvidenceSummaryOutput(response, "task capture", nil)
	if err != nil {
		t.Fatalf("build summary: %v", err)
	}
	text := string(output)
	for _, want := range []string{
		`"source_ready": true`,
		`"pass_fail_cases": 1`,
		`"identity_cases": 1`,
		`"informational_cases": 1`,
		`"baseline_probes": 1`,
		`"zh_reasoning"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary = %s, want %q", text, want)
		}
	}
}

func TestBuildCorpusEvidenceSummaryReportsMissingSourceMetadata(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider: tokenverifier.ProviderOpenAI,
		Model:    "gpt-task",
		Results: []tokenverifier.CheckResult{
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "quality",
				CheckKey:            tokenverifier.CheckProbeInstructionFollow,
				ModelName:           "gpt-task",
				Success:             true,
				PrivateResponseText: "Fortran",
			},
		},
	}

	output, err := buildCorpusEvidenceSummaryOutput(response, "", nil)
	if err != nil {
		t.Fatalf("build summary: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, `"source_ready": false`) ||
		!strings.Contains(text, `"captured_at"`) ||
		!strings.Contains(text, `"source_task_id"`) {
		t.Fatalf("summary = %s, want missing source_task_id and captured_at source metadata", text)
	}
}

func TestDecodeDirectProbeResponseRejectsMissingResults(t *testing.T) {
	if _, err := decodeDirectProbeResponse([]byte(`{"success":true,"data":{"results":[]}}`)); err == nil {
		t.Fatal("decode empty results succeeded; want error")
	}
}

func TestBuildCorpusDraftOutputSupportsIdentityMode(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider: tokenverifier.ProviderOpenAI,
		Model:    "gpt-test",
		Results: []tokenverifier.CheckResult{
			{
				Provider:            tokenverifier.ProviderOpenAI,
				Group:               "identity",
				CheckKey:            tokenverifier.CheckProbeIdentitySelfKnowledge,
				ModelName:           "gpt-test",
				Neutral:             true,
				Success:             true,
				PrivateResponseText: "I am ChatGPT, a model created by OpenAI.",
			},
		},
	}
	response.Report = tokenverifier.BuildReport(response.Results)

	output, err := buildCorpusDraftOutput(response, "identity", "identity capture", nil)
	if err != nil {
		t.Fatalf("build identity output: %v", err)
	}
	if !strings.Contains(string(output), "want_identity_status") || strings.Contains(string(output), "want_passed") {
		t.Fatalf("identity output = %s, want identity corpus shape", string(output))
	}
}

func TestBuildCorpusDraftOutputSupportsInformationalMode(t *testing.T) {
	response := tokenverifier.DirectProbeResponse{
		Provider: tokenverifier.ProviderOpenAI,
		Model:    "gpt-test",
		Results: []tokenverifier.CheckResult{
			{
				Provider:  tokenverifier.ProviderOpenAI,
				Group:     "security",
				CheckKey:  tokenverifier.CheckProbeChannelSignature,
				ModelName: "gpt-test",
				Neutral:   true,
				Success:   true,
				Raw: map[string]any{
					"channel": "openrouter",
					"headers": map[string]any{
						"x-generation-id": "gen-test",
					},
				},
			},
		},
	}

	output, err := buildCorpusDraftOutput(response, "informational", "info capture", nil)
	if err != nil {
		t.Fatalf("build informational output: %v", err)
	}
	if !strings.Contains(string(output), "want_channel") || !strings.Contains(string(output), "probe_channel_signature") {
		t.Fatalf("informational output = %s, want informational corpus shape", string(output))
	}
}

func TestBuildCorpusDraftOutputRejectsUnknownMode(t *testing.T) {
	_, err := buildCorpusDraftOutput(tokenverifier.DirectProbeResponse{}, "unknown", "", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown corpus mode") {
		t.Fatalf("unknown mode error = %v, want unknown corpus mode", err)
	}
}

func TestBuildAccuracyAuditOutputListsMissingEvidence(t *testing.T) {
	output, err := buildAccuracyAuditOutput(tokenverifier.ProbeAccuracyAuditOptions{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("build audit output: %v", err)
	}
	text := string(output)
	for _, want := range []string{
		`"passed": false`,
		"labeled_probe_corpus_real.json",
		"identity_assessment_corpus_real.json",
		"informational_probe_corpus_real.json",
		"judge_config",
		"probe_zh_reasoning:baseline:zh_reasoning",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("audit output = %s, want %q", text, want)
		}
	}
}

func TestBuildAccuracyAuditResultExposesPassedFlagForRequireMode(t *testing.T) {
	output, report, err := buildAccuracyAuditResult(tokenverifier.ProbeAccuracyAuditOptions{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("build audit result: %v", err)
	}
	if report.Passed {
		t.Fatalf("report passed with missing evidence: %+v", report)
	}
	if !strings.Contains(string(output), `"passed": false`) {
		t.Fatalf("output = %s, want failed audit JSON", string(output))
	}
}

func TestBuildAccuracyCoverageOutputListsFullSuiteRequirements(t *testing.T) {
	output, err := buildAccuracyCoverageOutput(tokenverifier.ProbeAccuracyAuditOptions{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("build coverage output: %v", err)
	}
	text := string(output)
	for _, want := range []string{
		`"probe_profile": "full"`,
		`"coverage_count":`,
		"probe_instruction_follow",
		"pass_fail_real_corpus",
		"manual_review.status=reviewed",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("coverage output = %s, want %q", text, want)
		}
	}
	if strings.Contains(text, `"missing"`) {
		t.Fatalf("coverage output = %s, want coverage requirements without current missing evidence state", text)
	}
}

func TestBuildAccuracyGapsOutputSummarizesActionableMissingEvidence(t *testing.T) {
	output, err := buildAccuracyGapsOutput(tokenverifier.ProbeAccuracyAuditOptions{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("build gaps output: %v", err)
	}
	text := string(output)
	for _, want := range []string{
		`"passed": false`,
		`"missing_corpus_files"`,
		`"pass_fail_real_corpus"`,
		`"target_check_keys"`,
		`"target_check_keys_csv"`,
		`"target_check_keys_by_audit_path"`,
		`"probe_instruction_follow"`,
		`"identity_real_corpus"`,
		`"review_only_missing"`,
		`"review_only_judge:judge_config"`,
		`"review_only_judge:baseline_config"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("gaps output = %s, want %q", text, want)
		}
	}
}

func TestBuildAccuracyGapsOutputTargetsOnlyMissingCoverageCheckKeys(t *testing.T) {
	report := tokenverifier.ProbeAccuracyAuditReport{
		Coverage: []tokenverifier.ProbeAccuracyCoverageItem{
			{CheckKey: tokenverifier.CheckProbeInstructionFollow, AuditPath: "pass_fail_real_corpus"},
			{CheckKey: tokenverifier.CheckProbeMathLogic, AuditPath: "pass_fail_real_corpus"},
			{CheckKey: tokenverifier.CheckProbeChannelSignature, AuditPath: "informational_real_corpus"},
			{CheckKey: tokenverifier.CheckProbeIdentitySelfKnowledge, AuditPath: "identity_real_corpus"},
			{CheckKey: tokenverifier.CheckProbeIdentityListFormat, AuditPath: "identity_real_corpus"},
		},
		EvidenceFiles: []tokenverifier.ProbeAccuracyEvidenceFile{
			{
				Kind:            "pass_fail_real_corpus",
				Present:         true,
				MissingCoverage: []string{"probe_math_logic:negative"},
			},
			{
				Kind:            "identity_real_corpus",
				Present:         true,
				MissingCoverage: []string{"status:mismatch"},
			},
			{
				Kind:    "informational_real_corpus",
				Present: true,
			},
		},
	}

	keys := targetCheckKeysFromReport(report)
	csv := targetCheckKeysCSV(keys)
	want := []tokenverifier.CheckKey{
		tokenverifier.CheckProbeMathLogic,
		tokenverifier.CheckProbeIdentitySelfKnowledge,
		tokenverifier.CheckProbeIdentityListFormat,
	}
	if strings.Join(checkKeysToStrings(keys), ",") != strings.Join(checkKeysToStrings(want), ",") {
		t.Fatalf("target keys = %v, want %v", keys, want)
	}
	if csv != "probe_math_logic,probe_identity_self_knowledge,probe_identity_list_format" {
		t.Fatalf("target csv = %q, want stable comma-separated check keys", csv)
	}
	byPath := targetCheckKeysByAuditPath(report)
	if got := targetCheckKeysCSV(byPath["pass_fail_real_corpus"]); got != "probe_math_logic" {
		t.Fatalf("pass/fail target csv = %q, want missing pass/fail check only", got)
	}
	if got := targetCheckKeysCSV(byPath["identity_real_corpus"]); got != "probe_identity_self_knowledge,probe_identity_list_format" {
		t.Fatalf("identity target csv = %q, want identity checks only", got)
	}
	if got := targetCheckKeysCSV(byPath["informational_real_corpus"]); got != "" {
		t.Fatalf("informational target csv = %q, want no completed informational checks", got)
	}
}

func TestBuildDirectProbeRunsTemplateOutputUsesGapsAuditPathsWithoutSecrets(t *testing.T) {
	report := tokenverifier.ProbeAccuracyAuditReport{
		Coverage: []tokenverifier.ProbeAccuracyCoverageItem{
			{CheckKey: tokenverifier.CheckProbeMathLogic, AuditPath: "pass_fail_real_corpus"},
			{CheckKey: tokenverifier.CheckProbeIdentitySelfKnowledge, AuditPath: "identity_real_corpus"},
		},
		EvidenceFiles: []tokenverifier.ProbeAccuracyEvidenceFile{
			{Kind: "pass_fail_real_corpus", Present: false},
			{Kind: "identity_real_corpus", Present: false},
		},
	}

	output, err := buildDirectProbeRunsTemplateOutputFromReport(report, directProbeRunsTemplateOptions{
		OutputDir: "/tmp/token-verification-evidence-targeted-runs",
	})
	if err != nil {
		t.Fatalf("build runs template: %v", err)
	}
	text := string(output)
	for _, want := range []string{
		`"runs"`,
		`"provider": "anthropic"`,
		`"model": "claude-opus-4-7"`,
		`"api_key_env": "ANTHROPIC_CAPTURE_KEY"`,
		`"base_url_env": "ANTHROPIC_CAPTURE_BASE_URL"`,
		`"gaps_audit_path": "pass_fail_real_corpus"`,
		`"provider": "openai"`,
		`"model": "gpt-5.5"`,
		`"api_key_env": "OPENAI_CAPTURE_KEY"`,
		`"base_url_env": "OPENAI_CAPTURE_BASE_URL"`,
		`"gaps_audit_path": "identity_real_corpus"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("runs template = %s, want %q", text, want)
		}
	}
	for _, forbidden := range []string{"sk-", "test-key", "api_key\":", "base_url\": \"https://"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("runs template = %s, must not contain secret-like value %q", text, forbidden)
		}
	}
}

func TestBuildDirectProbeRunsTemplateOutputReturnsEmptyWhenNoCorpusTargets(t *testing.T) {
	output, err := buildDirectProbeRunsTemplateOutputFromReport(tokenverifier.ProbeAccuracyAuditReport{}, directProbeRunsTemplateOptions{})
	if err != nil {
		t.Fatalf("build empty runs template: %v", err)
	}
	if !strings.Contains(string(output), `"runs": []`) {
		t.Fatalf("runs template = %s, want empty runs", string(output))
	}
}

func TestBuildAccuracyAuditOutputIncludesEvidenceGapDetails(t *testing.T) {
	baseDir := t.TempDir()
	for _, rel := range []string{
		"service/token_verifier/testdata/labeled_probe_corpus_real.json",
		"service/token_verifier/testdata/identity_assessment_corpus_real.json",
		"service/token_verifier/testdata/informational_probe_corpus_real.json",
	} {
		path := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir corpus dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(`{"manual_review":{"status":"draft","source":"detector_generated_draft"},"cases":[]}`), 0o600); err != nil {
			t.Fatalf("write corpus fixture: %v", err)
		}
	}
	baseline := tokenverifier.BaselineMap{}
	for _, probeID := range []string{"zh_reasoning", "code_gen", "en_reasoning"} {
		baseline[probeID] = "baseline response"
	}

	output, report, err := buildAccuracyAuditResult(tokenverifier.ProbeAccuracyAuditOptions{
		BaseDir:       baseDir,
		JudgeConfig:   &tokenverifier.ProbeJudgeConfig{BaseURL: "http://judge.invalid", APIKey: "test-key", ModelID: "judge-model"},
		ProbeBaseline: baseline,
	})
	if err != nil {
		t.Fatalf("build audit result: %v", err)
	}
	if report.Passed {
		t.Fatalf("report passed invalid corpus fixtures: %+v", report)
	}
	text := string(output)
	for _, want := range []string{`"case_count": 0`, `"manual_review_missing"`, `"missing_coverage"`, `"invalid"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("audit output = %s, want %q", text, want)
		}
	}
}

func TestBuildAccuracyAuditOptionsLoadsProbeBaselineDraft(t *testing.T) {
	baselinePath := filepath.Join(t.TempDir(), "probe_baseline.draft.json")
	if err := os.WriteFile(baselinePath, []byte(`{
		"probes": [
			{"probeId": "zh_reasoning", "responseText": "zh baseline"},
			{"probeId": "code_gen", "responseText": "code baseline"},
			{"probeId": "en_reasoning", "responseText": "en baseline"}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write baseline fixture: %v", err)
	}

	baseDir := t.TempDir()
	options, err := buildAccuracyAuditOptions(accuracyAuditFlagValues{BaseDir: baseDir, BaselinePath: baselinePath})
	if err != nil {
		t.Fatalf("build audit options: %v", err)
	}
	if options.BaseDir != baseDir {
		t.Fatalf("base dir = %q, want %q", options.BaseDir, baseDir)
	}
	for _, probeID := range []string{"zh_reasoning", "code_gen", "en_reasoning"} {
		if options.ProbeBaseline[probeID] == "" {
			t.Fatalf("baseline[%s] empty in %+v", probeID, options.ProbeBaseline)
		}
	}

	_, report, err := buildAccuracyAuditResult(options)
	if err != nil {
		t.Fatalf("build audit result: %v", err)
	}
	if report.ReviewOnlyJudge.BaselinePresent != true {
		t.Fatalf("baseline not present in review-only audit: %+v", report.ReviewOnlyJudge)
	}
	for _, missing := range report.ReviewOnlyJudge.Missing {
		if strings.Contains(missing, "baseline") {
			t.Fatalf("review-only audit missing baseline despite -baseline: %+v", report.ReviewOnlyJudge)
		}
	}
}

func TestBuildAccuracyAuditOptionsLoadsJudgeConfigFromNamedEnv(t *testing.T) {
	t.Setenv("TEST_TOKEN_VERIFIER_JUDGE_KEY", "test-key")

	options, err := buildAccuracyAuditOptions(accuracyAuditFlagValues{
		JudgeBaseURL:   "https://judge.invalid",
		JudgeModel:     "judge-model",
		JudgeAPIKeyEnv: "TEST_TOKEN_VERIFIER_JUDGE_KEY",
		JudgeThreshold: 8,
	})
	if err != nil {
		t.Fatalf("build audit options: %v", err)
	}
	if options.JudgeConfig == nil {
		t.Fatalf("judge config missing")
	}
	if options.JudgeConfig.APIKey != "test-key" || options.JudgeConfig.ModelID != "judge-model" || options.JudgeConfig.Threshold != 8 {
		t.Fatalf("judge config = %+v, want values from flags/env", options.JudgeConfig)
	}
}

func TestBuildAccuracyAuditOptionsRejectsMissingJudgeEnvSecret(t *testing.T) {
	_, err := buildAccuracyAuditOptions(accuracyAuditFlagValues{
		JudgeBaseURL:   "https://judge.invalid",
		JudgeModel:     "judge-model",
		JudgeAPIKeyEnv: "TEST_TOKEN_VERIFIER_MISSING_JUDGE_KEY",
	})
	if err == nil || !strings.Contains(err.Error(), "TEST_TOKEN_VERIFIER_MISSING_JUDGE_KEY") {
		t.Fatalf("error = %v, want missing judge env secret error", err)
	}
}

func TestBuildMergedLabeledProbeCorpusOutputRequiresReviewedInputs(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "base.json")
	incomingPath := filepath.Join(t.TempDir(), "incoming.json")
	if err := os.WriteFile(basePath, []byte(reviewedLabeledCorpusFixture("base", tokenverifier.CheckProbeInstructionFollow)), 0o600); err != nil {
		t.Fatalf("write base fixture: %v", err)
	}
	if err := os.WriteFile(incomingPath, []byte(`{
		"description": "draft",
		"manual_review": {"status":"draft","source":"detector_generated_draft"},
		"cases": []
	}`), 0o600); err != nil {
		t.Fatalf("write incoming fixture: %v", err)
	}

	_, err := buildMergedLabeledProbeCorpusOutput([]string{basePath, incomingPath}, "merged")
	if err == nil || !strings.Contains(err.Error(), "incoming.json") || !strings.Contains(err.Error(), "manual_review") {
		t.Fatalf("error = %v, want reviewed input validation error", err)
	}
}

func TestBuildMergedLabeledProbeCorpusOutputDeduplicatesCases(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	incomingPath := filepath.Join(dir, "incoming.json")
	if err := os.WriteFile(basePath, []byte(reviewedLabeledCorpusFixture("shared", tokenverifier.CheckProbeInstructionFollow)), 0o600); err != nil {
		t.Fatalf("write base fixture: %v", err)
	}
	if err := os.WriteFile(incomingPath, []byte(reviewedLabeledCorpusFixture("shared", tokenverifier.CheckProbeMathLogic)), 0o600); err != nil {
		t.Fatalf("write incoming fixture: %v", err)
	}

	output, err := buildMergedLabeledProbeCorpusOutput([]string{basePath, incomingPath}, "merged")
	if err != nil {
		t.Fatalf("merge labeled corpus: %v", err)
	}
	var merged tokenverifier.LabeledProbeCorpus
	if err := json.Unmarshal(output, &merged); err != nil {
		t.Fatalf("parse merged output: %v", err)
	}
	if merged.Description != "merged" {
		t.Fatalf("description = %q, want merge description", merged.Description)
	}
	if merged.ManualReview.Status != "reviewed" || merged.ManualReview.Source != "real_model_output" {
		t.Fatalf("manual review = %+v, want reviewed real output", merged.ManualReview)
	}
	if len(merged.Cases) != 1 {
		t.Fatalf("case count = %d, want duplicate case name merged once", len(merged.Cases))
	}
}

func TestBuildCleanMergedLabeledProbeCorpusOutputDropsMismatchedCases(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.json")
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(goodPath, []byte(reviewedLabeledCorpusFixture("good", tokenverifier.CheckProbeInstructionFollow)), 0o600); err != nil {
		t.Fatalf("write good fixture: %v", err)
	}
	if err := os.WriteFile(badPath, []byte(`{
		"description": "reviewed",
		"manual_review": {"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"},
		"cases": [
			{
				"name": "bad",
				"source": {"provider":"openai","model":"gpt-test","task_id":"task-1","captured_at":"2026-05-06T00:00:00Z"},
				"check_key": "probe_instruction_follow",
				"response_text": "this omits required languages",
				"want_passed": true
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write bad fixture: %v", err)
	}

	output, err := buildCleanMergedLabeledProbeCorpusOutput([]string{goodPath, badPath}, "clean merged")
	if err != nil {
		t.Fatalf("clean merge labeled corpus: %v", err)
	}
	var merged tokenverifier.LabeledProbeCorpus
	if err := json.Unmarshal(output, &merged); err != nil {
		t.Fatalf("parse merged output: %v", err)
	}
	if len(merged.Cases) != 1 || merged.Cases[0].Name != "good" {
		t.Fatalf("clean merged cases = %+v, want only stable good case", merged.Cases)
	}
	if merged.ManualReview.Status != "reviewed" || merged.ManualReview.Source != "real_model_output" {
		t.Fatalf("manual review = %+v, want reviewed real output", merged.ManualReview)
	}
}

func TestBuildCleanMergedLabeledProbeCorpusOutputFailsWhenNoStableCases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{
		"description": "reviewed",
		"manual_review": {"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"},
		"cases": [
			{
				"name": "bad",
				"source": {"provider":"openai","model":"gpt-test","task_id":"task-1","captured_at":"2026-05-06T00:00:00Z"},
				"check_key": "probe_instruction_follow",
				"response_text": "this omits required languages",
				"want_passed": true
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write bad fixture: %v", err)
	}

	_, err := buildCleanMergedLabeledProbeCorpusOutput([]string{path}, "clean merged")
	if err == nil || !strings.Contains(err.Error(), "no stable") {
		t.Fatalf("error = %v, want no stable cases error", err)
	}
}

func TestBuildMergedIdentityAssessmentCorpusOutputDeduplicatesCases(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	incomingPath := filepath.Join(dir, "incoming.json")
	if err := os.WriteFile(basePath, []byte(reviewedIdentityCorpusFixture("shared")), 0o600); err != nil {
		t.Fatalf("write base fixture: %v", err)
	}
	if err := os.WriteFile(incomingPath, []byte(reviewedIdentityCorpusFixture("shared")), 0o600); err != nil {
		t.Fatalf("write incoming fixture: %v", err)
	}

	output, err := buildMergedIdentityAssessmentCorpusOutput([]string{basePath, incomingPath}, "merged identity")
	if err != nil {
		t.Fatalf("merge identity corpus: %v", err)
	}
	var merged tokenverifier.IdentityAssessmentCorpus
	if err := json.Unmarshal(output, &merged); err != nil {
		t.Fatalf("parse merged output: %v", err)
	}
	if merged.Description != "merged identity" {
		t.Fatalf("description = %q, want merge description", merged.Description)
	}
	if len(merged.Cases) != 1 {
		t.Fatalf("case count = %d, want duplicate identity case once", len(merged.Cases))
	}
}

func TestBuildMergedInformationalProbeCorpusOutputRequiresReviewedInputs(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "base.json")
	incomingPath := filepath.Join(t.TempDir(), "incoming.json")
	if err := os.WriteFile(basePath, []byte(reviewedInformationalCorpusFixture("base")), 0o600); err != nil {
		t.Fatalf("write base fixture: %v", err)
	}
	if err := os.WriteFile(incomingPath, []byte(`{
		"description": "draft",
		"manual_review": {"status":"draft","source":"detector_generated_draft"},
		"cases": []
	}`), 0o600); err != nil {
		t.Fatalf("write incoming fixture: %v", err)
	}

	_, err := buildMergedInformationalProbeCorpusOutput([]string{basePath, incomingPath}, "merged info")
	if err == nil || !strings.Contains(err.Error(), "incoming.json") || !strings.Contains(err.Error(), "manual_review") {
		t.Fatalf("error = %v, want reviewed input validation error", err)
	}
}

func reviewedLabeledCorpusFixture(caseName string, checkKey tokenverifier.CheckKey) string {
	return fmt.Sprintf(`{
		"description": "reviewed",
		"manual_review": {"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"},
		"cases": [
			{
				"name": %q,
				"source": {"provider":"openai","model":"gpt-test","task_id":"task-1","captured_at":"2026-05-06T00:00:00Z"},
				"check_key": %q,
				"response_text": "Fortran\nLisp\nCOBOL\nBASIC\nC",
				"want_passed": true
			}
		]
	}`, caseName, checkKey)
}

func reviewedIdentityCorpusFixture(caseName string) string {
	return fmt.Sprintf(`{
		"description": "reviewed identity",
		"manual_review": {"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"},
		"cases": [
			{
				"name": %q,
				"source": {"provider":"openai","model":"gpt-test","task_id":"task-1","captured_at":"2026-05-06T00:00:00Z"},
				"results": [
					{"provider":"openai","group":"identity","check_key":"probe_identity_self_knowledge","model_name":"gpt-test","neutral":true,"success":true,"private_response_text":"I am ChatGPT by OpenAI."}
				],
				"want_identity_status": "match",
				"want_verdict_status": "plain_match",
				"want_predicted_family": "openai",
				"want_predicted_model": "gpt-test"
			}
		]
	}`, caseName)
}

func reviewedInformationalCorpusFixture(caseName string) string {
	return fmt.Sprintf(`{
		"description": "reviewed informational",
		"manual_review": {"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"},
		"cases": [
			{
				"name": %q,
				"source": {"provider":"anthropic","model":"claude-test","task_id":"task-1","captured_at":"2026-05-06T00:00:00Z"},
				"check_key": "probe_channel_signature",
				"headers": {"x-accel-buffering":"no"},
				"want_channel": "direct",
				"want_passed": true
			}
		]
	}`, caseName)
}

func TestSplitSecretsTrimsEmptyValues(t *testing.T) {
	secrets := splitSecrets(" alpha, ,beta ,, ")
	if len(secrets) != 2 || secrets[0] != "alpha" || secrets[1] != "beta" {
		t.Fatalf("secrets = %#v, want trimmed non-empty values", secrets)
	}
}
