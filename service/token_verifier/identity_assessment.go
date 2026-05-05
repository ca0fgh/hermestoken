package token_verifier

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

const (
	identityStatusMatch     = "match"
	identityStatusMismatch  = "mismatch"
	identityStatusUncertain = "uncertain"
)

type identityResultKey struct {
	provider string
	model    string
}

func buildIdentityAssessments(results []CheckResult) []IdentityAssessmentSummary {
	return buildIdentityAssessmentsWithOptions(context.Background(), results, ReportOptions{})
}

func buildIdentityAssessmentsWithOptions(ctx context.Context, results []CheckResult, options ReportOptions) []IdentityAssessmentSummary {
	grouped := make(map[identityResultKey][]CheckResult)
	for _, result := range results {
		modelName := strings.TrimSpace(result.ModelName)
		if modelName == "" {
			continue
		}
		provider := strings.TrimSpace(result.Provider)
		grouped[identityResultKey{provider: provider, model: modelName}] = append(grouped[identityResultKey{provider: provider, model: modelName}], result)
	}

	assessments := make([]IdentityAssessmentSummary, 0)
	for key, groupResults := range grouped {
		responses := identityProbeResponses(groupResults)
		if len(responses) == 0 {
			continue
		}
		claimedModel := identityClaimedModel(key.model)
		claimedFamily := claimedModelToIdentityFamily(claimedModel)
		behavioralCandidates := sourceBehavioralIdentityCandidates(groupResults, responses)
		candidates := behavioralCandidates
		sourceResponses := sourceResponseIDMap(responses)
		judgeCandidates, _ := runIdentityJudgeSignal(ctx, options, sourceResponses)
		vectorCandidates := runIdentityVectorSignal(ctx, options, sourceResponses)
		if len(judgeCandidates) > 0 || len(vectorCandidates) > 0 {
			candidates = fuseIdentityCandidates(behavioralCandidates, judgeCandidates, vectorCandidates)
		}
		candidates = stripIdentityCandidateReasons(candidates)
		submodelAssessments := buildSubmodelAssessments(claimedModel, claimedFamily, responses)
		assessment := deriveIdentityAssessmentWithSignals(key, claimedModel, claimedFamily, candidates, submodelAssessments, identityRiskFlags(groupResults), groupResults, responses)
		assessments = append(assessments, assessment)
	}

	sort.SliceStable(assessments, func(i, j int) bool {
		if assessments[i].Provider == assessments[j].Provider {
			return assessments[i].ModelName < assessments[j].ModelName
		}
		return assessments[i].Provider < assessments[j].Provider
	})
	return assessments
}

func runIdentityJudgeSignal(ctx context.Context, options ReportOptions, responses map[string]string) ([]IdentityCandidateSummary, *judgeIdentityResult) {
	if options.IdentityJudge == nil {
		return nil, nil
	}
	executor := options.Executor
	if executor == nil {
		executor = NewCurlExecutor(0)
	}
	return judgeFingerprint(ctx, executor, responses, *options.IdentityJudge)
}

func runIdentityVectorSignal(ctx context.Context, options ReportOptions, responses map[string]string) []IdentityCandidateSummary {
	if options.Embedding == nil || len(options.Embedding.References) == 0 {
		return nil
	}
	executor := options.Executor
	if executor == nil {
		executor = NewCurlExecutor(0)
	}
	embedding, ok := embedProbeResponses(ctx, executor, responses, *options.Embedding)
	if !ok {
		return nil
	}
	return pickTopVectorScores(embedding, options.Embedding.References)
}

func identityProbeResponses(results []CheckResult) map[CheckKey]string {
	responses := make(map[CheckKey]string)
	for _, result := range results {
		if !isIdentityFeatureCheck(result.CheckKey) {
			continue
		}
		if text := identityResponseText(result); text != "" {
			responses[result.CheckKey] = text
		}
	}
	return responses
}

func identityResponseText(result CheckResult) string {
	if text := strings.TrimSpace(result.PrivateResponseText); text != "" {
		return text
	}
	if result.Raw == nil {
		return ""
	}
	if sample, ok := result.Raw["response_sample"].(string); ok && strings.TrimSpace(sample) != "" {
		return strings.TrimSpace(sample)
	}
	if samples, ok := result.Raw["samples"].([]string); ok && len(samples) > 0 {
		return strings.Join(samples, "\n")
	}
	if samples, ok := result.Raw["samples"].([]any); ok && len(samples) > 0 {
		parts := make([]string, 0, len(samples))
		for _, sample := range samples {
			if text, ok := sample.(string); ok && strings.TrimSpace(text) != "" && text != "[redacted]" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	if distribution, ok := result.Raw["distribution"].(map[string]int); ok && len(distribution) > 0 {
		parts := make([]string, 0, len(distribution))
		for text := range distribution {
			if strings.TrimSpace(text) != "" && text != "[redacted]" {
				parts = append(parts, text)
			}
		}
		sort.Strings(parts)
		return strings.Join(parts, "\n")
	}
	if distribution, ok := result.Raw["distribution"].(map[string]any); ok && len(distribution) > 0 {
		parts := make([]string, 0, len(distribution))
		for text := range distribution {
			if strings.TrimSpace(text) != "" && text != "[redacted]" {
				parts = append(parts, text)
			}
		}
		sort.Strings(parts)
		return strings.Join(parts, "\n")
	}
	return ""
}

func identityClaimedModel(fallback string) string {
	return strings.TrimSpace(fallback)
}

func buildSubmodelAssessments(claimedModel string, claimedFamily string, responses map[CheckKey]string) []SubmodelAssessmentSummary {
	out := make([]SubmodelAssessmentSummary, 0, 3)
	if v3 := classifySubmodelV3Source(claimedFamily, responses); v3.Method != "" {
		out = append(out, v3)
	}
	if v3e := classifySubmodelV3ESource(claimedFamily, responses, false); v3e.Method != "" {
		out = append(out, v3e)
	}
	if v3f := classifySubmodelV3ESource(claimedFamily, responses, true); v3f.Method != "" {
		out = append(out, v3f)
	}
	return out
}

func deriveIdentityAssessment(key identityResultKey, claimedModel string, claimedFamily string, candidates []IdentityCandidateSummary, submodels []SubmodelAssessmentSummary, riskFlags []string) IdentityAssessmentSummary {
	return deriveIdentityAssessmentWithSignals(key, claimedModel, claimedFamily, candidates, submodels, riskFlags, nil, nil)
}

func deriveIdentityAssessmentWithSignals(key identityResultKey, claimedModel string, claimedFamily string, candidates []IdentityCandidateSummary, submodels []SubmodelAssessmentSummary, riskFlags []string, results []CheckResult, responses map[CheckKey]string) IdentityAssessmentSummary {
	top := topCandidate(candidates)
	predictedModel := ""
	for _, submodel := range submodels {
		if !submodel.Abstained && top.Family != "" && submodel.Family == top.Family && predictedModel == "" {
			predictedModel = firstNonEmptyString(submodel.DisplayName, submodel.ModelID)
		}
	}
	status, confidence, evidence := sourceIdentityVerdictFromCandidates(candidates, claimedFamily)
	if len(riskFlags) > 0 {
		confidence = math.Max(0, confidence-0.15*float64(min(len(riskFlags), 3)))
	}

	assessment := IdentityAssessmentSummary{
		Provider:            key.provider,
		ModelName:           key.model,
		ClaimedModel:        claimedModel,
		Status:              status,
		Confidence:          roundScore(confidence),
		PredictedFamily:     top.Family,
		PredictedModel:      predictedModel,
		Method:              "llmprobe_behavioral_fingerprint",
		Candidates:          candidates,
		SubmodelAssessments: submodels,
		RiskFlags:           riskFlags,
		Evidence:            uniqueStrings(evidence),
	}
	if responses != nil {
		verdict := computeIdentityVerdict(IdentityVerdictInput{ClaimedFamily: claimedFamily, ClaimedModel: claimedModel, Surface: surfaceIdentitySignal(responses), Behavior: behaviorIdentitySignalWithoutSelfClaim(results, responses), V3: v3IdentitySignal(submodels)})
		assessment.Verdict = &verdict
	}
	return assessment
}

func sourceIdentityVerdictFromCandidates(candidates []IdentityCandidateSummary, claimedFamily string) (string, float64, []string) {
	if len(candidates) == 0 {
		return identityStatusUncertain, 0, []string{"No behavioral signals detected"}
	}
	top := candidates[0]
	evidence := append([]string{}, firstNStrings(top.Reasons, 3)...)
	if claimedFamily == "" {
		return identityStatusUncertain, top.Score * 0.7, evidence
	}
	if top.Family == claimedFamily && top.Score > 0.5 {
		secondScore := 0.0
		if len(candidates) > 1 {
			secondScore = candidates[1].Score
		}
		margin := top.Score - secondScore
		return identityStatusMatch, math.Min(1, top.Score*(0.6+margin*0.4)), evidence
	}
	if top.Family != claimedFamily && top.Score > 0.4 {
		mismatchEvidence := []string{
			fmt.Sprintf("Behavior most consistent with %s (score: %.2f)", top.Model, top.Score),
			"Claimed family " + claimedFamily + " not in top candidates",
		}
		mismatchEvidence = append(mismatchEvidence, evidence...)
		return identityStatusMismatch, top.Score, mismatchEvidence
	}
	return identityStatusUncertain, top.Score * 0.5, evidence
}

func identityRiskFlags(results []CheckResult) []string {
	flags := make([]string, 0)
	for _, result := range results {
		if result.Skipped || result.Success {
			continue
		}
		if isWarningCheckResult(result) && result.CheckKey != CheckProbeConsistencyCache {
			continue
		}
		group := resultGroup(result)
		if group != probeGroupSecurity && group != probeGroupIntegrity {
			continue
		}
		message := result.Message
		if message == "" {
			message = result.ErrorCode
		}
		flags = append(flags, checkDisplayName(result.CheckKey)+": "+message)
	}
	return uniqueStrings(flags)
}

func claimedModelToIdentityFamily(model string) string {
	lower := strings.ToLower(strings.TrimSpace(model))
	switch {
	case hasAny(lower, "claude", "anthropic"):
		return "anthropic"
	case hasAny(lower, "gpt", "chatgpt", "openai") ||
		strings.HasPrefix(lower, "o1") ||
		strings.HasPrefix(lower, "o3") ||
		strings.HasPrefix(lower, "o4"):
		return "openai"
	case hasAny(lower, "qwen", "tongyi"):
		return "qwen"
	case hasAny(lower, "gemini", "bard", "google/gemini"):
		return "google"
	case strings.Contains(lower, "llama") || strings.Contains(lower, "meta/"):
		return "meta"
	case hasAny(lower, "mistral", "mixtral"):
		return "mistral"
	case hasAny(lower, "deepseek"):
		return "deepseek"
	case hasAny(lower, "glm", "zhipu", "z-ai"):
		return "zhipu"
	default:
		return ""
	}
}

func topCandidate(candidates []IdentityCandidateSummary) IdentityCandidateSummary {
	if len(candidates) == 0 {
		return IdentityCandidateSummary{}
	}
	return candidates[0]
}

func stripIdentityCandidateReasons(candidates []IdentityCandidateSummary) []IdentityCandidateSummary {
	if len(candidates) == 0 {
		return candidates
	}
	out := make([]IdentityCandidateSummary, len(candidates))
	for i, candidate := range candidates {
		out[i] = candidate
		out[i].Reasons = nil
	}
	return out
}

func responseFor(responses map[CheckKey]string, key CheckKey) string {
	return strings.TrimSpace(responses[key])
}

func hasAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func modelBareName(model string) string {
	value := strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndex(value, "/"); idx >= 0 {
		value = value[idx+1:]
	}
	value = regexp.MustCompile(`^\[.*?\]`).ReplaceAllString(value, "")
	value = strings.TrimSuffix(value, "-thinking")
	value = regexp.MustCompile(`(\d+)-(\d+)`).ReplaceAllString(value, "$1.$2")
	return value
}

func identityFamilyDisplayName(family string) string {
	switch family {
	case "anthropic":
		return "Anthropic / Claude"
	case "openai":
		return "OpenAI / GPT"
	case "google":
		return "Google / Gemini"
	case "qwen":
		return "Alibaba / Qwen"
	case "meta":
		return "Meta / Llama"
	case "mistral":
		return "Mistral AI"
	case "deepseek":
		return "DeepSeek"
	case "zhipu":
		return "Zhipu AI / GLM"
	default:
		return family
	}
}

func roundScore(score float64) float64 {
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*100) / 100
}

func firstNStrings(values []string, n int) []string {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[:n]
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var identityFeatureCheckKeySet = buildIdentityFeatureCheckKeys()

func isIdentityFeatureCheck(checkKey CheckKey) bool {
	return identityFeatureCheckKeySet[checkKey]
}

func buildIdentityFeatureCheckKeys() map[CheckKey]bool {
	probes := probeSuiteDefinitions(ProbeProfileFull)
	keys := make(map[CheckKey]bool, len(probes))
	for _, probe := range probes {
		if probe.Group == probeGroupIdentity || probe.Group == probeGroupSubmodel {
			keys[probe.Key] = true
		}
	}
	return keys
}
