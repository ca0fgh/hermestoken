package token_verifier

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	_ "embed"
)

//go:embed fingerprint_family_baselines.json
var sourceFamilyBaselinesJSON []byte

type sourceFamilyBaselineSnapshot struct {
	Baselines []sourceFamilyBaseline `json:"baselines"`
}

type sourceFamilyBaseline struct {
	Family      string               `json:"family"`
	DisplayName string               `json:"displayName"`
	Signals     []sourceFamilySignal `json:"signals"`
}

type sourceFamilySignal struct {
	Category string
	Key      string
	Weight   float64
}

func (signal *sourceFamilySignal) UnmarshalJSON(data []byte) error {
	var tuple []any
	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}
	if len(tuple) != 3 {
		return nil
	}
	if category, ok := tuple[0].(string); ok {
		signal.Category = category
	}
	if key, ok := tuple[1].(string); ok {
		signal.Key = key
	}
	if weight, ok := tuple[2].(float64); ok {
		signal.Weight = weight
	}
	return nil
}

type sourceFingerprintFeatures map[string]map[string]float64

const sourceFamilyMinEvidenceWeight = 1.0
const sourceFamilyMinPositiveEvidenceWeight = 1.0

var unstableSourceFamilySignals = map[string]map[string]bool{
	"linguisticFingerprint": {
		"de_chan_merz":        true,
		"de_chan_scholz":      true,
		"fr_pm_barnier":       true,
		"fr_pm_bayrou":        true,
		"fr_pm_old":           true,
		"jp_pm_ishiba":        true,
		"jp_pm_kishida":       true,
		"jp_pm_old":           true,
		"jp_pm_recent":        true,
		"kr_knows_crisis":     true,
		"uk_pm_starmer":       true,
		"uk_pm_sunak":         true,
		"overall_instability": true,
		"overall_stability":   true,
	},
	"subModelSignals": {
		"tps_bucket_slow":    true,
		"tps_fast":           true,
		"tps_slow":           true,
		"tps_unstable":       true,
		"ttft_bucket_slow":   true,
		"ttft_bucket_snappy": true,
		"ttft_fast":          true,
		"ttft_median_norm":   true,
	},
}

var (
	sourceFamilyBaselinesOnce sync.Once
	sourceFamilyBaselines     []sourceFamilyBaseline
	sourceFamilyBaselinesErr  error
)

func sourceBehavioralIdentityCandidates(results []CheckResult, responses map[CheckKey]string) []IdentityCandidateSummary {
	baselines := loadSourceFamilyBaselines()
	if len(baselines) == 0 {
		return nil
	}
	features := extractSourceFingerprintFeatures(results, responses)
	return sourceBehavioralIdentityCandidatesFromFeatures(features, baselines)
}

func sourceBehavioralIdentityCandidatesFromFeatures(features sourceFingerprintFeatures, baselines []sourceFamilyBaseline) []IdentityCandidateSummary {
	type familyLogScore struct {
		family      string
		displayName string
		logOdds     float64
		evidence    float64
		positive    float64
		reasons     []string
	}
	logScores := make([]familyLogScore, 0, len(baselines))
	maxEvidence := 0.0
	maxPositiveEvidence := 0.0
	for _, baseline := range baselines {
		logOdds := 0.0
		evidence := 0.0
		positive := 0.0
		reasons := make([]string, 0)
		for _, signal := range baseline.Signals {
			if isUnstableSourceFamilySignal(signal) {
				continue
			}
			value := features.value(signal.Category, signal.Key)
			if value == 0 {
				continue
			}
			logOdds += signal.Weight * value
			evidence += math.Abs(signal.Weight) * math.Abs(value)
			if signal.Weight > 0 {
				positive += signal.Weight * math.Abs(value)
			}
			label := strings.ReplaceAll(signal.Key, "_", " ")
			if signal.Weight > 0 {
				reasons = append(reasons, label+" detected (+"+formatFloat1(signal.Weight)+")")
			} else {
				reasons = append(reasons, label+" contradicts "+baseline.Family+" ("+formatFloat1(signal.Weight)+")")
			}
		}
		if evidence > maxEvidence {
			maxEvidence = evidence
		}
		if positive > maxPositiveEvidence {
			maxPositiveEvidence = positive
		}
		logScores = append(logScores, familyLogScore{family: baseline.Family, displayName: baseline.DisplayName, logOdds: logOdds, evidence: evidence, positive: positive, reasons: reasons})
	}
	if maxEvidence < sourceFamilyMinEvidenceWeight || maxPositiveEvidence < sourceFamilyMinPositiveEvidenceWeight {
		return nil
	}
	maxLogOdds := 0.0
	for _, score := range logScores {
		if score.logOdds > maxLogOdds {
			maxLogOdds = score.logOdds
		}
	}
	total := 0.0
	expScores := make([]float64, len(logScores))
	for i, score := range logScores {
		expScores[i] = math.Exp(score.logOdds - maxLogOdds)
		total += expScores[i]
	}
	if total == 0 {
		return nil
	}
	sort.SliceStable(logScores, func(i, j int) bool {
		left := math.Exp(logScores[i].logOdds-maxLogOdds) / total
		right := math.Exp(logScores[j].logOdds-maxLogOdds) / total
		if left == right {
			return logScores[i].family < logScores[j].family
		}
		return left > right
	})
	out := make([]IdentityCandidateSummary, 0, 3)
	for _, score := range logScores {
		if score.positive < sourceFamilyMinPositiveEvidenceWeight {
			continue
		}
		normalized := math.Exp(score.logOdds-maxLogOdds) / total
		out = append(out, IdentityCandidateSummary{
			Family:  score.family,
			Model:   score.displayName,
			Score:   roundScore(normalized),
			Reasons: firstNStrings(score.reasons, 5),
		})
		if len(out) == 3 {
			break
		}
	}
	return out
}

func isUnstableSourceFamilySignal(signal sourceFamilySignal) bool {
	return unstableSourceFamilySignals[signal.Category] != nil &&
		unstableSourceFamilySignals[signal.Category][signal.Key]
}

func loadSourceFamilyBaselines() []sourceFamilyBaseline {
	sourceFamilyBaselinesOnce.Do(func() {
		var snapshot sourceFamilyBaselineSnapshot
		sourceFamilyBaselinesErr = json.Unmarshal(sourceFamilyBaselinesJSON, &snapshot)
		if sourceFamilyBaselinesErr == nil {
			sourceFamilyBaselines = snapshot.Baselines
		}
	})
	return sourceFamilyBaselines
}

func extractSourceFingerprintFeatures(results []CheckResult, responses map[CheckKey]string) sourceFingerprintFeatures {
	sourceResponses := sourceResponseIDMap(responses)
	linguisticResults, singleRunFallbacks := sourceLinguisticInputs(responses)
	styleEn := strings.ToLower(sourceResponses["identity_style_en"])
	styleZh := strings.ToLower(sourceResponses["identity_style_zh_tw"])
	reasonText := strings.ToLower(sourceResponses["identity_reasoning_shape"])
	listText := strings.ToLower(sourceResponses["identity_list_format"])
	allText := strings.Join(mapValues(sourceResponses), " ")
	enWords := strings.Fields(styleEn)
	uniqueWords := make(map[string]bool)
	for _, word := range enWords {
		uniqueWords[strings.ToLower(word)] = true
	}

	features := sourceFingerprintFeatures{
		"selfClaim":             extractSourceSelfClaim(sourceResponses),
		"lexical":               extractSourceLexical(styleEn, styleZh),
		"reasoning":             extractSourceReasoning(reasonText),
		"jsonDiscipline":        extractSourceJSONDiscipline(sourceResponses["identity_json_discipline"]),
		"refusal":               extractSourceRefusal(strings.ToLower(sourceResponses["identity_refusal_pattern"])),
		"listFormat":            extractSourceListFormat(listText),
		"subModelSignals":       extractSourceSubmodelSignals(results, allText, styleEn, styleZh, reasonText, listText, enWords, uniqueWords),
		"linguisticFingerprint": extractSourceLinguisticFeatures(linguisticResults, singleRunFallbacks),
		"textStructure":         aggregateSourceTextStructure(sourceAllTexts(sourceResponses, linguisticResults)),
	}
	return features
}

func extractSourceSelfClaim(responses map[string]string) map[string]float64 {
	selfText := strings.ToLower(responses["identity_self_knowledge"])
	return map[string]float64{
		"claude":   boolFloat(sourceHas(selfText, "claude", "anthropic")),
		"openai":   boolFloat(sourceHas(selfText, "chatgpt", "gpt-4", "gpt-3", "openai")),
		"qwen":     boolFloat(sourceHas(selfText, "qwen", "通义千问", "阿里", "alibaba")),
		"gemini":   boolFloat(sourceHas(selfText, "gemini", "google deepmind", "bard")),
		"llama":    boolFloat(sourceHas(selfText, "llama", "meta ai", "meta llama")),
		"mistral":  boolFloat(strings.Contains(selfText, "mistral")),
		"deepseek": boolFloat(strings.Contains(selfText, "deepseek")),
		"kiro":     boolFloat(sourceHas(selfText, "kiro", "amazon q developer", "kiro-cli")),
		"zhipu":    boolFloat(sourceHas(selfText, "zhipu", "glm", "智谱")),
	}
}

func extractSourceLexical(styleEn string, styleZh string) map[string]float64 {
	combined := styleEn + " " + styleZh
	return map[string]float64{
		"opener_certainly":   boolFloat(sourceStartsWithAny(styleEn, "certainly", "of course", "sure!", "absolutely")),
		"opener_great":       boolFloat(sourceStartsWithAny(styleEn, "great question", "that's a great", "excellent question")),
		"opener_direct":      boolFloat(sourceStartsWithAny(styleEn, "the most", "in my view", "i think", "i believe")),
		"uses_bold_headers":  boolFloat(strings.Contains(combined, "**")),
		"uses_numbered_list": boolFloat(regexp.MustCompile(`\n\d+\.\s`).MatchString(combined)),
		"uses_dash_bullets":  boolFloat(regexp.MustCompile(`\n-\s`).MatchString(combined)),
		"verbose_zh":         boolFloat(len(styleZh) > 600),
		"concise_en":         boolFloat(len(styleEn) > 0 && len(styleEn) < 400),
	}
}

func extractSourceReasoning(text string) map[string]float64 {
	return map[string]float64{
		"starts_with_letme":     boolFloat(sourceStartsWithAny(text, "let me", "let's", "let us")),
		"starts_with_first":     boolFloat(sourceStartsWithAny(text, "first,", "first:", "step 1", "to solve")),
		"uses_chain_of_thought": boolFloat(strings.Contains(text, "step by step") || strings.Contains(text, "thinking through")),
		"uses_therefore":        boolFloat(strings.Contains(text, "therefore") || strings.Contains(text, "thus,")),
	}
}

func extractSourceJSONDiscipline(text string) map[string]float64 {
	trimmed := strings.TrimSpace(text)
	preamble := regexp.MustCompile(`(?i)^(certainly|sure|of course|here is|here's)`).MatchString(trimmed)
	markdownPolluted := strings.Contains(text, "```") || preamble
	return map[string]float64{
		"pure_json":         boolFloat(!markdownPolluted && strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")),
		"markdown_polluted": boolFloat(markdownPolluted),
		"preamble_text":     boolFloat(preamble),
	}
}

func extractSourceRefusal(text string) map[string]float64 {
	claudeStyle := sourceHas(text,
		"i'm not able to assist", "i'm unable to assist", "i can't assist with",
		"isn't something i'll help", "not something i'll help",
		"i'm not going to help", "i don't help with", "i won't help with",
		"this would cause real harm", "i can't provide", "i'm not able to provide",
	)
	gptStyle := sourceHas(text, "i'm sorry, but i cannot", "i apologize, but", "i'm sorry, i can't")
	genericCannot := sourceHas(text, "i cannot", "i can't", "i won't", "i'm not going to", "i don't provide")
	noRefusal := len(text) > 20 && !sourceHas(text,
		"cannot", "can't", "won't", "unable", "无法", "不能",
		"isn't something i'll", "not something i'll",
		"i'm not going to", "i don't help", "i won't help",
		"i don't provide", "cause real harm", "i'm not able",
		"i will not", "i refuse",
	)
	return map[string]float64{
		"claude_style":    boolFloat(claudeStyle),
		"gpt_style":       boolFloat(gptStyle),
		"generic_cannot":  boolFloat(genericCannot),
		"chinese_refusal": boolFloat(sourceHas(text, "无法", "不能提供", "不应该", "不合适")),
		"no_refusal":      boolFloat(noRefusal),
	}
}

func extractSourceListFormat(text string) map[string]float64 {
	return map[string]float64{
		"bold_headers":     boolFloat(strings.Contains(text, "**")),
		"plain_numbered":   boolFloat(regexp.MustCompile(`(?m)^\d+\.\s`).MatchString(text) && !strings.Contains(text, "**")),
		"has_explanations": boolFloat(len(strings.Split(text, "\n")) > 10),
		"emoji_bullets":    boolFloat(regexp.MustCompile(`[🔸🔹✅❌💡🌟]`).MatchString(text)),
	}
}

func extractSourceSubmodelSignals(results []CheckResult, allText string, styleEn string, styleZh string, reasonText string, listText string, enWords []string, uniqueWords map[string]bool) map[string]float64 {
	vocabRichness := 0.0
	if len(enWords) > 10 {
		vocabRichness = float64(len(uniqueWords)) / float64(len(enWords))
	}
	listDetail := 0.0
	listItems := make([]string, 0)
	for _, line := range strings.Split(listText, "\n") {
		if regexp.MustCompile(`^\s*[\d\-\*]`).MatchString(line) {
			listItems = append(listItems, line)
		}
	}
	if len(listItems) > 0 {
		total := 0
		for _, item := range listItems {
			total += len(item)
		}
		listDetail = math.Min(1, float64(total)/float64(len(listItems))/200)
	}
	features := map[string]float64{
		"total_response_length":  math.Min(1, float64(len(allText))/15000),
		"en_response_length":     math.Min(1, float64(len(styleEn))/3000),
		"zh_response_length":     math.Min(1, float64(len(styleZh))/3000),
		"vocab_richness":         vocabRichness,
		"reasoning_length":       math.Min(1, float64(len(reasonText))/3000),
		"list_detail_level":      listDetail,
		"en_paragraph_count":     math.Min(1, float64(len(regexp.MustCompile(`\n\n+`).Split(styleEn, -1)))/8),
		"uses_markdown_headings": boolFloat(regexp.MustCompile(`(?m)^#{1,3}\s`).MatchString(styleEn + " " + styleZh)),
	}
	for key, value := range extractSourcePerformanceFeatures(results) {
		features[key] = value
	}
	for key, value := range aggregateSourceTimingFeatures(results) {
		features[key] = value
	}
	return features
}

func extractSourceLinguisticFeatures(results map[string][]string, fallback map[string]string) map[string]float64 {
	features := make(map[string]float64)
	stabilities := make([]float64, 0)
	addStability := func(dist map[string]float64) float64 {
		stability := sourceStability(dist)
		if len(dist) > 0 {
			stabilities = append(stabilities, stability)
		}
		return stability
	}
	if answers := sourceEffectiveAnswers("ling_kr_num", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(sourceModeAnswer(dist))
		features["kr_num_sino"] = boolFloat(strings.Contains(mode, "사십이") || strings.Contains(mode, "사십"))
		features["kr_num_native"] = boolFloat(strings.Contains(mode, "마흔"))
		features["kr_num_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("ling_jp_pm", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := sourceModeAnswer(dist)
		lower := strings.ToLower(mode)
		isIshiba := strings.Contains(mode, "石破") || strings.Contains(lower, "ishiba")
		isKishida := strings.Contains(mode, "岸田") || strings.Contains(lower, "kishida")
		features["jp_pm_ishiba"] = boolFloat(isIshiba)
		features["jp_pm_kishida"] = boolFloat(isKishida)
		features["jp_pm_recent"] = boolFloat(isIshiba || isKishida)
		features["jp_pm_old"] = boolFloat(!isIshiba && !isKishida && mode != "")
		features["jp_pm_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("ling_fr_pm", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(sourceModeAnswer(dist))
		isBayrou := strings.Contains(mode, "bayrou")
		isBarnier := strings.Contains(mode, "barnier")
		features["fr_pm_bayrou"] = boolFloat(isBayrou)
		features["fr_pm_barnier"] = boolFloat(isBarnier)
		features["fr_pm_old"] = boolFloat(!isBayrou && !isBarnier && mode != "")
		features["fr_pm_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("ling_ru_pres", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := sourceModeAnswer(dist)
		features["ru_pres_surname_first"] = boolFloat(strings.HasPrefix(strings.ToLower(strings.TrimSpace(mode)), "пут"))
		features["ru_pres_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("tok_count_num", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		count := atoiOrZero(strings.TrimSpace(sourceModeAnswer(dist)))
		features["tok_count_1"] = boolFloat(count == 1)
		features["tok_count_2"] = boolFloat(count == 2)
		features["tok_count_3"] = boolFloat(count == 3)
		features["tok_count_4plus"] = boolFloat(count >= 4)
		features["tok_num_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("tok_split_word", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		parts := strings.Split(mode, "|")
		nonEmpty := 0
		for _, part := range parts {
			if part != "" {
				nonEmpty++
			}
		}
		features["tok_split_2parts"] = boolFloat(nonEmpty == 2)
		features["tok_split_3parts"] = boolFloat(nonEmpty == 3)
		features["tok_split_4plus"] = boolFloat(nonEmpty >= 4)
		features["tok_split_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("tok_self_knowledge", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		features["tok_knows_bpe"] = boolFloat(strings.Contains(mode, "bpe"))
		features["tok_knows_tiktoken"] = boolFloat(strings.Contains(mode, "tiktoken") || strings.Contains(mode, "cl100k"))
		features["tok_knows_none"] = boolFloat(!strings.Contains(mode, "bpe") && !strings.Contains(mode, "tiktoken") && !strings.Contains(mode, "token") && mode != "")
		features["tok_self_stability"] = addStability(dist)
	}
	extractSourceCodeLinguistic(features, results, fallback, addStability)
	extractSourceMetaLinguistic(features, results, fallback, addStability)
	if len(stabilities) > 0 {
		total := 0.0
		for _, stability := range stabilities {
			total += stability
		}
		avg := math.Round((total/float64(len(stabilities)))*1000) / 1000
		features["overall_stability"] = avg
		features["overall_instability"] = math.Round((1-avg)*1000) / 1000
	}
	return features
}

func extractSourceCodeLinguistic(features map[string]float64, results map[string][]string, fallback map[string]string, addStability func(map[string]float64) float64) {
	if answers := sourceEffectiveAnswers("code_reverse_list", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := sourceModeAnswer(dist)
		features["code_uses_slice"] = boolFloat(strings.Contains(mode, "[::-1]"))
		features["code_uses_reversed"] = boolFloat(strings.Contains(mode, "reversed("))
		features["code_uses_loop"] = boolFloat(regexp.MustCompile(`for\s+\w+\s+in\s+range`).MatchString(mode))
		features["code_has_type_hints"] = boolFloat(strings.Contains(mode, "->") || regexp.MustCompile(`:\s*(list|List|int|str)`).MatchString(mode))
		features["code_has_docstring"] = boolFloat(strings.Contains(mode, `"""`) || strings.Contains(mode, `'''`))
		features["code_rev_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("code_comment_lang", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := sourceModeAnswer(dist)
		hasChinese := regexp.MustCompile(`[\x{4e00}-\x{9fff}]`).MatchString(mode)
		features["code_comment_zh"] = boolFloat(hasChinese)
		features["code_comment_en"] = boolFloat(!hasChinese && mode != "")
		features["code_comment_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("code_error_style", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := sourceModeAnswer(dist)
		features["code_raises_error"] = boolFloat(strings.Contains(mode, "raise") || strings.Contains(mode, "ValueError"))
		features["code_returns_none"] = boolFloat(regexp.MustCompile(`return\s+None`).MatchString(mode))
		features["code_uses_assert"] = boolFloat(strings.Contains(mode, "assert"))
		features["code_err_stability"] = addStability(dist)
	}
}

func extractSourceMetaLinguistic(features map[string]float64, results map[string][]string, fallback map[string]string, addStability func(map[string]float64) float64) {
	if answers := sourceEffectiveAnswers("meta_context_len", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.NewReplacer(",", "", "_", "", " ", "").Replace(sourceModeAnswer(dist))
		value := atoiOrZero(mode)
		features["meta_ctx_200k"] = boolFloat(value >= 180000)
		features["meta_ctx_128k"] = boolFloat(value >= 100000 && value < 180000)
		features["meta_ctx_small"] = boolFloat(value > 0 && value < 100000)
		features["meta_ctx_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("meta_thinking_mode", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		features["meta_thinking_yes"] = boolFloat(mode == "yes" || strings.HasPrefix(mode, "yes"))
		features["meta_thinking_no"] = boolFloat(mode == "no" || strings.HasPrefix(mode, "no"))
		features["meta_think_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("meta_creator", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		features["meta_creator_anthropic"] = boolFloat(strings.Contains(mode, "anthropic"))
		features["meta_creator_openai"] = boolFloat(strings.Contains(mode, "openai"))
		features["meta_creator_zhipu"] = boolFloat(strings.Contains(mode, "zhipu") || strings.Contains(mode, "智谱"))
		features["meta_creator_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("ling_uk_pm", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		features["uk_pm_starmer"] = boolFloat(strings.Contains(mode, "starmer") || strings.Contains(mode, "keir"))
		features["uk_pm_sunak"] = boolFloat(strings.Contains(mode, "sunak") || strings.Contains(mode, "rishi"))
		features["uk_pm_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("ling_kr_crisis", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		knows := sourceHas(mode, "impeach", "martial", "탄핵", "계엄", "removed", "suspended")
		features["kr_knows_crisis"] = boolFloat(knows)
		features["kr_crisis_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("ling_de_chan", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.ToLower(strings.TrimSpace(sourceModeAnswer(dist)))
		features["de_chan_merz"] = boolFloat(strings.Contains(mode, "merz") || strings.Contains(mode, "friedrich"))
		features["de_chan_scholz"] = boolFloat(strings.Contains(mode, "scholz") || strings.Contains(mode, "olaf"))
		features["de_chan_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("comp_py_float", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.TrimSpace(sourceModeAnswer(dist))
		features["comp_float_exact"] = boolFloat(strings.Contains(mode, "0.30000000000000004"))
		features["comp_float_approx"] = boolFloat(mode == "0.3" || mode == "0.30")
		features["comp_float_stability"] = addStability(dist)
	}
	if answers := sourceEffectiveAnswers("comp_large_exp", results, fallback); len(answers) > 0 {
		dist := sourceDistribution(answers)
		mode := strings.TrimSpace(sourceModeAnswer(dist))
		normalized := strings.NewReplacer(",", "", "_", "", " ", "").Replace(mode)
		features["comp_exp_correct"] = boolFloat(normalized == "4294967296")
		features["comp_exp_commas"] = boolFloat(strings.Contains(mode, ","))
		features["comp_exp_stability"] = addStability(dist)
	}
}

func extractSourcePerformanceFeatures(results []CheckResult) map[string]float64 {
	tpsValues := make([]float64, 0)
	ttftValues := make([]float64, 0)
	for _, result := range results {
		if result.TokensPS > 0 {
			tpsValues = append(tpsValues, result.TokensPS)
		}
		if result.TTFTMs > 0 {
			ttftValues = append(ttftValues, float64(result.TTFTMs))
		}
	}
	if len(tpsValues) == 0 && len(ttftValues) == 0 {
		return map[string]float64{"avg_tps_norm": 0, "avg_ttft_norm": 0, "tps_slow": 0, "tps_fast": 0, "ttft_fast": 0}
	}
	avgTps := averageFloat64(tpsValues)
	avgTTFT := averageFloat64(ttftValues)
	return map[string]float64{
		"avg_tps_norm":  math.Round(math.Min(1, avgTps/200)*1000) / 1000,
		"avg_ttft_norm": math.Round(math.Min(1, math.Max(0, 1-avgTTFT/5000))*1000) / 1000,
		"tps_slow":      boolFloat(avgTps > 0 && avgTps < 60),
		"tps_fast":      boolFloat(avgTps >= 90),
		"ttft_fast":     boolFloat(avgTTFT > 0 && avgTTFT < 500),
	}
}

func aggregateSourceTimingFeatures(results []CheckResult) map[string]float64 {
	tpsValues := make([]float64, 0)
	ttftValues := make([]float64, 0)
	for _, result := range results {
		if result.TokensPS > 0 {
			tpsValues = append(tpsValues, result.TokensPS)
		}
		if result.TTFTMs >= 0 {
			ttftValues = append(ttftValues, float64(result.TTFTMs))
		}
	}
	tpsMedian := medianFloat64(tpsValues)
	ttftMedian := medianFloat64(ttftValues)
	return map[string]float64{
		"tps_bucket_slow":    boolFloat(tpsMedian > 0 && tpsMedian < 20),
		"tps_bucket_medium":  boolFloat(tpsMedian >= 20 && tpsMedian < 60),
		"tps_bucket_fast":    boolFloat(tpsMedian >= 60),
		"ttft_bucket_snappy": boolFloat(ttftMedian > 0 && ttftMedian < 500),
		"ttft_bucket_normal": boolFloat(ttftMedian >= 500 && ttftMedian < 1500),
		"ttft_bucket_slow":   boolFloat(ttftMedian >= 1500),
		"tps_median_norm":    math.Min(1, tpsMedian/100),
		"ttft_median_norm":   math.Min(1, ttftMedian/3000),
		"tps_unstable":       boolFloat(len(tpsValues) >= 3 && varianceFloat64(tpsValues) > 400),
	}
}

func aggregateSourceTextStructure(texts []string) map[string]float64 {
	if len(texts) == 0 {
		return map[string]float64{}
	}
	items := make([]map[string]float64, 0, len(texts))
	for _, text := range texts {
		items = append(items, extractSourceTextStructure(text))
	}
	keys := mapKeys(items[0])
	out := make(map[string]float64, len(keys))
	continuous := map[string]bool{"avg_sent_len": true, "paragraph_count": true}
	for _, key := range keys {
		total := 0.0
		any := false
		for _, item := range items {
			value := item[key]
			total += value
			if value == 1 {
				any = true
			}
		}
		if continuous[key] {
			out[key] = total / float64(len(items))
		} else {
			out[key] = boolFloat(any)
		}
	}
	return out
}

func extractSourceTextStructure(text string) map[string]float64 {
	return map[string]float64{
		"smart_quotes":    boolFloat(regexp.MustCompile(`[\x{201C}\x{201D}\x{2018}\x{2019}]`).MatchString(text)),
		"em_dash":         boolFloat(strings.Contains(text, "—")),
		"ellipsis_style":  boolFloat(strings.Contains(text, "…")),
		"table_style":     boolFloat(regexp.MustCompile(`\|[^\n]*\|[^\n]*\n\s*\|[\s:-]*-{3,}[\s:|-]*\|`).MatchString(text)),
		"bold_style":      boolFloat(regexp.MustCompile(`\*\*[^\n*]+\*\*`).MatchString(text)),
		"numbered_dot":    boolFloat(regexp.MustCompile(`(?m)^\s*\d+\.\s+`).MatchString(text)),
		"cjk_punct":       boolFloat(regexp.MustCompile(`[，。：；！？]`).MatchString(text)),
		"opening_hedge":   boolFloat(regexp.MustCompile(`(?i)^(i think|let me|sure|certainly|of course|absolutely|great question)`).MatchString(strings.TrimSpace(text))),
		"closing_offer":   boolFloat(regexp.MustCompile(`(?i)(let me know|would you like|feel free to|happy to help|if you.{0,10}(need|want))`).MatchString(trailingString(text, 200))),
		"avg_sent_len":    sourceAvgSentenceLen(text),
		"paragraph_count": math.Min(1, float64(len(regexp.MustCompile(`\n\s*\n`).Split(text, -1)))/5),
		"code_block":      boolFloat(strings.Count(text, "```")/2 >= 1),
		"emoji_usage":     boolFloat(regexp.MustCompile(`[\x{1F300}-\x{1F5FF}\x{1F600}-\x{1F64F}\x{1F680}-\x{1F6FF}\x{1F900}-\x{1F9FF}\x{1FA70}-\x{1FAFF}]`).MatchString(text)),
		"latex_style":     boolFloat(regexp.MustCompile(`(\\[\(\)]|\$\$[^$]+\$\$|\\begin\{)`).MatchString(text)),
	}
}

func sourceResponseIDMap(responses map[CheckKey]string) map[string]string {
	out := make(map[string]string)
	for checkKey, text := range responses {
		if id := sourceProbeIDForCheckKey(checkKey); id != "" {
			out[id] = text
		}
	}
	return out
}

func sourceLinguisticInputs(responses map[CheckKey]string) (map[string][]string, map[string]string) {
	results := make(map[string][]string)
	fallbacks := make(map[string]string)
	for checkKey, text := range responses {
		id := sourceProbeIDForCheckKey(checkKey)
		if id == "" || !sourceIsLinguisticProbeID(id) {
			continue
		}
		answers := splitSourceRepeatedAnswers(text)
		results[id] = answers
		if strings.TrimSpace(text) != "" {
			fallbacks[id] = strings.TrimSpace(answers[0])
		}
	}
	return results, fallbacks
}

func sourceProbeIDForCheckKey(checkKey CheckKey) string {
	switch checkKey {
	case CheckProbeIdentityStyleEN:
		return "identity_style_en"
	case CheckProbeIdentityStyleZHTW:
		return "identity_style_zh_tw"
	case CheckProbeIdentityReasoningShape:
		return "identity_reasoning_shape"
	case CheckProbeIdentitySelfKnowledge:
		return "identity_self_knowledge"
	case CheckProbeIdentityListFormat:
		return "identity_list_format"
	case CheckProbeIdentityRefusalPattern:
		return "identity_refusal_pattern"
	case CheckProbeIdentityJSONDiscipline:
		return "identity_json_discipline"
	case CheckProbeIdentityCapabilityClaim:
		return "identity_capability_claim"
	case CheckProbeLingKRNum:
		return "ling_kr_num"
	case CheckProbeLingJPPM:
		return "ling_jp_pm"
	case CheckProbeLingFRPM:
		return "ling_fr_pm"
	case CheckProbeLingRUPres:
		return "ling_ru_pres"
	case CheckProbeTokenCountNum:
		return "tok_count_num"
	case CheckProbeTokenSplitWord:
		return "tok_split_word"
	case CheckProbeTokenSelfKnowledge:
		return "tok_self_knowledge"
	case CheckProbeCodeReverseList:
		return "code_reverse_list"
	case CheckProbeCodeCommentLang:
		return "code_comment_lang"
	case CheckProbeCodeErrorStyle:
		return "code_error_style"
	case CheckProbeMetaContextLen:
		return "meta_context_len"
	case CheckProbeMetaThinkingMode:
		return "meta_thinking_mode"
	case CheckProbeMetaCreator:
		return "meta_creator"
	case CheckProbeLingUKPM:
		return "ling_uk_pm"
	case CheckProbeLingKRCrisis:
		return "ling_kr_crisis"
	case CheckProbeLingDEChan:
		return "ling_de_chan"
	case CheckProbeCompPyFloat:
		return "comp_py_float"
	case CheckProbeCompLargeExp:
		return "comp_large_exp"
	default:
		return ""
	}
}

func sourceIsLinguisticProbeID(id string) bool {
	return strings.HasPrefix(id, "ling_") ||
		strings.HasPrefix(id, "tok_") ||
		strings.HasPrefix(id, "code_") ||
		strings.HasPrefix(id, "meta_") ||
		strings.HasPrefix(id, "comp_")
}

func splitSourceRepeatedAnswers(text string) []string {
	if strings.Contains(text, "\n---\n") {
		parts := strings.Split(text, "\n---\n")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if strings.TrimSpace(part) != "" {
				out = append(out, strings.TrimSpace(part))
			}
		}
		return out
	}
	return []string{strings.TrimSpace(text)}
}

func sourceEffectiveAnswers(id string, results map[string][]string, fallback map[string]string) []string {
	answers := results[id]
	for _, answer := range answers {
		if answer != "ERR" && strings.TrimSpace(answer) != "" {
			return answers
		}
	}
	if fb := strings.TrimSpace(fallback[id]); fb != "" && fb != "ERR" {
		return []string{fb}
	}
	return answers
}

func sourceDistribution(answers []string) map[string]float64 {
	counts := make(map[string]int)
	total := 0
	for _, answer := range answers {
		normalized := sourceNormalizeAnswer(answer)
		if normalized == "" || sourceSkipAnswer(normalized) {
			continue
		}
		counts[normalized]++
		total++
	}
	out := make(map[string]float64)
	if total == 0 {
		return out
	}
	for answer, count := range counts {
		out[answer] = float64(count) / float64(total)
	}
	return out
}

func sourceNormalizeAnswer(answer string) string {
	if strings.Contains(answer, "</think>") {
		parts := strings.Split(answer, "</think>")
		answer = parts[len(parts)-1]
	}
	answer = strings.ReplaceAll(answer, "**", "")
	answer = regexp.MustCompile(`^#+\s*`).ReplaceAllString(answer, "")
	answer = strings.TrimSpace(answer)
	return truncateRunes(answer, 60)
}

func sourceSkipAnswer(answer string) bool {
	switch answer {
	case "ERR", "T/O", "TIMEOUT", "PARSE_ERR", "NET_ERR":
		return true
	default:
		return false
	}
}

func sourceStability(dist map[string]float64) float64 {
	best := 0.0
	for _, value := range dist {
		if value > best {
			best = value
		}
	}
	return best
}

func sourceModeAnswer(dist map[string]float64) string {
	best := ""
	bestScore := -1.0
	for answer, score := range dist {
		if score > bestScore {
			bestScore = score
			best = answer
		}
	}
	return best
}

func (features sourceFingerprintFeatures) value(category string, key string) float64 {
	if features[category] == nil {
		return 0
	}
	return features[category][key]
}

func sourceAllTexts(responses map[string]string, linguistic map[string][]string) []string {
	out := make([]string, 0, len(responses)+len(linguistic))
	for _, text := range responses {
		if text != "" {
			out = append(out, text)
		}
	}
	for _, answers := range linguistic {
		for _, answer := range answers {
			if answer != "" {
				out = append(out, answer)
			}
		}
	}
	return out
}

func sourceAvgSentenceLen(text string) float64 {
	sentences := regexp.MustCompile(`[.!?]\s+`).Split(text, -1)
	nonEmpty := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		if strings.TrimSpace(sentence) != "" {
			nonEmpty = append(nonEmpty, sentence)
		}
	}
	if len(nonEmpty) == 0 {
		return 0
	}
	total := 0
	for _, sentence := range nonEmpty {
		total += len(sentence)
	}
	return math.Min(1, float64(total)/float64(len(nonEmpty))/100)
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func mapKeys(values map[string]float64) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func trailingString(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[len(runes)-limit:])
}

func sourceHas(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func sourceStartsWithAny(text string, prefixes ...string) bool {
	trimmed := strings.TrimSpace(text)
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func averageFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func varianceFloat64(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	avg := averageFloat64(values)
	total := 0.0
	for _, value := range values {
		diff := value - avg
		total += diff * diff
	}
	return total / float64(len(values))
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func formatFloat1(value float64) string {
	value = math.Round(value*10) / 10
	whole := int(value)
	fraction := int(math.Round(math.Abs(value-float64(whole)) * 10))
	return strconvItoa(whole) + "." + strconvItoa(fraction)
}
