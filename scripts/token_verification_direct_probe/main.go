package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
)

const directProbeTimeout = 45 * time.Minute

type directProbeConfig struct {
	APIKey     string
	OutputPath string
	Request    tokenverifier.DirectProbeRequest
	Runs       []directProbeRun
}

type directProbeRun struct {
	OutputPath string
	Request    tokenverifier.DirectProbeRequest
}

type directProbeRunsFile struct {
	Runs []directProbeRunSpec `json:"runs"`
}

type directProbeRunSpec struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	APIKeyEnv     string `json:"api_key_env"`
	BaseURLEnv    string `json:"base_url_env"`
	ClientProfile string `json:"client_profile"`
	CheckKeysCSV  string `json:"check_keys_csv"`
	GapsAuditPath string `json:"gaps_audit_path"`
	OutputPath    string `json:"output_path"`
}

func main() {
	config, err := directProbeConfigFromEnv()
	if err != nil {
		exitWithError(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), directProbeTimeout)
	defer cancel()

	for _, run := range config.Runs {
		response, err := tokenverifier.RunDirectProbe(ctx, run.Request)
		if err != nil {
			exitWithError(err.Error())
		}
		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			exitWithError(err.Error())
		}
		if err := os.MkdirAll(filepath.Dir(run.OutputPath), 0o755); err != nil {
			exitWithError(err.Error())
		}
		if err := os.WriteFile(run.OutputPath, append(data, '\n'), 0o600); err != nil {
			exitWithError(err.Error())
		}
		fmt.Println(run.OutputPath)
	}
}

func directProbeConfigFromEnv() (directProbeConfig, error) {
	if runsPath := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_RUNS_FILE")); runsPath != "" {
		return directProbeConfigFromRunsFile(runsPath)
	}
	apiKey := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_API_KEY"))
	if apiKey == "" {
		return directProbeConfig{}, fmt.Errorf("missing HERMESTOKEN_PROBE_API_KEY")
	}
	baseURL := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_BASE_URL"))
	if baseURL == "" {
		return directProbeConfig{}, fmt.Errorf("missing HERMESTOKEN_PROBE_BASE_URL")
	}
	provider := getenvDefault("HERMESTOKEN_PROBE_PROVIDER", tokenverifier.ProviderAnthropic)
	clientProfile := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_CLIENT_PROFILE"))
	if clientProfile == "" && provider == tokenverifier.ProviderAnthropic {
		clientProfile = tokenverifier.ClientProfileClaudeCode
	}
	checkKeys, err := probeCheckKeysFromEnv()
	if err != nil {
		return directProbeConfig{}, err
	}
	models := probeModelsFromEnv()
	if len(models) == 0 {
		models = []string{"claude-opus-4-7"}
	}
	runs, err := directProbeRunsFromEnv(models, tokenverifier.DirectProbeRequest{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Provider:      provider,
		ProbeProfile:  getenvDefault("HERMESTOKEN_PROBE_PROFILE", tokenverifier.ProbeProfileFull),
		CheckKeys:     checkKeys,
		ClientProfile: clientProfile,
	})
	if err != nil {
		return directProbeConfig{}, err
	}
	config := directProbeConfig{
		APIKey:     apiKey,
		OutputPath: runs[0].OutputPath,
		Request:    runs[0].Request,
		Runs:       runs,
	}
	return config, nil
}

func directProbeConfigFromRunsFile(path string) (directProbeConfig, error) {
	checkKeys, err := probeCheckKeysFromEnv()
	if err != nil {
		return directProbeConfig{}, err
	}
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return directProbeConfig{}, fmt.Errorf("read HERMESTOKEN_PROBE_RUNS_FILE: %w", err)
	}
	var file directProbeRunsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return directProbeConfig{}, fmt.Errorf("parse HERMESTOKEN_PROBE_RUNS_FILE: %w", err)
	}
	if len(file.Runs) == 0 {
		return directProbeConfig{}, fmt.Errorf("HERMESTOKEN_PROBE_RUNS_FILE contains no runs")
	}
	runs := make([]directProbeRun, 0, len(file.Runs))
	for i, spec := range file.Runs {
		run, err := directProbeRunFromSpec(i, spec, checkKeys)
		if err != nil {
			return directProbeConfig{}, err
		}
		runs = append(runs, run)
	}
	return directProbeConfig{
		OutputPath: runs[0].OutputPath,
		Request:    runs[0].Request,
		Runs:       runs,
	}, nil
}

func directProbeRunFromSpec(index int, spec directProbeRunSpec, fallbackCheckKeys []tokenverifier.CheckKey) (directProbeRun, error) {
	provider := strings.ToLower(strings.TrimSpace(spec.Provider))
	if provider == "" {
		provider = tokenverifier.ProviderAnthropic
	}
	model := strings.TrimSpace(spec.Model)
	if model == "" {
		return directProbeRun{}, fmt.Errorf("run %d: missing model", index+1)
	}
	apiKey, err := secretEnvValue("api_key_env", spec.APIKeyEnv)
	if err != nil {
		return directProbeRun{}, fmt.Errorf("run %d: %w", index+1, err)
	}
	baseURL, err := secretEnvValue("base_url_env", spec.BaseURLEnv)
	if err != nil {
		return directProbeRun{}, fmt.Errorf("run %d: %w", index+1, err)
	}
	outputPath := strings.TrimSpace(spec.OutputPath)
	if outputPath == "" {
		return directProbeRun{}, fmt.Errorf("run %d: missing output_path", index+1)
	}
	clientProfile := strings.TrimSpace(spec.ClientProfile)
	if clientProfile == "" && provider == tokenverifier.ProviderAnthropic {
		clientProfile = tokenverifier.ClientProfileClaudeCode
	}
	checkKeys, err := checkKeysForRunSpec(spec, fallbackCheckKeys)
	if err != nil {
		return directProbeRun{}, fmt.Errorf("run %d: %w", index+1, err)
	}
	return directProbeRun{
		OutputPath: outputPath,
		Request: tokenverifier.DirectProbeRequest{
			BaseURL:       baseURL,
			APIKey:        apiKey,
			Provider:      provider,
			Model:         model,
			ProbeProfile:  getenvDefault("HERMESTOKEN_PROBE_PROFILE", tokenverifier.ProbeProfileFull),
			CheckKeys:     checkKeys,
			ClientProfile: clientProfile,
		},
	}, nil
}

func checkKeysForRunSpec(spec directProbeRunSpec, fallback []tokenverifier.CheckKey) ([]tokenverifier.CheckKey, error) {
	if strings.TrimSpace(spec.CheckKeysCSV) != "" {
		return parseCheckKeys(spec.CheckKeysCSV), nil
	}
	if strings.TrimSpace(spec.GapsAuditPath) == "" {
		return fallback, nil
	}
	gapsPath := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_GAPS_FILE"))
	if gapsPath == "" {
		return nil, fmt.Errorf("gaps_audit_path requires HERMESTOKEN_PROBE_GAPS_FILE")
	}
	data, err := os.ReadFile(gapsPath)
	if err != nil {
		return nil, fmt.Errorf("read HERMESTOKEN_PROBE_GAPS_FILE: %w", err)
	}
	keys, err := parseGapsFileCheckKeys(data, spec.GapsAuditPath)
	if err != nil {
		return nil, fmt.Errorf("parse HERMESTOKEN_PROBE_GAPS_FILE: %w", err)
	}
	return keys, nil
}

func secretEnvValue(field string, envName string) (string, error) {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return "", fmt.Errorf("missing %s", field)
	}
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return "", fmt.Errorf("%s env %s is not set", field, envName)
	}
	return value, nil
}

func directProbeRunsFromEnv(models []string, request tokenverifier.DirectProbeRequest) ([]directProbeRun, error) {
	outputPath := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_OUTPUT"))
	outputDir := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_OUTPUT_DIR"))
	if len(models) > 1 && outputPath != "" {
		return nil, fmt.Errorf("HERMESTOKEN_PROBE_OUTPUT_DIR is required for multi-model HERMESTOKEN_PROBE_MODELS runs; HERMESTOKEN_PROBE_OUTPUT would be overwritten")
	}
	if len(models) == 1 {
		request.Model = models[0]
		return []directProbeRun{{
			OutputPath: getenvDefault("HERMESTOKEN_PROBE_OUTPUT", "/tmp/token-verification-evidence-direct-probe.json"),
			Request:    request,
		}}, nil
	}
	if outputDir == "" {
		outputDir = "/tmp/token-verification-evidence-direct-probe-runs"
	}
	runs := make([]directProbeRun, 0, len(models))
	for i, model := range models {
		runRequest := request
		runRequest.Model = model
		runs = append(runs, directProbeRun{
			OutputPath: filepath.Join(outputDir, fmt.Sprintf("%02d-%s-%s.json", i+1, safeFilenamePart(request.Provider), safeFilenamePart(model))),
			Request:    runRequest,
		})
	}
	return runs, nil
}

func probeModelsFromEnv() []string {
	if raw := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_MODELS")); raw != "" {
		return parseUniqueCSV(raw)
	}
	return parseUniqueCSV(getenvDefault("HERMESTOKEN_PROBE_MODEL", "claude-opus-4-7"))
}

func probeCheckKeysFromEnv() ([]tokenverifier.CheckKey, error) {
	if raw := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_CHECK_KEYS")); raw != "" {
		return parseCheckKeys(raw), nil
	}
	gapsPath := strings.TrimSpace(os.Getenv("HERMESTOKEN_PROBE_GAPS_FILE"))
	if gapsPath == "" {
		return nil, nil
	}
	auditPath := getenvDefault("HERMESTOKEN_PROBE_GAPS_AUDIT_PATH", "pass_fail_real_corpus")
	data, err := os.ReadFile(gapsPath)
	if err != nil {
		return nil, fmt.Errorf("read HERMESTOKEN_PROBE_GAPS_FILE: %w", err)
	}
	keys, err := parseGapsFileCheckKeys(data, auditPath)
	if err != nil {
		return nil, fmt.Errorf("parse HERMESTOKEN_PROBE_GAPS_FILE: %w", err)
	}
	return keys, nil
}

func parseGapsFileCheckKeys(data []byte, auditPath string) ([]tokenverifier.CheckKey, error) {
	var gaps struct {
		TargetCheckKeysByAuditPath map[string]struct {
			CSV       string                   `json:"csv"`
			CheckKeys []tokenverifier.CheckKey `json:"check_keys"`
		} `json:"target_check_keys_by_audit_path"`
	}
	if err := json.Unmarshal(data, &gaps); err != nil {
		return nil, err
	}
	target, ok := gaps.TargetCheckKeysByAuditPath[strings.TrimSpace(auditPath)]
	if !ok {
		return nil, fmt.Errorf("audit path %q not found", auditPath)
	}
	if strings.TrimSpace(target.CSV) != "" {
		return parseCheckKeys(target.CSV), nil
	}
	return target.CheckKeys, nil
}

func parseCheckKeys(value string) []tokenverifier.CheckKey {
	parts := parseUniqueCSV(value)
	out := make([]tokenverifier.CheckKey, 0, len(parts))
	for _, part := range parts {
		out = append(out, tokenverifier.CheckKey(part))
	}
	return out
}

func parseUniqueCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		out = append(out, part)
		seen[part] = true
	}
	return out
}

var unsafeFilenamePartPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeFilenamePart(value string) string {
	value = strings.Trim(unsafeFilenamePartPattern.ReplaceAllString(strings.TrimSpace(value), "-"), "-")
	if value == "" {
		return "unknown"
	}
	return value
}

func getenvDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func exitWithError(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
