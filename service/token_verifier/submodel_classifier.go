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

//go:embed submodel_baselines_v3.json
var sourceSubmodelBaselinesV3JSON []byte

//go:embed submodel_baselines_v3e.json
var sourceSubmodelBaselinesV3EJSON []byte

const (
	sourceSubmodelThreshold = 0.60
	sourceSubmodelTieGap    = 0.05
)

type sourceSubmodelV3Snapshot struct {
	Baselines []sourceSubmodelBaselineV3 `json:"baselines"`
}

type sourceSubmodelBaselineV3 struct {
	ModelID            string                   `json:"modelId"`
	Family             string                   `json:"family"`
	DisplayName        string                   `json:"displayName"`
	Cutoff             string                   `json:"cutoff"`
	Capability         sourceSubmodelCapability `json:"capability"`
	Refusal            sourceSubmodelV3Refusal  `json:"refusal"`
	SourceIteration    string                   `json:"sourceIteration"`
	RejectsTemperature bool                     `json:"rejectsTemperature"`
}

type sourceSubmodelCapability struct {
	Q1Strawberry string `json:"q1_strawberry"`
	Q21000Days   string `json:"q2_1000days"`
	Q3Apples     string `json:"q3_apples"`
	Q4Prime      string `json:"q4_prime"`
	Q5Backwards  string `json:"q5_backwards"`
}

type sourceSubmodelV3Refusal struct {
	Lead                 string  `json:"lead"`
	StartsWithNo         bool    `json:"starts_with_no"`
	StartsWithSorry      bool    `json:"starts_with_sorry"`
	StartsWithCant       bool    `json:"starts_with_cant"`
	Cites18USC           bool    `json:"cites_18_usc"`
	Mentions988          bool    `json:"mentions_988"`
	MentionsVirtuallyAll bool    `json:"mentions_virtually_all"`
	MentionsHistoryAlt   bool    `json:"mentions_history_alt"`
	MentionsPyrotechnics bool    `json:"mentions_pyrotechnics"`
	MentionsPolicies     bool    `json:"mentions_policies"`
	MentionsGuidelines   bool    `json:"mentions_guidelines"`
	MentionsIllegal      bool    `json:"mentions_illegal"`
	MentionsHarmful      bool    `json:"mentions_harmful"`
	LengthAvg            float64 `json:"length_avg"`
}

type sourceSubmodelV3Features struct {
	Cutoff             string
	Capability         sourceSubmodelCapability
	Refusal            sourceObservedV3Refusal
	RejectsTemperature *bool
}

type sourceObservedV3Refusal struct {
	Lead                 string
	StartsWithNo         bool
	StartsWithSorry      bool
	StartsWithCant       bool
	Cites18USC           bool
	Mentions988          bool
	MentionsVirtuallyAll bool
	MentionsHistoryAlt   bool
	MentionsPyrotechnics bool
	MentionsPolicies     bool
	MentionsGuidelines   bool
	MentionsIllegal      bool
	MentionsHarmful      bool
	Length               int
}

type sourceSubmodelBaselineV3ESnapshot struct {
	Baselines []sourceSubmodelBaselineV3E `json:"baselines"`
}

type sourceSubmodelBaselineV3E struct {
	ModelID         string                   `json:"modelId"`
	Family          string                   `json:"family"`
	DisplayName     string                   `json:"displayName"`
	RefusalLadder   sourceV3ERefusalBaseline `json:"refusalLadder"`
	Formatting      sourceV3EFormatBaseline  `json:"formatting"`
	Uncertainty     sourceV3EUncertainty     `json:"uncertainty"`
	SourceIteration string                   `json:"sourceIteration"`
	SampleSize      int                      `json:"sampleSize"`
	UpdatedAt       string                   `json:"updatedAt"`
}

type sourceV3ERefusalBaseline struct {
	VectorAvg           []float64 `json:"vectorAvg"`
	RefusedCountAvg     float64   `json:"refusedCountAvg"`
	FirstRefusalRungAvg float64   `json:"firstRefusalRungAvg"`
	CitesLegalRate      float64   `json:"citesLegalRate"`
	Cites988Rate        float64   `json:"cites988Rate"`
	AvgRefusalLengthAvg float64   `json:"avgRefusalLengthAvg"`
}

type sourceV3EFormatBaseline struct {
	BulletCharMode  string  `json:"bulletCharMode"`
	HeaderDepthAvg  float64 `json:"headerDepthAvg"`
	CodeLangTagMode *string `json:"codeLangTagMode"`
	UsesEmDashRate  float64 `json:"usesEmDashRate"`
}

type sourceV3EUncertainty struct {
	ValueAvg    *float64 `json:"valueAvg"`
	ValueStdDev *float64 `json:"valueStdDev"`
	IsRoundRate float64  `json:"isRoundRate"`
}

type sourceV3EObserved struct {
	RefusalLadder sourceV3ERefusalObserved
	Formatting    sourceV3EFormattingObserved
	Uncertainty   sourceV3EUncertaintyObserved
}

type sourceV3ERefusalObserved struct {
	Vector           []int
	RefusedCount     int
	PartialCount     int
	FirstRefusalRung int
	CitesLegal       bool
	Cites988         bool
	AvgRefusalLength float64
}

type sourceV3EFormattingObserved struct {
	BulletChar  string
	HeaderDepth int
	CodeLangTag *string
	UsesEmDash  bool
}

type sourceV3EUncertaintyObserved struct {
	Value   *int
	IsRound bool
}

type sourceSubmodelMatch struct {
	ModelID     string
	Family      string
	DisplayName string
	Score       float64
	Matched     []string
	Divergent   []string
}

var (
	sourceSubmodelV3Once      sync.Once
	sourceSubmodelV3Baselines []sourceSubmodelBaselineV3
	sourceSubmodelV3Err       error

	sourceSubmodelV3EOnce      sync.Once
	sourceSubmodelV3EBaselines []sourceSubmodelBaselineV3E
	sourceSubmodelV3EErr       error
)

func classifySubmodelV3Source(claimedFamily string, responses map[CheckKey]string) SubmodelAssessmentSummary {
	baselines := loadSourceSubmodelV3Baselines()
	if len(baselines) == 0 {
		return SubmodelAssessmentSummary{}
	}
	features := extractSourceV3Features(responses)
	if features.Cutoff == "" && sourceCapabilityEmpty(features.Capability) && features.Refusal.Lead == "" {
		return SubmodelAssessmentSummary{}
	}
	familyImplied := implySourceV3Family(features)
	pool := filterSourceV3Baselines(baselines, claimedFamily)
	if len(pool) == 0 && claimedFamily == "" {
		pool = baselines
	}
	uniquenessMap := buildSourceV3UniquenessMap(baselines)
	scored := make([]sourceSubmodelMatch, 0, len(pool))
	for _, baseline := range pool {
		score, matched, divergent := scoreSourceV3Match(features, baseline, uniquenessMap)
		scored = append(scored, sourceSubmodelMatch{
			ModelID:     baseline.ModelID,
			Family:      baseline.Family,
			DisplayName: baseline.DisplayName,
			Score:       roundScore(score),
			Matched:     matched,
			Divergent:   divergent,
		})
	}
	sortSourceSubmodelMatches(scored)
	top, abstained := sourceTopSubmodelMatch(scored, sourceSubmodelThreshold)
	evidence := []string{}
	if familyImplied != "" {
		evidence = append(evidence, "family_implied="+familyImplied)
	}
	if len(pool) > 0 && claimedFamily != "" && familyImplied != "" && claimedFamily != familyImplied {
		evidence = append(evidence, "family_mismatch=true")
	}
	return sourceSubmodelSummaryFromMatches("v3", top, scored, abstained, evidence)
}

func classifySubmodelV3ESource(claimedFamily string, responses map[CheckKey]string, v3f bool) SubmodelAssessmentSummary {
	baselines := loadSourceSubmodelV3EBaselines()
	if len(baselines) == 0 {
		return SubmodelAssessmentSummary{}
	}
	observed := extractSourceV3EObserved(responses)
	if len(observed.RefusalLadder.Vector) == 0 && observed.Formatting.BulletChar == "none" && observed.Formatting.HeaderDepth == 0 && observed.Formatting.CodeLangTag == nil && observed.Uncertainty.Value == nil {
		return SubmodelAssessmentSummary{}
	}
	pool := filterSourceV3EBaselines(baselines, claimedFamily)
	if len(pool) == 0 && claimedFamily == "" {
		pool = baselines
	}
	method := "v3e"
	if v3f {
		method = "v3f"
	}
	scored := make([]sourceSubmodelMatch, 0, len(pool))
	for _, baseline := range pool {
		score, matched, divergent := scoreSourceV3EMatch(observed, baseline, v3f)
		scored = append(scored, sourceSubmodelMatch{
			ModelID:     baseline.ModelID,
			Family:      baseline.Family,
			DisplayName: baseline.DisplayName,
			Score:       roundScore(score),
			Matched:     matched,
			Divergent:   divergent,
		})
	}
	sortSourceSubmodelMatches(scored)
	top, abstained := sourceTopSubmodelMatch(scored, sourceSubmodelThreshold)
	evidence := []string{
		"refusal_vector=" + intsToString(observed.RefusalLadder.Vector),
		"bullet=" + observed.Formatting.BulletChar,
	}
	if observed.Uncertainty.Value != nil {
		evidence = append(evidence, "uncertainty="+strconvItoa(*observed.Uncertainty.Value))
	}
	return sourceSubmodelSummaryFromMatches(method, top, scored, abstained, evidence)
}

func loadSourceSubmodelV3Baselines() []sourceSubmodelBaselineV3 {
	sourceSubmodelV3Once.Do(func() {
		var snapshot sourceSubmodelV3Snapshot
		sourceSubmodelV3Err = json.Unmarshal(sourceSubmodelBaselinesV3JSON, &snapshot)
		if sourceSubmodelV3Err == nil {
			sourceSubmodelV3Baselines = snapshot.Baselines
		}
	})
	return sourceSubmodelV3Baselines
}

func loadSourceSubmodelV3EBaselines() []sourceSubmodelBaselineV3E {
	sourceSubmodelV3EOnce.Do(func() {
		var snapshot sourceSubmodelBaselineV3ESnapshot
		sourceSubmodelV3EErr = json.Unmarshal(sourceSubmodelBaselinesV3EJSON, &snapshot)
		if sourceSubmodelV3EErr == nil {
			sourceSubmodelV3EBaselines = snapshot.Baselines
		}
	})
	return sourceSubmodelV3EBaselines
}

func extractSourceV3Features(responses map[CheckKey]string) sourceSubmodelV3Features {
	return sourceSubmodelV3Features{
		Cutoff:     extractSourceV3Cutoff(responseFor(responses, CheckProbeSubmodelCutoff)),
		Capability: extractSourceV3Capability(responseFor(responses, CheckProbeSubmodelCapability)),
		Refusal:    extractSourceV3Refusal(responseFor(responses, CheckProbeSubmodelRefusal)),
	}
}

func extractSourceV3Cutoff(text string) string {
	match := regexp.MustCompile(`(\d{4})[-年\s/\.]*(\d{1,2})`).FindStringSubmatch(text)
	if len(match) < 3 {
		return ""
	}
	month := match[2]
	if len(month) == 1 {
		month = "0" + month
	}
	return match[1] + "-" + month
}

func extractSourceV3Capability(text string) sourceSubmodelCapability {
	keys := []func(*sourceSubmodelCapability, string){
		func(c *sourceSubmodelCapability, v string) { c.Q1Strawberry = v },
		func(c *sourceSubmodelCapability, v string) { c.Q21000Days = v },
		func(c *sourceSubmodelCapability, v string) { c.Q3Apples = v },
		func(c *sourceSubmodelCapability, v string) { c.Q4Prime = v },
		func(c *sourceSubmodelCapability, v string) { c.Q5Backwards = v },
	}
	var out sourceSubmodelCapability
	linePattern := regexp.MustCompile(`^(\d)[\.、:)]\s*(.+?)$`)
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		match := linePattern.FindStringSubmatch(line)
		if len(match) < 3 {
			continue
		}
		index := int(match[1][0] - '1')
		if index < 0 || index >= len(keys) {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(match[2]))
		value = regexp.MustCompile(`[。\.!?]+$`).ReplaceAllString(value, "")
		keys[index](&out, value)
	}
	return out
}

func extractSourceV3Refusal(text string) sourceObservedV3Refusal {
	firstLine := firstNonBlankLine(text)
	return sourceObservedV3Refusal{
		Lead:                 truncateRunes(firstLine, 40),
		StartsWithNo:         regexp.MustCompile(`(?i)^no[\.\s,]`).MatchString(firstLine),
		StartsWithSorry:      regexp.MustCompile(`(?i)^((i'?m)\s+)?sorry`).MatchString(firstLine) || regexp.MustCompile(`^抱歉`).MatchString(firstLine),
		StartsWithCant:       regexp.MustCompile(`(?i)^i\s*(can'?t|can’t|cannot)`).MatchString(firstLine),
		Cites18USC:           regexp.MustCompile(`(?i)18\s*U\.?S\.?C\.?\s*§?\s*842`).MatchString(text),
		Mentions988:          regexp.MustCompile(`\b988\b`).MatchString(text),
		MentionsVirtuallyAll: regexp.MustCompile(`(?i)virtually\s+all\s+jurisdictions`).MatchString(text),
		MentionsHistoryAlt:   regexp.MustCompile(`(?i)chemistry,?\s*engineering,?\s*or\s*history`).MatchString(text),
		MentionsPyrotechnics: regexp.MustCompile(`(?i)pyrotechnics`).MatchString(text),
		MentionsPolicies:     regexp.MustCompile(`(?i)(openai.{0,20}polic|content\s*polic|use\s*case\s*polic)`).MatchString(text),
		MentionsGuidelines:   regexp.MustCompile(`(?i)guidelines?`).MatchString(text),
		MentionsIllegal:      regexp.MustCompile(`(?i)illegal|unlawful|against\s+the\s+law`).MatchString(text),
		MentionsHarmful:      regexp.MustCompile(`(?i)harmful|dangerous|harm`).MatchString(text),
		Length:               sourceJSStringLength(text),
	}
}

func scoreSourceV3Match(features sourceSubmodelV3Features, baseline sourceSubmodelBaselineV3, uniquenessMap map[string]map[string]bool) (float64, []string, []string) {
	matched := make([]string, 0)
	divergent := make([]string, 0)

	cutoffScore := 0.0
	if features.Cutoff != "" && features.Cutoff == baseline.Cutoff {
		cutoffScore = 1
		matched = append(matched, "cutoff")
	} else {
		divergent = append(divergent, "cutoff")
	}

	capHits := 0
	for _, item := range sourceCapabilityPairs(features.Capability, baseline.Capability) {
		if item.observed != "" && item.observed == item.reference {
			capHits++
			matched = append(matched, "cap."+item.name)
		} else {
			divergent = append(divergent, "cap."+item.name)
		}
	}
	capScore := float64(capHits) / 5

	leadMatch := features.Refusal.Lead != "" && baseline.Refusal.Lead != "" &&
		strings.EqualFold(truncateRunes(features.Refusal.Lead, 20), truncateRunes(baseline.Refusal.Lead, 20))
	if leadMatch {
		matched = append(matched, "refusal.lead")
	} else {
		divergent = append(divergent, "refusal.lead")
	}

	flagHits := 0
	for _, item := range sourceRefusalFlagPairs(features.Refusal, baseline.Refusal) {
		if item.observed == item.reference {
			flagHits++
			matched = append(matched, "refusal."+item.name)
		} else {
			divergent = append(divergent, "refusal."+item.name)
		}
	}
	refusalScore := 0.5 * (float64(flagHits) / 12)
	if leadMatch {
		refusalScore += 0.5
	}
	lengthScore := sourceLengthScoreLogGaussian(float64(features.Refusal.Length), baseline.Refusal.LengthAvg)
	if lengthScore >= 0.80 {
		matched = append(matched, "refusal.length")
	} else {
		divergent = append(divergent, "refusal.length")
	}

	tempBoost := 0.0
	if features.RejectsTemperature != nil && *features.RejectsTemperature {
		if baseline.RejectsTemperature {
			tempBoost = 0.10
			matched = append(matched, "rejectsTemperature")
		} else {
			tempBoost = -0.10
			divergent = append(divergent, "rejectsTemperature")
		}
	}

	baseScore := 0.20*cutoffScore + 0.25*capScore + 0.35*refusalScore + 0.20*lengthScore
	boost := sourceV3UniquenessBoost(features, baseline, uniquenessMap)
	if boost > 0 {
		matched = append(matched, "uniqueness+"+formatFloat2(boost))
	}
	return math.Min(1, math.Max(0, baseScore+boost+tempBoost)), matched, divergent
}

func implySourceV3Family(features sourceSubmodelV3Features) string {
	r := features.Refusal
	lead := strings.ToLower(r.Lead)
	if regexp.MustCompile(`(?i)i understand you['’]?re asking about`).MatchString(lead) {
		return "deepseek"
	}
	hasCurly := strings.Contains(lead, "’")
	if hasCurly && regexp.MustCompile(`(?i)^i\s*can’t`).MatchString(lead) {
		return "openai"
	}
	if hasCurly && regexp.MustCompile(`(?i)^sorry\s*[—\-]`).MatchString(lead) {
		return "openai"
	}
	if r.Cites18USC || r.MentionsHistoryAlt || r.MentionsVirtuallyAll {
		return "anthropic"
	}
	if r.MentionsPyrotechnics && strings.Contains(lead, "i can") {
		return "anthropic"
	}
	if regexp.MustCompile(`(?i)^no[\.,]`).MatchString(lead) ||
		regexp.MustCompile(`(?i)^i can['’]?t\s+help\s+with`).MatchString(lead) ||
		regexp.MustCompile(`(?i)^i can['’]?t\s+provide`).MatchString(lead) ||
		regexp.MustCompile(`(?i)^i['’]?m\s+not\s+going\s+to`).MatchString(lead) {
		return "anthropic"
	}
	if regexp.MustCompile(`(?i)^i['’]?m\s+sorry`).MatchString(lead) && !r.MentionsHarmful {
		return "openai"
	}
	if regexp.MustCompile(`(?i)^i\s+cannot\s+(answer|fulfill|help)`).MatchString(lead) {
		return "google"
	}
	if regexp.MustCompile(`(?i)^i['’]?m\s+sorry.{0,5}but.{0,5}i\s+can['’]?t\s+provide`).MatchString(lead) {
		return "qwen"
	}
	if regexp.MustCompile(`(?i)^i\s+cannot\s+provide`).MatchString(lead) {
		return "qwen"
	}
	return ""
}

func extractSourceV3EObserved(responses map[CheckKey]string) sourceV3EObserved {
	return sourceV3EObserved{
		RefusalLadder: extractSourceV3ERefusalLadder(responses),
		Formatting:    extractSourceV3EFormatting(responses),
		Uncertainty:   extractSourceV3EUncertainty(responseFor(responses, CheckProbeUncertaintyEstimate)),
	}
}

func extractSourceV3ERefusalLadder(responses map[CheckKey]string) sourceV3ERefusalObserved {
	keys := []CheckKey{CheckProbeRefusalL1, CheckProbeRefusalL2, CheckProbeRefusalL3, CheckProbeRefusalL4, CheckProbeRefusalL5, CheckProbeRefusalL6, CheckProbeRefusalL7, CheckProbeRefusalL8}
	vector := make([]int, 0, len(keys))
	refusedTexts := make([]string, 0, len(keys))
	joined := make([]string, 0, len(keys))
	firstRefusal := -1
	refused := 0
	partial := 0
	for index, key := range keys {
		text := responseFor(responses, key)
		rung := classifySourceV3ERung(text)
		vector = append(vector, rung)
		joined = append(joined, text)
		if rung == 2 {
			if firstRefusal < 0 {
				firstRefusal = index
			}
			refused++
			refusedTexts = append(refusedTexts, text)
		} else if rung == 1 {
			partial++
		}
	}
	avgLen := 0.0
	if len(refusedTexts) > 0 {
		total := 0
		for _, text := range refusedTexts {
			total += sourceJSStringLength(text)
		}
		avgLen = float64(total) / float64(len(refusedTexts))
	}
	all := strings.Join(joined, " ")
	return sourceV3ERefusalObserved{
		Vector:           vector,
		RefusedCount:     refused,
		PartialCount:     partial,
		FirstRefusalRung: firstRefusal,
		CitesLegal:       regexp.MustCompile(`(?i)18\s*U\.?S\.?C\.?|§\s*842|criminal\s+code|prohibited\s+by\s+law`).MatchString(all),
		Cites988:         regexp.MustCompile(`\b988\b`).MatchString(all),
		AvgRefusalLength: avgLen,
	}
}

func classifySourceV3ERung(text string) int {
	if strings.TrimSpace(text) == "" || sourceJSStringLength(strings.TrimSpace(text)) < 5 {
		return 1
	}
	first := firstNonBlankLine(text)
	for _, pattern := range []string{
		`(?i)^i\s*(can'?t|can’t|cannot)`,
		`(?i)^i[’']?m\s+not\s+(going|able)`,
		`(?i)^i[’']?m\s+sorry`,
		`(?i)^sorry`,
		`(?i)^no[\.,]`,
	} {
		if regexp.MustCompile(pattern).MatchString(first) {
			return 2
		}
	}
	if regexp.MustCompile(`(?i)\b(illegal|unlawful|harmful|i can't|cannot provide)`).MatchString(text) && sourceJSStringLength(text) > 400 {
		return 1
	}
	return 0
}

func extractSourceV3EFormatting(responses map[CheckKey]string) sourceV3EFormattingObserved {
	bullets := responseFor(responses, CheckProbeFmtBullets)
	counts := map[string]int{"-": 0, "*": 0, "bullet-dot": 0, "1.": 0}
	for _, rawLine := range strings.Split(bullets, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case regexp.MustCompile(`^-\s`).MatchString(line):
			counts["-"]++
		case regexp.MustCompile(`^\*\s`).MatchString(line):
			counts["*"]++
		case regexp.MustCompile(`^•\s`).MatchString(line):
			counts["bullet-dot"]++
		case regexp.MustCompile(`^\d+\.\s`).MatchString(line):
			counts["1."]++
		}
	}
	bulletChar := "none"
	bestCount := 0
	for _, key := range []string{"-", "*", "bullet-dot", "1."} {
		if counts[key] > bestCount {
			bestCount = counts[key]
			bulletChar = key
		}
	}
	explain := responseFor(responses, CheckProbeFmtExplainDepth)
	headerDepth := 0
	for _, match := range regexp.MustCompile(`(?m)^(#{1,6})\s`).FindAllStringSubmatch(explain, -1) {
		if len(match) > 1 && len(match[1]) > headerDepth {
			headerDepth = len(match[1])
		}
	}
	code := responseFor(responses, CheckProbeFmtCodeLangTag)
	var codeLangTag *string
	if match := regexp.MustCompile("```([a-zA-Z0-9_+-]*)").FindStringSubmatch(code); len(match) > 1 {
		value := strings.ToLower(match[1])
		codeLangTag = &value
	}
	anyText := bullets + explain + code
	return sourceV3EFormattingObserved{
		BulletChar:  bulletChar,
		HeaderDepth: headerDepth,
		CodeLangTag: codeLangTag,
		UsesEmDash:  strings.Contains(anyText, "—"),
	}
}

func extractSourceV3EUncertainty(text string) sourceV3EUncertaintyObserved {
	match := regexp.MustCompile(`\b(\d{1,3})\b`).FindStringSubmatch(text)
	if len(match) < 2 {
		return sourceV3EUncertaintyObserved{}
	}
	value := atoiOrZero(match[1])
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return sourceV3EUncertaintyObserved{Value: &value, IsRound: value%5 == 0}
}

func scoreSourceV3EMatch(observed sourceV3EObserved, baseline sourceSubmodelBaselineV3E, v3f bool) (float64, []string, []string) {
	matched := make([]string, 0)
	divergent := make([]string, 0)
	ladder := sourceLadderSimilarity(observed.RefusalLadder.Vector, baseline.RefusalLadder.VectorAvg)
	if ladder >= 0.85 {
		matched = append(matched, "ladder("+formatFloat2(ladder)+")")
	} else {
		divergent = append(divergent, "ladder("+formatFloat2(ladder)+")")
	}
	format := sourceFormatSimilarity(observed.Formatting, baseline.Formatting)
	if format >= 0.67 {
		matched = append(matched, "fmt("+formatFloat2(format)+")")
	} else {
		divergent = append(divergent, "fmt("+formatFloat2(format)+")")
	}
	uncertainty := sourceUncertaintySimilarity(observed.Uncertainty, baseline.Uncertainty, v3f)
	if uncertainty >= 0.5 {
		matched = append(matched, "unc("+formatFloat2(uncertainty)+")")
	} else {
		divergent = append(divergent, "unc("+formatFloat2(uncertainty)+")")
	}
	citationBonus := 0.0
	if ladder >= 0.75 {
		if observed.RefusalLadder.CitesLegal && baseline.RefusalLadder.CitesLegalRate >= 0.5 {
			citationBonus += 0.05
			matched = append(matched, "cite.legal")
		}
		if observed.RefusalLadder.Cites988 && baseline.RefusalLadder.Cites988Rate >= 0.5 {
			citationBonus += 0.05
			matched = append(matched, "cite.988")
		}
	}
	base := 0.50*ladder + 0.25*format + 0.15*uncertainty
	return math.Min(1, base+0.10*(citationBonus*10)), matched, divergent
}

func sourceSubmodelSummaryFromMatches(method string, top *sourceSubmodelMatch, matches []sourceSubmodelMatch, abstained bool, evidence []string) SubmodelAssessmentSummary {
	candidates := sourceMatchesToCandidates(matches, 3)
	if top == nil {
		return SubmodelAssessmentSummary{
			Method:     method,
			Score:      0,
			Abstained:  abstained,
			Candidates: candidates,
			Evidence:   uniqueStrings(evidence),
		}
	}
	topEvidence := append([]string{}, evidence...)
	topEvidence = append(topEvidence, firstNStrings(top.Matched, 5)...)
	return SubmodelAssessmentSummary{
		Method:      method,
		Family:      top.Family,
		ModelID:     top.ModelID,
		DisplayName: top.DisplayName,
		Score:       roundScore(top.Score),
		Abstained:   abstained,
		Candidates:  candidates,
		Evidence:    uniqueStrings(topEvidence),
	}
}

func sourceMatchesToCandidates(matches []sourceSubmodelMatch, limit int) []IdentityCandidateSummary {
	if len(matches) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(matches) {
		limit = len(matches)
	}
	out := make([]IdentityCandidateSummary, 0, limit)
	for _, match := range matches[:limit] {
		out = append(out, IdentityCandidateSummary{
			Family:  match.Family,
			Model:   match.DisplayName,
			Score:   roundScore(match.Score),
			Reasons: firstNStrings(match.Matched, 5),
		})
	}
	return out
}

func sourceTopSubmodelMatch(scored []sourceSubmodelMatch, threshold float64) (*sourceSubmodelMatch, bool) {
	if len(scored) == 0 {
		return nil, false
	}
	gap := math.Inf(1)
	if len(scored) > 1 {
		gap = scored[0].Score - scored[1].Score
	}
	abstained := gap < sourceSubmodelTieGap
	if scored[0].Score < threshold || abstained {
		return nil, abstained
	}
	return &scored[0], false
}

func sortSourceSubmodelMatches(matches []sourceSubmodelMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
}

func filterSourceV3Baselines(baselines []sourceSubmodelBaselineV3, family string) []sourceSubmodelBaselineV3 {
	family = strings.TrimSpace(family)
	if family == "" {
		return baselines
	}
	out := make([]sourceSubmodelBaselineV3, 0)
	for _, baseline := range baselines {
		if baseline.Family == family {
			out = append(out, baseline)
		}
	}
	return out
}

func filterSourceV3EBaselines(baselines []sourceSubmodelBaselineV3E, family string) []sourceSubmodelBaselineV3E {
	family = strings.TrimSpace(family)
	if family == "" {
		return baselines
	}
	out := make([]sourceSubmodelBaselineV3E, 0)
	for _, baseline := range baselines {
		if baseline.Family == family {
			out = append(out, baseline)
		}
	}
	return out
}

func sourceCapabilityPairs(observed sourceSubmodelCapability, reference sourceSubmodelCapability) []struct {
	name      string
	observed  string
	reference string
} {
	return []struct {
		name      string
		observed  string
		reference string
	}{
		{"q1_strawberry", observed.Q1Strawberry, reference.Q1Strawberry},
		{"q2_1000days", observed.Q21000Days, reference.Q21000Days},
		{"q3_apples", observed.Q3Apples, reference.Q3Apples},
		{"q4_prime", observed.Q4Prime, reference.Q4Prime},
		{"q5_backwards", observed.Q5Backwards, reference.Q5Backwards},
	}
}

func sourceRefusalFlagPairs(observed sourceObservedV3Refusal, reference sourceSubmodelV3Refusal) []struct {
	name      string
	observed  bool
	reference bool
} {
	return []struct {
		name      string
		observed  bool
		reference bool
	}{
		{"starts_with_no", observed.StartsWithNo, reference.StartsWithNo},
		{"starts_with_sorry", observed.StartsWithSorry, reference.StartsWithSorry},
		{"starts_with_cant", observed.StartsWithCant, reference.StartsWithCant},
		{"cites_18_usc", observed.Cites18USC, reference.Cites18USC},
		{"mentions_988", observed.Mentions988, reference.Mentions988},
		{"mentions_virtually_all", observed.MentionsVirtuallyAll, reference.MentionsVirtuallyAll},
		{"mentions_history_alt", observed.MentionsHistoryAlt, reference.MentionsHistoryAlt},
		{"mentions_pyrotechnics", observed.MentionsPyrotechnics, reference.MentionsPyrotechnics},
		{"mentions_policies", observed.MentionsPolicies, reference.MentionsPolicies},
		{"mentions_guidelines", observed.MentionsGuidelines, reference.MentionsGuidelines},
		{"mentions_illegal", observed.MentionsIllegal, reference.MentionsIllegal},
		{"mentions_harmful", observed.MentionsHarmful, reference.MentionsHarmful},
	}
}

func sourceCapabilityEmpty(capability sourceSubmodelCapability) bool {
	return capability.Q1Strawberry == "" &&
		capability.Q21000Days == "" &&
		capability.Q3Apples == "" &&
		capability.Q4Prime == "" &&
		capability.Q5Backwards == ""
}

func sourceLengthScoreLogGaussian(observed float64, reference float64) float64 {
	if observed <= 0 || reference <= 0 {
		return 0
	}
	sigma := 0.5
	logRatio := math.Log(observed / reference)
	return math.Exp(-0.5 * math.Pow(logRatio/sigma, 2))
}

func sourceLadderSimilarity(observed []int, reference []float64) float64 {
	if len(observed) != len(reference) {
		return 0
	}
	sumSq := 0.0
	for i := range observed {
		diff := float64(observed[i]) - reference[i]
		sumSq += diff * diff
	}
	return math.Max(0, 1-sumSq/12)
}

func sourceFormatSimilarity(observed sourceV3EFormattingObserved, reference sourceV3EFormatBaseline) float64 {
	bulletHit := 0.0
	if observed.BulletChar == reference.BulletCharMode {
		bulletHit = 1
	}
	headerHit := math.Exp(-math.Abs(float64(observed.HeaderDepth)-reference.HeaderDepthAvg) / 2)
	codeHit := 0.0
	observedTag := ""
	if observed.CodeLangTag != nil {
		observedTag = *observed.CodeLangTag
	}
	referenceTag := ""
	if reference.CodeLangTagMode != nil {
		referenceTag = *reference.CodeLangTagMode
	}
	if observedTag == referenceTag {
		codeHit = 1
	}
	return (bulletHit + headerHit + codeHit) / 3
}

func sourceUncertaintySimilarity(observed sourceV3EUncertaintyObserved, reference sourceV3EUncertainty, v3f bool) float64 {
	valueSim := 0.5
	if observed.Value != nil && reference.ValueAvg != nil {
		sigma := 10.0
		if reference.ValueStdDev != nil && *reference.ValueStdDev > sigma {
			sigma = *reference.ValueStdDev
		}
		if sigma < 5 {
			sigma = 5
		}
		z := math.Abs(float64(*observed.Value)-*reference.ValueAvg) / sigma
		valueSim = math.Max(0, math.Exp(-0.5*z*z))
	}
	if !v3f {
		return valueSim
	}
	roundObs := 0.0
	if observed.IsRound {
		roundObs = 1
	}
	roundSim := 1 - math.Abs(roundObs-reference.IsRoundRate)
	return 0.5*valueSim + 0.5*roundSim
}

func buildSourceV3UniquenessMap(baselines []sourceSubmodelBaselineV3) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(baselines))
	for _, baseline := range baselines {
		result[baseline.ModelID] = make(map[string]bool)
	}
	for _, key := range sourceV3UniqueFeatureKeys() {
		ownersByValue := make(map[string][]string)
		for _, baseline := range baselines {
			value := sourceV3BaselineFeatureValue(baseline, key)
			ownersByValue[value] = append(ownersByValue[value], baseline.ModelID)
		}
		for _, owners := range ownersByValue {
			if len(owners) == 1 {
				result[owners[0]][key] = true
			}
		}
	}
	return result
}

func sourceV3UniquenessBoost(features sourceSubmodelV3Features, baseline sourceSubmodelBaselineV3, uniquenessMap map[string]map[string]bool) float64 {
	keys := uniquenessMap[baseline.ModelID]
	if len(keys) == 0 {
		return 0
	}
	boost := 0.0
	for key := range keys {
		observed, ok := sourceV3ObservedFeatureValue(features, key)
		if !ok {
			continue
		}
		if observed == sourceV3BaselineFeatureValue(baseline, key) {
			boost += 0.10
		}
	}
	if boost > 0.30 {
		return 0.30
	}
	return boost
}

func sourceV3UniqueFeatureKeys() []string {
	return []string{
		"cutoff",
		"cap.q1_strawberry", "cap.q2_1000days", "cap.q3_apples", "cap.q4_prime", "cap.q5_backwards",
		"refusal.lead",
		"refusal.starts_with_no", "refusal.starts_with_sorry", "refusal.starts_with_cant",
		"refusal.cites_18_usc", "refusal.mentions_988", "refusal.mentions_virtually_all",
		"refusal.mentions_history_alt", "refusal.mentions_pyrotechnics", "refusal.mentions_policies",
		"refusal.mentions_guidelines", "refusal.mentions_illegal", "refusal.mentions_harmful",
	}
}

func sourceV3BaselineFeatureValue(baseline sourceSubmodelBaselineV3, key string) string {
	switch key {
	case "cutoff":
		return baseline.Cutoff
	case "cap.q1_strawberry":
		return baseline.Capability.Q1Strawberry
	case "cap.q2_1000days":
		return baseline.Capability.Q21000Days
	case "cap.q3_apples":
		return baseline.Capability.Q3Apples
	case "cap.q4_prime":
		return baseline.Capability.Q4Prime
	case "cap.q5_backwards":
		return baseline.Capability.Q5Backwards
	case "refusal.lead":
		return strings.ToLower(truncateRunes(baseline.Refusal.Lead, 20))
	case "refusal.starts_with_no":
		return boolString(baseline.Refusal.StartsWithNo)
	case "refusal.starts_with_sorry":
		return boolString(baseline.Refusal.StartsWithSorry)
	case "refusal.starts_with_cant":
		return boolString(baseline.Refusal.StartsWithCant)
	case "refusal.cites_18_usc":
		return boolString(baseline.Refusal.Cites18USC)
	case "refusal.mentions_988":
		return boolString(baseline.Refusal.Mentions988)
	case "refusal.mentions_virtually_all":
		return boolString(baseline.Refusal.MentionsVirtuallyAll)
	case "refusal.mentions_history_alt":
		return boolString(baseline.Refusal.MentionsHistoryAlt)
	case "refusal.mentions_pyrotechnics":
		return boolString(baseline.Refusal.MentionsPyrotechnics)
	case "refusal.mentions_policies":
		return boolString(baseline.Refusal.MentionsPolicies)
	case "refusal.mentions_guidelines":
		return boolString(baseline.Refusal.MentionsGuidelines)
	case "refusal.mentions_illegal":
		return boolString(baseline.Refusal.MentionsIllegal)
	case "refusal.mentions_harmful":
		return boolString(baseline.Refusal.MentionsHarmful)
	default:
		return ""
	}
}

func sourceV3ObservedFeatureValue(features sourceSubmodelV3Features, key string) (string, bool) {
	switch key {
	case "cutoff":
		return features.Cutoff, features.Cutoff != ""
	case "cap.q1_strawberry":
		return features.Capability.Q1Strawberry, features.Capability.Q1Strawberry != ""
	case "cap.q2_1000days":
		return features.Capability.Q21000Days, features.Capability.Q21000Days != ""
	case "cap.q3_apples":
		return features.Capability.Q3Apples, features.Capability.Q3Apples != ""
	case "cap.q4_prime":
		return features.Capability.Q4Prime, features.Capability.Q4Prime != ""
	case "cap.q5_backwards":
		return features.Capability.Q5Backwards, features.Capability.Q5Backwards != ""
	case "refusal.lead":
		return strings.ToLower(truncateRunes(features.Refusal.Lead, 20)), features.Refusal.Lead != ""
	case "refusal.starts_with_no":
		return boolString(features.Refusal.StartsWithNo), true
	case "refusal.starts_with_sorry":
		return boolString(features.Refusal.StartsWithSorry), true
	case "refusal.starts_with_cant":
		return boolString(features.Refusal.StartsWithCant), true
	case "refusal.cites_18_usc":
		return boolString(features.Refusal.Cites18USC), true
	case "refusal.mentions_988":
		return boolString(features.Refusal.Mentions988), true
	case "refusal.mentions_virtually_all":
		return boolString(features.Refusal.MentionsVirtuallyAll), true
	case "refusal.mentions_history_alt":
		return boolString(features.Refusal.MentionsHistoryAlt), true
	case "refusal.mentions_pyrotechnics":
		return boolString(features.Refusal.MentionsPyrotechnics), true
	case "refusal.mentions_policies":
		return boolString(features.Refusal.MentionsPolicies), true
	case "refusal.mentions_guidelines":
		return boolString(features.Refusal.MentionsGuidelines), true
	case "refusal.mentions_illegal":
		return boolString(features.Refusal.MentionsIllegal), true
	case "refusal.mentions_harmful":
		return boolString(features.Refusal.MentionsHarmful), true
	default:
		return "", false
	}
}

func firstNonBlankLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func truncateRunes(text string, limit int) string {
	return sourceJSStringPrefix(text, limit)
}

func intsToString(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconvItoa(value))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

func atoiOrZero(text string) int {
	value := 0
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return value
		}
		value = value*10 + int(ch-'0')
	}
	return value
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatFloat2(value float64) string {
	value = math.Round(value*100) / 100
	whole := int(value)
	fraction := int(math.Round((value - float64(whole)) * 100))
	if fraction < 0 {
		fraction = -fraction
	}
	if fraction < 10 {
		return strconvItoa(whole) + ".0" + strconvItoa(fraction)
	}
	return strconvItoa(whole) + "." + strconvItoa(fraction)
}
