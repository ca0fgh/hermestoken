package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
)

const (
	labeledProbeCorpusDraftFilename       = "labeled_probe_corpus_real.draft.json"
	identityAssessmentCorpusDraftFilename = "identity_assessment_corpus_real.draft.json"
	informationalProbeCorpusDraftFilename = "informational_probe_corpus_real.draft.json"
	probeBaselineDraftFilename            = "probe_baseline.draft.json"
)

func main() {
	inputPath := flag.String("input", "", "path to a direct probe JSON response or API envelope")
	outputPath := flag.String("output", "", "path to write the labeled corpus draft; stdout when empty")
	description := flag.String("description", "", "corpus description")
	secretList := flag.String("secret", "", "comma-separated secrets to redact from corpus text")
	mode := flag.String("mode", "probe", "corpus mode: probe, identity, informational, bundle, summary, coverage, gaps, inspect, or audit")
	sourceTaskID := flag.String("source-task-id", "", "source task id to add when the input JSON does not contain one")
	capturedAt := flag.String("captured-at", "", "RFC3339 capture time to add when the input JSON does not contain one")
	baseDir := flag.String("base-dir", "", "base directory for real corpus files in audit modes")
	baselinePath := flag.String("baseline", "", "path to a reviewed probe baseline JSON/draft for audit modes")
	judgeBaseURL := flag.String("judge-base-url", "", "judge API base URL for audit modes")
	judgeModel := flag.String("judge-model", "", "judge model id for audit modes")
	judgeAPIKeyEnv := flag.String("judge-api-key-env", "", "environment variable name containing the judge API key for audit modes")
	judgeThreshold := flag.Int("judge-threshold", 0, "judge pass threshold for audit modes")
	runsOutputDir := flag.String("runs-output-dir", "", "output directory to use in runs-template mode")
	audit := flag.Bool("audit", false, "emit a token verification accuracy audit report instead of building a corpus draft")
	requireAudit := flag.Bool("require", false, "exit non-zero when audit mode reports missing accuracy evidence")
	flag.Parse()

	switch {
	case isMergePassFailCleanMode(*mode):
		output, err := buildCleanMergedLabeledProbeCorpusOutput(flag.Args(), *description)
		if err != nil {
			exitWithError("clean merge pass/fail corpus: " + err.Error())
		}
		writeOutput(*outputPath, output)
		return
	case isMergePassFailMode(*mode):
		output, err := buildMergedLabeledProbeCorpusOutput(flag.Args(), *description)
		if err != nil {
			exitWithError("merge pass/fail corpus: " + err.Error())
		}
		writeOutput(*outputPath, output)
		return
	case isMergeIdentityMode(*mode):
		output, err := buildMergedIdentityAssessmentCorpusOutput(flag.Args(), *description)
		if err != nil {
			exitWithError("merge identity corpus: " + err.Error())
		}
		writeOutput(*outputPath, output)
		return
	case isMergeInformationalMode(*mode):
		output, err := buildMergedInformationalProbeCorpusOutput(flag.Args(), *description)
		if err != nil {
			exitWithError("merge informational corpus: " + err.Error())
		}
		writeOutput(*outputPath, output)
		return
	}

	if *audit || isAuditMode(*mode) || isGapsMode(*mode) || isRunsTemplateMode(*mode) {
		options, err := buildAccuracyAuditOptions(accuracyAuditFlagValues{
			BaseDir:        *baseDir,
			BaselinePath:   *baselinePath,
			JudgeBaseURL:   *judgeBaseURL,
			JudgeModel:     *judgeModel,
			JudgeAPIKeyEnv: *judgeAPIKeyEnv,
			JudgeThreshold: *judgeThreshold,
		})
		if err != nil {
			exitWithError("build audit options: " + err.Error())
		}
		if isGapsMode(*mode) {
			output, err := buildAccuracyGapsOutput(options)
			if err != nil {
				exitWithError("build gaps: " + err.Error())
			}
			writeOutput(*outputPath, output)
			return
		}
		if isRunsTemplateMode(*mode) {
			output, err := buildDirectProbeRunsTemplateOutput(options, directProbeRunsTemplateOptions{OutputDir: *runsOutputDir})
			if err != nil {
				exitWithError("build runs template: " + err.Error())
			}
			writeOutput(*outputPath, output)
			return
		}
		output, report, err := buildAccuracyAuditResult(options)
		if err != nil {
			exitWithError("build audit: " + err.Error())
		}
		writeOutput(*outputPath, output)
		if *requireAudit && !report.Passed {
			os.Exit(1)
		}
		return
	}

	if isCoverageMode(*mode) && strings.TrimSpace(*inputPath) == "" {
		options, err := buildAccuracyAuditOptions(accuracyAuditFlagValues{
			BaseDir:        *baseDir,
			BaselinePath:   *baselinePath,
			JudgeBaseURL:   *judgeBaseURL,
			JudgeModel:     *judgeModel,
			JudgeAPIKeyEnv: *judgeAPIKeyEnv,
			JudgeThreshold: *judgeThreshold,
		})
		if err != nil {
			exitWithError("build coverage options: " + err.Error())
		}
		output, err := buildAccuracyCoverageOutput(options)
		if err != nil {
			exitWithError("build coverage: " + err.Error())
		}
		writeOutput(*outputPath, output)
		return
	}

	if strings.TrimSpace(*inputPath) == "" {
		exitWithError("missing -input")
	}
	data, err := os.ReadFile(*inputPath)
	if err != nil {
		exitWithError("read input: " + err.Error())
	}
	response, err := decodeDirectProbeResponse(data)
	if err != nil {
		exitWithError("parse input: " + err.Error())
	}
	sourceFallback, err := parseSourceMetadataFlags(*sourceTaskID, *capturedAt)
	if err != nil {
		exitWithError("parse source metadata: " + err.Error())
	}
	*response = directProbeResponseWithSourceFallback(*response, sourceFallback)
	secrets := splitSecrets(*secretList)
	if isSummaryMode(*mode) {
		output, err := buildCorpusEvidenceSummaryOutput(*response, *description, secrets)
		if err != nil {
			exitWithError("build summary: " + err.Error())
		}
		writeOutput(*outputPath, output)
		return
	}
	if isBundleMode(*mode) {
		bundle := buildCorpusBundleDrafts(*response, *description, secrets)
		if strings.TrimSpace(*outputPath) != "" {
			if err := writeCorpusBundleFiles(*outputPath, bundle); err != nil {
				exitWithError("write bundle: " + err.Error())
			}
			return
		}
		output, err := marshalCorpusBundleDraft(bundle)
		if err != nil {
			exitWithError("build bundle: " + err.Error())
		}
		writeOutput("", output)
		return
	}
	output, err := buildCorpusDraftOutput(*response, *mode, *description, secrets)
	if err != nil {
		exitWithError("build corpus: " + err.Error())
	}
	writeOutput(*outputPath, output)
}

func writeOutput(outputPath string, output []byte) {
	output = append(output, '\n')
	if strings.TrimSpace(outputPath) == "" {
		_, _ = os.Stdout.Write(output)
		return
	}
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		exitWithError("write output: " + err.Error())
	}
}

func buildAccuracyAuditOutput(options tokenverifier.ProbeAccuracyAuditOptions) ([]byte, error) {
	output, _, err := buildAccuracyAuditResult(options)
	return output, err
}

func buildAccuracyAuditResult(options tokenverifier.ProbeAccuracyAuditOptions) ([]byte, tokenverifier.ProbeAccuracyAuditReport, error) {
	report := tokenverifier.BuildProbeAccuracyAuditReport(options)
	output, err := json.MarshalIndent(report, "", "  ")
	return output, report, err
}

type accuracyAuditFlagValues struct {
	BaseDir        string
	BaselinePath   string
	JudgeBaseURL   string
	JudgeModel     string
	JudgeAPIKeyEnv string
	JudgeThreshold int
}

func buildAccuracyAuditOptions(flags accuracyAuditFlagValues) (tokenverifier.ProbeAccuracyAuditOptions, error) {
	options := tokenverifier.ProbeAccuracyAuditOptions{BaseDir: strings.TrimSpace(flags.BaseDir)}
	if strings.TrimSpace(flags.BaselinePath) != "" {
		baseline, err := loadProbeBaselineFile(flags.BaselinePath)
		if err != nil {
			return options, err
		}
		options.ProbeBaseline = baseline
	}
	judgeConfig, err := probeJudgeConfigFromFlags(flags)
	if err != nil {
		return options, err
	}
	options.JudgeConfig = judgeConfig
	return options, nil
}

func loadProbeBaselineFile(path string) (tokenverifier.BaselineMap, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("missing baseline path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	baseline, err := parseProbeBaselineFileData(data)
	if err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	return baseline, nil
}

func parseProbeBaselineFileData(data []byte) (tokenverifier.BaselineMap, error) {
	var draft tokenverifier.ProbeBaselineCorpus
	if err := json.Unmarshal(data, &draft); err == nil && len(draft.Probes) > 0 {
		out := make(tokenverifier.BaselineMap, len(draft.Probes))
		for _, probe := range draft.Probes {
			probeID := strings.TrimSpace(probe.ProbeID)
			if probeID != "" {
				out[probeID] = probe.ResponseText
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	var direct map[string]string
	if err := json.Unmarshal(data, &direct); err != nil {
		return nil, err
	}
	out := make(tokenverifier.BaselineMap, len(direct))
	for probeID, response := range direct {
		probeID = strings.TrimSpace(probeID)
		if probeID != "" {
			out[probeID] = response
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("baseline contains no probe responses")
	}
	return out, nil
}

func probeJudgeConfigFromFlags(flags accuracyAuditFlagValues) (*tokenverifier.ProbeJudgeConfig, error) {
	baseURL := strings.TrimSpace(flags.JudgeBaseURL)
	model := strings.TrimSpace(flags.JudgeModel)
	apiKeyEnv := strings.TrimSpace(flags.JudgeAPIKeyEnv)
	if baseURL == "" && model == "" && apiKeyEnv == "" && flags.JudgeThreshold == 0 {
		return nil, nil
	}
	if baseURL == "" {
		return nil, fmt.Errorf("missing -judge-base-url")
	}
	if model == "" {
		return nil, fmt.Errorf("missing -judge-model")
	}
	if apiKeyEnv == "" {
		return nil, fmt.Errorf("missing -judge-api-key-env")
	}
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	if apiKey == "" {
		return nil, fmt.Errorf("judge API key env %s is not set", apiKeyEnv)
	}
	return &tokenverifier.ProbeJudgeConfig{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		ModelID:   model,
		Threshold: flags.JudgeThreshold,
	}, nil
}

type accuracyCoverageOutput struct {
	ProbeProfile  string                                    `json:"probe_profile"`
	CoverageCount int                                       `json:"coverage_count"`
	Coverage      []tokenverifier.ProbeAccuracyCoverageItem `json:"coverage"`
}

func buildAccuracyCoverageOutput(options tokenverifier.ProbeAccuracyAuditOptions) ([]byte, error) {
	report := tokenverifier.BuildProbeAccuracyAuditReport(options)
	output := accuracyCoverageOutput{
		ProbeProfile:  report.ProbeProfile,
		CoverageCount: len(report.Coverage),
		Coverage:      report.Coverage,
	}
	return json.MarshalIndent(output, "", "  ")
}

type accuracyGapsOutput struct {
	Passed                     bool                            `json:"passed"`
	TargetCheckKeys            []tokenverifier.CheckKey        `json:"target_check_keys,omitempty"`
	TargetCheckKeysCSV         string                          `json:"target_check_keys_csv,omitempty"`
	TargetCheckKeysByAuditPath map[string]accuracyTargetChecks `json:"target_check_keys_by_audit_path,omitempty"`
	MissingCorpusFiles         []accuracyCorpusGap             `json:"missing_corpus_files,omitempty"`
	MissingCoverage            []accuracyCorpusGap             `json:"missing_coverage,omitempty"`
	FalsePositive              []accuracyCorpusGap             `json:"false_positive,omitempty"`
	FalseNegative              []accuracyCorpusGap             `json:"false_negative,omitempty"`
	Invalid                    []accuracyCorpusGap             `json:"invalid,omitempty"`
	ReviewOnlyMissing          []string                        `json:"review_only_missing,omitempty"`
	Missing                    []string                        `json:"missing,omitempty"`
}

type accuracyCorpusGap struct {
	AuditPath   string `json:"audit_path"`
	Path        string `json:"path,omitempty"`
	Requirement string `json:"requirement,omitempty"`
}

type accuracyTargetChecks struct {
	CheckKeys []tokenverifier.CheckKey `json:"check_keys,omitempty"`
	CSV       string                   `json:"csv,omitempty"`
}

type directProbeRunsTemplateOptions struct {
	OutputDir string
}

type directProbeRunsTemplate struct {
	Runs []directProbeRunTemplate `json:"runs"`
}

type directProbeRunTemplate struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	APIKeyEnv     string `json:"api_key_env"`
	BaseURLEnv    string `json:"base_url_env"`
	ClientProfile string `json:"client_profile,omitempty"`
	GapsAuditPath string `json:"gaps_audit_path,omitempty"`
	OutputPath    string `json:"output_path"`
}

func buildAccuracyGapsOutput(options tokenverifier.ProbeAccuracyAuditOptions) ([]byte, error) {
	report := tokenverifier.BuildProbeAccuracyAuditReport(options)
	targetCheckKeys := targetCheckKeysFromReport(report)
	output := accuracyGapsOutput{
		Passed:                     report.Passed,
		TargetCheckKeys:            targetCheckKeys,
		TargetCheckKeysCSV:         targetCheckKeysCSV(targetCheckKeys),
		TargetCheckKeysByAuditPath: targetCheckKeysByAuditPathOutput(report),
		ReviewOnlyMissing:          report.ReviewOnlyJudge.Missing,
		Missing:                    report.Missing,
	}
	for _, file := range report.EvidenceFiles {
		if !file.Present {
			output.MissingCorpusFiles = append(output.MissingCorpusFiles, accuracyCorpusGap{AuditPath: file.Kind, Path: file.Path})
		}
		for _, value := range file.MissingCoverage {
			output.MissingCoverage = append(output.MissingCoverage, accuracyCorpusGap{AuditPath: file.Kind, Path: file.Path, Requirement: value})
		}
		for _, value := range file.FalsePositive {
			output.FalsePositive = append(output.FalsePositive, accuracyCorpusGap{AuditPath: file.Kind, Path: file.Path, Requirement: value})
		}
		for _, value := range file.FalseNegative {
			output.FalseNegative = append(output.FalseNegative, accuracyCorpusGap{AuditPath: file.Kind, Path: file.Path, Requirement: value})
		}
		for _, value := range file.Invalid {
			output.Invalid = append(output.Invalid, accuracyCorpusGap{AuditPath: file.Kind, Path: file.Path, Requirement: value})
		}
	}
	return json.MarshalIndent(output, "", "  ")
}

func buildDirectProbeRunsTemplateOutput(options tokenverifier.ProbeAccuracyAuditOptions, templateOptions directProbeRunsTemplateOptions) ([]byte, error) {
	report := tokenverifier.BuildProbeAccuracyAuditReport(options)
	return buildDirectProbeRunsTemplateOutputFromReport(report, templateOptions)
}

func buildDirectProbeRunsTemplateOutputFromReport(report tokenverifier.ProbeAccuracyAuditReport, options directProbeRunsTemplateOptions) ([]byte, error) {
	byPath := targetCheckKeysByAuditPath(report)
	outputDir := strings.TrimSpace(options.OutputDir)
	if outputDir == "" {
		outputDir = "/tmp/token-verification-evidence-targeted-runs"
	}
	template := directProbeRunsTemplate{Runs: make([]directProbeRunTemplate, 0, len(byPath))}
	if len(byPath["pass_fail_real_corpus"]) > 0 {
		template.Runs = append(template.Runs, directProbeRunTemplate{
			Provider:      tokenverifier.ProviderAnthropic,
			Model:         "claude-opus-4-7",
			APIKeyEnv:     "ANTHROPIC_CAPTURE_KEY",
			BaseURLEnv:    "ANTHROPIC_CAPTURE_BASE_URL",
			ClientProfile: tokenverifier.ClientProfileClaudeCode,
			GapsAuditPath: "pass_fail_real_corpus",
			OutputPath:    filepath.Join(outputDir, "01-anthropic-passfail.json"),
		})
	}
	if len(byPath["identity_real_corpus"]) > 0 {
		template.Runs = append(template.Runs, directProbeRunTemplate{
			Provider:      tokenverifier.ProviderOpenAI,
			Model:         "gpt-5.5",
			APIKeyEnv:     "OPENAI_CAPTURE_KEY",
			BaseURLEnv:    "OPENAI_CAPTURE_BASE_URL",
			GapsAuditPath: "identity_real_corpus",
			OutputPath:    filepath.Join(outputDir, fmt.Sprintf("%02d-openai-identity.json", len(template.Runs)+1)),
		})
	}
	if len(byPath["informational_real_corpus"]) > 0 {
		template.Runs = append(template.Runs, directProbeRunTemplate{
			Provider:      tokenverifier.ProviderAnthropic,
			Model:         "claude-opus-4-7",
			APIKeyEnv:     "ANTHROPIC_CAPTURE_KEY",
			BaseURLEnv:    "ANTHROPIC_CAPTURE_BASE_URL",
			ClientProfile: tokenverifier.ClientProfileClaudeCode,
			GapsAuditPath: "informational_real_corpus",
			OutputPath:    filepath.Join(outputDir, fmt.Sprintf("%02d-anthropic-informational.json", len(template.Runs)+1)),
		})
	}
	return json.MarshalIndent(template, "", "  ")
}

func targetCheckKeysByAuditPathOutput(report tokenverifier.ProbeAccuracyAuditReport) map[string]accuracyTargetChecks {
	byPath := targetCheckKeysByAuditPath(report)
	out := make(map[string]accuracyTargetChecks, len(byPath))
	for path, keys := range byPath {
		if len(keys) == 0 {
			continue
		}
		out[path] = accuracyTargetChecks{CheckKeys: keys, CSV: targetCheckKeysCSV(keys)}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func targetCheckKeysFromReport(report tokenverifier.ProbeAccuracyAuditReport) []tokenverifier.CheckKey {
	byPath := targetCheckKeysByAuditPath(report)
	keys := make([]tokenverifier.CheckKey, 0, len(report.Coverage))
	seen := make(map[tokenverifier.CheckKey]bool)
	for _, item := range report.Coverage {
		for _, key := range byPath[item.AuditPath] {
			if !seen[key] {
				keys = append(keys, key)
				seen[key] = true
			}
		}
	}
	return keys
}

func targetCheckKeysByAuditPath(report tokenverifier.ProbeAccuracyAuditReport) map[string][]tokenverifier.CheckKey {
	coverageByPath := make(map[string][]tokenverifier.CheckKey)
	for _, item := range report.Coverage {
		if item.AuditPath != "" {
			coverageByPath[item.AuditPath] = append(coverageByPath[item.AuditPath], item.CheckKey)
		}
	}
	out := make(map[string][]tokenverifier.CheckKey)
	seenByPath := make(map[string]map[tokenverifier.CheckKey]bool)
	addPath := func(path string) {
		if seenByPath[path] == nil {
			seenByPath[path] = make(map[tokenverifier.CheckKey]bool)
		}
		for _, key := range coverageByPath[path] {
			if !seenByPath[path][key] {
				out[path] = append(out[path], key)
				seenByPath[path][key] = true
			}
		}
	}
	addKey := func(path string, key tokenverifier.CheckKey) {
		if key == "" {
			return
		}
		if seenByPath[path] == nil {
			seenByPath[path] = make(map[tokenverifier.CheckKey]bool)
		}
		if !seenByPath[path][key] {
			out[path] = append(out[path], key)
			seenByPath[path][key] = true
		}
	}
	for _, file := range report.EvidenceFiles {
		if !file.Present {
			addPath(file.Kind)
			continue
		}
		switch file.Kind {
		case "pass_fail_real_corpus", "informational_real_corpus":
			for _, value := range file.MissingCoverage {
				addKey(file.Kind, tokenverifier.CheckKey(strings.SplitN(value, ":", 2)[0]))
			}
		case "identity_real_corpus":
			if len(file.MissingCoverage) > 0 {
				addPath(file.Kind)
			}
		}
	}
	return out
}

func checkKeysToStrings(keys []tokenverifier.CheckKey) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, string(key))
	}
	return out
}

func targetCheckKeysCSV(keys []tokenverifier.CheckKey) string {
	return strings.Join(checkKeysToStrings(keys), ",")
}

func isAuditMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "audit")
}

func isGapsMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "gaps", "missing":
		return true
	default:
		return false
	}
}

func isMergePassFailMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "merge-passfail", "merge_passfail", "merge-pass-fail", "merge_pass_fail":
		return true
	default:
		return false
	}
}

func isMergePassFailCleanMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "merge-passfail-clean", "merge_passfail_clean", "merge-pass-fail-clean", "merge_pass_fail_clean":
		return true
	default:
		return false
	}
}

func isMergeIdentityMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "merge-identity", "merge_identity":
		return true
	default:
		return false
	}
}

func isMergeInformationalMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "merge-informational", "merge_informational", "merge-info", "merge_info":
		return true
	default:
		return false
	}
}

func isRunsTemplateMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "runs-template", "runs_template", "probe-runs-template", "probe_runs_template":
		return true
	default:
		return false
	}
}

func isCoverageMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "coverage")
}

func isBundleMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "bundle", "all":
		return true
	default:
		return false
	}
}

func isSummaryMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "summary", "coverage", "inspect":
		return true
	default:
		return false
	}
}

type corpusBundleDraft struct {
	LabeledProbe       tokenverifier.LabeledProbeCorpus       `json:"labeled_probe_corpus"`
	IdentityAssessment tokenverifier.IdentityAssessmentCorpus `json:"identity_assessment_corpus"`
	InformationalProbe tokenverifier.InformationalProbeCorpus `json:"informational_probe_corpus"`
	ProbeBaseline      tokenverifier.ProbeBaselineCorpus      `json:"probe_baseline"`
}

type corpusEvidenceSummary struct {
	SourceReady        bool     `json:"source_ready"`
	MissingSource      []string `json:"missing_source,omitempty"`
	PassFailCases      int      `json:"pass_fail_cases"`
	IdentityCases      int      `json:"identity_cases"`
	InformationalCases int      `json:"informational_cases"`
	BaselineProbes     int      `json:"baseline_probes"`
	BaselineProbeIDs   []string `json:"baseline_probe_ids,omitempty"`
}

func buildCorpusBundleDrafts(response tokenverifier.DirectProbeResponse, description string, secrets []string) corpusBundleDraft {
	source := corpusCaseSourceFromResponse(response)
	return corpusBundleDraft{
		LabeledProbe:       tokenverifier.BuildLabeledProbeCorpusDraftFromResultsWithSource(description, response.Results, source, secrets...),
		IdentityAssessment: tokenverifier.BuildIdentityAssessmentCorpusDraftFromDirectProbeResponse(description, response, secrets...),
		InformationalProbe: tokenverifier.BuildInformationalProbeCorpusDraftFromResultsWithSource(description, response.Results, source, secrets...),
		ProbeBaseline:      tokenverifier.BuildProbeBaselineDraftFromResults(description, response.Results, secrets...),
	}
}

func buildCorpusEvidenceSummaryOutput(response tokenverifier.DirectProbeResponse, description string, secrets []string) ([]byte, error) {
	bundle := buildCorpusBundleDrafts(response, description, secrets)
	missingSource := missingCorpusEvidenceSource(response)
	probeIDs := make([]string, 0, len(bundle.ProbeBaseline.Probes))
	for _, probe := range bundle.ProbeBaseline.Probes {
		probeIDs = append(probeIDs, probe.ProbeID)
	}
	summary := corpusEvidenceSummary{
		SourceReady:        len(missingSource) == 0,
		MissingSource:      missingSource,
		PassFailCases:      len(bundle.LabeledProbe.Cases),
		IdentityCases:      len(bundle.IdentityAssessment.Cases),
		InformationalCases: len(bundle.InformationalProbe.Cases),
		BaselineProbes:     len(bundle.ProbeBaseline.Probes),
		BaselineProbeIDs:   probeIDs,
	}
	return json.MarshalIndent(summary, "", "  ")
}

func missingCorpusEvidenceSource(response tokenverifier.DirectProbeResponse) []string {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(response.Provider) == "" {
		missing = append(missing, "provider")
	}
	if strings.TrimSpace(response.Model) == "" {
		missing = append(missing, "model")
	}
	if strings.TrimSpace(response.SourceTaskID) == "" {
		missing = append(missing, "source_task_id")
	}
	if strings.TrimSpace(response.CapturedAt) == "" {
		missing = append(missing, "captured_at")
	}
	return missing
}

func marshalCorpusBundleDraft(bundle corpusBundleDraft) ([]byte, error) {
	return json.MarshalIndent(bundle, "", "  ")
}

func buildMergedLabeledProbeCorpusOutput(inputPaths []string, description string) ([]byte, error) {
	if len(inputPaths) == 0 {
		return nil, fmt.Errorf("missing input corpus paths")
	}
	merged := tokenverifier.LabeledProbeCorpus{
		Description:  mergeCorpusDescription(description, "Merged reviewed real-model pass/fail probe corpus."),
		ManualReview: mergedCorpusManualReview(),
		Cases:        make([]tokenverifier.LabeledProbeCorpusCase, 0),
	}
	seen := make(map[string]bool)
	for _, path := range inputPaths {
		corpus, err := readReviewedLabeledProbeCorpus(path)
		if err != nil {
			return nil, err
		}
		for _, item := range corpus.Cases {
			if strings.TrimSpace(item.Name) == "" {
				return nil, fmt.Errorf("%s: corpus case missing name", path)
			}
			if seen[item.Name] {
				continue
			}
			merged.Cases = append(merged.Cases, item)
			seen[item.Name] = true
		}
	}
	return json.MarshalIndent(merged, "", "  ")
}

func buildCleanMergedLabeledProbeCorpusOutput(inputPaths []string, description string) ([]byte, error) {
	if len(inputPaths) == 0 {
		return nil, fmt.Errorf("missing input corpus paths")
	}
	merged := tokenverifier.LabeledProbeCorpus{
		Description:  mergeCorpusDescription(description, "Clean merged reviewed real-model pass/fail probe corpus."),
		ManualReview: mergedCorpusManualReview(),
		Cases:        make([]tokenverifier.LabeledProbeCorpusCase, 0),
	}
	seen := make(map[string]bool)
	for _, path := range inputPaths {
		corpus, err := readReviewedLabeledProbeCorpus(path)
		if err != nil {
			return nil, err
		}
		for _, item := range corpus.Cases {
			if !isStableLabeledProbeCorpusCase(item) {
				continue
			}
			if strings.TrimSpace(item.Name) == "" {
				return nil, fmt.Errorf("%s: corpus case missing name", path)
			}
			if seen[item.Name] {
				continue
			}
			merged.Cases = append(merged.Cases, item)
			seen[item.Name] = true
		}
	}
	if len(merged.Cases) == 0 {
		return nil, fmt.Errorf("no stable labeled probe corpus cases")
	}
	return json.MarshalIndent(merged, "", "  ")
}

func isStableLabeledProbeCorpusCase(item tokenverifier.LabeledProbeCorpusCase) bool {
	return tokenverifier.LabeledProbeCorpusCaseMatchesCurrentScorer(item)
}

func buildMergedIdentityAssessmentCorpusOutput(inputPaths []string, description string) ([]byte, error) {
	if len(inputPaths) == 0 {
		return nil, fmt.Errorf("missing input corpus paths")
	}
	merged := tokenverifier.IdentityAssessmentCorpus{
		Description:  mergeCorpusDescription(description, "Merged reviewed real-model identity assessment corpus."),
		ManualReview: mergedCorpusManualReview(),
		Cases:        make([]tokenverifier.IdentityAssessmentCorpusCase, 0),
	}
	seen := make(map[string]bool)
	for _, path := range inputPaths {
		corpus, err := readReviewedIdentityAssessmentCorpus(path)
		if err != nil {
			return nil, err
		}
		for _, item := range corpus.Cases {
			if strings.TrimSpace(item.Name) == "" {
				return nil, fmt.Errorf("%s: corpus case missing name", path)
			}
			if seen[item.Name] {
				continue
			}
			merged.Cases = append(merged.Cases, item)
			seen[item.Name] = true
		}
	}
	return json.MarshalIndent(merged, "", "  ")
}

func buildMergedInformationalProbeCorpusOutput(inputPaths []string, description string) ([]byte, error) {
	if len(inputPaths) == 0 {
		return nil, fmt.Errorf("missing input corpus paths")
	}
	merged := tokenverifier.InformationalProbeCorpus{
		Description:  mergeCorpusDescription(description, "Merged reviewed real-model informational probe corpus."),
		ManualReview: mergedCorpusManualReview(),
		Cases:        make([]tokenverifier.InformationalProbeCorpusCase, 0),
	}
	seen := make(map[string]bool)
	for _, path := range inputPaths {
		corpus, err := readReviewedInformationalProbeCorpus(path)
		if err != nil {
			return nil, err
		}
		for _, item := range corpus.Cases {
			if strings.TrimSpace(item.Name) == "" {
				return nil, fmt.Errorf("%s: corpus case missing name", path)
			}
			if seen[item.Name] {
				continue
			}
			merged.Cases = append(merged.Cases, item)
			seen[item.Name] = true
		}
	}
	return json.MarshalIndent(merged, "", "  ")
}

func readReviewedLabeledProbeCorpus(path string) (tokenverifier.LabeledProbeCorpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tokenverifier.LabeledProbeCorpus{}, fmt.Errorf("%s: read: %w", path, err)
	}
	var corpus tokenverifier.LabeledProbeCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return tokenverifier.LabeledProbeCorpus{}, fmt.Errorf("%s: parse: %w", path, err)
	}
	if err := validateReviewedCorpusManualReview(path, corpus.ManualReview); err != nil {
		return tokenverifier.LabeledProbeCorpus{}, err
	}
	return corpus, nil
}

func readReviewedIdentityAssessmentCorpus(path string) (tokenverifier.IdentityAssessmentCorpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tokenverifier.IdentityAssessmentCorpus{}, fmt.Errorf("%s: read: %w", path, err)
	}
	var corpus tokenverifier.IdentityAssessmentCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return tokenverifier.IdentityAssessmentCorpus{}, fmt.Errorf("%s: parse: %w", path, err)
	}
	if err := validateReviewedCorpusManualReview(path, corpus.ManualReview); err != nil {
		return tokenverifier.IdentityAssessmentCorpus{}, err
	}
	return corpus, nil
}

func readReviewedInformationalProbeCorpus(path string) (tokenverifier.InformationalProbeCorpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tokenverifier.InformationalProbeCorpus{}, fmt.Errorf("%s: read: %w", path, err)
	}
	var corpus tokenverifier.InformationalProbeCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return tokenverifier.InformationalProbeCorpus{}, fmt.Errorf("%s: parse: %w", path, err)
	}
	if err := validateReviewedCorpusManualReview(path, corpus.ManualReview); err != nil {
		return tokenverifier.InformationalProbeCorpus{}, err
	}
	return corpus, nil
}

func validateReviewedCorpusManualReview(path string, review tokenverifier.CorpusManualReview) error {
	if strings.TrimSpace(review.Status) != "reviewed" || strings.TrimSpace(review.Source) != "real_model_output" {
		return fmt.Errorf("%s: manual_review must be reviewed real_model_output", path)
	}
	return nil
}

func mergeCorpusDescription(description string, fallback string) string {
	description = strings.TrimSpace(description)
	if description != "" {
		return description
	}
	return fallback
}

func mergedCorpusManualReview() tokenverifier.CorpusManualReview {
	return tokenverifier.CorpusManualReview{
		Status:     "reviewed",
		Source:     "real_model_output",
		ReviewedBy: "merged-local-audit",
		ReviewedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func writeCorpusBundleFiles(outputDir string, bundle corpusBundleDraft) error {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return fmt.Errorf("missing output directory")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{}
	var err error
	if files[labeledProbeCorpusDraftFilename], err = tokenverifier.MarshalLabeledProbeCorpusDraft(bundle.LabeledProbe); err != nil {
		return err
	}
	if files[identityAssessmentCorpusDraftFilename], err = tokenverifier.MarshalIdentityAssessmentCorpusDraft(bundle.IdentityAssessment); err != nil {
		return err
	}
	if files[informationalProbeCorpusDraftFilename], err = tokenverifier.MarshalInformationalProbeCorpusDraft(bundle.InformationalProbe); err != nil {
		return err
	}
	if files[probeBaselineDraftFilename], err = tokenverifier.MarshalProbeBaselineDraft(bundle.ProbeBaseline); err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(outputDir, name), append(data, '\n'), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func buildCorpusDraftOutput(response tokenverifier.DirectProbeResponse, mode string, description string, secrets []string) ([]byte, error) {
	source := corpusCaseSourceFromResponse(response)
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "probe", "passfail", "pass_fail":
		corpus := tokenverifier.BuildLabeledProbeCorpusDraftFromResultsWithSource(description, response.Results, source, secrets...)
		return tokenverifier.MarshalLabeledProbeCorpusDraft(corpus)
	case "identity", "identity_assessment":
		corpus := tokenverifier.BuildIdentityAssessmentCorpusDraftFromDirectProbeResponse(description, response, secrets...)
		return tokenverifier.MarshalIdentityAssessmentCorpusDraft(corpus)
	case "informational", "info":
		corpus := tokenverifier.BuildInformationalProbeCorpusDraftFromResultsWithSource(description, response.Results, source, secrets...)
		return tokenverifier.MarshalInformationalProbeCorpusDraft(corpus)
	default:
		return nil, fmt.Errorf("unknown corpus mode %q", mode)
	}
}

func corpusCaseSourceFromResponse(response tokenverifier.DirectProbeResponse) tokenverifier.CorpusCaseSource {
	return tokenverifier.CorpusCaseSource{
		Provider:   response.Provider,
		Model:      response.Model,
		TaskID:     response.SourceTaskID,
		CapturedAt: response.CapturedAt,
	}
}

func parseSourceMetadataFlags(sourceTaskID string, capturedAt string) (tokenverifier.CorpusCaseSource, error) {
	capturedAt = strings.TrimSpace(capturedAt)
	if capturedAt != "" {
		if _, err := time.Parse(time.RFC3339, capturedAt); err != nil {
			return tokenverifier.CorpusCaseSource{}, fmt.Errorf("-captured-at must be RFC3339: %w", err)
		}
	}
	return tokenverifier.CorpusCaseSource{
		TaskID:     strings.TrimSpace(sourceTaskID),
		CapturedAt: capturedAt,
	}, nil
}

func directProbeResponseWithSourceFallback(response tokenverifier.DirectProbeResponse, source tokenverifier.CorpusCaseSource) tokenverifier.DirectProbeResponse {
	if strings.TrimSpace(response.SourceTaskID) == "" {
		response.SourceTaskID = source.TaskID
	}
	if strings.TrimSpace(response.CapturedAt) == "" {
		response.CapturedAt = source.CapturedAt
	}
	return response
}

func decodeDirectProbeResponse(data []byte) (*tokenverifier.DirectProbeResponse, error) {
	var response tokenverifier.DirectProbeResponse
	if err := json.Unmarshal(data, &response); err == nil && len(response.Results) > 0 {
		return &response, nil
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 {
		return nil, fmt.Errorf("input does not contain direct probe results")
	}
	if response, err := decodeDirectProbeResponseData(envelope.Data); err == nil && len(response.Results) > 0 {
		return response, nil
	} else if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("direct probe response has no results")
}

func decodeDirectProbeResponseData(data []byte) (*tokenverifier.DirectProbeResponse, error) {
	var response tokenverifier.DirectProbeResponse
	if err := json.Unmarshal(data, &response); err == nil && len(response.Results) > 0 {
		return &response, nil
	}
	var detail tokenVerificationTaskDetailEnvelope
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	response = tokenverifier.DirectProbeResponse{
		Provider:     firstString(detail.Task.Providers),
		Model:        firstString(detail.Task.Models),
		ProbeProfile: detail.Task.ProbeProfile,
		SourceTaskID: detail.Task.IDString(),
		CapturedAt:   tokenverifier.CorpusRFC3339FromUnixForExport(detail.Task.CreatedAt),
		Results:      make([]tokenverifier.CheckResult, 0, len(detail.Results)),
		Report:       detail.Report,
	}
	for _, item := range detail.Results {
		result, err := item.toCheckResult()
		if err != nil {
			return nil, err
		}
		response.Results = append(response.Results, result)
		if response.Provider == "" {
			response.Provider = result.Provider
		}
		if response.Model == "" {
			response.Model = result.ModelName
		}
	}
	if len(response.Results) == 0 {
		return nil, fmt.Errorf("direct probe response has no results")
	}
	return &response, nil
}

type tokenVerificationTaskDetailEnvelope struct {
	Task    tokenVerificationTaskCapture  `json:"task"`
	Results []tokenVerificationResultJSON `json:"results"`
	Report  tokenverifier.ReportSummary   `json:"report"`
}

type tokenVerificationTaskCapture struct {
	ID           int64    `json:"id"`
	Models       []string `json:"models"`
	Providers    []string `json:"providers"`
	ProbeProfile string   `json:"probe_profile"`
	CreatedAt    int64    `json:"created_at"`
}

func (task tokenVerificationTaskCapture) IDString() string {
	if task.ID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", task.ID)
}

type tokenVerificationResultJSON struct {
	Provider     string                 `json:"provider"`
	Group        string                 `json:"group"`
	CheckKey     tokenverifier.CheckKey `json:"check_key"`
	ModelName    string                 `json:"model_name"`
	Neutral      bool                   `json:"neutral"`
	Skipped      bool                   `json:"skipped"`
	Success      bool                   `json:"success"`
	Score        int                    `json:"score"`
	LatencyMs    int64                  `json:"latency_ms"`
	TTFTMs       int64                  `json:"ttft_ms"`
	InputTokens  *int                   `json:"input_tokens,omitempty"`
	OutputTokens *int                   `json:"output_tokens,omitempty"`
	TokensPS     float64                `json:"tokens_ps"`
	ErrorCode    string                 `json:"error_code"`
	Message      string                 `json:"message"`
	RiskLevel    string                 `json:"risk_level"`
	Evidence     []string               `json:"evidence,omitempty"`
	Raw          json.RawMessage        `json:"raw"`
}

func (item tokenVerificationResultJSON) toCheckResult() (tokenverifier.CheckResult, error) {
	raw, err := decodeResultRaw(item.Raw)
	if err != nil {
		return tokenverifier.CheckResult{}, err
	}
	return tokenverifier.CheckResult{
		Provider:     item.Provider,
		Group:        item.Group,
		CheckKey:     item.CheckKey,
		ModelName:    item.ModelName,
		Neutral:      item.Neutral,
		Skipped:      item.Skipped,
		Success:      item.Success,
		Score:        item.Score,
		LatencyMs:    item.LatencyMs,
		TTFTMs:       item.TTFTMs,
		InputTokens:  item.InputTokens,
		OutputTokens: item.OutputTokens,
		TokensPS:     item.TokensPS,
		ErrorCode:    item.ErrorCode,
		Message:      item.Message,
		RiskLevel:    item.RiskLevel,
		Evidence:     item.Evidence,
		Raw:          raw,
	}, nil
}

func decodeResultRaw(data json.RawMessage) (map[string]any, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err == nil {
		return raw, nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitSecrets(value string) []string {
	parts := strings.Split(value, ",")
	secrets := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			secrets = append(secrets, part)
		}
	}
	return secrets
}

func exitWithError(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
