package token_verifier

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

const (
	defaultLabeledProbeRealCorpusPath       = "service/token_verifier/testdata/labeled_probe_corpus_real.json"
	defaultIdentityAssessmentRealCorpusPath = "service/token_verifier/testdata/identity_assessment_corpus_real.json"
	defaultInformationalProbeRealCorpusPath = "service/token_verifier/testdata/informational_probe_corpus_real.json"
	auditPathPassFailRealCorpus             = "pass_fail_real_corpus"
	auditPathIdentityRealCorpus             = "identity_real_corpus"
	auditPathInformationalRealCorpus        = "informational_real_corpus"
	auditPathReviewOnlyJudge                = "review_only_judge"
)

type ProbeAccuracyAuditOptions struct {
	BaseDir       string
	JudgeConfig   *ProbeJudgeConfig
	ProbeBaseline BaselineMap
}

type ProbeAccuracyAuditReport struct {
	Passed          bool                        `json:"passed"`
	ProbeProfile    string                      `json:"probe_profile"`
	Missing         []string                    `json:"missing,omitempty"`
	EvidenceFiles   []ProbeAccuracyEvidenceFile `json:"evidence_files"`
	ReviewOnlyJudge ProbeAccuracyJudgeAudit     `json:"review_only_judge"`
	Coverage        []ProbeAccuracyCoverageItem `json:"coverage"`
}

type ProbeAccuracyEvidenceFile struct {
	Kind                string   `json:"kind"`
	Path                string   `json:"path"`
	Present             bool     `json:"present"`
	CaseCount           *int     `json:"case_count,omitempty"`
	ManualReviewMissing []string `json:"manual_review_missing,omitempty"`
	CaseSourceMissing   []string `json:"case_source_missing,omitempty"`
	MissingCoverage     []string `json:"missing_coverage,omitempty"`
	FalsePositive       []string `json:"false_positive,omitempty"`
	FalseNegative       []string `json:"false_negative,omitempty"`
	Invalid             []string `json:"invalid,omitempty"`
}

type ProbeAccuracyJudgeAudit struct {
	ConfigPresent   bool     `json:"config_present"`
	BaselinePresent bool     `json:"baseline_present"`
	RequiredProbes  []string `json:"required_probes"`
	Missing         []string `json:"missing,omitempty"`
}

type ProbeAccuracyCoverageItem struct {
	CheckKey            CheckKey `json:"check_key"`
	CheckName           string   `json:"check_name"`
	Group               string   `json:"group,omitempty"`
	AuditPath           string   `json:"audit_path,omitempty"`
	EvidenceRequirement string   `json:"evidence_requirement,omitempty"`
	ProbeID             string   `json:"probe_id,omitempty"`
	Neutral             bool     `json:"neutral,omitempty"`
	ReviewOnly          bool     `json:"review_only,omitempty"`
	MissingReason       string   `json:"missing_reason,omitempty"`
}

func BuildProbeAccuracyAuditReport(options ProbeAccuracyAuditOptions) ProbeAccuracyAuditReport {
	judge := options.JudgeConfig
	if judge == nil {
		judge = probeJudgeConfigFromEnv()
	}
	baseline := options.ProbeBaseline
	if baseline == nil {
		baseline = probeBaselineFromEnv()
	}

	report := ProbeAccuracyAuditReport{
		ProbeProfile: ProbeProfileFull,
	}
	report.EvidenceFiles = buildProbeAccuracyEvidenceFiles(options.BaseDir)
	for i := range report.EvidenceFiles {
		file := &report.EvidenceFiles[i]
		if !file.Present {
			report.Missing = append(report.Missing, "real_corpus:"+file.Kind+":"+file.Path)
			continue
		}
		if missing := validateProbeAccuracyEvidenceFile(options.BaseDir, file); len(missing) > 0 {
			report.Missing = append(report.Missing, missing...)
		}
	}

	report.ReviewOnlyJudge = buildProbeAccuracyJudgeAudit(judge, baseline)
	report.Missing = append(report.Missing, report.ReviewOnlyJudge.Missing...)
	report.Coverage = buildProbeAccuracyCoverageItems()
	for _, item := range report.Coverage {
		if item.MissingReason != "" {
			report.Missing = append(report.Missing, string(item.CheckKey)+":"+item.MissingReason)
		}
	}

	report.Passed = len(report.Missing) == 0
	return report
}

func buildProbeAccuracyEvidenceFiles(baseDir string) []ProbeAccuracyEvidenceFile {
	files := []ProbeAccuracyEvidenceFile{
		{Kind: auditPathPassFailRealCorpus, Path: defaultLabeledProbeRealCorpusPath},
		{Kind: auditPathIdentityRealCorpus, Path: defaultIdentityAssessmentRealCorpusPath},
		{Kind: auditPathInformationalRealCorpus, Path: defaultInformationalProbeRealCorpusPath},
	}
	out := make([]ProbeAccuracyEvidenceFile, 0, len(files))
	for _, file := range files {
		file.Present = auditFileExists(baseDir, file.Path)
		out = append(out, file)
	}
	return out
}

func validateProbeAccuracyEvidenceFile(baseDir string, file *ProbeAccuracyEvidenceFile) []string {
	data, err := readAuditEvidenceFile(baseDir, file.Path)
	if err != nil {
		file.Invalid = append(file.Invalid, "read:"+err.Error())
		return []string{file.Kind + ":invalid:read:" + err.Error()}
	}
	switch file.Kind {
	case auditPathPassFailRealCorpus:
		return validatePassFailRealCorpusEvidence(data, file)
	case auditPathIdentityRealCorpus:
		return validateIdentityRealCorpusEvidence(data, file)
	case auditPathInformationalRealCorpus:
		return validateInformationalRealCorpusEvidence(data, file)
	default:
		file.Invalid = append(file.Invalid, "unknown_evidence_kind")
		return []string{file.Kind + ":invalid:unknown_evidence_kind"}
	}
}

func validatePassFailRealCorpusEvidence(data []byte, detail *ProbeAccuracyEvidenceFile) []string {
	var corpus LabeledProbeCorpus
	if err := common.Unmarshal(data, &corpus); err != nil {
		recordAuditInvalid(detail, "parse:"+err.Error())
		return []string{auditPathPassFailRealCorpus + ":invalid:parse:" + err.Error()}
	}
	if detail != nil {
		recordAuditCaseCount(detail, len(corpus.Cases))
	}
	missing := validateCorpusManualReview(auditPathPassFailRealCorpus, corpus.ManualReview, detail)
	missing = append(missing, validateLabeledCorpusCaseSources(auditPathPassFailRealCorpus, corpus.Cases, detail)...)
	metrics, invalid := evaluateLabeledProbeCorpusCasesForAudit(corpus)
	for _, value := range invalid {
		recordAuditInvalid(detail, "evaluate:"+value)
		missing = append(missing, auditPathPassFailRealCorpus+":invalid:evaluate:"+value)
	}
	if len(corpus.Cases) == 0 {
		recordAuditInvalid(detail, "empty_cases")
		missing = append(missing, auditPathPassFailRealCorpus+":invalid:empty_cases")
	}
	for _, requirement := range realCorpusMissingCoverage(metrics) {
		recordAuditCoverageGap(detail, requirement)
		missing = append(missing, auditPathPassFailRealCorpus+":missing_coverage:"+requirement)
	}
	for key, metric := range metrics {
		if metric.FalsePositive > 0 {
			recordAuditFalsePositive(detail, string(key))
			missing = append(missing, auditPathPassFailRealCorpus+":false_positive:"+string(key))
		}
		if metric.FalseNegative > 0 {
			recordAuditFalseNegative(detail, string(key))
			missing = append(missing, auditPathPassFailRealCorpus+":false_negative:"+string(key))
		}
	}
	return missing
}

func validateIdentityRealCorpusEvidence(data []byte, detail *ProbeAccuracyEvidenceFile) []string {
	var corpus IdentityAssessmentCorpus
	if err := common.Unmarshal(data, &corpus); err != nil {
		recordAuditInvalid(detail, "parse:"+err.Error())
		return []string{auditPathIdentityRealCorpus + ":invalid:parse:" + err.Error()}
	}
	if detail != nil {
		recordAuditCaseCount(detail, len(corpus.Cases))
	}
	missing := validateCorpusManualReview(auditPathIdentityRealCorpus, corpus.ManualReview, detail)
	missing = append(missing, validateIdentityCorpusCaseSources(auditPathIdentityRealCorpus, corpus.Cases, detail)...)
	if len(corpus.Cases) == 0 {
		recordAuditInvalid(detail, "empty_cases")
		missing = append(missing, auditPathIdentityRealCorpus+":invalid:empty_cases")
	}
	for _, requirement := range realIdentityCorpusMissingCoverage(corpus) {
		recordAuditCoverageGap(detail, requirement)
		missing = append(missing, auditPathIdentityRealCorpus+":missing_coverage:"+requirement)
	}
	metric, err := evaluateIdentityAssessmentCorpusCases(corpus)
	if err != nil {
		recordAuditInvalid(detail, "evaluate:"+err.Error())
		return append(missing, auditPathIdentityRealCorpus+":invalid:evaluate:"+err.Error())
	}
	if metric.Total == 0 {
		recordAuditInvalid(detail, "no_scored_samples")
		return append(missing, auditPathIdentityRealCorpus+":invalid:no_scored_samples")
	}
	if metric.StatusCorrect != metric.Total {
		recordAuditInvalid(detail, "status_mismatch")
		missing = append(missing, auditPathIdentityRealCorpus+":status_mismatch")
	}
	if metric.VerdictCorrect != metric.Total {
		recordAuditInvalid(detail, "verdict_mismatch")
		missing = append(missing, auditPathIdentityRealCorpus+":verdict_mismatch")
	}
	if metric.FamilyCorrect != metric.Total {
		recordAuditInvalid(detail, "family_mismatch")
		missing = append(missing, auditPathIdentityRealCorpus+":family_mismatch")
	}
	if metric.ModelCorrect != metric.Total {
		recordAuditInvalid(detail, "model_mismatch")
		missing = append(missing, auditPathIdentityRealCorpus+":model_mismatch")
	}
	if metric.IdentityRiskCorrect != metric.Total {
		recordAuditInvalid(detail, "identity_risk_mismatch")
		missing = append(missing, auditPathIdentityRealCorpus+":identity_risk_mismatch")
	}
	return missing
}

func validateInformationalRealCorpusEvidence(data []byte, detail *ProbeAccuracyEvidenceFile) []string {
	var corpus InformationalProbeCorpus
	if err := common.Unmarshal(data, &corpus); err != nil {
		recordAuditInvalid(detail, "parse:"+err.Error())
		return []string{auditPathInformationalRealCorpus + ":invalid:parse:" + err.Error()}
	}
	if detail != nil {
		recordAuditCaseCount(detail, len(corpus.Cases))
	}
	missing := validateCorpusManualReview(auditPathInformationalRealCorpus, corpus.ManualReview, detail)
	missing = append(missing, validateInformationalCorpusCaseSources(auditPathInformationalRealCorpus, corpus.Cases, detail)...)
	metrics, invalid := evaluateInformationalProbeCorpusCasesForAudit(corpus)
	for _, value := range invalid {
		recordAuditInvalid(detail, "evaluate:"+value)
		missing = append(missing, auditPathInformationalRealCorpus+":invalid:evaluate:"+value)
	}
	if len(corpus.Cases) == 0 {
		recordAuditInvalid(detail, "empty_cases")
		missing = append(missing, auditPathInformationalRealCorpus+":invalid:empty_cases")
	}
	for _, requirement := range realInformationalCorpusMissingCoverage(metrics) {
		recordAuditCoverageGap(detail, requirement)
		missing = append(missing, auditPathInformationalRealCorpus+":missing_coverage:"+requirement)
	}
	for key, metric := range metrics {
		if metric.FalsePositive > 0 {
			recordAuditFalsePositive(detail, string(key))
			missing = append(missing, auditPathInformationalRealCorpus+":false_positive:"+string(key))
		}
		if metric.FalseNegative > 0 {
			recordAuditFalseNegative(detail, string(key))
			missing = append(missing, auditPathInformationalRealCorpus+":false_negative:"+string(key))
		}
	}
	return missing
}

func evaluateLabeledProbeCorpusCasesForAudit(corpus LabeledProbeCorpus) (map[CheckKey]goldenAccuracyMetric, []string) {
	probes := make(map[CheckKey]verifierProbe)
	for _, probe := range probeSuiteDefinitions(ProbeProfileFull) {
		probes[probe.Key] = probe
	}
	metrics := make(map[CheckKey]goldenAccuracyMetric)
	invalid := make([]string, 0)
	for _, item := range corpus.Cases {
		if strings.TrimSpace(item.Name) == "" {
			invalid = append(invalid, errCorpusCase(item, "missing name").Error())
			continue
		}
		probe, ok := probes[item.CheckKey]
		if !ok {
			invalid = append(invalid, errCorpusCase(item, "unknown check_key").Error())
			continue
		}
		if !isPassFailCorpusProbe(probe) {
			invalid = append(invalid, errCorpusCase(item, "check_key is not eligible for pass/fail corpus; use identity assessment corpus for neutral fingerprint probes").Error())
			continue
		}
		result := scoreLabeledProbeCorpusCase(probe, item)
		if item.WantErrorCode != "" && result.ErrorCode != item.WantErrorCode {
			invalid = append(invalid, errCorpusCase(item, "error_code mismatch").Error())
		}
		if item.WantSkipped != result.Skipped {
			invalid = append(invalid, errCorpusCase(item, "skipped mismatch").Error())
		}
		recordGoldenMetric(metrics, item.CheckKey, item.WantPassed && !item.WantSkipped, result)
	}
	return metrics, invalid
}

func evaluateInformationalProbeCorpusCasesForAudit(corpus InformationalProbeCorpus) (map[CheckKey]goldenAccuracyMetric, []string) {
	metrics := make(map[CheckKey]goldenAccuracyMetric)
	invalid := make([]string, 0)
	for _, item := range corpus.Cases {
		if strings.TrimSpace(item.Name) == "" {
			invalid = append(invalid, informationalCorpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: "missing name"}.Error())
			continue
		}
		if _, ok := informationalProbeCorpusCoverageRequirements()[item.CheckKey]; !ok {
			invalid = append(invalid, informationalCorpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: "check_key is not eligible for informational corpus"}.Error())
			continue
		}
		result := scoreInformationalProbeCorpusCase(item)
		if item.WantErrorCode != "" && result.ErrorCode != item.WantErrorCode {
			invalid = append(invalid, informationalCorpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: "error_code mismatch"}.Error())
		}
		recordGoldenMetric(metrics, item.CheckKey, item.WantPassed && !item.WantSkipped, result)
	}
	return metrics, invalid
}

func validateLabeledCorpusCaseSources(kind string, cases []LabeledProbeCorpusCase, detail *ProbeAccuracyEvidenceFile) []string {
	missing := make([]string, 0)
	for _, item := range cases {
		missing = append(missing, validateCorpusCaseSource(kind, item.Name, item.Source, detail)...)
	}
	return missing
}

func validateIdentityCorpusCaseSources(kind string, cases []IdentityAssessmentCorpusCase, detail *ProbeAccuracyEvidenceFile) []string {
	missing := make([]string, 0)
	for _, item := range cases {
		missing = append(missing, validateCorpusCaseSource(kind, item.Name, item.Source, detail)...)
	}
	return missing
}

func validateInformationalCorpusCaseSources(kind string, cases []InformationalProbeCorpusCase, detail *ProbeAccuracyEvidenceFile) []string {
	missing := make([]string, 0)
	for _, item := range cases {
		missing = append(missing, validateCorpusCaseSource(kind, item.Name, item.Source, detail)...)
	}
	return missing
}

func validateCorpusCaseSource(kind string, caseName string, source CorpusCaseSource, detail *ProbeAccuracyEvidenceFile) []string {
	caseName = strings.TrimSpace(caseName)
	if caseName == "" {
		caseName = "unnamed"
	}
	missing := make([]string, 0)
	for field, value := range map[string]string{
		"provider":    source.Provider,
		"model":       source.Model,
		"task_id":     source.TaskID,
		"captured_at": source.CapturedAt,
	} {
		if strings.TrimSpace(value) == "" {
			gap := caseName + ":" + field
			recordAuditCaseSourceGap(detail, gap)
			missing = append(missing, kind+":case_source:"+gap)
		}
	}
	if strings.TrimSpace(source.CapturedAt) != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(source.CapturedAt)); err != nil {
			gap := caseName + ":captured_at_format"
			recordAuditCaseSourceGap(detail, gap)
			missing = append(missing, kind+":case_source:"+gap)
		}
	}
	return missing
}

func validateCorpusManualReview(kind string, review CorpusManualReview, detail *ProbeAccuracyEvidenceFile) []string {
	missing := make([]string, 0)
	if strings.TrimSpace(review.Status) != corpusManualReviewStatusReviewed {
		recordAuditManualReviewGap(detail, "status")
		missing = append(missing, kind+":manual_review:status")
	}
	if strings.TrimSpace(review.Source) != corpusSourceRealModelOutput {
		recordAuditManualReviewGap(detail, "source")
		missing = append(missing, kind+":manual_review:source")
	}
	if strings.TrimSpace(review.ReviewedBy) == "" {
		recordAuditManualReviewGap(detail, "reviewed_by")
		missing = append(missing, kind+":manual_review:reviewed_by")
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(review.ReviewedAt)); err != nil {
		recordAuditManualReviewGap(detail, "reviewed_at")
		missing = append(missing, kind+":manual_review:reviewed_at")
	}
	return missing
}

func recordAuditManualReviewGap(detail *ProbeAccuracyEvidenceFile, value string) {
	if detail != nil {
		detail.ManualReviewMissing = append(detail.ManualReviewMissing, value)
	}
}

func recordAuditCaseCount(detail *ProbeAccuracyEvidenceFile, value int) {
	if detail != nil {
		detail.CaseCount = &value
	}
}

func recordAuditCaseSourceGap(detail *ProbeAccuracyEvidenceFile, value string) {
	if detail != nil {
		detail.CaseSourceMissing = append(detail.CaseSourceMissing, value)
	}
}

func recordAuditCoverageGap(detail *ProbeAccuracyEvidenceFile, value string) {
	if detail != nil {
		detail.MissingCoverage = append(detail.MissingCoverage, value)
	}
}

func recordAuditFalsePositive(detail *ProbeAccuracyEvidenceFile, value string) {
	if detail != nil {
		detail.FalsePositive = append(detail.FalsePositive, value)
	}
}

func recordAuditFalseNegative(detail *ProbeAccuracyEvidenceFile, value string) {
	if detail != nil {
		detail.FalseNegative = append(detail.FalseNegative, value)
	}
}

func recordAuditInvalid(detail *ProbeAccuracyEvidenceFile, value string) {
	if detail != nil {
		detail.Invalid = append(detail.Invalid, value)
	}
}

func buildProbeAccuracyJudgeAudit(judge *ProbeJudgeConfig, baseline BaselineMap) ProbeAccuracyJudgeAudit {
	audit := ProbeAccuracyJudgeAudit{
		ConfigPresent:   judge != nil,
		BaselinePresent: baseline != nil,
		RequiredProbes:  make([]string, 0),
		Missing:         make([]string, 0),
	}
	requirements := reviewOnlyProbeJudgeCoverageRequirements()
	if len(requirements) == 0 {
		return audit
	}
	if judge == nil {
		audit.Missing = append(audit.Missing, auditPathReviewOnlyJudge+":judge_config")
	}
	if baseline == nil {
		audit.Missing = append(audit.Missing, auditPathReviewOnlyJudge+":baseline_config")
	}
	for checkKey, probeID := range requirements {
		audit.RequiredProbes = append(audit.RequiredProbes, probeID)
		if strings.TrimSpace(probeID) == "" {
			audit.Missing = append(audit.Missing, string(checkKey)+":probe_id")
			continue
		}
		if strings.TrimSpace(baseline[probeID]) == "" {
			audit.Missing = append(audit.Missing, string(checkKey)+":baseline:"+probeID)
		}
	}
	return audit
}

func buildProbeAccuracyCoverageItems() []ProbeAccuracyCoverageItem {
	probes := runtimeProbeSuiteDefinitions(ProbeProfileFull)
	items := make([]ProbeAccuracyCoverageItem, 0, len(probes))
	for _, probe := range probes {
		paths := probeAccuracyAuditPaths(probe)
		item := ProbeAccuracyCoverageItem{
			CheckKey:   probe.Key,
			CheckName:  checkDisplayName(probe.Key),
			Group:      probe.Group,
			Neutral:    probe.Neutral,
			ReviewOnly: probe.ReviewOnly,
		}
		if probeID := sourceProbeIDForCheckKey(probe.Key); probeID != "" {
			item.ProbeID = probeID
		}
		switch len(paths) {
		case 0:
			item.MissingReason = "no_hard_accuracy_audit_path"
		case 1:
			item.AuditPath = paths[0]
			item.EvidenceRequirement = probeAccuracyEvidenceRequirement(probe, paths[0])
		default:
			item.MissingReason = "ambiguous_hard_accuracy_audit_paths:" + strings.Join(paths, ",")
		}
		items = append(items, item)
	}
	return items
}

func probeAccuracyEvidenceRequirement(probe verifierProbe, auditPath string) string {
	switch auditPath {
	case auditPathPassFailRealCorpus:
		return "manual_review.status=reviewed, manual_review.source=real_model_output, at least one replayable real labeled sample for " + string(probe.Key) + "; positive/negative scorer polarity is covered by schema golden corpus"
	case auditPathIdentityRealCorpus:
		return "manual_review.status=reviewed, manual_review.source=real_model_output, report-level identity corpus covering this neutral fingerprint check"
	case auditPathInformationalRealCorpus:
		return "manual_review.status=reviewed, manual_review.source=real_model_output, at least one positive and one negative real evidence sample for " + string(probe.Key)
	case auditPathReviewOnlyJudge:
		probeID := sourceProbeIDForCheckKey(probe.Key)
		if strings.TrimSpace(probeID) == "" {
			return "judge config and official baseline response for the review-only probe"
		}
		return "judge config and official baseline response for probe_id=" + probeID
	default:
		return ""
	}
}

func probeAccuracyAuditPaths(probe verifierProbe) []string {
	paths := make([]string, 0, 1)
	if auditPassFailCorpusProbe(probe) {
		paths = append(paths, auditPathPassFailRealCorpus)
	}
	if auditReviewOnlyProbeRequiresJudge(probe) {
		paths = append(paths, auditPathReviewOnlyJudge)
	}
	if auditInformationalProbeKey(probe.Key) {
		paths = append(paths, auditPathInformationalRealCorpus)
	}
	if isIdentityFeatureCheck(probe.Key) {
		paths = append(paths, auditPathIdentityRealCorpus)
	}
	return paths
}

func auditPassFailCorpusProbe(probe verifierProbe) bool {
	if probe.Neutral {
		return false
	}
	if probe.ReviewOnly && probe.Key != CheckProbeHallucination {
		return false
	}
	return true
}

func auditReviewOnlyProbeRequiresJudge(probe verifierProbe) bool {
	return probe.ReviewOnly && !auditPassFailCorpusProbe(probe)
}

func auditInformationalProbeKey(checkKey CheckKey) bool {
	switch checkKey {
	case CheckProbeChannelSignature, CheckProbeSignatureRoundtrip:
		return true
	default:
		return false
	}
}

func auditFileExists(baseDir string, relPath string) bool {
	if _, err := os.Stat(auditEvidencePath(baseDir, relPath)); err == nil {
		return true
	}
	const repoTestdataPrefix = "service/token_verifier/"
	if strings.TrimSpace(baseDir) == "" && strings.HasPrefix(relPath, repoTestdataPrefix) {
		if _, err := os.Stat(strings.TrimPrefix(relPath, repoTestdataPrefix)); err == nil {
			return true
		}
	}
	return false
}

func readAuditEvidenceFile(baseDir string, relPath string) ([]byte, error) {
	data, err := os.ReadFile(auditEvidencePath(baseDir, relPath))
	if err == nil {
		return data, nil
	}
	const repoTestdataPrefix = "service/token_verifier/"
	if strings.TrimSpace(baseDir) == "" && strings.HasPrefix(relPath, repoTestdataPrefix) {
		return os.ReadFile(strings.TrimPrefix(relPath, repoTestdataPrefix))
	}
	return nil, err
}

func auditEvidencePath(baseDir string, relPath string) string {
	if strings.TrimSpace(baseDir) != "" {
		return filepath.Join(baseDir, relPath)
	}
	return relPath
}
