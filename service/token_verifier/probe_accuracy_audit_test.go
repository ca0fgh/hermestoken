package token_verifier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildProbeAccuracyAuditReportListsMissingEvidence(t *testing.T) {
	baseDir := t.TempDir()
	report := BuildProbeAccuracyAuditReport(ProbeAccuracyAuditOptions{BaseDir: baseDir})

	if report.Passed {
		t.Fatalf("audit report passed with no real evidence: %+v", report)
	}
	for _, want := range []string{
		"labeled_probe_corpus_real.json",
		"identity_assessment_corpus_real.json",
		"informational_probe_corpus_real.json",
		"judge_config",
		"baseline_config",
		"probe_zh_reasoning:baseline:zh_reasoning",
		"probe_code_generation:baseline:code_gen",
		"probe_en_reasoning:baseline:en_reasoning",
	} {
		if !auditMissingContains(report.Missing, want) {
			t.Fatalf("missing = %#v, want entry containing %q", report.Missing, want)
		}
	}
	if len(report.Coverage) != len(probeSuiteDefinitions(ProbeProfileFull)) {
		t.Fatalf("coverage count = %d, want full suite count %d", len(report.Coverage), len(probeSuiteDefinitions(ProbeProfileFull)))
	}
	assertAuditPath(t, report, CheckProbeInstructionFollow, "pass_fail_real_corpus")
	assertAuditPath(t, report, CheckProbeZHReasoning, "review_only_judge")
	assertAuditPath(t, report, CheckProbeChannelSignature, "informational_real_corpus")
	assertAuditPath(t, report, CheckProbeIdentitySelfKnowledge, "identity_real_corpus")
	assertAuditRequirement(t, report, CheckProbeInstructionFollow, "one positive and one negative")
	assertAuditRequirement(t, report, CheckProbeZHReasoning, "probe_id=zh_reasoning")
	assertAuditRequirement(t, report, CheckProbeIdentitySelfKnowledge, "identity corpus")
}

func TestBuildProbeAccuracyAuditReportRejectsPlaceholderCorpusFiles(t *testing.T) {
	baseDir := t.TempDir()
	for _, rel := range []string{
		defaultLabeledProbeRealCorpusPath,
		defaultIdentityAssessmentRealCorpusPath,
		defaultInformationalProbeRealCorpusPath,
	} {
		path := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir corpus dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(`{"cases":[{}]}`), 0o600); err != nil {
			t.Fatalf("write placeholder corpus: %v", err)
		}
	}
	baseline := BaselineMap{}
	for _, probeID := range reviewOnlyProbeJudgeCoverageRequirements() {
		baseline[probeID] = "baseline response"
	}

	report := BuildProbeAccuracyAuditReport(ProbeAccuracyAuditOptions{
		BaseDir:       baseDir,
		JudgeConfig:   &ProbeJudgeConfig{BaseURL: "http://judge.invalid", APIKey: "test-key", ModelID: "judge-model"},
		ProbeBaseline: baseline,
	})

	if report.Passed {
		t.Fatalf("audit report passed placeholder corpora: %+v", report)
	}
	for _, want := range []string{
		"pass_fail_real_corpus:invalid",
		"identity_real_corpus:invalid",
		"informational_real_corpus:invalid",
	} {
		if !auditMissingContains(report.Missing, want) {
			t.Fatalf("missing = %#v, want entry containing %q", report.Missing, want)
		}
	}
	passFailEvidence := auditEvidenceFile(t, report, auditPathPassFailRealCorpus)
	if passFailEvidence.CaseCount == nil || *passFailEvidence.CaseCount != 1 || len(passFailEvidence.Invalid) == 0 {
		t.Fatalf("pass/fail evidence detail = %+v, want one invalid placeholder case", passFailEvidence)
	}
}

func TestBuildProbeAccuracyAuditReportRejectsDraftCorpusMetadata(t *testing.T) {
	baseDir := t.TempDir()
	writeAuditFixtureFile(t, baseDir, defaultLabeledProbeRealCorpusPath, `{
		"manual_review": {"status":"draft","source":"detector_generated_draft"},
		"cases": []
	}`)
	writeAuditFixtureFile(t, baseDir, defaultIdentityAssessmentRealCorpusPath, `{
		"manual_review": {"status":"draft","source":"detector_generated_draft"},
		"cases": []
	}`)
	writeAuditFixtureFile(t, baseDir, defaultInformationalProbeRealCorpusPath, `{
		"manual_review": {"status":"draft","source":"detector_generated_draft"},
		"cases": []
	}`)
	baseline := BaselineMap{}
	for _, probeID := range reviewOnlyProbeJudgeCoverageRequirements() {
		baseline[probeID] = "baseline response"
	}

	report := BuildProbeAccuracyAuditReport(ProbeAccuracyAuditOptions{
		BaseDir:       baseDir,
		JudgeConfig:   &ProbeJudgeConfig{BaseURL: "http://judge.invalid", APIKey: "test-key", ModelID: "judge-model"},
		ProbeBaseline: baseline,
	})

	if report.Passed {
		t.Fatalf("audit report passed draft corpus metadata: %+v", report)
	}
	for _, want := range []string{
		"pass_fail_real_corpus:manual_review:status",
		"identity_real_corpus:manual_review:source",
		"informational_real_corpus:manual_review:reviewed_by",
		"informational_real_corpus:manual_review:reviewed_at",
	} {
		if !auditMissingContains(report.Missing, want) {
			t.Fatalf("missing = %#v, want entry containing %q", report.Missing, want)
		}
	}
	passFailEvidence := auditEvidenceFile(t, report, auditPathPassFailRealCorpus)
	if !containsString(passFailEvidence.ManualReviewMissing, "status") || !containsString(passFailEvidence.ManualReviewMissing, "source") {
		t.Fatalf("manual review detail = %+v, want status/source gaps", passFailEvidence.ManualReviewMissing)
	}
	if passFailEvidence.CaseCount == nil || *passFailEvidence.CaseCount != 0 || len(passFailEvidence.MissingCoverage) == 0 {
		t.Fatalf("pass/fail evidence detail = %+v, want empty corpus coverage gaps", passFailEvidence)
	}
}

func TestBuildProbeAccuracyAuditReportRejectsMissingCaseSourceMetadata(t *testing.T) {
	baseDir := t.TempDir()
	review := `"manual_review":{"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"}`
	writeAuditFixtureFile(t, baseDir, defaultLabeledProbeRealCorpusPath, `{
		`+review+`,
		"cases": [
			{"name":"case_without_source","check_key":"probe_instruction_follow","response_text":"Fortran\nLisp\nCOBOL\nBASIC\nC","want_passed":true}
		]
	}`)
	writeAuditFixtureFile(t, baseDir, defaultIdentityAssessmentRealCorpusPath, `{
		`+review+`,
		"cases": [
			{
				"name":"identity_without_source",
				"results":[
					{"provider":"openai","group":"identity","check_key":"probe_identity_self_knowledge","model_name":"gpt-5.5","neutral":true,"success":true,"private_response_text":"I am ChatGPT, a model created by OpenAI."}
				],
				"want_identity_status":"match"
			}
		]
	}`)
	writeAuditFixtureFile(t, baseDir, defaultInformationalProbeRealCorpusPath, `{
		`+review+`,
		"cases": [
			{"name":"info_without_source","check_key":"probe_channel_signature","headers":{"x-generation-id":"gen-test"},"message_id":"gen-test","raw_body":"{\"id\":\"gen-test\"}","want_channel":"openrouter","want_passed":true}
		]
	}`)
	baseline := BaselineMap{}
	for _, probeID := range reviewOnlyProbeJudgeCoverageRequirements() {
		baseline[probeID] = "baseline response"
	}

	report := BuildProbeAccuracyAuditReport(ProbeAccuracyAuditOptions{
		BaseDir:       baseDir,
		JudgeConfig:   &ProbeJudgeConfig{BaseURL: "http://judge.invalid", APIKey: "test-key", ModelID: "judge-model"},
		ProbeBaseline: baseline,
	})

	for _, want := range []string{
		"pass_fail_real_corpus:case_source:case_without_source:provider",
		"pass_fail_real_corpus:case_source:case_without_source:task_id",
		"identity_real_corpus:case_source:identity_without_source:model",
		"identity_real_corpus:case_source:identity_without_source:task_id",
		"informational_real_corpus:case_source:info_without_source:captured_at",
		"informational_real_corpus:case_source:info_without_source:task_id",
	} {
		if !auditMissingContains(report.Missing, want) {
			t.Fatalf("missing = %#v, want entry containing %q", report.Missing, want)
		}
	}
	passFailEvidence := auditEvidenceFile(t, report, auditPathPassFailRealCorpus)
	if !auditMissingContains(passFailEvidence.CaseSourceMissing, "case_without_source:provider") {
		t.Fatalf("case source detail = %+v, want provider gap", passFailEvidence.CaseSourceMissing)
	}
	if !auditMissingContains(passFailEvidence.CaseSourceMissing, "case_without_source:task_id") {
		t.Fatalf("case source detail = %+v, want task_id gap", passFailEvidence.CaseSourceMissing)
	}
}

func TestBuildProbeAccuracyAuditReportContinuesCoverageAfterCaseInvalid(t *testing.T) {
	baseDir := t.TempDir()
	review := `"manual_review":{"status":"reviewed","source":"real_model_output","reviewed_by":"tester","reviewed_at":"2026-05-06T00:00:00Z"}`
	source := `"source":{"provider":"openai","model":"gpt-test","task_id":"task-1","captured_at":"2026-05-06T00:00:00Z"}`
	writeAuditFixtureFile(t, baseDir, defaultLabeledProbeRealCorpusPath, `{
		`+review+`,
		"cases": [
			{"name":"invalid_error_code","check_key":"probe_instruction_follow",`+source+`,"response_text":"Fortran\nLisp\nCOBOL\nBASIC\nC","want_passed":true,"want_error_code":"wrong_error"}
		]
	}`)
	writeAuditFixtureFile(t, baseDir, defaultIdentityAssessmentRealCorpusPath, `{
		`+review+`,
		"cases": []
	}`)
	writeAuditFixtureFile(t, baseDir, defaultInformationalProbeRealCorpusPath, `{
		`+review+`,
		"cases": []
	}`)
	baseline := BaselineMap{}
	for _, probeID := range reviewOnlyProbeJudgeCoverageRequirements() {
		baseline[probeID] = "baseline response"
	}

	report := BuildProbeAccuracyAuditReport(ProbeAccuracyAuditOptions{
		BaseDir:       baseDir,
		JudgeConfig:   &ProbeJudgeConfig{BaseURL: "http://judge.invalid", APIKey: "test-key", ModelID: "judge-model"},
		ProbeBaseline: baseline,
	})
	passFailEvidence := auditEvidenceFile(t, report, auditPathPassFailRealCorpus)
	if !auditMissingContains(passFailEvidence.Invalid, "error_code mismatch") {
		t.Fatalf("invalid detail = %+v, want error_code mismatch", passFailEvidence.Invalid)
	}
	if len(passFailEvidence.MissingCoverage) == 0 {
		t.Fatalf("pass/fail evidence detail = %+v, want coverage gaps even when one case is invalid", passFailEvidence)
	}
}

func writeAuditFixtureFile(t *testing.T, baseDir string, relPath string, content string) {
	t.Helper()
	path := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir audit fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write audit fixture: %v", err)
	}
}

func assertAuditPath(t *testing.T, report ProbeAccuracyAuditReport, checkKey CheckKey, want string) {
	t.Helper()
	for _, item := range report.Coverage {
		if item.CheckKey == checkKey {
			if item.AuditPath != want {
				t.Fatalf("%s audit path = %q, want %q", checkKey, item.AuditPath, want)
			}
			if item.MissingReason != "" {
				t.Fatalf("%s missing reason = %q, want empty", checkKey, item.MissingReason)
			}
			return
		}
	}
	t.Fatalf("coverage missing check %s", checkKey)
}

func assertAuditRequirement(t *testing.T, report ProbeAccuracyAuditReport, checkKey CheckKey, want string) {
	t.Helper()
	for _, item := range report.Coverage {
		if item.CheckKey == checkKey {
			if !strings.Contains(item.EvidenceRequirement, want) {
				t.Fatalf("%s evidence requirement = %q, want containing %q", checkKey, item.EvidenceRequirement, want)
			}
			return
		}
	}
	t.Fatalf("coverage missing check %s", checkKey)
}

func auditEvidenceFile(t *testing.T, report ProbeAccuracyAuditReport, kind string) ProbeAccuracyEvidenceFile {
	t.Helper()
	for _, file := range report.EvidenceFiles {
		if file.Kind == kind {
			return file
		}
	}
	t.Fatalf("evidence file %s missing from report", kind)
	return ProbeAccuracyEvidenceFile{}
}

func auditMissingContains(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
