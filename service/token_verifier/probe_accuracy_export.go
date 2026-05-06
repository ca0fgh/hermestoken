package token_verifier

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultLabeledProbeCorpusDraftDescription       = "Draft generated from direct probe results. Review every case against the real model output before saving as testdata/labeled_probe_corpus_real.json."
	defaultIdentityAssessmentCorpusDraftDescription = "Draft generated from direct probe results. Review every identity case against the real model output before saving as testdata/identity_assessment_corpus_real.json."
	defaultInformationalProbeCorpusDraftDescription = "Draft generated from direct probe results. Review every informational case against the real probe evidence before saving as testdata/informational_probe_corpus_real.json."
	defaultProbeBaselineDraftDescription            = "Draft generated from review-only probe results. Review each response before using as TOKEN_VERIFIER_PROBE_BASELINE_FILE."
	corpusManualReviewStatusDraft                   = "draft"
	corpusManualReviewStatusReviewed                = "reviewed"
	corpusSourceDetectorGeneratedDraft              = "detector_generated_draft"
	corpusSourceRealModelOutput                     = "real_model_output"
)

var generatedCorpusNameUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

type LabeledProbeCorpus struct {
	Description  string                   `json:"description"`
	ManualReview CorpusManualReview       `json:"manual_review,omitempty"`
	Cases        []LabeledProbeCorpusCase `json:"cases"`
}

type LabeledProbeCorpusCase struct {
	Name          string           `json:"name"`
	Source        CorpusCaseSource `json:"source,omitempty"`
	CheckKey      CheckKey         `json:"check_key"`
	ResponseText  string           `json:"response_text"`
	Decoded       map[string]any   `json:"decoded,omitempty"`
	RawSSE        string           `json:"raw_sse,omitempty"`
	CacheHeader   string           `json:"cache_header,omitempty"`
	First         string           `json:"first,omitempty"`
	Second        string           `json:"second,omitempty"`
	Neutral       string           `json:"neutral,omitempty"`
	Trigger       string           `json:"trigger,omitempty"`
	ContextLevels []struct {
		Chars int `json:"chars"`
		Found int `json:"found"`
		Total int `json:"total"`
	} `json:"context_levels,omitempty"`
	ExpectedExact string `json:"expected_exact,omitempty"`
	WantPassed    bool   `json:"want_passed"`
	WantSkipped   bool   `json:"want_skipped,omitempty"`
	WantErrorCode string `json:"want_error_code,omitempty"`
}

type IdentityAssessmentCorpus struct {
	Description  string                         `json:"description"`
	ManualReview CorpusManualReview             `json:"manual_review,omitempty"`
	Cases        []IdentityAssessmentCorpusCase `json:"cases"`
}

type IdentityAssessmentCorpusCase struct {
	Name                      string                           `json:"name"`
	Source                    CorpusCaseSource                 `json:"source,omitempty"`
	Results                   []IdentityAssessmentCorpusResult `json:"results"`
	WantIdentityStatus        string                           `json:"want_identity_status"`
	WantVerdictStatus         string                           `json:"want_verdict_status,omitempty"`
	WantPredictedFamily       string                           `json:"want_predicted_family,omitempty"`
	WantPredictedModel        string                           `json:"want_predicted_model,omitempty"`
	WantTopCandidateFamily    string                           `json:"want_top_candidate_family,omitempty"`
	WantIdentityRisk          *bool                            `json:"want_identity_risk,omitempty"`
	WantNoPredictedFamily     bool                             `json:"want_no_predicted_family,omitempty"`
	WantNoPredictedModel      bool                             `json:"want_no_predicted_model,omitempty"`
	WantEvidenceNotContaining []string                         `json:"want_evidence_not_containing,omitempty"`
}

type IdentityAssessmentCorpusResult struct {
	CheckResult
	PrivateResponseText string `json:"private_response_text,omitempty"`
}

type InformationalProbeCorpus struct {
	Description  string                         `json:"description"`
	ManualReview CorpusManualReview             `json:"manual_review,omitempty"`
	Cases        []InformationalProbeCorpusCase `json:"cases"`
}

type CorpusManualReview struct {
	Status     string `json:"status,omitempty"`
	Source     string `json:"source,omitempty"`
	ReviewedBy string `json:"reviewed_by,omitempty"`
	ReviewedAt string `json:"reviewed_at,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type InformationalProbeCorpusCase struct {
	Name            string            `json:"name"`
	Source          CorpusCaseSource  `json:"source,omitempty"`
	CheckKey        CheckKey          `json:"check_key"`
	Headers         map[string]string `json:"headers,omitempty"`
	MessageID       string            `json:"message_id,omitempty"`
	RawBody         string            `json:"raw_body,omitempty"`
	ThinkingPresent bool              `json:"thinking_present,omitempty"`
	RoundtripStatus int               `json:"roundtrip_status,omitempty"`
	WantChannel     string            `json:"want_channel,omitempty"`
	WantPassed      bool              `json:"want_passed"`
	WantSkipped     bool              `json:"want_skipped,omitempty"`
	WantErrorCode   string            `json:"want_error_code,omitempty"`
}

type CorpusCaseSource struct {
	Provider   string   `json:"provider,omitempty"`
	Model      string   `json:"model,omitempty"`
	CheckKey   CheckKey `json:"check_key,omitempty"`
	TaskID     string   `json:"task_id,omitempty"`
	CapturedAt string   `json:"captured_at,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

type ProbeBaselineCorpus struct {
	Description string                    `json:"description,omitempty"`
	Probes      []ProbeBaselineCorpusItem `json:"probes"`
}

type ProbeBaselineCorpusItem struct {
	ProbeID      string   `json:"probeId"`
	ResponseText string   `json:"responseText"`
	CheckKey     CheckKey `json:"checkKey,omitempty"`
	ModelName    string   `json:"modelName,omitempty"`
}

// BuildLabeledProbeCorpusDraftFromResults converts captured probe results into a
// manually reviewable corpus draft. The current detector verdict is copied as an
// initial label, but the draft must be reviewed before it is used as empirical evidence.
func BuildLabeledProbeCorpusDraftFromResults(description string, results []CheckResult, secrets ...string) LabeledProbeCorpus {
	return BuildLabeledProbeCorpusDraftFromResultsWithSource(description, results, CorpusCaseSource{}, secrets...)
}

func BuildLabeledProbeCorpusDraftFromResultsWithSource(description string, results []CheckResult, source CorpusCaseSource, secrets ...string) LabeledProbeCorpus {
	description = strings.TrimSpace(description)
	if description == "" {
		description = defaultLabeledProbeCorpusDraftDescription
	}
	redactedResults := redactCheckResultsForCorpusDraft(results, secrets)
	cases := make([]LabeledProbeCorpusCase, 0, len(redactedResults))
	seen := make(map[string]int)
	for _, result := range redactedResults {
		if !isScoredCorpusDraftResult(result) {
			continue
		}
		item := LabeledProbeCorpusCase{
			Name:        generatedCorpusCaseName(result, seen),
			Source:      mergeCorpusCaseSource(corpusCaseSourceFromResult(result), source),
			CheckKey:    result.CheckKey,
			WantPassed:  result.Success && !result.Skipped,
			WantSkipped: result.Skipped,
		}
		populateCorpusDraftCaseFields(&item, result)
		cases = append(cases, item)
	}
	return LabeledProbeCorpus{
		Description:  description,
		ManualReview: draftCorpusManualReview(),
		Cases:        cases,
	}
}

func MarshalLabeledProbeCorpusDraft(corpus LabeledProbeCorpus) ([]byte, error) {
	return json.MarshalIndent(corpus, "", "  ")
}

func BuildIdentityAssessmentCorpusDraftFromDirectProbeResponse(description string, response DirectProbeResponse, secrets ...string) IdentityAssessmentCorpus {
	description = strings.TrimSpace(description)
	if description == "" {
		description = defaultIdentityAssessmentCorpusDraftDescription
	}
	redactedResults := redactCheckResultsForCorpusDraft(response.Results, secrets)
	report := BuildReport(redactedResults)
	if len(report.IdentityAssessments) == 0 {
		report = response.Report
	}
	seen := make(map[string]int)
	cases := make([]IdentityAssessmentCorpusCase, 0, len(report.IdentityAssessments))
	for _, assessment := range report.IdentityAssessments {
		results := identityCorpusResultsForAssessment(redactedResults, assessment)
		if len(results) == 0 {
			continue
		}
		item := IdentityAssessmentCorpusCase{
			Name:                   generatedIdentityCorpusCaseName(assessment, seen),
			Source:                 mergeCorpusCaseSource(corpusCaseSourceFromResults(results), corpusCaseSourceFromDirectProbeResponse(response)),
			Results:                results,
			WantIdentityStatus:     assessment.Status,
			WantPredictedFamily:    assessment.PredictedFamily,
			WantPredictedModel:     assessment.PredictedModel,
			WantTopCandidateFamily: topIdentityCandidateFamily(assessment),
			WantIdentityRisk:       boolPtr(hasIdentityMismatchRiskText(report.Risks)),
			WantNoPredictedFamily:  assessment.PredictedFamily == "",
			WantNoPredictedModel:   assessment.PredictedModel == "",
		}
		if assessment.Verdict != nil {
			item.WantVerdictStatus = assessment.Verdict.Status
		}
		if assessment.Status == identityStatusUncertain {
			item.WantEvidenceNotContaining = []string{"Behavior most consistent", "not in top candidates", "score: 1.00"}
		}
		cases = append(cases, item)
	}
	return IdentityAssessmentCorpus{
		Description:  description,
		ManualReview: draftCorpusManualReview(),
		Cases:        cases,
	}
}

func MarshalIdentityAssessmentCorpusDraft(corpus IdentityAssessmentCorpus) ([]byte, error) {
	return json.MarshalIndent(corpus, "", "  ")
}

func BuildInformationalProbeCorpusDraftFromResults(description string, results []CheckResult, secrets ...string) InformationalProbeCorpus {
	return BuildInformationalProbeCorpusDraftFromResultsWithSource(description, results, CorpusCaseSource{}, secrets...)
}

func BuildInformationalProbeCorpusDraftFromResultsWithSource(description string, results []CheckResult, source CorpusCaseSource, secrets ...string) InformationalProbeCorpus {
	description = strings.TrimSpace(description)
	if description == "" {
		description = defaultInformationalProbeCorpusDraftDescription
	}
	redactedResults := redactCheckResultsForCorpusDraft(results, secrets)
	cases := make([]InformationalProbeCorpusCase, 0)
	seen := make(map[string]int)
	for _, result := range redactedResults {
		if !isInformationalProbeCorpusKey(result.CheckKey) {
			continue
		}
		item := InformationalProbeCorpusCase{
			Name:        generatedCorpusCaseName(result, seen),
			Source:      mergeCorpusCaseSource(corpusCaseSourceFromResult(result), source),
			CheckKey:    result.CheckKey,
			WantPassed:  result.Success && !result.Skipped,
			WantSkipped: result.Skipped,
		}
		populateInformationalCorpusDraftCaseFields(&item, result)
		cases = append(cases, item)
	}
	return InformationalProbeCorpus{Description: description, ManualReview: draftCorpusManualReview(), Cases: cases}
}

func MarshalInformationalProbeCorpusDraft(corpus InformationalProbeCorpus) ([]byte, error) {
	return json.MarshalIndent(corpus, "", "  ")
}

func BuildProbeBaselineDraftFromResults(description string, results []CheckResult, secrets ...string) ProbeBaselineCorpus {
	description = strings.TrimSpace(description)
	if description == "" {
		description = defaultProbeBaselineDraftDescription
	}
	redactedResults := redactCheckResultsForCorpusDraft(results, secrets)
	probes := make([]ProbeBaselineCorpusItem, 0)
	seen := make(map[string]bool)
	for _, result := range redactedResults {
		probe, ok := probeDefinitionByCheckKey(result.CheckKey)
		if !ok || !probe.ReviewOnly || isPassFailProbeCorpusEligible(probe) {
			continue
		}
		if !result.Success {
			continue
		}
		if result.Skipped && result.ErrorCode != "judge_unconfigured" && result.ErrorCode != "judge_unparseable" {
			continue
		}
		probeID := sourceProbeIDForCheckKey(result.CheckKey)
		if probeID == "" || seen[probeID] {
			continue
		}
		responseText := corpusDraftResponseText(result)
		if strings.TrimSpace(responseText) == "" {
			continue
		}
		probes = append(probes, ProbeBaselineCorpusItem{
			ProbeID:      probeID,
			ResponseText: responseText,
			CheckKey:     result.CheckKey,
			ModelName:    result.ModelName,
		})
		seen[probeID] = true
	}
	return ProbeBaselineCorpus{Description: description, Probes: probes}
}

func MarshalProbeBaselineDraft(corpus ProbeBaselineCorpus) ([]byte, error) {
	return json.MarshalIndent(corpus, "", "  ")
}

func draftCorpusManualReview() CorpusManualReview {
	return CorpusManualReview{
		Status: corpusManualReviewStatusDraft,
		Source: corpusSourceDetectorGeneratedDraft,
	}
}

func corpusCaseSourceFromResult(result CheckResult) CorpusCaseSource {
	return CorpusCaseSource{
		Provider: result.Provider,
		Model:    result.ModelName,
		CheckKey: result.CheckKey,
	}
}

func corpusCaseSourceFromResults(results []IdentityAssessmentCorpusResult) CorpusCaseSource {
	if len(results) == 0 {
		return CorpusCaseSource{}
	}
	return corpusCaseSourceFromResult(results[0].CheckResult)
}

func corpusCaseSourceFromDirectProbeResponse(response DirectProbeResponse) CorpusCaseSource {
	return CorpusCaseSource{
		Provider:   response.Provider,
		Model:      response.Model,
		TaskID:     response.SourceTaskID,
		CapturedAt: response.CapturedAt,
	}
}

func mergeCorpusCaseSource(primary CorpusCaseSource, fallback CorpusCaseSource) CorpusCaseSource {
	out := primary
	if strings.TrimSpace(out.Provider) == "" {
		out.Provider = fallback.Provider
	}
	if strings.TrimSpace(out.Model) == "" {
		out.Model = fallback.Model
	}
	if out.CheckKey == "" {
		out.CheckKey = fallback.CheckKey
	}
	if strings.TrimSpace(out.TaskID) == "" {
		out.TaskID = fallback.TaskID
	}
	if strings.TrimSpace(out.CapturedAt) == "" {
		out.CapturedAt = fallback.CapturedAt
	}
	if strings.TrimSpace(out.Notes) == "" {
		out.Notes = fallback.Notes
	}
	return out
}

func CorpusRFC3339FromUnixForExport(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func redactCheckResultsForCorpusDraft(results []CheckResult, secrets []string) []CheckResult {
	normalizedSecrets := normalizeRedactionSecrets(secrets)
	out := make([]CheckResult, len(results))
	for i, result := range results {
		out[i] = result
		out[i].Provider = redactSecretString(result.Provider, normalizedSecrets)
		out[i].Group = redactSecretString(result.Group, normalizedSecrets)
		out[i].ModelName = redactSecretString(result.ModelName, normalizedSecrets)
		out[i].ErrorCode = redactSecretString(result.ErrorCode, normalizedSecrets)
		out[i].Message = redactSecretString(result.Message, normalizedSecrets)
		out[i].RiskLevel = redactSecretString(result.RiskLevel, normalizedSecrets)
		out[i].Evidence = redactSecretStrings(result.Evidence, normalizedSecrets)
		out[i].PrivateResponseText = redactSecretString(result.PrivateResponseText, normalizedSecrets)
		out[i].Raw = redactSecretMap(result.Raw, normalizedSecrets)
	}
	return out
}

func isScoredCorpusDraftResult(result CheckResult) bool {
	if result.CheckKey == "" || result.Skipped {
		return false
	}
	probe, ok := probeDefinitionByCheckKey(result.CheckKey)
	if !ok || !isPassFailProbeCorpusEligible(probe) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(result.RiskLevel), "unknown") {
		return false
	}
	return true
}

func isPassFailProbeCorpusEligible(probe verifierProbe) bool {
	if probe.Neutral {
		return false
	}
	if probe.ReviewOnly && probe.Key != CheckProbeHallucination {
		return false
	}
	return true
}

func probeDefinitionByCheckKey(checkKey CheckKey) (verifierProbe, bool) {
	for _, probe := range probeSuiteDefinitions(ProbeProfileFull) {
		if probe.Key == checkKey {
			return probe, true
		}
	}
	return verifierProbe{}, false
}

func generatedCorpusCaseName(result CheckResult, seen map[string]int) string {
	model := sanitizeCorpusNamePart(result.ModelName)
	if model == "" {
		model = sanitizeCorpusNamePart(result.Provider)
	}
	if model == "" {
		model = "result"
	}
	base := model + "_" + sanitizeCorpusNamePart(string(result.CheckKey))
	seen[base]++
	return fmt.Sprintf("%s_%d", base, seen[base])
}

func sanitizeCorpusNamePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = generatedCorpusNameUnsafeChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	return value
}

func populateCorpusDraftCaseFields(item *LabeledProbeCorpusCase, result CheckResult) {
	switch result.CheckKey {
	case CheckProbeTokenInflation:
		item.ResponseText = corpusDraftResponseText(result)
		item.Decoded = corpusDraftDecodedUsage(result)
	case CheckProbeSSECompliance:
		item.RawSSE = stringFromMap(result.Raw, "raw_sse")
	case CheckProbeCacheDetection:
		item.CacheHeader = corpusDraftCacheHeader(result)
		item.ResponseText = corpusDraftResponseText(result)
		item.Decoded = corpusDraftDecodedUsage(result)
	case CheckProbeThinkingBlock:
		item.RawSSE = stringFromMap(result.Raw, "raw_sse")
		if item.RawSSE == "" && result.Provider != ProviderAnthropic {
			item.ResponseText = corpusDraftResponseText(result)
		}
	case CheckProbeConsistencyCache:
		item.First = firstNonEmptyString(
			stringFromMap(result.Raw, "first_response"),
			stringFromMap(result.Raw, "first"),
		)
		item.Second = firstNonEmptyString(
			stringFromMap(result.Raw, "second_response"),
			stringFromMap(result.Raw, "second"),
		)
	case CheckProbeAdaptiveInjection:
		item.Neutral = firstNonEmptyString(
			stringFromMap(result.Raw, "neutral_response"),
			stringFromMap(result.Raw, "neutral"),
		)
		item.Trigger = firstNonEmptyString(
			stringFromMap(result.Raw, "trigger_response"),
			stringFromMap(result.Raw, "trigger"),
		)
	case CheckProbeContextLength:
		item.ContextLevels = corpusDraftContextLevels(result.Raw)
	default:
		item.ResponseText = corpusDraftResponseText(result)
		item.Decoded = corpusDraftDecodedUsage(result)
	}
	if expected := stringFromMap(result.Raw, "expected_exact"); expected != "" {
		item.ExpectedExact = expected
	}
}

func populateInformationalCorpusDraftCaseFields(item *InformationalProbeCorpusCase, result CheckResult) {
	switch result.CheckKey {
	case CheckProbeChannelSignature:
		item.WantChannel = stringFromMap(result.Raw, "channel")
		item.Headers = stringMapFromRaw(result.Raw, "headers")
		item.MessageID = stringFromMap(result.Raw, "message_id")
		item.RawBody = stringFromMap(result.Raw, "raw_body")
	case CheckProbeSignatureRoundtrip:
		item.ThinkingPresent = boolFromMap(result.Raw, "thinking_present")
		item.RoundtripStatus = intFromMap(result.Raw, "roundtrip_status")
		item.RawBody = stringFromMap(result.Raw, "raw_body")
	}
}

func corpusDraftResponseText(result CheckResult) string {
	if text := strings.TrimSpace(result.PrivateResponseText); text != "" {
		return result.PrivateResponseText
	}
	if result.Raw == nil {
		return ""
	}
	for _, key := range []string{"response_text", "response_sample"} {
		if text := strings.TrimSpace(stringFromMap(result.Raw, key)); text != "" {
			return stringFromMap(result.Raw, key)
		}
	}
	if samples, ok := result.Raw["samples"].([]string); ok && len(samples) > 0 {
		return strings.Join(samples, "\n---\n")
	}
	if samples, ok := result.Raw["samples"].([]any); ok && len(samples) > 0 {
		values := make([]string, 0, len(samples))
		for _, sample := range samples {
			if text, ok := sample.(string); ok && strings.TrimSpace(text) != "" {
				values = append(values, text)
			}
		}
		if len(values) > 0 {
			return strings.Join(values, "\n---\n")
		}
	}
	return ""
}

func corpusDraftDecodedUsage(result CheckResult) map[string]any {
	usage := usageMapFromRaw(result.Raw)
	if usage == nil {
		usage = make(map[string]any)
	}
	if result.InputTokens != nil {
		if _, ok := usage["prompt_tokens"]; !ok {
			usage["prompt_tokens"] = *result.InputTokens
		}
		if _, ok := usage["input_tokens"]; !ok && result.Provider == ProviderAnthropic {
			usage["input_tokens"] = *result.InputTokens
		}
	}
	if result.OutputTokens != nil {
		if _, ok := usage["completion_tokens"]; !ok {
			usage["completion_tokens"] = *result.OutputTokens
		}
		if _, ok := usage["output_tokens"]; !ok && result.Provider == ProviderAnthropic {
			usage["output_tokens"] = *result.OutputTokens
		}
	}
	if len(usage) == 0 {
		return nil
	}
	keys := make([]string, 0, len(usage))
	for key := range usage {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	orderedUsage := make(map[string]any, len(usage))
	for _, key := range keys {
		orderedUsage[key] = usage[key]
	}
	return map[string]any{"usage": orderedUsage}
}

func usageMapFromRaw(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	usage, ok := raw["usage"].(map[string]any)
	if !ok || len(usage) == 0 {
		return nil
	}
	out := make(map[string]any, len(usage))
	for key, value := range usage {
		out[key] = value
	}
	return out
}

func corpusDraftCacheHeader(result CheckResult) string {
	return firstNonEmptyString(
		stringFromMap(result.Raw, "header_value"),
		stringFromMap(result.Raw, "cache_header"),
		stringFromMap(result.Raw, "x-cache"),
	)
}

func corpusDraftContextLevels(raw map[string]any) []struct {
	Chars int `json:"chars"`
	Found int `json:"found"`
	Total int `json:"total"`
} {
	items, ok := raw["length_results"].([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	levels := make([]struct {
		Chars int `json:"chars"`
		Found int `json:"found"`
		Total int `json:"total"`
	}, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		chars, charsOK := numberFromAny(entry["chars"])
		found, foundOK := numberFromAny(firstPresentMapValue(entry, "found_canaries", "found"))
		total, totalOK := numberFromAny(firstPresentMapValue(entry, "total_canaries", "total"))
		if !charsOK || !foundOK || !totalOK {
			continue
		}
		levels = append(levels, struct {
			Chars int `json:"chars"`
			Found int `json:"found"`
			Total int `json:"total"`
		}{Chars: chars, Found: found, Total: total})
	}
	return levels
}

func firstPresentMapValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func anyMapFromRaw(values map[string]any, key string) map[string]any {
	if values == nil {
		return nil
	}
	if value, ok := values[key].(map[string]any); ok {
		return value
	}
	return nil
}

func stringMapFromRaw(values map[string]any, key string) map[string]string {
	if values == nil {
		return nil
	}
	if typed, ok := values[key].(map[string]string); ok {
		if len(typed) == 0 {
			return nil
		}
		out := make(map[string]string, len(typed))
		for name, value := range typed {
			out[name] = value
		}
		return out
	}
	return stringMapFromAnyMap(anyMapFromRaw(values, key))
}

func stringMapFromAnyMap(values map[string]any) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		if text, ok := value.(string); ok {
			out[key] = text
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func boolFromMap(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	if value, ok := values[key].(bool); ok {
		return value
	}
	return false
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	value, ok := numberFromAny(values[key])
	if !ok {
		return 0
	}
	return value
}

func isInformationalProbeCorpusKey(checkKey CheckKey) bool {
	switch checkKey {
	case CheckProbeChannelSignature, CheckProbeSignatureRoundtrip:
		return true
	default:
		return false
	}
}

func isSensitiveProbeKey(key CheckKey) bool {
	for _, probe := range probeSuiteDefinitions(ProbeProfileFull) {
		if probe.Key == key {
			return probe.Sensitive
		}
	}
	return false
}

func identityCorpusResultsForAssessment(results []CheckResult, assessment IdentityAssessmentSummary) []IdentityAssessmentCorpusResult {
	out := make([]IdentityAssessmentCorpusResult, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Provider) != strings.TrimSpace(assessment.Provider) ||
			strings.TrimSpace(result.ModelName) != strings.TrimSpace(assessment.ModelName) {
			continue
		}
		if !isIdentityFeatureCheck(result.CheckKey) && !isIdentityRiskContextResult(result) {
			continue
		}
		out = append(out, IdentityAssessmentCorpusResult{
			CheckResult:         result,
			PrivateResponseText: result.PrivateResponseText,
		})
	}
	return out
}

func isIdentityRiskContextResult(result CheckResult) bool {
	if result.Skipped || result.Success {
		return false
	}
	if isWarningCheckResult(result) && result.CheckKey != CheckProbeConsistencyCache {
		return false
	}
	group := resultGroup(result)
	return group == probeGroupSecurity || group == probeGroupIntegrity
}

func generatedIdentityCorpusCaseName(assessment IdentityAssessmentSummary, seen map[string]int) string {
	model := sanitizeCorpusNamePart(assessment.ModelName)
	if model == "" {
		model = sanitizeCorpusNamePart(assessment.Provider)
	}
	if model == "" {
		model = "identity"
	}
	base := model + "_identity_assessment"
	seen[base]++
	return fmt.Sprintf("%s_%d", base, seen[base])
}

func topIdentityCandidateFamily(assessment IdentityAssessmentSummary) string {
	if len(assessment.Candidates) == 0 {
		return ""
	}
	return assessment.Candidates[0].Family
}

func boolPtr(value bool) *bool {
	return &value
}

func hasIdentityMismatchRiskText(risks []string) bool {
	for _, risk := range risks {
		if strings.Contains(risk, "行为指纹与声明模型不一致") {
			return true
		}
	}
	return false
}
