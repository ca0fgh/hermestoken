package token_verifier

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	identityStatusMatch            = "match"
	identityStatusMismatch         = "mismatch"
	identityStatusUncertain        = "uncertain"
	identityStatusInsufficientData = "insufficient_data"
)

type identityResultKey struct {
	provider string
	model    string
}

type familyScoreAccumulator struct {
	family  string
	model   string
	score   float64
	reasons []string
}

type familySignalSet map[string]*familyScoreAccumulator

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
		claimedModel := identityClaimedModel(key.model, groupResults)
		claimedFamily := claimedModelToIdentityFamily(claimedModel)
		behavioralCandidates := sourceBehavioralIdentityCandidates(groupResults, responses)
		if len(behavioralCandidates) == 0 {
			behavioralCandidates = behavioralIdentityCandidates(responses)
		}
		candidates := behavioralCandidates
		sourceResponses := sourceResponseIDMap(responses)
		judgeCandidates, _ := runIdentityJudgeSignal(ctx, options, sourceResponses)
		vectorCandidates := runIdentityVectorSignal(ctx, options, sourceResponses)
		if len(judgeCandidates) > 0 || len(vectorCandidates) > 0 {
			candidates = fuseIdentityCandidates(behavioralCandidates, judgeCandidates, vectorCandidates)
		}
		submodelAssessments := buildSubmodelAssessments(claimedModel, claimedFamily, responses)
		submodelAssessments = applySubmodelAbstainGuard(submodelAssessments, candidates, claimedFamily)
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
	if options.Embedding == nil {
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

func identityClaimedModel(fallback string, results []CheckResult) string {
	for _, result := range results {
		if result.CheckKey == CheckModelIdentity && strings.TrimSpace(result.ClaimedModel) != "" {
			return strings.TrimSpace(result.ClaimedModel)
		}
	}
	return strings.TrimSpace(fallback)
}

func behavioralIdentityCandidates(responses map[CheckKey]string) []IdentityCandidateSummary {
	signals := make(familySignalSet)
	add := func(family string, score float64, reason string) {
		signals.add(family, "", score, reason)
	}

	selfText := lowerProbeText(responses, CheckProbeIdentitySelfKnowledge, CheckProbeMetaCreator, CheckProbeTokenSelfKnowledge)
	switch {
	case hasAny(selfText, "claude", "anthropic"):
		add("anthropic", 2.2, "self-claim references Claude/Anthropic")
	case hasAny(selfText, "chatgpt", "gpt-5", "gpt-4", "gpt-3", "openai", "o1", "o3"):
		add("openai", 2.2, "self-claim references OpenAI/GPT")
	case hasAny(selfText, "gemini", "google deepmind", "bard"):
		add("google", 2.0, "self-claim references Gemini/Google")
	case hasAny(selfText, "qwen", "通义", "alibaba", "阿里"):
		add("qwen", 2.0, "self-claim references Qwen/Alibaba")
	case hasAny(selfText, "llama", "meta ai", "meta llama"):
		add("meta", 1.8, "self-claim references Llama/Meta")
	case hasAny(selfText, "mistral"):
		add("mistral", 1.8, "self-claim references Mistral")
	case hasAny(selfText, "deepseek"):
		add("deepseek", 1.8, "self-claim references DeepSeek")
	}

	styleText := lowerProbeText(responses, CheckProbeIdentityStyleEN, CheckProbeIdentityStyleZHTW)
	if strings.HasPrefix(strings.TrimSpace(styleText), "certainly") || strings.HasPrefix(strings.TrimSpace(styleText), "of course") {
		add("openai", 0.55, "OpenAI-like helpful opening")
	}
	if strings.Count(styleText, "**") >= 2 {
		add("anthropic", 0.35, "markdown heading style detected")
	}
	if strings.Contains(styleText, "i can't verify") || strings.Contains(styleText, "i cannot verify") {
		add("anthropic", 0.45, "careful verification caveat")
	}

	reasonText := lowerProbeText(responses, CheckProbeIdentityReasoningShape)
	if strings.HasPrefix(strings.TrimSpace(reasonText), "let's") || strings.HasPrefix(strings.TrimSpace(reasonText), "let me") {
		add("openai", 0.5, "step-by-step opening style")
	}
	if strings.Contains(reasonText, "therefore") || strings.Contains(reasonText, "thus") {
		add("anthropic", 0.25, "formal reasoning connector")
	}

	refusalText := lowerProbeText(responses, CheckProbeIdentityRefusalPattern, CheckProbeSubmodelRefusal, CheckProbeRefusalL3, CheckProbeRefusalL8)
	switch {
	case hasAny(refusalText, "i'm not able to assist", "i'm unable to assist", "i can't assist with", "i can't help with", "isn't something i'll help", "i won't help with"):
		add("anthropic", 1.4, "Anthropic-like refusal phrasing")
	case hasAny(refusalText, "i'm sorry, but i cannot", "i apologize, but", "i'm sorry, i can't"):
		add("openai", 1.1, "OpenAI-like apology refusal phrasing")
	case hasAny(refusalText, "i cannot fulfill", "cannot comply with"):
		add("google", 0.85, "Google-like compliance refusal phrasing")
	case hasAny(refusalText, "i cannot provide", "无法提供"):
		add("qwen", 0.75, "Qwen-like direct refusal phrasing")
	}
	if strings.Contains(refusalText, "988") || strings.Contains(refusalText, "18 u.s.c") || strings.Contains(refusalText, "18 usc") {
		add("anthropic", 0.65, "safety resource/legal citation in refusal")
	}

	jsonText := strings.TrimSpace(responseFor(responses, CheckProbeIdentityJSONDiscipline))
	if jsonText != "" && strings.HasPrefix(jsonText, "{") && strings.HasSuffix(jsonText, "}") && !strings.Contains(jsonText, "```") {
		add("openai", 0.25, "pure JSON discipline")
	}
	listText := responseFor(responses, CheckProbeIdentityListFormat)
	if regexp.MustCompile(`(?m)^\d+\.\s`).MatchString(listText) {
		add("openai", 0.25, "plain numbered list format")
	}
	if strings.Contains(listText, "**") {
		add("anthropic", 0.25, "bold list heading format")
	}

	return signals.candidates(3)
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

func classifySubmodelV3Like(claimedModel string, claimedFamily string, responses map[CheckKey]string) SubmodelAssessmentSummary {
	cutoff := strings.TrimSpace(responseFor(responses, CheckProbeSubmodelCutoff))
	capability := responseFor(responses, CheckProbeSubmodelCapability)
	refusal := responseFor(responses, CheckProbeSubmodelRefusal)
	if cutoff == "" && capability == "" && refusal == "" {
		return SubmodelAssessmentSummary{}
	}

	signals := make(familySignalSet)
	capScore, capEvidence := scoreSubmodelCapability(capability)
	familyHint := familyFromRefusal(refusal)
	if familyHint != "" {
		signals.add(familyHint, "", 1.35, "V3 refusal lead implies "+familyHint)
	}
	if cutoff != "" && claimedFamily != "" {
		signals.add(claimedFamily, "", 0.35, "cutoff probe collected")
	}
	if capScore > 0 {
		targetFamily := firstNonEmptyString(familyHint, claimedFamily)
		if targetFamily != "" {
			signals.add(targetFamily, "", 0.55*capScore, fmt.Sprintf("capability vector %.0f%%", capScore*100))
		}
	}
	for _, candidate := range identitySubmodelBaselines(claimedModel, claimedFamily, familyHint) {
		score := 0.15
		if claimedFamily != "" && candidate.Family == claimedFamily {
			score += 0.18
		}
		if familyHint != "" && candidate.Family == familyHint {
			score += 0.35
		}
		if modelBareName(candidate.Model) == modelBareName(claimedModel) {
			score += 0.24
		}
		if capScore > 0 {
			score += 0.18 * capScore
		}
		signals.add(candidate.Family, candidate.Model, score, "V3 baseline candidate "+candidate.Model)
	}
	candidates := signals.candidates(3)
	top := topCandidate(candidates)
	evidence := append([]string{}, capEvidence...)
	if cutoff != "" {
		evidence = append(evidence, "cutoff="+truncate(cutoff, 40))
	}
	if familyHint != "" {
		evidence = append(evidence, "refusal_family="+familyHint)
	}
	return submodelSummaryFromTop("v3", top, candidates, evidence)
}

func classifySubmodelV3ELike(claimedModel string, claimedFamily string, responses map[CheckKey]string, v3f bool) SubmodelAssessmentSummary {
	vector := refusalLadderVector(responses)
	format := formattingFingerprint(responses)
	uncertaintyValue, uncertaintyRound, hasUncertainty := uncertaintyFingerprint(responseFor(responses, CheckProbeUncertaintyEstimate))
	if len(vector) == 0 && format == "" && !hasUncertainty {
		return SubmodelAssessmentSummary{}
	}

	method := "v3e"
	if v3f {
		method = "v3f"
	}
	refusalFamily := familyFromRefusal(strings.Join(refusalLadderTexts(responses), "\n"))
	signals := make(familySignalSet)
	if refusalFamily != "" {
		signals.add(refusalFamily, "", 1.1, strings.ToUpper(method)+" refusal ladder implies "+refusalFamily)
	}
	refusedCount := 0
	for _, rung := range vector {
		if rung == 2 {
			refusedCount++
		}
	}
	if refusedCount >= 5 {
		signals.add("anthropic", "", 0.35, "high refusal ladder strictness")
	}
	if format == "-" {
		signals.add("openai", "", 0.2, "dash bullet formatting")
	}
	if format == "1." {
		signals.add("openai", "", 0.18, "numbered formatting")
	}
	if hasUncertainty && v3f {
		if uncertaintyRound {
			signals.add("openai", "", 0.18, "round uncertainty estimate")
		} else if uncertaintyValue > 0 {
			signals.add("anthropic", "", 0.12, "non-round uncertainty estimate")
		}
	}
	for _, candidate := range identitySubmodelBaselines(claimedModel, claimedFamily, refusalFamily) {
		score := 0.14
		if claimedFamily != "" && candidate.Family == claimedFamily {
			score += 0.14
		}
		if refusalFamily != "" && candidate.Family == refusalFamily {
			score += 0.3
		}
		if modelBareName(candidate.Model) == modelBareName(claimedModel) {
			score += 0.18
		}
		score += math.Min(0.18, float64(refusedCount)*0.02)
		signals.add(candidate.Family, candidate.Model, score, strings.ToUpper(method)+" baseline candidate "+candidate.Model)
	}
	candidates := signals.candidates(3)
	evidence := []string{
		fmt.Sprintf("refusal_vector=%v", vector),
		"bullet=" + firstNonEmptyString(format, "none"),
	}
	if hasUncertainty {
		evidence = append(evidence, fmt.Sprintf("uncertainty=%d round=%v", uncertaintyValue, uncertaintyRound))
	}
	top := topCandidate(candidates)
	return submodelSummaryFromTop(method, top, candidates, evidence)
}

func deriveIdentityAssessment(key identityResultKey, claimedModel string, claimedFamily string, candidates []IdentityCandidateSummary, submodels []SubmodelAssessmentSummary, riskFlags []string) IdentityAssessmentSummary {
	return deriveIdentityAssessmentWithSignals(key, claimedModel, claimedFamily, candidates, submodels, riskFlags, nil, nil)
}

func deriveIdentityAssessmentWithSignals(key identityResultKey, claimedModel string, claimedFamily string, candidates []IdentityCandidateSummary, submodels []SubmodelAssessmentSummary, riskFlags []string, results []CheckResult, responses map[CheckKey]string) IdentityAssessmentSummary {
	top := topCandidate(candidates)
	if top.Family == "" {
		for _, submodel := range submodels {
			if submodel.Family != "" {
				top = IdentityCandidateSummary{Family: submodel.Family, Model: submodel.DisplayName, Score: submodel.Score, Reasons: submodel.Evidence}
				break
			}
		}
	}
	if top.Family == "" {
		assessment := IdentityAssessmentSummary{
			Provider:            key.provider,
			ModelName:           key.model,
			ClaimedModel:        claimedModel,
			Status:              identityStatusInsufficientData,
			Method:              "llmprobe_behavioral_fingerprint",
			SubmodelAssessments: submodels,
			RiskFlags:           riskFlags,
			Evidence:            []string{"no fingerprint features available"},
		}
		if responses != nil {
			verdict := computeIdentityVerdict(IdentityVerdictInput{ClaimedFamily: claimedFamily, ClaimedModel: claimedModel, Surface: surfaceIdentitySignal(responses), Behavior: behaviorIdentitySignalWithoutSelfClaim(results, responses), V3: v3IdentitySignal(submodels)})
			assessment.Verdict = &verdict
		}
		return assessment
	}

	submodelVotes := 0
	submodelAgreement := 0
	predictedModel := ""
	for _, submodel := range submodels {
		if submodel.Family == "" {
			continue
		}
		submodelVotes++
		if submodel.Family == top.Family {
			submodelAgreement++
			if predictedModel == "" {
				predictedModel = firstNonEmptyString(submodel.DisplayName, submodel.ModelID)
			}
		}
	}
	confidence := top.Score
	if submodelVotes > 0 {
		confidence = math.Min(1, confidence+0.08*float64(submodelAgreement))
	}
	if len(riskFlags) > 0 {
		confidence = math.Max(0, confidence-0.08*float64(min(len(riskFlags), 3)))
	}

	status := identityStatusUncertain
	switch {
	case claimedFamily == "":
		status = identityStatusUncertain
	case top.Family == claimedFamily:
		status = identityStatusMatch
	case top.Score >= 0.45 || submodelAgreement > 0:
		status = identityStatusMismatch
	}
	evidence := append([]string{}, top.Reasons...)
	if claimedFamily != "" {
		evidence = append(evidence, "claimed_family="+claimedFamily)
	}
	if submodelAgreement > 0 {
		evidence = append(evidence, fmt.Sprintf("submodel_agreement=%d/%d", submodelAgreement, submodelVotes))
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

func identityRiskFlags(results []CheckResult) []string {
	flags := make([]string, 0)
	for _, result := range results {
		if result.Skipped || result.Success {
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

func scoreSubmodelCapability(text string) (float64, []string) {
	lower := strings.ToLower(text)
	if strings.TrimSpace(lower) == "" {
		return 0, nil
	}
	expected := []string{"3", "tuesday", "6", "541", "etadommocca"}
	hits := 0
	evidence := make([]string, 0, len(expected))
	for _, token := range expected {
		if strings.Contains(lower, token) {
			hits++
			evidence = append(evidence, "cap."+token)
		}
	}
	return float64(hits) / float64(len(expected)), evidence
}

func refusalLadderVector(responses map[CheckKey]string) []int {
	keys := []CheckKey{CheckProbeRefusalL1, CheckProbeRefusalL2, CheckProbeRefusalL3, CheckProbeRefusalL4, CheckProbeRefusalL5, CheckProbeRefusalL6, CheckProbeRefusalL7, CheckProbeRefusalL8}
	vector := make([]int, 0, len(keys))
	for _, key := range keys {
		text := responseFor(responses, key)
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		switch {
		case hasAny(lower, "i can't", "i cannot", "unable", "won't", "not able", "无法", "不能"):
			vector = append(vector, 2)
		case hasAny(lower, "safety", "legal", "ethical", "permission", "defensive"):
			vector = append(vector, 1)
		default:
			vector = append(vector, 0)
		}
	}
	return vector
}

func refusalLadderTexts(responses map[CheckKey]string) []string {
	keys := []CheckKey{CheckProbeRefusalL1, CheckProbeRefusalL2, CheckProbeRefusalL3, CheckProbeRefusalL4, CheckProbeRefusalL5, CheckProbeRefusalL6, CheckProbeRefusalL7, CheckProbeRefusalL8}
	texts := make([]string, 0, len(keys))
	for _, key := range keys {
		if text := responseFor(responses, key); text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func formattingFingerprint(responses map[CheckKey]string) string {
	text := responseFor(responses, CheckProbeFmtBullets)
	lines := strings.Split(text, "\n")
	counts := map[string]int{"-": 0, "*": 0, "1.": 0, "bullet-dot": 0}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- "):
			counts["-"]++
		case strings.HasPrefix(trimmed, "* "):
			counts["*"]++
		case regexp.MustCompile(`^\d+\.\s`).MatchString(trimmed):
			counts["1."]++
		case strings.HasPrefix(trimmed, "• "):
			counts["bullet-dot"]++
		}
	}
	best := ""
	bestCount := 0
	for key, count := range counts {
		if count > bestCount {
			best = key
			bestCount = count
		}
	}
	return best
}

func uncertaintyFingerprint(text string) (int, bool, bool) {
	match := regexp.MustCompile(`\b(\d{1,3})\b`).FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false, false
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value, value%5 == 0, true
}

func familyFromRefusal(text string) string {
	lower := strings.ToLower(text)
	switch {
	case hasAny(lower, "i understand you're asking about"):
		return "deepseek"
	case hasAny(lower, "18 u.s.c", "18 usc", "virtually all", "i can't help with", "i can’t help with", "isn't something i'll help", "i'm not going to"):
		return "anthropic"
	case hasAny(lower, "i'm sorry, but i cannot", "i’m sorry, but i can’t", "sorry, but i can't"):
		return "openai"
	case hasAny(lower, "i cannot fulfill", "cannot comply"):
		return "google"
	case hasAny(lower, "i cannot provide", "无法提供"):
		return "qwen"
	default:
		return ""
	}
}

func claimedModelToIdentityFamily(model string) string {
	lower := strings.ToLower(strings.TrimSpace(model))
	switch {
	case hasAny(lower, "claude", "anthropic"):
		return "anthropic"
	case hasAny(lower, "gpt", "chatgpt", "openai", "o1", "o3", "o4"):
		return "openai"
	case hasAny(lower, "gemini", "palm", "google"):
		return "google"
	case hasAny(lower, "qwen", "dashscope", "通义"):
		return "qwen"
	case hasAny(lower, "llama", "meta"):
		return "meta"
	case hasAny(lower, "mistral", "mixtral"):
		return "mistral"
	case hasAny(lower, "deepseek"):
		return "deepseek"
	case hasAny(lower, "grok", "xai"):
		return "xai"
	case hasAny(lower, "glm", "zhipu", "z-ai"):
		return "zhipu"
	default:
		return ""
	}
}

type identitySubmodelBaseline struct {
	Family string
	Model  string
}

func identitySubmodelBaselines(claimedModel string, claimedFamily string, hintedFamily string) []identitySubmodelBaseline {
	baselines := []identitySubmodelBaseline{
		{Family: "openai", Model: "gpt-5.5"},
		{Family: "openai", Model: "gpt-5.4"},
		{Family: "openai", Model: "gpt-4o"},
		{Family: "openai", Model: "gpt-4o-mini"},
		{Family: "anthropic", Model: "claude-opus-4-7"},
		{Family: "anthropic", Model: "claude-opus-4-6"},
		{Family: "anthropic", Model: "claude-sonnet-4-5"},
		{Family: "google", Model: "gemini-2.5-pro"},
		{Family: "google", Model: "gemini-2.5-flash"},
		{Family: "qwen", Model: "qwen3-max"},
		{Family: "qwen", Model: "qwen3-coder"},
		{Family: "deepseek", Model: "deepseek-v3.2"},
		{Family: "meta", Model: "llama-4"},
		{Family: "mistral", Model: "mistral-large"},
	}
	if strings.TrimSpace(claimedModel) != "" {
		family := firstNonEmptyString(claimedFamily, claimedModelToIdentityFamily(claimedModel))
		baselines = append([]identitySubmodelBaseline{{Family: family, Model: claimedModel}}, baselines...)
	}
	if hintedFamily != "" && hintedFamily != claimedFamily {
		baselines = append([]identitySubmodelBaseline{{Family: hintedFamily, Model: identityFamilyDisplayName(hintedFamily)}}, baselines...)
	}
	return baselines
}

func submodelSummaryFromTop(method string, top IdentityCandidateSummary, candidates []IdentityCandidateSummary, evidence []string) SubmodelAssessmentSummary {
	if top.Family == "" {
		return SubmodelAssessmentSummary{
			Method:    method,
			Score:     0,
			Abstained: true,
			Evidence:  uniqueStrings(evidence),
		}
	}
	return SubmodelAssessmentSummary{
		Method:      method,
		Family:      top.Family,
		ModelID:     top.Model,
		DisplayName: top.Model,
		Score:       top.Score,
		Abstained:   top.Score < 0.45,
		Candidates:  candidates,
		Evidence:    uniqueStrings(evidence),
	}
}

func (signals familySignalSet) add(family string, model string, score float64, reason string) {
	family = strings.TrimSpace(family)
	if family == "" || score <= 0 {
		return
	}
	key := family
	if model != "" {
		key += ":" + model
	}
	existing, ok := signals[key]
	if !ok {
		existing = &familyScoreAccumulator{family: family, model: model}
		signals[key] = existing
	}
	existing.score += score
	if reason != "" {
		existing.reasons = append(existing.reasons, reason)
	}
}

func (signals familySignalSet) candidates(limit int) []IdentityCandidateSummary {
	if len(signals) == 0 {
		return nil
	}
	accumulators := make([]*familyScoreAccumulator, 0, len(signals))
	maxScore := 0.0
	for _, accumulator := range signals {
		if accumulator.score > maxScore {
			maxScore = accumulator.score
		}
		accumulators = append(accumulators, accumulator)
	}
	sort.SliceStable(accumulators, func(i, j int) bool {
		if accumulators[i].score == accumulators[j].score {
			return accumulators[i].family < accumulators[j].family
		}
		return accumulators[i].score > accumulators[j].score
	})
	if limit <= 0 || limit > len(accumulators) {
		limit = len(accumulators)
	}
	out := make([]IdentityCandidateSummary, 0, limit)
	for _, accumulator := range accumulators[:limit] {
		score := 0.0
		if maxScore > 0 {
			score = accumulator.score / maxScore
		}
		out = append(out, IdentityCandidateSummary{
			Family:  accumulator.family,
			Model:   firstNonEmptyString(accumulator.model, identityFamilyDisplayName(accumulator.family)),
			Score:   roundScore(score),
			Reasons: uniqueStrings(firstNStrings(accumulator.reasons, 5)),
		})
	}
	return out
}

func topCandidate(candidates []IdentityCandidateSummary) IdentityCandidateSummary {
	if len(candidates) == 0 {
		return IdentityCandidateSummary{}
	}
	return candidates[0]
}

func responseFor(responses map[CheckKey]string, key CheckKey) string {
	return strings.TrimSpace(responses[key])
}

func lowerProbeText(responses map[CheckKey]string, keys ...CheckKey) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if text := responseFor(responses, key); text != "" {
			parts = append(parts, strings.ToLower(text))
		}
	}
	return strings.Join(parts, "\n")
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
		return "Anthropic Claude"
	case "openai":
		return "OpenAI GPT"
	case "google":
		return "Google Gemini"
	case "qwen":
		return "Alibaba Qwen"
	case "meta":
		return "Meta Llama"
	case "mistral":
		return "Mistral"
	case "deepseek":
		return "DeepSeek"
	case "xai":
		return "xAI Grok"
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

func isIdentityFeatureCheck(checkKey CheckKey) bool {
	switch checkKey {
	case CheckProbeIdentitySelfClaim,
		CheckProbeTokenizerAware,
		CheckProbeIdentityStyleEN,
		CheckProbeIdentityStyleZHTW,
		CheckProbeIdentityReasoningShape,
		CheckProbeIdentitySelfKnowledge,
		CheckProbeIdentityListFormat,
		CheckProbeIdentityRefusalPattern,
		CheckProbeIdentityJSONDiscipline,
		CheckProbeIdentityCapabilityClaim,
		CheckProbeLingKRNum,
		CheckProbeLingJPPM,
		CheckProbeLingFRPM,
		CheckProbeLingRUPres,
		CheckProbeTokenCountNum,
		CheckProbeTokenSplitWord,
		CheckProbeTokenSelfKnowledge,
		CheckProbeCodeReverseList,
		CheckProbeCodeCommentLang,
		CheckProbeCodeErrorStyle,
		CheckProbeMetaContextLen,
		CheckProbeMetaThinkingMode,
		CheckProbeMetaCreator,
		CheckProbeLingUKPM,
		CheckProbeLingKRCrisis,
		CheckProbeLingDEChan,
		CheckProbeCompPyFloat,
		CheckProbeCompLargeExp,
		CheckProbeSubmodelCutoff,
		CheckProbeSubmodelCapability,
		CheckProbeSubmodelRefusal,
		CheckProbePIFingerprint,
		CheckProbeRefusalL1,
		CheckProbeRefusalL2,
		CheckProbeRefusalL3,
		CheckProbeRefusalL4,
		CheckProbeRefusalL5,
		CheckProbeRefusalL6,
		CheckProbeRefusalL7,
		CheckProbeRefusalL8,
		CheckProbeFmtBullets,
		CheckProbeFmtExplainDepth,
		CheckProbeFmtCodeLangTag,
		CheckProbeUncertaintyEstimate:
		return true
	default:
		return false
	}
}
