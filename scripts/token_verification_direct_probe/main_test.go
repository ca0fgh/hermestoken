package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
)

func TestDirectProbeConfigFromEnvParsesTargetedCheckKeys(t *testing.T) {
	t.Setenv("HERMESTOKEN_PROBE_API_KEY", "test-key")
	t.Setenv("HERMESTOKEN_PROBE_BASE_URL", "https://probe.invalid")
	t.Setenv("HERMESTOKEN_PROBE_MODEL", "claude-test")
	t.Setenv("HERMESTOKEN_PROBE_PROFILE", tokenverifier.ProbeProfileFull)
	t.Setenv("HERMESTOKEN_PROBE_OUTPUT", "/tmp/probe.json")
	t.Setenv("HERMESTOKEN_PROBE_CHECK_KEYS", " probe_math_logic,probe_json_output ,, ")

	config, err := directProbeConfigFromEnv()
	if err != nil {
		t.Fatalf("directProbeConfigFromEnv returned error: %v", err)
	}
	if config.APIKey != "test-key" {
		t.Fatalf("api key = %q, want env value", config.APIKey)
	}
	if config.OutputPath != "/tmp/probe.json" {
		t.Fatalf("output path = %q, want env value", config.OutputPath)
	}
	want := []tokenverifier.CheckKey{tokenverifier.CheckProbeMathLogic, tokenverifier.CheckProbeJSONOutput}
	if !reflect.DeepEqual(config.Request.CheckKeys, want) {
		t.Fatalf("check keys = %#v, want %#v", config.Request.CheckKeys, want)
	}
}

func TestDirectProbeConfigFromEnvRejectsMissingAPIKey(t *testing.T) {
	_, err := directProbeConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "HERMESTOKEN_PROBE_API_KEY") {
		t.Fatalf("error = %v, want missing API key error", err)
	}
}

func TestDirectProbeConfigFromEnvRejectsMissingBaseURL(t *testing.T) {
	t.Setenv("HERMESTOKEN_PROBE_API_KEY", "test-key")

	_, err := directProbeConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "HERMESTOKEN_PROBE_BASE_URL") {
		t.Fatalf("error = %v, want missing base URL error", err)
	}
}

func TestDirectProbeConfigFromEnvLoadsCheckKeysFromGapsFile(t *testing.T) {
	gapsPath := filepath.Join(t.TempDir(), "gaps.json")
	if err := os.WriteFile(gapsPath, []byte(`{
		"target_check_keys_by_audit_path": {
			"pass_fail_real_corpus": {
				"csv": "probe_math_logic,probe_json_output"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write gaps fixture: %v", err)
	}
	t.Setenv("HERMESTOKEN_PROBE_API_KEY", "test-key")
	t.Setenv("HERMESTOKEN_PROBE_BASE_URL", "https://probe.invalid")
	t.Setenv("HERMESTOKEN_PROBE_GAPS_FILE", gapsPath)
	t.Setenv("HERMESTOKEN_PROBE_GAPS_AUDIT_PATH", "pass_fail_real_corpus")

	config, err := directProbeConfigFromEnv()
	if err != nil {
		t.Fatalf("directProbeConfigFromEnv returned error: %v", err)
	}
	want := []tokenverifier.CheckKey{tokenverifier.CheckProbeMathLogic, tokenverifier.CheckProbeJSONOutput}
	if !reflect.DeepEqual(config.Request.CheckKeys, want) {
		t.Fatalf("check keys = %#v, want %#v", config.Request.CheckKeys, want)
	}
}

func TestDirectProbeConfigFromEnvPrefersExplicitCheckKeysOverGapsFile(t *testing.T) {
	gapsPath := filepath.Join(t.TempDir(), "gaps.json")
	if err := os.WriteFile(gapsPath, []byte(`{
		"target_check_keys_by_audit_path": {
			"pass_fail_real_corpus": {
				"csv": "probe_math_logic"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write gaps fixture: %v", err)
	}
	t.Setenv("HERMESTOKEN_PROBE_API_KEY", "test-key")
	t.Setenv("HERMESTOKEN_PROBE_BASE_URL", "https://probe.invalid")
	t.Setenv("HERMESTOKEN_PROBE_CHECK_KEYS", "probe_json_output")
	t.Setenv("HERMESTOKEN_PROBE_GAPS_FILE", gapsPath)
	t.Setenv("HERMESTOKEN_PROBE_GAPS_AUDIT_PATH", "pass_fail_real_corpus")

	config, err := directProbeConfigFromEnv()
	if err != nil {
		t.Fatalf("directProbeConfigFromEnv returned error: %v", err)
	}
	want := []tokenverifier.CheckKey{tokenverifier.CheckProbeJSONOutput}
	if !reflect.DeepEqual(config.Request.CheckKeys, want) {
		t.Fatalf("check keys = %#v, want explicit env keys %#v", config.Request.CheckKeys, want)
	}
}

func TestDirectProbeConfigFromEnvBuildsRunsForModelList(t *testing.T) {
	outputDir := t.TempDir()
	t.Setenv("HERMESTOKEN_PROBE_API_KEY", "test-key")
	t.Setenv("HERMESTOKEN_PROBE_BASE_URL", "https://probe.invalid")
	t.Setenv("HERMESTOKEN_PROBE_MODELS", " claude-opus-4-7, claude/sonnet:4.6, claude-opus-4-7 ")
	t.Setenv("HERMESTOKEN_PROBE_OUTPUT_DIR", outputDir)
	t.Setenv("HERMESTOKEN_PROBE_CHECK_KEYS", "probe_math_logic")

	config, err := directProbeConfigFromEnv()
	if err != nil {
		t.Fatalf("directProbeConfigFromEnv returned error: %v", err)
	}
	if len(config.Runs) != 2 {
		t.Fatalf("run count = %d, want unique model runs: %+v", len(config.Runs), config.Runs)
	}
	if config.Runs[0].Request.Model != "claude-opus-4-7" || config.Runs[1].Request.Model != "claude/sonnet:4.6" {
		t.Fatalf("models = %q, %q", config.Runs[0].Request.Model, config.Runs[1].Request.Model)
	}
	if config.Runs[0].OutputPath != filepath.Join(outputDir, "01-anthropic-claude-opus-4-7.json") {
		t.Fatalf("first output path = %q", config.Runs[0].OutputPath)
	}
	if config.Runs[1].OutputPath != filepath.Join(outputDir, "02-anthropic-claude-sonnet-4.6.json") {
		t.Fatalf("second output path = %q", config.Runs[1].OutputPath)
	}
	wantKeys := []tokenverifier.CheckKey{tokenverifier.CheckProbeMathLogic}
	if !reflect.DeepEqual(config.Runs[0].Request.CheckKeys, wantKeys) || !reflect.DeepEqual(config.Runs[1].Request.CheckKeys, wantKeys) {
		t.Fatalf("run check keys = %#v / %#v, want %#v", config.Runs[0].Request.CheckKeys, config.Runs[1].Request.CheckKeys, wantKeys)
	}
}

func TestDirectProbeConfigFromEnvRejectsSingleOutputFileForModelList(t *testing.T) {
	t.Setenv("HERMESTOKEN_PROBE_API_KEY", "test-key")
	t.Setenv("HERMESTOKEN_PROBE_BASE_URL", "https://probe.invalid")
	t.Setenv("HERMESTOKEN_PROBE_MODELS", "claude-a,claude-b")
	t.Setenv("HERMESTOKEN_PROBE_OUTPUT", "/tmp/direct-probe.json")

	_, err := directProbeConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "HERMESTOKEN_PROBE_OUTPUT_DIR") {
		t.Fatalf("error = %v, want output dir error for multi-model run", err)
	}
}

func TestDirectProbeConfigFromEnvLoadsRunsFileWithSecretEnvNames(t *testing.T) {
	runsPath := filepath.Join(t.TempDir(), "runs.json")
	if err := os.WriteFile(runsPath, []byte(`{
		"runs": [
			{
				"provider": "anthropic",
				"model": "claude-opus-4-7",
				"api_key_env": "ANTHROPIC_CAPTURE_KEY",
				"base_url_env": "ANTHROPIC_CAPTURE_BASE_URL",
				"client_profile": "claude_code",
				"output_path": "/tmp/anthropic.json"
			},
			{
				"provider": "openai",
				"model": "gpt-5.5",
				"api_key_env": "OPENAI_CAPTURE_KEY",
				"base_url_env": "OPENAI_CAPTURE_BASE_URL",
				"output_path": "/tmp/openai.json"
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write runs fixture: %v", err)
	}
	t.Setenv("HERMESTOKEN_PROBE_RUNS_FILE", runsPath)
	t.Setenv("ANTHROPIC_CAPTURE_KEY", "anthropic-key")
	t.Setenv("ANTHROPIC_CAPTURE_BASE_URL", "https://anthropic.invalid")
	t.Setenv("OPENAI_CAPTURE_KEY", "openai-key")
	t.Setenv("OPENAI_CAPTURE_BASE_URL", "https://openai.invalid")
	t.Setenv("HERMESTOKEN_PROBE_PROFILE", tokenverifier.ProbeProfileFull)
	t.Setenv("HERMESTOKEN_PROBE_CHECK_KEYS", "probe_math_logic")

	config, err := directProbeConfigFromEnv()
	if err != nil {
		t.Fatalf("directProbeConfigFromEnv returned error: %v", err)
	}
	if config.APIKey != "" {
		t.Fatalf("config APIKey = %q, want empty top-level key for runs file", config.APIKey)
	}
	if len(config.Runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(config.Runs))
	}
	first := config.Runs[0]
	if first.Request.APIKey != "anthropic-key" || first.Request.BaseURL != "https://anthropic.invalid" {
		t.Fatalf("first request secret env resolution failed: %+v", first.Request)
	}
	if first.Request.Provider != tokenverifier.ProviderAnthropic || first.Request.Model != "claude-opus-4-7" || first.Request.ClientProfile != tokenverifier.ClientProfileClaudeCode {
		t.Fatalf("first request = %+v", first.Request)
	}
	second := config.Runs[1]
	if second.Request.APIKey != "openai-key" || second.Request.BaseURL != "https://openai.invalid" {
		t.Fatalf("second request secret env resolution failed: %+v", second.Request)
	}
	if second.Request.Provider != tokenverifier.ProviderOpenAI || second.Request.Model != "gpt-5.5" || second.Request.ClientProfile != "" {
		t.Fatalf("second request = %+v", second.Request)
	}
	wantKeys := []tokenverifier.CheckKey{tokenverifier.CheckProbeMathLogic}
	if !reflect.DeepEqual(first.Request.CheckKeys, wantKeys) || !reflect.DeepEqual(second.Request.CheckKeys, wantKeys) {
		t.Fatalf("check keys = %#v / %#v, want %#v", first.Request.CheckKeys, second.Request.CheckKeys, wantKeys)
	}
}

func TestDirectProbeConfigFromEnvRunsFileRejectsMissingSecretEnv(t *testing.T) {
	runsPath := filepath.Join(t.TempDir(), "runs.json")
	if err := os.WriteFile(runsPath, []byte(`{
		"runs": [
			{
				"provider": "openai",
				"model": "gpt-5.5",
				"api_key_env": "MISSING_CAPTURE_KEY",
				"base_url_env": "OPENAI_CAPTURE_BASE_URL",
				"output_path": "/tmp/openai.json"
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write runs fixture: %v", err)
	}
	t.Setenv("HERMESTOKEN_PROBE_RUNS_FILE", runsPath)
	t.Setenv("OPENAI_CAPTURE_BASE_URL", "https://openai.invalid")

	_, err := directProbeConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "MISSING_CAPTURE_KEY") {
		t.Fatalf("error = %v, want missing secret env error", err)
	}
}

func TestDirectProbeConfigFromEnvRunsFileSupportsPerRunGapsAuditPath(t *testing.T) {
	tempDir := t.TempDir()
	gapsPath := filepath.Join(tempDir, "gaps.json")
	if err := os.WriteFile(gapsPath, []byte(`{
		"target_check_keys_by_audit_path": {
			"pass_fail_real_corpus": {
				"csv": "probe_math_logic"
			},
			"identity_real_corpus": {
				"csv": "probe_identity_self_knowledge,probe_identity_list_format"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write gaps fixture: %v", err)
	}
	runsPath := filepath.Join(tempDir, "runs.json")
	if err := os.WriteFile(runsPath, []byte(`{
		"runs": [
			{
				"provider": "anthropic",
				"model": "claude-opus-4-7",
				"api_key_env": "ANTHROPIC_CAPTURE_KEY",
				"base_url_env": "ANTHROPIC_CAPTURE_BASE_URL",
				"gaps_audit_path": "pass_fail_real_corpus",
				"output_path": "/tmp/passfail.json"
			},
			{
				"provider": "openai",
				"model": "gpt-5.5",
				"api_key_env": "OPENAI_CAPTURE_KEY",
				"base_url_env": "OPENAI_CAPTURE_BASE_URL",
				"gaps_audit_path": "identity_real_corpus",
				"output_path": "/tmp/identity.json"
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write runs fixture: %v", err)
	}
	t.Setenv("HERMESTOKEN_PROBE_RUNS_FILE", runsPath)
	t.Setenv("HERMESTOKEN_PROBE_GAPS_FILE", gapsPath)
	t.Setenv("ANTHROPIC_CAPTURE_KEY", "anthropic-key")
	t.Setenv("ANTHROPIC_CAPTURE_BASE_URL", "https://anthropic.invalid")
	t.Setenv("OPENAI_CAPTURE_KEY", "openai-key")
	t.Setenv("OPENAI_CAPTURE_BASE_URL", "https://openai.invalid")

	config, err := directProbeConfigFromEnv()
	if err != nil {
		t.Fatalf("directProbeConfigFromEnv returned error: %v", err)
	}
	passFailKeys := []tokenverifier.CheckKey{tokenverifier.CheckProbeMathLogic}
	if !reflect.DeepEqual(config.Runs[0].Request.CheckKeys, passFailKeys) {
		t.Fatalf("pass/fail keys = %#v, want %#v", config.Runs[0].Request.CheckKeys, passFailKeys)
	}
	identityKeys := []tokenverifier.CheckKey{
		tokenverifier.CheckProbeIdentitySelfKnowledge,
		tokenverifier.CheckProbeIdentityListFormat,
	}
	if !reflect.DeepEqual(config.Runs[1].Request.CheckKeys, identityKeys) {
		t.Fatalf("identity keys = %#v, want %#v", config.Runs[1].Request.CheckKeys, identityKeys)
	}
}
