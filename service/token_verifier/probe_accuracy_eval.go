package token_verifier

import (
	"net/http"
	"strings"
)

type goldenAccuracyMetric struct {
	Positive      int
	Negative      int
	TruePositive  int
	TrueNegative  int
	FalsePositive int
	FalseNegative int
	Skipped       int
}

type identityAccuracyMetric struct {
	Total               int
	StatusCorrect       int
	VerdictCorrect      int
	FamilyCorrect       int
	ModelCorrect        int
	IdentityRiskCorrect int
}

type realProbeCorpusCoverageRequirement struct {
	Total    int
	Positive int
	Negative int
}

func evaluateLabeledProbeCorpusCases(corpus LabeledProbeCorpus) (map[CheckKey]goldenAccuracyMetric, error) {
	probes := make(map[CheckKey]verifierProbe)
	for _, probe := range probeSuiteDefinitions(ProbeProfileFull) {
		probes[probe.Key] = probe
	}
	metrics := make(map[CheckKey]goldenAccuracyMetric)
	for _, item := range corpus.Cases {
		if strings.TrimSpace(item.Name) == "" {
			return nil, errCorpusCase(item, "missing name")
		}
		probe, ok := probes[item.CheckKey]
		if !ok {
			return nil, errCorpusCase(item, "unknown check_key")
		}
		if !isPassFailCorpusProbe(probe) {
			return nil, errCorpusCase(item, "check_key is not eligible for pass/fail corpus; use identity assessment corpus for neutral fingerprint probes")
		}
		result := scoreLabeledProbeCorpusCase(probe, item)
		if item.WantErrorCode != "" && result.ErrorCode != item.WantErrorCode {
			return nil, errCorpusCase(item, "error_code mismatch")
		}
		if item.WantSkipped != result.Skipped {
			return nil, errCorpusCase(item, "skipped mismatch")
		}
		recordGoldenMetric(metrics, item.CheckKey, item.WantPassed && !item.WantSkipped, result)
	}
	return metrics, nil
}

func LabeledProbeCorpusCaseMatchesCurrentScorer(item LabeledProbeCorpusCase) bool {
	probes := make(map[CheckKey]verifierProbe)
	for _, probe := range probeSuiteDefinitions(ProbeProfileFull) {
		probes[probe.Key] = probe
	}
	probe, ok := probes[item.CheckKey]
	if !ok || !isPassFailCorpusProbe(probe) || strings.TrimSpace(item.Name) == "" {
		return false
	}
	result := scoreLabeledProbeCorpusCase(probe, item)
	if item.WantErrorCode != "" && result.ErrorCode != item.WantErrorCode {
		return false
	}
	if item.WantSkipped != result.Skipped {
		return false
	}
	wantPassed := item.WantPassed && !item.WantSkipped
	return result.Passed == wantPassed
}

func realCorpusMissingCoverage(metrics map[CheckKey]goldenAccuracyMetric) []string {
	requirements := realCorpusCoverageRequirements()
	missing := make([]string, 0)
	for key, requirement := range requirements {
		metric := metrics[key]
		if requirement.Total > 0 && metric.Positive+metric.Negative < requirement.Total {
			missing = append(missing, string(key)+":sample")
		}
		if metric.Positive < requirement.Positive {
			missing = append(missing, string(key)+":positive")
		}
		if metric.Negative < requirement.Negative {
			missing = append(missing, string(key)+":negative")
		}
	}
	return missing
}

func realCorpusCoverageRequirements() map[CheckKey]realProbeCorpusCoverageRequirement {
	requirements := make(map[CheckKey]realProbeCorpusCoverageRequirement)
	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		if isPassFailCorpusProbe(probe) {
			requirements[probe.Key] = realProbeCorpusCoverageRequirement{Total: 1}
		}
	}
	return requirements
}

func realInformationalCorpusMissingCoverage(metrics map[CheckKey]goldenAccuracyMetric) []string {
	requirements := informationalProbeCorpusCoverageRequirements()
	missing := make([]string, 0)
	for key, requirement := range requirements {
		metric := metrics[key]
		if metric.Positive < requirement.Positive {
			missing = append(missing, string(key)+":positive")
		}
		if metric.Negative < requirement.Negative {
			missing = append(missing, string(key)+":negative")
		}
	}
	return missing
}

func informationalProbeCorpusCoverageRequirements() map[CheckKey]goldenAccuracyMetric {
	requirements := make(map[CheckKey]goldenAccuracyMetric)
	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		if isInformationalProbeCorpusKey(probe.Key) {
			requirements[probe.Key] = goldenAccuracyMetric{Positive: 1, Negative: 1}
		}
	}
	return requirements
}

func isPassFailCorpusProbe(probe verifierProbe) bool {
	if probe.Neutral {
		return false
	}
	if probe.ReviewOnly && probe.Key != CheckProbeHallucination {
		return false
	}
	return true
}

func reviewOnlyProbeJudgeAuditMissingRequirements(judge *ProbeJudgeConfig, baseline BaselineMap) []string {
	requirements := reviewOnlyProbeJudgeCoverageRequirements()
	if len(requirements) == 0 {
		return nil
	}
	missing := make([]string, 0)
	if judge == nil {
		missing = append(missing, "judge_config")
	}
	if baseline == nil {
		missing = append(missing, "baseline_config")
	}
	for checkKey, probeID := range requirements {
		if strings.TrimSpace(probeID) == "" {
			missing = append(missing, string(checkKey)+":probe_id")
			continue
		}
		if strings.TrimSpace(baseline[probeID]) == "" {
			missing = append(missing, string(checkKey)+":baseline:"+probeID)
		}
	}
	return missing
}

func reviewOnlyProbeJudgeCoverageRequirements() map[CheckKey]string {
	requirements := make(map[CheckKey]string)
	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		if probe.ReviewOnly && !isPassFailCorpusProbe(probe) {
			requirements[probe.Key] = sourceProbeIDForCheckKey(probe.Key)
		}
	}
	return requirements
}

func evaluateInformationalProbeCorpusCases(corpus InformationalProbeCorpus) (map[CheckKey]goldenAccuracyMetric, error) {
	metrics := make(map[CheckKey]goldenAccuracyMetric)
	for _, item := range corpus.Cases {
		if strings.TrimSpace(item.Name) == "" {
			return nil, informationalCorpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: "missing name"}
		}
		if _, ok := informationalProbeCorpusCoverageRequirements()[item.CheckKey]; !ok {
			return nil, informationalCorpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: "check_key is not eligible for informational corpus"}
		}
		result := scoreInformationalProbeCorpusCase(item)
		if item.WantErrorCode != "" && result.ErrorCode != item.WantErrorCode {
			return nil, informationalCorpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: "error_code mismatch"}
		}
		recordGoldenMetric(metrics, item.CheckKey, item.WantPassed && !item.WantSkipped, result)
	}
	return metrics, nil
}

func scoreInformationalProbeCorpusCase(item InformationalProbeCorpusCase) probeScoreResult {
	switch item.CheckKey {
	case CheckProbeChannelSignature:
		signature := classifyProbeChannelSignature(item.Headers, item.MessageID, item.RawBody)
		passed := signature.Channel == item.WantChannel
		return probeScoreResult{Passed: passed, Score: boolScore(passed)}
	case CheckProbeSignatureRoundtrip:
		passed := item.RoundtripStatus >= 200 && item.RoundtripStatus < 300 && item.ThinkingPresent
		errorCode := ""
		if !passed {
			errorCode = "signature_roundtrip_failed"
			if item.RoundtripStatus == http.StatusBadRequest && strings.Contains(strings.ToLower(item.RawBody), "invalid") {
				errorCode = "signature_rejected"
			}
		}
		return probeScoreResult{Passed: passed, Score: boolScore(passed), ErrorCode: errorCode}
	default:
		return probeScoreResult{Passed: false, ErrorCode: "unsupported_informational_probe"}
	}
}

func scoreLabeledProbeCorpusCase(probe verifierProbe, item LabeledProbeCorpusCase) probeScoreResult {
	if strings.TrimSpace(item.ExpectedExact) != "" {
		probe.ExpectedExact = item.ExpectedExact
	}
	switch probe.Key {
	case CheckProbeSSECompliance:
		result := checkProbeSSECompliance(item.RawSSE)
		return probeScoreResult{Passed: result.Passed, Score: boolScore(result.Passed)}
	case CheckProbeCacheDetection:
		passed := !isCacheHitHeaderValue(item.CacheHeader)
		return probeScoreResult{Passed: passed, Score: boolScore(passed)}
	case CheckProbeThinkingBlock:
		hasThinking, _ := parseProbeThinkingSSE(item.RawSSE)
		return probeScoreResult{Passed: hasThinking, Score: boolScore(hasThinking)}
	case CheckProbeConsistencyCache:
		return scoreConsistencyCacheSample(item.First, item.Second)
	case CheckProbeAdaptiveInjection:
		return scoreAdaptiveInjectionSample(item.Neutral, item.Trigger)
	case CheckProbeContextLength:
		levels := make([]contextLengthSampleLevel, 0, len(item.ContextLevels))
		for _, level := range item.ContextLevels {
			levels = append(levels, contextLengthSampleLevel{Chars: level.Chars, Found: level.Found, Total: level.Total})
		}
		return scoreContextLengthSample(levels)
	default:
		return scoreVerifierProbeDetailed(probe, item.ResponseText, item.Decoded)
	}
}

func recordGoldenMetric(metrics map[CheckKey]goldenAccuracyMetric, key CheckKey, wantPassed bool, result probeScoreResult) {
	metric := metrics[key]
	if wantPassed {
		metric.Positive++
		if result.Passed && !result.Skipped {
			metric.TruePositive++
		} else {
			metric.FalseNegative++
		}
	} else {
		metric.Negative++
		if result.Skipped {
			metric.Skipped++
		} else if result.Passed {
			metric.FalsePositive++
		} else {
			metric.TrueNegative++
		}
	}
	metrics[key] = metric
}

func boolScore(passed bool) int {
	if passed {
		return 100
	}
	return 0
}

func scoreConsistencyCacheSample(first string, second string) probeScoreResult {
	firstText := strings.TrimSpace(first)
	secondText := strings.TrimSpace(second)
	if firstText == "" || secondText == "" {
		return probeScoreResult{Passed: false, Score: 0, Skipped: true, ErrorCode: "consistency_unassessable", RiskLevel: "unknown"}
	}
	if sha256Hex(firstText) == sha256Hex(secondText) {
		return probeScoreResult{Passed: false, Score: 50, ErrorCode: "possible_cache_hit"}
	}
	return probeScoreResult{Passed: true, Score: 100}
}

func scoreAdaptiveInjectionSample(neutral string, trigger string) probeScoreResult {
	neutralText := strings.TrimSpace(neutral)
	triggerText := strings.TrimSpace(trigger)
	if neutralText == "" || triggerText == "" {
		return probeScoreResult{Passed: false, Score: 0, Skipped: true, ErrorCode: "adaptive_unassessable", RiskLevel: "unknown"}
	}
	if neutralText != triggerText {
		return probeScoreResult{Passed: false, Score: 0, ErrorCode: "adaptive_probe_diverged"}
	}
	return probeScoreResult{Passed: true, Score: 100}
}

type contextLengthSampleLevel struct {
	Chars int
	Found int
	Total int
}

func scoreContextLengthSample(levels []contextLengthSampleLevel) probeScoreResult {
	lastPass := 0
	firstFail := 0
	for _, level := range levels {
		passed := level.Total > 0 && float64(level.Found)/float64(level.Total) >= 0.8
		if !passed {
			firstFail = level.Chars
			break
		}
		lastPass = level.Chars
	}
	if firstFail == 0 {
		return probeScoreResult{Passed: true, Score: 100}
	}
	if lastPass == 0 {
		return probeScoreResult{Passed: false, Score: 0, ErrorCode: "context_smallest_failed"}
	}
	return probeScoreResult{Passed: false, Score: 50, ErrorCode: "context_truncated_warning"}
}

type informationalCorpusCaseError struct {
	name     string
	checkKey CheckKey
	reason   string
}

func (e informationalCorpusCaseError) Error() string {
	return "informational probe corpus case " + e.name + " (" + string(e.checkKey) + "): " + e.reason
}

func errCorpusCase(item LabeledProbeCorpusCase, reason string) error {
	return corpusCaseError{name: item.Name, checkKey: item.CheckKey, reason: reason}
}

type corpusCaseError struct {
	name     string
	checkKey CheckKey
	reason   string
}

func (e corpusCaseError) Error() string {
	return "labeled probe corpus case " + e.name + " (" + string(e.checkKey) + "): " + e.reason
}

func evaluateIdentityAssessmentCorpusCases(corpus IdentityAssessmentCorpus) (identityAccuracyMetric, error) {
	metric := identityAccuracyMetric{}
	for _, item := range corpus.Cases {
		if strings.TrimSpace(item.Name) == "" {
			return metric, identityCorpusCaseError{name: item.Name, reason: "missing name"}
		}
		if strings.TrimSpace(item.WantIdentityStatus) == "" {
			return metric, identityCorpusCaseError{name: item.Name, reason: "missing want_identity_status"}
		}
		if len(item.Results) == 0 {
			return metric, identityCorpusCaseError{name: item.Name, reason: "missing results"}
		}

		report := BuildReport(identityCorpusCheckResults(item.Results))
		if len(report.IdentityAssessments) == 0 {
			return metric, identityCorpusCaseError{name: item.Name, reason: "missing identity assessment"}
		}
		assessment := report.IdentityAssessments[0]
		metric.Total++

		if assessment.Status == item.WantIdentityStatus {
			metric.StatusCorrect++
		} else {
			return metric, identityCorpusCaseError{name: item.Name, reason: "identity status mismatch: got " + assessment.Status}
		}
		if item.WantVerdictStatus == "" || (assessment.Verdict != nil && assessment.Verdict.Status == item.WantVerdictStatus) {
			metric.VerdictCorrect++
		} else {
			got := ""
			if assessment.Verdict != nil {
				got = assessment.Verdict.Status
			}
			return metric, identityCorpusCaseError{name: item.Name, reason: "identity verdict mismatch: got " + got}
		}
		if item.WantNoPredictedFamily {
			if assessment.PredictedFamily != "" {
				return metric, identityCorpusCaseError{name: item.Name, reason: "predicted family should be empty: got " + assessment.PredictedFamily}
			}
			metric.FamilyCorrect++
		} else if item.WantPredictedFamily == "" || assessment.PredictedFamily == item.WantPredictedFamily {
			metric.FamilyCorrect++
		} else {
			return metric, identityCorpusCaseError{name: item.Name, reason: "predicted family mismatch: got " + assessment.PredictedFamily}
		}
		if item.WantNoPredictedModel {
			if assessment.PredictedModel != "" {
				return metric, identityCorpusCaseError{name: item.Name, reason: "predicted model should be empty: got " + assessment.PredictedModel}
			}
			metric.ModelCorrect++
		} else if item.WantPredictedModel == "" || assessment.PredictedModel == item.WantPredictedModel {
			metric.ModelCorrect++
		} else {
			return metric, identityCorpusCaseError{name: item.Name, reason: "predicted model mismatch: got " + assessment.PredictedModel}
		}
		if item.WantTopCandidateFamily != "" {
			if len(assessment.Candidates) == 0 || assessment.Candidates[0].Family != item.WantTopCandidateFamily {
				return metric, identityCorpusCaseError{name: item.Name, reason: "top candidate family mismatch"}
			}
		}
		if item.WantIdentityRisk == nil || hasIdentityMismatchRisk(report.Risks) == *item.WantIdentityRisk {
			metric.IdentityRiskCorrect++
		} else {
			return metric, identityCorpusCaseError{name: item.Name, reason: "identity risk mismatch"}
		}
		evidence := strings.Join(assessment.Evidence, "\n")
		for _, disallowed := range item.WantEvidenceNotContaining {
			if strings.Contains(evidence, disallowed) {
				return metric, identityCorpusCaseError{name: item.Name, reason: "identity evidence contains disallowed text: " + disallowed}
			}
		}
	}
	return metric, nil
}

func realIdentityCorpusMissingCoverage(corpus IdentityAssessmentCorpus) []string {
	seenStatus := make(map[string]bool)
	seenVerdict := make(map[string]bool)
	seenFamily := make(map[string]bool)
	seenCheckKey := make(map[CheckKey]bool)
	for _, item := range corpus.Cases {
		seenStatus[item.WantIdentityStatus] = true
		if item.WantVerdictStatus != "" {
			seenVerdict[item.WantVerdictStatus] = true
		}
		if item.WantPredictedFamily != "" {
			seenFamily[item.WantPredictedFamily] = true
		}
		for _, result := range item.Results {
			if isIdentityFeatureCheck(result.CheckKey) {
				seenCheckKey[result.CheckKey] = true
			}
		}
	}

	missing := make([]string, 0)
	for _, status := range []string{identityStatusMatch, identityStatusMismatch, identityStatusUncertain} {
		if !seenStatus[status] {
			missing = append(missing, "status:"+status)
		}
	}
	for _, verdict := range []string{"clean_match", "plain_mismatch", "insufficient_data"} {
		if !seenVerdict[verdict] {
			missing = append(missing, "verdict:"+verdict)
		}
	}
	for _, family := range []string{"openai", "anthropic"} {
		if !seenFamily[family] {
			missing = append(missing, "family:"+family)
		}
	}
	for _, checkKey := range identityFeatureCorpusCoverageRequirements() {
		if !seenCheckKey[checkKey] {
			missing = append(missing, "identity_check:"+string(checkKey))
		}
	}
	return missing
}

func identityFeatureCorpusCoverageRequirements() []CheckKey {
	keys := make([]CheckKey, 0)
	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		if isIdentityFeatureCheck(probe.Key) {
			keys = append(keys, probe.Key)
		}
	}
	return keys
}

func hasIdentityMismatchRisk(risks []string) bool {
	for _, risk := range risks {
		if strings.Contains(risk, "行为指纹与声明模型不一致") {
			return true
		}
	}
	return false
}

func identityCorpusCheckResults(results []IdentityAssessmentCorpusResult) []CheckResult {
	out := make([]CheckResult, 0, len(results))
	for _, result := range results {
		item := result.CheckResult
		item.PrivateResponseText = result.PrivateResponseText
		out = append(out, item)
	}
	return out
}

type identityCorpusCaseError struct {
	name   string
	reason string
}

func (e identityCorpusCaseError) Error() string {
	return "identity assessment corpus case " + e.name + ": " + e.reason
}
