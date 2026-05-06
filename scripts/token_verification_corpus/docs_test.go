package main

import (
	"os"
	"strings"
	"testing"
)

func TestAccuracyAuditRunbookDocumentsHardGate(t *testing.T) {
	data, err := os.ReadFile("../../docs/token-verification-accuracy-audit.zh-CN.md")
	if err != nil {
		t.Fatalf("read accuracy audit runbook: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"go run ./scripts/token_verification_corpus -mode audit -require",
		"-mode summary",
		"source_ready",
		"missing_source",
		"source_task_id",
		"labeled_probe_corpus_real.json",
		"identity_assessment_corpus_real.json",
		"informational_probe_corpus_real.json",
		"TOKEN_VERIFICATION_REQUIRE_REAL_CORPUS=1",
		"manual_review",
		"case_source_missing",
		"source.task_id",
		"captured_at",
		"-source-task-id",
		"-captured-at",
		"RFC3339",
		"status",
		"reviewed",
		"real_model_output",
		"runtime `full`",
		"pass_fail_real_corpus`：28",
		"可重放真实样本",
		"schema golden corpus",
		"identity_real_corpus`：6",
		"当前 runtime `full` 没有 `review_only_judge` 默认项",
		"legacy review-only 定向项仍可显式运行",
		"target_check_keys_csv",
		"go run ./scripts/token_verification_direct_probe",
		"HERMESTOKEN_PROBE_API_KEY",
		"HERMESTOKEN_PROBE_MODELS",
		"HERMESTOKEN_PROBE_OUTPUT_DIR",
		"HERMESTOKEN_PROBE_RUNS_FILE",
		"api_key_env",
		"base_url_env",
		"gaps_audit_path",
		"check_keys_csv",
		"-mode runs-template",
		"HERMESTOKEN_PROBE_RUNS_FILE=\"/tmp/token-verification-runs.json\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("runbook missing %q", want)
		}
	}
}
