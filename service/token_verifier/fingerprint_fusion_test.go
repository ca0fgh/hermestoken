package token_verifier

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

func TestSourceWeightedCandidatesProduceNormalizedScores(t *testing.T) {
	responses := map[CheckKey]string{
		CheckProbeIdentitySelfKnowledge:  "I am Claude, created by Anthropic.",
		CheckProbeIdentityRefusalPattern: "I can't help with that request, but I can offer safer alternatives.",
		CheckProbeIdentityStyleEN:        "I can't verify private deployment details, but I can reason through the tradeoffs carefully.",
	}
	results := []CheckResult{
		identityProbeResultFor(ProviderAnthropic, "claude-opus-4-7", CheckProbeIdentitySelfKnowledge, responses[CheckProbeIdentitySelfKnowledge]),
		identityProbeResultFor(ProviderAnthropic, "claude-opus-4-7", CheckProbeIdentityRefusalPattern, responses[CheckProbeIdentityRefusalPattern]),
		identityProbeResultFor(ProviderAnthropic, "claude-opus-4-7", CheckProbeIdentityStyleEN, responses[CheckProbeIdentityStyleEN]),
	}

	candidates := sourceBehavioralIdentityCandidates(results, responses)

	if len(candidates) == 0 {
		t.Fatal("expected source weighted behavioral candidates")
	}
	if candidates[0].Family != "anthropic" {
		t.Fatalf("top family = %q, want anthropic: %+v", candidates[0].Family, candidates)
	}
	if candidates[0].Score <= 0 || candidates[0].Score > 1 {
		t.Fatalf("top score = %.2f, want normalized score in (0,1]", candidates[0].Score)
	}
}

func TestAggregateSourceTimingFeaturesIncludesOutputLengthBuckets(t *testing.T) {
	features := aggregateSourceTimingFeatures([]CheckResult{
		{CheckKey: CheckProbeIdentityStyleEN, TTFTMs: 400, TokensPS: 10, OutputTokens: intPtr(25)},
		{CheckKey: CheckProbeIdentitySelfKnowledge, TTFTMs: 800, TokensPS: 30, OutputTokens: intPtr(75)},
		{CheckKey: CheckProbeIdentityListFormat, TTFTMs: 1600, TokensPS: 80, OutputTokens: intPtr(240)},
		{CheckKey: CheckCanaryMathMul, TTFTMs: 10, TokensPS: 900, OutputTokens: intPtr(500)},
		{CheckKey: CheckProbeSignatureRoundtrip, TTFTMs: 10, TokensPS: 900, OutputTokens: intPtr(500)},
	})

	if features["out_len_normal"] != 1 || features["out_len_terse"] != 0 || features["out_len_verbose"] != 0 {
		t.Fatalf("output length buckets = %+v, want LLMprobe median output bucket normal", features)
	}
	if math.Abs(features["out_median_norm"]-0.15) > 1e-9 {
		t.Fatalf("out_median_norm = %v, want 0.15", features["out_median_norm"])
	}
	if features["tps_bucket_medium"] != 1 || features["ttft_bucket_normal"] != 1 {
		t.Fatalf("timing buckets = %+v, want source timed items only", features)
	}
}

func TestFuseIdentityCandidatesRedistributesMissingOptionalWeights(t *testing.T) {
	rule := []IdentityCandidateSummary{{Family: "openai", Model: "OpenAI GPT", Score: 0.6, Reasons: []string{"rule"}}}
	judge := []IdentityCandidateSummary{{Family: "anthropic", Model: "Anthropic Claude", Score: 1, Reasons: []string{"judge"}}}

	ruleOnly := fuseIdentityCandidates(rule, nil, nil)
	if len(ruleOnly) != 1 || ruleOnly[0].Family != "openai" || ruleOnly[0].Score != 0.6 {
		t.Fatalf("rule-only fusion = %+v, want unchanged OpenAI score", ruleOnly)
	}

	fused := fuseIdentityCandidates(rule, judge, nil)
	if len(fused) == 0 || fused[0].Family != "anthropic" {
		t.Fatalf("fused candidates = %+v, want judge-supported Anthropic first", fused)
	}
}

func TestFuseIdentityCandidatesPreservesSourceOrderForTies(t *testing.T) {
	rule := []IdentityCandidateSummary{
		{Family: "z-family", Model: "Z Family", Score: 0.5},
		{Family: "a-family", Model: "A Family", Score: 0.5},
	}

	fused := fuseIdentityCandidates(rule, nil, nil)
	if len(fused) < 2 {
		t.Fatalf("fused candidates = %+v, want at least two tied candidates", fused)
	}
	if fused[0].Family != "z-family" || fused[1].Family != "a-family" {
		t.Fatalf("fused candidates = %+v, want LLMprobe stable source order for equal scores", fused)
	}
}

func TestJudgeFingerprintParsesAndDoesNotExposeAPIKey(t *testing.T) {
	const apiKey = "judge-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Fatalf("Authorization = %q, want bearer key", got)
		}
		responseBytes, _ := common.Marshal(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"family":"anthropic","confidence":0.91,"reasons":["refusal style"]}`}},
			},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	candidates, result := judgeFingerprint(context.Background(), NewCurlExecutor(time.Second), map[string]string{
		"identity_refusal_pattern": "I can't help with that.",
	}, IdentityJudgeConfig{BaseURL: server.URL, APIKey: apiKey, ModelID: "judge-model"})

	if result == nil || result.Family != "anthropic" || result.Confidence < 0.9 {
		t.Fatalf("judge result = %+v, want anthropic", result)
	}
	if len(candidates) == 0 || candidates[0].Family != "anthropic" {
		t.Fatalf("judge candidates = %+v, want anthropic", candidates)
	}
	rendered := mustMarshalForTest(candidates)
	if strings.Contains(rendered, apiKey) {
		t.Fatalf("judge output leaked API key: %s", rendered)
	}
}

func TestSourceFingerprintTextInputsUseLLMProbeSuiteOrder(t *testing.T) {
	responses := map[string]string{
		"submodel_refusal":        "refusal text",
		"identity_style_en":       "style text",
		"identity_self_knowledge": "self text",
	}
	ordered := sourceOrderedResponseIDs(responses)
	want := []string{"identity_style_en", "identity_self_knowledge", "submodel_refusal"}
	if strings.Join(ordered, ",") != strings.Join(want, ",") {
		t.Fatalf("ordered response IDs = %v, want LLMprobe suite order %v", ordered, want)
	}

	prompt := buildJudgeIdentityPrompt(responses)
	styleIdx := strings.Index(prompt, "[identity_style_en]")
	selfIdx := strings.Index(prompt, "[identity_self_knowledge]")
	refusalIdx := strings.Index(prompt, "[submodel_refusal]")
	if styleIdx < 0 || selfIdx < 0 || refusalIdx < 0 || !(styleIdx < selfIdx && selfIdx < refusalIdx) {
		t.Fatalf("judge prompt order drifted:\n%s", prompt)
	}

	texts := sourceAllTexts(responses, map[string][]string{
		"tok_split_word": {"tok split"},
		"ling_kr_num":    {"kr answer"},
	})
	joined := strings.Join(texts, "|")
	if joined != "style text|self text|refusal text|kr answer|tok split" {
		t.Fatalf("sourceAllTexts = %q, want LLMprobe response order then linguistic order", joined)
	}
}

func TestBuildReportWithOptionsUsesOptionalJudgeSignalOnHigherFusedScore(t *testing.T) {
	const apiKey = "judge-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseBytes, _ := common.Marshal(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"family":"anthropic","confidence":1,"reasons":["judge override"]}`}},
			},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	results := []CheckResult{
		identityProbeResultFor(ProviderOpenAI, "gpt-5.5", CheckProbeIdentityStyleEN, "Neutral writing style without a strong family self-claim."),
	}
	report := BuildReportWithOptions(context.Background(), results, ReportOptions{
		IdentityJudge: &IdentityJudgeConfig{BaseURL: server.URL, APIKey: apiKey, ModelID: "judge-model"},
		Executor:      NewCurlExecutor(time.Second),
	})

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	if report.IdentityAssessments[0].PredictedFamily != "anthropic" {
		t.Fatalf("assessment = %+v, want optional judge to affect fused predicted family", report.IdentityAssessments[0])
	}
	if strings.Contains(mustMarshalForTest(report), apiKey) {
		t.Fatal("report leaked judge API key")
	}
}

func TestBuildReportWithOptionsPreservesRuleOrderOnJudgeTie(t *testing.T) {
	const apiKey = "judge-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseBytes, _ := common.Marshal(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"family":"anthropic","confidence":1,"reasons":["judge tie"]}`}},
			},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	results := []CheckResult{
		identityProbeResultFor(ProviderOpenAI, "gpt-5.5", CheckProbeIdentitySelfKnowledge, "I am ChatGPT from OpenAI."),
		identityProbeResultFor(ProviderOpenAI, "gpt-5.5", CheckProbeIdentityStyleEN, "Certainly. I can help with a concise answer."),
	}
	report := BuildReportWithOptions(context.Background(), results, ReportOptions{
		IdentityJudge: &IdentityJudgeConfig{BaseURL: server.URL, APIKey: apiKey, ModelID: "judge-model"},
		Executor:      NewCurlExecutor(time.Second),
	})

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	if report.IdentityAssessments[0].PredictedFamily != "openai" {
		t.Fatalf("assessment = %+v, want LLMprobe stable rule-first ordering on equal fused score", report.IdentityAssessments[0])
	}
}

func TestVectorFingerprintScores(t *testing.T) {
	if got := cosineSimilarity([]float64{1, 0}, []float64{0, 1}); got != 0 {
		t.Fatalf("orthogonal cosine = %v, want 0", got)
	}
	scores := pickTopVectorScores([]float64{1, 0}, []EmbeddingReference{
		{Family: "openai", Embedding: []float64{1, 0}},
		{Family: "anthropic", Embedding: []float64{0, 1}},
	})
	openai := identityCandidateByFamily(scores, "openai")
	if openai == nil || openai.Score != 1 {
		t.Fatalf("vector scores = %+v, want OpenAI score 1", scores)
	}
	if len(scores) == 0 || scores[0].Family != "anthropic" {
		t.Fatalf("vector scores = %+v, want LLMprobe fixed family order before fusion", scores)
	}
}

func TestVectorFingerprintScoresReturnZeroFamiliesWhenUnavailable(t *testing.T) {
	noRefs := pickTopVectorScores([]float64{1, 0}, nil)
	if len(noRefs) != len(sourceVectorFamilies) {
		t.Fatalf("no-ref vector scores length = %d, want known family count %d", len(noRefs), len(sourceVectorFamilies))
	}
	for _, score := range noRefs {
		if score.Score != 0 {
			t.Fatalf("no-ref vector scores = %+v, want all zero", noRefs)
		}
	}

	emptyQuery := pickTopVectorScores(nil, []EmbeddingReference{{Family: "anthropic", Embedding: []float64{1, 0}}})
	if len(emptyQuery) != len(sourceVectorFamilies) {
		t.Fatalf("empty-query vector scores length = %d, want known family count %d", len(emptyQuery), len(sourceVectorFamilies))
	}
	for _, score := range emptyQuery {
		if score.Score != 0 {
			t.Fatalf("empty-query vector scores = %+v, want all zero", emptyQuery)
		}
	}
}

func TestRunIdentityVectorSignalRequiresReferencesLikeLLMProbe(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		responseBytes, _ := common.Marshal(map[string]any{
			"data": []map[string]any{{"embedding": []float64{1, 0}}},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	candidates := runIdentityVectorSignal(context.Background(), ReportOptions{
		Embedding: &EmbeddingConfig{BaseURL: server.URL, APIKey: "embed-key", ModelID: "embed-model"},
		Executor:  NewCurlExecutor(time.Second),
	}, map[string]string{"identity_style_en": "hello"})

	if called {
		t.Fatal("embedding endpoint was called without references; LLMprobe skips vector signal when references.length is 0")
	}
	if len(candidates) != 0 {
		t.Fatalf("vector candidates = %+v, want none without references", candidates)
	}
}

func TestEmbedProbeResponsesUsesLLMProbeSuiteOrder(t *testing.T) {
	var input string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := common.DecodeJson(r.Body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		input, _ = payload["input"].(string)
		responseBytes, _ := common.Marshal(map[string]any{
			"data": []map[string]any{{"embedding": []float64{1, 0}}},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	_, ok := embedProbeResponses(context.Background(), NewCurlExecutor(time.Second), map[string]string{
		"submodel_refusal":        "refusal text",
		"identity_style_en":       "style text",
		"identity_self_knowledge": "self text",
	}, EmbeddingConfig{BaseURL: server.URL, APIKey: "embed-key", ModelID: "embed-model"})
	if !ok {
		t.Fatal("embedding request failed")
	}
	styleIdx := strings.Index(input, "[identity_style_en]")
	selfIdx := strings.Index(input, "[identity_self_knowledge]")
	refusalIdx := strings.Index(input, "[submodel_refusal]")
	if styleIdx < 0 || selfIdx < 0 || refusalIdx < 0 || !(styleIdx < selfIdx && selfIdx < refusalIdx) {
		t.Fatalf("embedding input order drifted: %q", input)
	}
}

func TestVectorFingerprintScoresPreserveLLMProbeFamilyOrderForTies(t *testing.T) {
	scores := pickTopVectorScores([]float64{1, 0}, []EmbeddingReference{
		{Family: "qwen", Embedding: []float64{1, 0}},
		{Family: "deepseek", Embedding: []float64{1, 0}},
	})
	qwenIndex := identityCandidateIndex(scores, "qwen")
	deepseekIndex := identityCandidateIndex(scores, "deepseek")
	if qwenIndex < 0 || deepseekIndex < 0 {
		t.Fatalf("vector scores = %+v, want qwen and deepseek entries", scores)
	}
	if qwenIndex > deepseekIndex {
		t.Fatalf("vector scores = %+v, want LLMprobe known-family order qwen before deepseek on ties", scores)
	}
}

func identityCandidateByFamily(items []IdentityCandidateSummary, family string) *IdentityCandidateSummary {
	for i := range items {
		if items[i].Family == family {
			return &items[i]
		}
	}
	return nil
}

func identityCandidateIndex(items []IdentityCandidateSummary, family string) int {
	for i, item := range items {
		if item.Family == family {
			return i
		}
	}
	return -1
}
