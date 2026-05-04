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

func TestBuildReportWithOptionsUsesOptionalJudgeSignal(t *testing.T) {
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
	if report.IdentityAssessments[0].PredictedFamily != "anthropic" {
		t.Fatalf("assessment = %+v, want optional judge to affect fused predicted family", report.IdentityAssessments[0])
	}
	if strings.Contains(mustMarshalForTest(report), apiKey) {
		t.Fatal("report leaked judge API key")
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
	if len(scores) == 0 || scores[0].Family != "openai" || scores[0].Score != 1 {
		t.Fatalf("vector scores = %+v, want OpenAI first with score 1", scores)
	}
}
