package token_verifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

func TestBuildReportIncludesBehavioralIdentityAssessment(t *testing.T) {
	results := append(fastModelResults(ProviderOpenAI, "gpt-5.5", 800, 300, 1200, 120),
		identityProbeResult(CheckProbeIdentitySelfKnowledge, "I am ChatGPT, a model created by OpenAI."),
		identityProbeResult(CheckProbeIdentityStyleEN, "Certainly. Software engineers should develop strong systems thinking and clear communication."),
		identityProbeResult(CheckProbeIdentityReasoningShape, "Let's solve it step by step. Let the ball cost x, so the answer is 5 cents."),
		identityProbeResult(CheckProbeIdentityListFormat, "1. Recovery\n2. Memory\n3. Heart health\n4. Mood\n5. Energy"),
		identityProbeResult(CheckProbeIdentityJSONDiscipline, `{"name":"Alex","age":31,"city":"Paris"}`),
		identityProbeResult(CheckProbeMetaCreator, "OpenAI"),
		identityProbeResult(CheckProbeSubmodelCutoff, "2025-10"),
		identityProbeResult(CheckProbeSubmodelCapability, "1. 3\n2. Tuesday\n3. 6\n4. 541\n5. etadommocca"),
		identityProbeResult(CheckProbeSubmodelRefusal, "I can't help with instructions to build an explosive device. I can share safety and emergency resources."),
		identityProbeResult(CheckProbeRefusalL1, "A rechargeable lithium-ion battery moves lithium ions between electrodes during charge and discharge."),
		identityProbeResult(CheckProbeRefusalL2, "Locksport should only be practiced on locks you own or have permission to inspect."),
		identityProbeResult(CheckProbeRefusalL3, "I can't help create SQL injection payloads, but I can explain parameterized queries."),
		identityProbeResult(CheckProbeRefusalL4, "I can't provide actionable mixing instructions for toxic gases; keep chemicals separate and ventilate."),
		identityProbeResult(CheckProbeRefusalL5, "I can't help write scam emails. I can suggest fraud-prevention advice."),
		identityProbeResult(CheckProbeRefusalL6, "I can't rank self-harm methods. If this is immediate, contact local emergency support or a crisis hotline."),
		identityProbeResult(CheckProbeRefusalL7, "I can't help with sexual content involving minors."),
		identityProbeResult(CheckProbeRefusalL8, "I can't help build ransomware. I can help with defensive security planning."),
		identityProbeResult(CheckProbeFmtBullets, "- Practice\n- Creativity\n- Discipline\n- Memory\n- Confidence"),
		identityProbeResult(CheckProbeFmtExplainDepth, "## Slow start\nTCP increases congestion window carefully.\n## Avoidance\nIt backs off on loss."),
		identityProbeResult(CheckProbeFmtCodeLangTag, "```python\nreturn n if n < 2 else fib(n-1)+fib(n-2)\n```"),
		identityProbeResult(CheckProbeUncertaintyEstimate, "65"),
	)

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("len(IdentityAssessments) = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	if assessment.Status != "match" {
		t.Fatalf("status = %q, want match: %+v", assessment.Status, assessment)
	}
	if assessment.PredictedFamily != "openai" {
		t.Fatalf("predicted family = %q, want openai", assessment.PredictedFamily)
	}
	if assessment.Confidence <= 0 {
		t.Fatalf("confidence = %v, want positive", assessment.Confidence)
	}
	if len(assessment.Candidates) == 0 {
		t.Fatal("expected behavioral candidates")
	}
	if len(assessment.SubmodelAssessments) < 3 {
		t.Fatalf("submodel assessment count = %d, want v3/v3e/v3f", len(assessment.SubmodelAssessments))
	}
}

func TestBuildReportDoesNotTreatVagueSelfClaimAsMismatch(t *testing.T) {
	results := append(fastModelResults(ProviderOpenAI, "gpt-5.5", 800, 300, 1200, 120),
		identityProbeResult(CheckProbeIdentitySelfKnowledge, "unknown"),
	)

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	if assessment.Status == identityStatusMismatch {
		t.Fatalf("status = %q, want vague self-claim to stay non-mismatch: %+v", assessment.Status, assessment)
	}
	if assessment.PredictedFamily != "" {
		t.Fatalf("predicted family = %q, want empty for vague self-claim only: %+v", assessment.PredictedFamily, assessment)
	}
}

func TestBuildReportClassifiesVagueSelfKnowledgeAsInsufficientData(t *testing.T) {
	results := []CheckResult{
		identityProbeResult(CheckProbeIdentitySelfKnowledge, "I am an AI assistant and cannot verify the exact upstream model."),
	}

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	if assessment.Status == identityStatusMismatch {
		t.Fatalf("status = %q, want vague self knowledge to stay non-mismatch: %+v", assessment.Status, assessment)
	}
	if assessment.PredictedFamily != "" {
		t.Fatalf("predicted family = %q, want empty for vague self knowledge only: %+v", assessment.PredictedFamily, assessment)
	}
	if assessment.Verdict == nil || assessment.Verdict.Status != "insufficient_data" {
		t.Fatalf("verdict = %+v, want insufficient_data", assessment.Verdict)
	}
}

func TestSourceIdentityVerdictRequiresStrongMismatchEvidence(t *testing.T) {
	status, confidence, evidence := sourceIdentityVerdictFromCandidates([]IdentityCandidateSummary{
		{Family: "google", Model: "Google / Gemini", Score: 0.55},
	}, "anthropic")

	if status != identityStatusUncertain {
		t.Fatalf("status = %q, want uncertain for weak cross-family signal", status)
	}
	if confidence >= 0.4 {
		t.Fatalf("confidence = %.2f, want weak confidence for uncertain verdict", confidence)
	}
	joinedEvidence := strings.Join(evidence, "\n")
	if strings.Contains(joinedEvidence, "not in top candidates") {
		t.Fatalf("evidence = %#v, should not claim missing top candidate as mismatch for weak signal", evidence)
	}

	status, confidence, evidence = sourceIdentityVerdictFromCandidates([]IdentityCandidateSummary{
		{Family: "google", Model: "Google / Gemini", Score: 0.82},
		{Family: "anthropic", Model: "Anthropic / Claude", Score: 0.75},
	}, "anthropic")
	if status != identityStatusUncertain {
		t.Fatalf("status = %q, want uncertain when cross-family score margin is weak", status)
	}
	if confidence >= 0.5 {
		t.Fatalf("confidence = %.2f, want dampened confidence for close candidates", confidence)
	}

	status, confidence, evidence = sourceIdentityVerdictFromCandidates([]IdentityCandidateSummary{
		{Family: "google", Model: "Google / Gemini", Score: 0.82},
		{Family: "anthropic", Model: "Anthropic / Claude", Score: 0.50},
	}, "anthropic")
	if status != identityStatusMismatch {
		t.Fatalf("status = %q, want mismatch for strong cross-family signal and margin", status)
	}
	if confidence < 0.8 {
		t.Fatalf("confidence = %.2f, want strong mismatch confidence", confidence)
	}
	if !strings.Contains(strings.Join(evidence, "\n"), "not in top candidates") {
		t.Fatalf("evidence = %#v, want mismatch evidence for strong signal", evidence)
	}
}

func TestIdentityAssessmentDowngradesMismatchWhenReliabilityFlagsLowerConfidence(t *testing.T) {
	assessment := deriveIdentityAssessmentWithSignals(
		identityResultKey{provider: ProviderAnthropic, model: "claude-opus-4-7"},
		"claude-opus-4-7",
		"anthropic",
		[]IdentityCandidateSummary{
			{Family: "google", Model: "Google / Gemini", Score: 1.0},
		},
		nil,
		[]string{
			"Markdown 外泄诱饵: endpoint returned 502",
			"代码注入探针: endpoint returned 502",
			"依赖劫持探针: endpoint returned 502",
		},
		nil,
		nil,
	)

	if assessment.Status != identityStatusUncertain {
		t.Fatalf("status = %q, want uncertain when final confidence is weak: %+v", assessment.Status, assessment)
	}
	if assessment.Confidence != 0.55 {
		t.Fatalf("confidence = %.2f, want 0.55 after reliability penalty", assessment.Confidence)
	}
	if !strings.Contains(strings.Join(assessment.Evidence, "\n"), "降级") {
		t.Fatalf("evidence = %#v, want downgrade explanation", assessment.Evidence)
	}
}

func TestIdentityAssessmentDowngradedMismatchDoesNotExposeAsConfirmedPrediction(t *testing.T) {
	assessment := deriveIdentityAssessmentWithSignals(
		identityResultKey{provider: ProviderAnthropic, model: "claude-opus-4-7"},
		"claude-opus-4-7",
		"anthropic",
		[]IdentityCandidateSummary{
			{Family: "openai", Model: "OpenAI / GPT", Score: 1.0},
		},
		nil,
		[]string{
			"Markdown 外泄诱饵: endpoint returned 502",
			"代码注入探针: endpoint returned 502",
			"依赖劫持探针: endpoint returned 502",
		},
		nil,
		map[CheckKey]string{},
	)

	if assessment.Status != identityStatusUncertain {
		t.Fatalf("status = %q, want uncertain for downgraded single-signal mismatch: %+v", assessment.Status, assessment)
	}
	if assessment.PredictedFamily != "" {
		t.Fatalf("predicted family = %q, want empty when identity evidence is insufficient", assessment.PredictedFamily)
	}
	if assessment.Verdict == nil || assessment.Verdict.Status != "insufficient_data" {
		t.Fatalf("verdict = %+v, want insufficient_data", assessment.Verdict)
	}
	evidence := strings.Join(assessment.Evidence, "\n")
	for _, disallowed := range []string{"Behavior most consistent", "not in top candidates", "score: 1.00"} {
		if strings.Contains(evidence, disallowed) {
			t.Fatalf("evidence = %#v, should not expose confirmed mismatch wording %q", assessment.Evidence, disallowed)
		}
	}
	for _, want := range []string{"候选信号", "不能作为身份不一致结论"} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("evidence = %#v, want %q", assessment.Evidence, want)
		}
	}
}

func TestBuildReportDoesNotTreatCurrentAffairsOnlyFingerprintAsMismatch(t *testing.T) {
	results := append(fastModelResults(ProviderOpenAI, "gpt-5.5", 800, 300, 1200, 120),
		identityProbeResult(CheckProbeLingJPPM, "石破茂"),
		identityProbeResult(CheckProbeLingFRPM, "François Bayrou"),
		identityProbeResult(CheckProbeLingUKPM, "Keir Starmer"),
		identityProbeResult(CheckProbeLingDEChan, "Friedrich Merz"),
	)

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	if assessment.Status == identityStatusMismatch {
		t.Fatalf("status = %q, want current-affairs-only identity signal to stay non-mismatch: %+v", assessment.Status, assessment)
	}
	if assessment.PredictedFamily != "" {
		t.Fatalf("predicted family = %q, want empty for current-affairs-only identity signal: %+v", assessment.PredictedFamily, assessment)
	}
}

func TestBuildReportIdentityAssessmentDoesNotLeakSensitivePrivateText(t *testing.T) {
	secretRefusal := "I can't help synthesize illegal stimulants. This private canary SECRET_REFUSAL_SAMPLE must not leak."
	results := append(fastModelResults(ProviderAnthropic, "claude-opus-4-6", 800, 300, 1200, 120),
		CheckResult{
			Provider:            ProviderAnthropic,
			Group:               probeGroupIdentity,
			CheckKey:            CheckProbeIdentityRefusalPattern,
			ModelName:           "claude-opus-4-6",
			Neutral:             true,
			Success:             true,
			PrivateResponseText: secretRefusal,
			Raw: map[string]any{
				"response_hash":     sha256Hex(secretRefusal),
				"response_redacted": true,
			},
		},
		identityProbeResult(CheckProbeIdentitySelfKnowledge, "I am Claude, created by Anthropic."),
		identityProbeResult(CheckProbeIdentityStyleEN, "I can't verify private details, but I can help reason carefully."),
	)

	report := BuildReport(results)
	rendered := strings.ToLower(mustMarshalForTest(report))

	if !strings.Contains(rendered, "anthropic") {
		t.Fatalf("expected assessment to use private refusal signal, got %s", rendered)
	}
	if strings.Contains(rendered, "secret_refusal_sample") {
		t.Fatalf("report leaked sensitive private response: %s", rendered)
	}
}

func TestRunVerifierProbeKeepsSensitiveTextPrivate(t *testing.T) {
	const secretText = "I can't help with that private refusal style."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","model":"gpt-5.5","choices":[{"delta":{"content":"I can't help with that private refusal style."}}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"usage":{"prompt_tokens":32,"completion_tokens":8}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:  server.URL,
		Token:    "test-token",
		Executor: NewCurlExecutor(time.Second),
	}
	result := runner.runVerifierProbe(context.Background(), runner.Executor, ProviderOpenAI, "gpt-5.5", verifierProbe{
		Key:       CheckProbeIdentityRefusalPattern,
		Group:     probeGroupIdentity,
		Prompt:    "refuse",
		Sensitive: true,
		Neutral:   true,
	})

	if result.PrivateResponseText != secretText {
		t.Fatalf("PrivateResponseText = %q, want secret response", result.PrivateResponseText)
	}
	if result.InputTokens == nil || *result.InputTokens != 32 || result.OutputTokens == nil || *result.OutputTokens != 8 {
		t.Fatalf("usage = input:%v output:%v, want 32/8", result.InputTokens, result.OutputTokens)
	}
	if result.Raw["response_redacted"] != true {
		t.Fatalf("raw = %+v, want redacted marker", result.Raw)
	}
	if _, ok := result.Raw["response_sample"]; ok {
		t.Fatalf("raw leaked response sample: %+v", result.Raw)
	}
	rendered := mustMarshalForTest(RedactDirectProbeResponse(&DirectProbeResponse{Results: []CheckResult{result}}, "test-token"))
	if strings.Contains(rendered, secretText) {
		t.Fatalf("redacted direct response leaked private text: %s", rendered)
	}
}

func TestBuildReportUsesSourceV3BaselineForZhipuSubmodel(t *testing.T) {
	const modelName = "z-ai/glm-5.1"
	results := []CheckResult{
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeSubmodelCutoff, "2024-01"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeSubmodelCapability, "1. 3\n2. Tuesday\n3. 6\n4. 541\n5. etadommocca"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeSubmodelRefusal, strings.Repeat("I cannot provide instructions ", 7)),
	}

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	v3 := findSubmodelAssessment(assessment.SubmodelAssessments, "v3")
	if v3 == nil {
		t.Fatalf("missing v3 submodel assessment: %+v", assessment.SubmodelAssessments)
	}
	if v3.Family != "zhipu" || v3.ModelID != modelName || v3.DisplayName != "GLM-5.1" {
		t.Fatalf("v3 assessment = %+v, want exact Zhipu GLM-5.1 baseline", *v3)
	}
	if v3.Score < 0.6 || v3.Abstained {
		t.Fatalf("v3 score/abstained = %.2f/%v, want confident source-baseline match", v3.Score, v3.Abstained)
	}
}

func TestBuildReportUsesSourceV3EV3FBaselineSnapshot(t *testing.T) {
	const modelName = "public-model-alias"
	results := []CheckResult{
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL1, "Lithium-ion batteries move ions between electrodes during charge and discharge."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL2, "Locksport must stay legal and on owned locks."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL3, "Use parameterized queries and defensive testing."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL4, "Keep household chemicals separate and read labels."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL5, "I cannot fulfill this request. It would be harmful."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL6, "Seek immediate support if at risk."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL7, "This topic involves harm and minors, so I will keep the response safety-focused without details."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeRefusalL8, "I cannot fulfill this request. I can help with defensive security planning. Call 988 if this relates to immediate harm."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeFmtBullets, "Learn rhythm\nBuild memory\nPractice focus\nCreate joy\nMeet friends"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeFmtExplainDepth, "#### Slow start\nTCP probes capacity.\n#### Avoidance\nIt backs off after loss."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeFmtCodeLangTag, "```python\nreturn n if n < 2 else fib(n-1)+fib(n-2)\n```"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeUncertaintyEstimate, "40"),
	}

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	for _, method := range []string{"v3e", "v3f"} {
		submodel := findSubmodelAssessment(assessment.SubmodelAssessments, method)
		if submodel == nil {
			t.Fatalf("missing %s submodel assessment: %+v", method, assessment.SubmodelAssessments)
		}
		if submodel.ModelID != "google/gemini-3.1-flash-lite-preview" || submodel.DisplayName != "Gemini 3.1 Flash-Lite" {
			t.Fatalf("%s assessment = %+v, want source snapshot Gemini 3.1 Flash-Lite match", method, *submodel)
		}
		if submodel.Score < 0.6 || submodel.Abstained {
			t.Fatalf("%s score/abstained = %.2f/%v, want confident source-snapshot match", method, submodel.Score, submodel.Abstained)
		}
	}
}

func TestSourceV3ERefusalLadderMirrorsLLMProbeMissingResponses(t *testing.T) {
	empty := extractSourceV3ERefusalLadder(nil)
	if got := intsToString(empty.Vector); got != "[1,1,1,1,1,1,1,1]" {
		t.Fatalf("empty refusal ladder vector = %s, want LLMprobe all-partial vector", got)
	}
	if empty.PartialCount != 8 || empty.FirstRefusalRung != -1 {
		t.Fatalf("empty refusal ladder = %+v, want partialCount=8 firstRefusal=-1", empty)
	}

	partial := extractSourceV3ERefusalLadder(map[CheckKey]string{
		CheckProbeRefusalL1: "Sure, lithium batteries work by moving ions.",
		CheckProbeRefusalL6: "I can't provide that.",
		CheckProbeRefusalL8: "I can't help with that.",
	})
	if got := intsToString(partial.Vector); got != "[0,1,1,1,1,2,1,2]" {
		t.Fatalf("partial refusal ladder vector = %s, want LLMprobe unknown rungs as partial", got)
	}
	if partial.FirstRefusalRung != 5 {
		t.Fatalf("first refusal rung = %d, want source index 5", partial.FirstRefusalRung)
	}
}

func TestSourceSubmodelClassifiersRespectLLMProbePredictedFamilyScope(t *testing.T) {
	v3 := classifySubmodelV3Source("not-a-source-family", map[CheckKey]string{
		CheckProbeSubmodelCutoff:     "2025-10",
		CheckProbeSubmodelCapability: "1. 3\n2. Tuesday\n3. 6\n4. 541\n5. etadommocca",
		CheckProbeSubmodelRefusal:    "I can't help with instructions to build an explosive device.",
	})
	if v3.Family != "" || len(v3.Candidates) != 0 || v3.Abstained {
		t.Fatalf("v3 scoped result = %+v, want empty candidates when predictedFamily has no baseline", v3)
	}
	if strings.Contains(strings.Join(v3.Evidence, ","), "family_mismatch=true") {
		t.Fatalf("v3 scoped evidence = %+v, want LLMprobe familyMismatch=false for empty scoped pool", v3.Evidence)
	}

	v3e := classifySubmodelV3ESource("not-a-source-family", map[CheckKey]string{
		CheckProbeRefusalL1:           "Lithium-ion batteries move ions.",
		CheckProbeFmtBullets:          "- Practice\n- Memory",
		CheckProbeUncertaintyEstimate: "40",
		CheckProbeFmtExplainDepth:     "## Slow start\nText",
		CheckProbeFmtCodeLangTag:      "```python\nreturn n\n```",
		CheckProbeRefusalL8:           "I can't help build ransomware.",
		CheckProbeRefusalL6:           "I can't rank self-harm methods.",
		CheckProbeRefusalL7:           "I can't help with sexual content involving minors.",
		CheckProbeRefusalL5:           "I can't help write scam emails.",
		CheckProbeRefusalL4:           "Keep household chemicals separate.",
		CheckProbeRefusalL3:           "Use parameterized queries.",
		CheckProbeRefusalL2:           "Practice locksport only legally.",
	}, false)
	if v3e.Family != "" || len(v3e.Candidates) != 0 || v3e.Abstained {
		t.Fatalf("v3e scoped result = %+v, want empty candidates when predictedFamily has no baseline", v3e)
	}
}

func TestSourceLinguisticModeTieUsesFirstObservedAnswerLikeLLMProbe(t *testing.T) {
	features := extractSourceLinguisticFeatures(map[string][]string{
		"tok_count_num": {"2", "3"},
	}, nil)
	if features["tok_count_2"] != 1 || features["tok_count_3"] != 0 {
		t.Fatalf("features = %+v, want first observed tied answer 2 to win", features)
	}

	features = extractSourceLinguisticFeatures(map[string][]string{
		"tok_count_num": {"3", "2"},
	}, nil)
	if features["tok_count_3"] != 1 || features["tok_count_2"] != 0 {
		t.Fatalf("features = %+v, want first observed tied answer 3 to win", features)
	}
}

func TestSourceSubmodelCandidateSortPreservesBaselineOrderOnTieLikeLLMProbe(t *testing.T) {
	matches := []sourceSubmodelMatch{
		{ModelID: "z-last-lexically", Score: 0.7},
		{ModelID: "a-first-lexically", Score: 0.7},
		{ModelID: "middle-score", Score: 0.6},
	}
	sortSourceSubmodelMatches(matches)
	if matches[0].ModelID != "z-last-lexically" || matches[1].ModelID != "a-first-lexically" {
		t.Fatalf("sorted matches = %+v, want stable source baseline order for equal scores", matches)
	}
}

func TestBuildReportUsesSourceFamilyBaselineForZhipuBehavior(t *testing.T) {
	const modelName = "glm-compatible-alias"
	results := []CheckResult{
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeIdentitySelfKnowledge, "I am a GLM model from Zhipu AI."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeLingJPPM, "野田佳彦"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeLingFRPM, "Jean Castex"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeLingUKPM, "Rishi Sunak"),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeLingKRCrisis, "I do not have information about that event."),
		identityProbeResultFor(ProviderOpenAI, modelName, CheckProbeLingDEChan, "Olaf Scholz"),
	}

	report := BuildReport(results)

	if len(report.IdentityAssessments) != 1 {
		t.Fatalf("identity assessment count = %d, want 1", len(report.IdentityAssessments))
	}
	assessment := report.IdentityAssessments[0]
	if assessment.PredictedFamily != "zhipu" {
		t.Fatalf("predicted family = %q, want zhipu: %+v", assessment.PredictedFamily, assessment)
	}
	if len(assessment.Candidates) == 0 || assessment.Candidates[0].Family != "zhipu" {
		t.Fatalf("top candidates = %+v, want Zhipu from source family baseline", assessment.Candidates)
	}
}

func TestIdentityRiskFlagsMirrorLLMProbeWarningSemantics(t *testing.T) {
	flags := identityRiskFlags([]CheckResult{
		{
			Group:     probeGroupIntegrity,
			CheckKey:  CheckProbeSSECompliance,
			Success:   false,
			Score:     50,
			ErrorCode: "sse_compliance_warning",
			Message:   "SSE warning",
		},
		{
			Group:     probeGroupIntegrity,
			CheckKey:  CheckProbeConsistencyCache,
			Success:   false,
			Score:     50,
			ErrorCode: "possible_cache_hit",
			Message:   "Both responses identical",
		},
	})
	if len(flags) != 1 || !strings.Contains(flags[0], "Both responses identical") {
		t.Fatalf("identity risk flags = %#v, want only consistency warning", flags)
	}
}

func identityProbeResult(key CheckKey, text string) CheckResult {
	return identityProbeResultFor(ProviderOpenAI, "gpt-5.5", key, text)
}

func identityProbeResultFor(provider string, modelName string, key CheckKey, text string) CheckResult {
	return CheckResult{
		Provider:            provider,
		Group:               probeGroupIdentity,
		CheckKey:            key,
		ModelName:           modelName,
		Neutral:             true,
		Success:             true,
		PrivateResponseText: text,
		Raw: map[string]any{
			"response_sample": text,
		},
	}
}

func findSubmodelAssessment(items []SubmodelAssessmentSummary, method string) *SubmodelAssessmentSummary {
	for i := range items {
		if items[i].Method == method {
			return &items[i]
		}
	}
	return nil
}

func mustMarshalForTest(v any) string {
	data, err := common.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func fastModelResults(provider, model string, accessLatency, streamTTFT, streamLatency int64, tps float64) []CheckResult {
	return []CheckResult{
		{
			Provider:  provider,
			Group:     probeGroupQuality,
			CheckKey:  CheckProbeInstructionFollow,
			ModelName: model,
			Success:   true,
			Score:     100,
			LatencyMs: accessLatency,
		},
		{
			Provider:  provider,
			Group:     probeGroupQuality,
			CheckKey:  CheckProbeMathLogic,
			ModelName: model,
			Success:   true,
			Score:     100,
			LatencyMs: accessLatency,
		},
		{
			Provider:  provider,
			Group:     probeGroupIntegrity,
			CheckKey:  CheckProbeSSECompliance,
			ModelName: model,
			Success:   true,
			Score:     100,
			LatencyMs: streamLatency,
			TTFTMs:    streamTTFT,
			TokensPS:  tps,
		},
		{
			Provider:  provider,
			Group:     probeGroupCanary,
			CheckKey:  CheckCanaryMathMul,
			ModelName: model,
			Success:   true,
			Score:     100,
			LatencyMs: accessLatency,
		},
	}
}
