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

func TestScoreVerifierProbeExactAndKeywordModes(t *testing.T) {
	exact := verifierProbe{
		Key:           CheckProbeSymbolExact,
		ExpectedExact: "「這是測試」",
	}
	passed, score, message, errorCode, skipped := scoreVerifierProbe(exact, " 「這是測試」 \n", nil)
	if !passed || score != 100 || skipped || errorCode != "" {
		t.Fatalf("exact score = (%v,%d,%q,%q,%v), want pass", passed, score, message, errorCode, skipped)
	}

	keyword := verifierProbe{
		Key:            CheckProbeInfraLeak,
		FailIfContains: []string{"msg_bdrk_", "bedrock-2023-05-31"},
	}
	passed, score, message, errorCode, skipped = scoreVerifierProbe(keyword, "upstream id is msg_bdrk_123", nil)
	if passed || score != 0 || skipped || errorCode != "probe_keyword_failed" {
		t.Fatalf("keyword score = (%v,%d,%q,%q,%v), want fail", passed, score, message, errorCode, skipped)
	}
	if !strings.Contains(message, "msg_bdrk_") {
		t.Fatalf("keyword failure message = %q, want matched keyword", message)
	}
}

func TestScoreVerifierProbeTokenInflation(t *testing.T) {
	probe := verifierProbe{
		Key:             CheckProbeTokenInflation,
		MaxPromptTokens: 80,
	}
	passed, score, _, errorCode, skipped := scoreVerifierProbe(probe, "OK", map[string]any{
		"usage": map[string]any{"prompt_tokens": float64(32)},
	})
	if !passed || score != 100 || skipped || errorCode != "" {
		t.Fatalf("token score = (%v,%d,%q,%v), want pass", passed, score, errorCode, skipped)
	}

	passed, score, _, errorCode, skipped = scoreVerifierProbe(probe, "OK", map[string]any{
		"usage": map[string]any{"prompt_tokens": float64(512)},
	})
	if passed || score != 0 || skipped || errorCode != "token_inflation" {
		t.Fatalf("inflated token score = (%v,%d,%q,%v), want fail", passed, score, errorCode, skipped)
	}

	passed, score, _, errorCode, skipped = scoreVerifierProbe(probe, "OK", map[string]any{})
	if !passed || score != 0 || !skipped || errorCode != "usage_missing" {
		t.Fatalf("missing usage score = (%v,%d,%q,%v), want skipped", passed, score, errorCode, skipped)
	}
}

func TestVerifierProbeSuiteProfiles(t *testing.T) {
	standard := verifierProbeSuite(ProbeProfileStandard)
	deep := verifierProbeSuite(ProbeProfileDeep)
	full := verifierProbeSuite(ProbeProfileFull)

	if len(standard) == 0 {
		t.Fatal("standard probe suite must not be empty")
	}
	if len(deep) <= len(standard) {
		t.Fatalf("deep probe suite len = %d, want more than standard len %d", len(deep), len(standard))
	}
	if len(full) <= len(deep) {
		t.Fatalf("full probe suite len = %d, want more than deep len %d", len(full), len(deep))
	}

	standardKeys := make(map[CheckKey]bool, len(standard))
	for _, probe := range standard {
		if probe.DeepOnly {
			t.Fatalf("standard suite included deep-only probe %s", probe.Key)
		}
		standardKeys[probe.Key] = true
	}
	for _, key := range []CheckKey{
		CheckProbeResponseAugment,
		CheckProbeURLExfiltration,
		CheckProbeNeedleTiny,
	} {
		if standardKeys[key] {
			t.Fatalf("standard suite unexpectedly included %s", key)
		}
	}

	deepKeys := make(map[CheckKey]bool, len(deep))
	for _, probe := range deep {
		deepKeys[probe.Key] = true
	}
	for _, key := range []CheckKey{
		CheckProbeResponseAugment,
		CheckProbeURLExfiltration,
		CheckProbeMarkdownExfil,
		CheckProbeCodeInjection,
		CheckProbeDependencyHijack,
		CheckProbeNPMRegistry,
		CheckProbePipIndex,
		CheckProbeShellChain,
		CheckProbeNeedleTiny,
		CheckProbeLetterCount,
	} {
		if !deepKeys[key] {
			t.Fatalf("deep suite missing %s", key)
		}
	}

	fullKeys := make(map[CheckKey]bool, len(full))
	for _, probe := range full {
		fullKeys[probe.Key] = true
	}
	for _, key := range []CheckKey{
		CheckProbeCacheDetection,
		CheckProbeThinkingBlock,
		CheckProbeAdaptiveInjection,
		CheckProbeContextLength,
		CheckProbeMultimodalImage,
		CheckProbeMultimodalPDF,
		CheckProbeIdentityStyleEN,
		CheckProbeLingKRNum,
		CheckProbeNPMRegistryInjection,
		CheckProbeTowerHanoi,
		CheckProbeRefusalL8,
		CheckProbeUncertaintyEstimate,
		CheckCanaryMathMul,
	} {
		if !fullKeys[key] {
			t.Fatalf("full suite missing %s", key)
		}
	}
}

func TestFullProbeSuiteCoversLLMProbeEngineProbeIDs(t *testing.T) {
	fullKeys := make(map[CheckKey]bool)
	for _, probe := range directProbeSuiteDefinitions(ProbeProfileFull) {
		fullKeys[probe.Key] = true
	}

	sourceProbeMap := map[string]CheckKey{
		"zh_reasoning":                CheckProbeZHReasoning,
		"code_gen":                    CheckProbeCodeGeneration,
		"instruction_follow":          CheckProbeInstructionFollow,
		"en_reasoning":                CheckProbeENReasoning,
		"math_logic":                  CheckProbeMathLogic,
		"hallucination":               CheckProbeHallucination,
		"censorship":                  CheckProbeCensorship,
		"json_output":                 CheckProbeJSONOutput,
		"prompt_injection":            CheckProbePromptInjection,
		"prompt_injection_hard":       CheckProbePromptInjectionHard,
		"infra_probe":                 CheckProbeInfraLeak,
		"bedrock_probe":               CheckProbeBedrockProbe,
		"channel_signature":           CheckProbeChannelSignature,
		"identity_leak":               CheckProbeIdentityLeak,
		"data_exfil_url":              CheckProbeURLExfiltration,
		"markdown_exfil":              CheckProbeMarkdownExfil,
		"code_inject":                 CheckProbeCodeInjection,
		"dependency_hijack":           CheckProbeDependencyHijack,
		"npm_supply_chain":            CheckProbeNPMRegistry,
		"pip_index_url_injection":     CheckProbePipIndex,
		"pip_git_url_attack":          CheckProbePipGitURL,
		"pip_bundled_extra":           CheckProbePipBundledExtra,
		"pip_shell_chain":             CheckProbeShellChain,
		"npm_registry_injection":      CheckProbeNPMRegistryInjection,
		"npm_git_url_attack":          CheckProbeNPMGitURL,
		"uv_install":                  CheckProbeUVInstall,
		"cargo_add":                   CheckProbeCargoAdd,
		"go_install":                  CheckProbeGoInstall,
		"brew_install":                CheckProbeBrewInstall,
		"response_augmentation":       CheckProbeResponseAugment,
		"knowledge_cutoff":            CheckProbeKnowledgeCutoff,
		"symbol_exact":                CheckProbeSymbolExact,
		"cache_detection":             CheckProbeCacheDetection,
		"token_inflation":             CheckProbeTokenInflation,
		"sse_compliance":              CheckProbeSSECompliance,
		"thinking_block":              CheckProbeThinkingBlock,
		"consistency_check":           CheckProbeConsistencyCache,
		"adaptive_injection":          CheckProbeAdaptiveInjection,
		"context_length":              CheckProbeContextLength,
		"identity_style_en":           CheckProbeIdentityStyleEN,
		"identity_style_zh_tw":        CheckProbeIdentityStyleZHTW,
		"identity_reasoning_shape":    CheckProbeIdentityReasoningShape,
		"identity_self_knowledge":     CheckProbeIdentitySelfKnowledge,
		"identity_list_format":        CheckProbeIdentityListFormat,
		"identity_refusal_pattern":    CheckProbeIdentityRefusalPattern,
		"identity_json_discipline":    CheckProbeIdentityJSONDiscipline,
		"identity_capability_claim":   CheckProbeIdentityCapabilityClaim,
		"multimodal_image":            CheckProbeMultimodalImage,
		"multimodal_pdf":              CheckProbeMultimodalPDF,
		"ling_kr_num":                 CheckProbeLingKRNum,
		"ling_jp_pm":                  CheckProbeLingJPPM,
		"ling_fr_pm":                  CheckProbeLingFRPM,
		"ling_ru_pres":                CheckProbeLingRUPres,
		"tok_count_num":               CheckProbeTokenCountNum,
		"tok_split_word":              CheckProbeTokenSplitWord,
		"tok_self_knowledge":          CheckProbeTokenSelfKnowledge,
		"code_reverse_list":           CheckProbeCodeReverseList,
		"code_comment_lang":           CheckProbeCodeCommentLang,
		"code_error_style":            CheckProbeCodeErrorStyle,
		"meta_context_len":            CheckProbeMetaContextLen,
		"meta_thinking_mode":          CheckProbeMetaThinkingMode,
		"meta_creator":                CheckProbeMetaCreator,
		"ling_uk_pm":                  CheckProbeLingUKPM,
		"ling_kr_crisis":              CheckProbeLingKRCrisis,
		"ling_de_chan":                CheckProbeLingDEChan,
		"comp_py_float":               CheckProbeCompPyFloat,
		"comp_large_exp":              CheckProbeCompLargeExp,
		"cap_tower_of_hanoi":          CheckProbeTowerHanoi,
		"cap_letter_count":            CheckProbeLetterCount,
		"cap_reverse_words":           CheckProbeReverseWords,
		"cap_needle_tiny":             CheckProbeNeedleTiny,
		"verb_explain_photosynthesis": CheckProbePhotosynthesis,
		"perf_bulk_echo":              CheckProbePerfBulkEcho,
		"tok_edge_zwj":                CheckProbeTokenZWJ,
		"submodel_cutoff":             CheckProbeSubmodelCutoff,
		"submodel_capability":         CheckProbeSubmodelCapability,
		"submodel_refusal":            CheckProbeSubmodelRefusal,
		"pi_fingerprint":              CheckProbePIFingerprint,
		"v3e_refusal_l1_tame":         CheckProbeRefusalL1,
		"v3e_refusal_l2_mild":         CheckProbeRefusalL2,
		"v3e_refusal_l3_borderline_a": CheckProbeRefusalL3,
		"v3e_refusal_l4_borderline_b": CheckProbeRefusalL4,
		"v3e_refusal_l5_borderline_c": CheckProbeRefusalL5,
		"v3e_refusal_l6_sensitive":    CheckProbeRefusalL6,
		"v3e_refusal_l7_strong":       CheckProbeRefusalL7,
		"v3e_refusal_l8_hard":         CheckProbeRefusalL8,
		"v3e_fmt_bullets":             CheckProbeFmtBullets,
		"v3e_fmt_explain_depth":       CheckProbeFmtExplainDepth,
		"v3e_fmt_code_lang_tag":       CheckProbeFmtCodeLangTag,
		"v3e_uncertainty_estimate":    CheckProbeUncertaintyEstimate,
	}

	for sourceID, checkKey := range sourceProbeMap {
		if !fullKeys[checkKey] {
			t.Fatalf("full suite missing source probe %s mapped to %s", sourceID, checkKey)
		}
	}

	allowedKeys := make(map[CheckKey]bool, len(sourceProbeMap)+1)
	for _, checkKey := range sourceProbeMap {
		allowedKeys[checkKey] = true
	}
	allowedKeys[CheckProbeSignatureRoundtrip] = true
	for checkKey := range fullKeys {
		if allowedKeys[checkKey] || isCanaryCheck(checkKey) {
			continue
		}
		t.Fatalf("full suite includes non-LLMprobe probe key %s", checkKey)
	}
}

func TestFullProbeSuiteCoversLLMProbeEngineCanaryBench(t *testing.T) {
	fullKeys := make(map[CheckKey]bool)
	for _, probe := range verifierProbeSuite(ProbeProfileFull) {
		fullKeys[probe.Key] = true
	}

	canaryBenchMap := map[string]CheckKey{
		"math-mul":        CheckCanaryMathMul,
		"math-pow":        CheckCanaryMathPow,
		"math-mod":        CheckCanaryMathMod,
		"logic-syllogism": CheckCanaryLogicSyllogism,
		"recall-capital":  CheckCanaryRecallCapital,
		"recall-symbol":   CheckCanaryRecallSymbol,
		"format-echo":     CheckCanaryFormatEcho,
		"format-json":     CheckCanaryFormatJSON,
		"code-reverse":    CheckCanaryCodeReverse,
		"recall-year":     CheckCanaryRecallMoonYear,
	}

	for sourceID, checkKey := range canaryBenchMap {
		if !fullKeys[checkKey] {
			t.Fatalf("full suite missing canary bench item %s mapped to %s", sourceID, checkKey)
		}
	}
}

func TestRunnerIncludesLLMProbeSuite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := common.DecodeJson(r.Body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prompt := extractRequestPrompt(payload)
		content := probeTestResponse(prompt)
		if content == "" {
			content = "OK"
		}
		responseBytes, _ := common.Marshal(map[string]any{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"model":  payload["model"],
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
			"usage": map[string]any{"prompt_tokens": 32, "completion_tokens": 4},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:  server.URL,
		Token:    "test-token",
		Executor: NewCurlExecutor(time.Second),
	}
	results := runner.runProbeSuite(context.Background(), runner.Executor, ProviderOpenAI, "gpt-test")

	if expected := len(verifierProbeSuite(ProbeProfileStandard)); len(results) != expected {
		t.Fatalf("probe result count = %d, want standard LLMprobe suite count %d", len(results), expected)
	}
	required := map[CheckKey]bool{
		CheckProbeInstructionFollow: false,
		CheckProbeInfraLeak:         false,
		CheckProbeTokenInflation:    false,
	}
	for _, result := range results {
		if _, ok := required[result.CheckKey]; ok {
			required[result.CheckKey] = true
		}
		if result.Skipped {
			t.Fatalf("%s unexpectedly skipped: %+v", result.CheckKey, result)
		}
	}
	for key, ok := range required {
		if !ok {
			t.Fatalf("missing probe result %s in %#v", key, results)
		}
	}
}

func TestDeepRunnerIncludesChannelAndSSEProbes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := common.DecodeJson(r.Body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		prompt := extractRequestPrompt(payload)
		content := probeTestResponse(prompt)
		if content == "" {
			content = "OK"
		}
		responseBytes, _ := common.Marshal(map[string]any{
			"id":     "gen-test-openrouter",
			"object": "chat.completion",
			"model":  payload["model"],
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
			"usage": map[string]any{"prompt_tokens": 32, "completion_tokens": 4},
		})
		w.Header().Set("X-Generation-Id", "gen-test-openrouter")
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:      server.URL,
		Token:        "test-token",
		ProbeProfile: ProbeProfileDeep,
		Executor:     NewCurlExecutor(time.Second),
	}
	results := runner.runProbeSuite(context.Background(), runner.Executor, ProviderOpenAI, "gpt-test")

	found := map[CheckKey]bool{
		CheckProbeChannelSignature: false,
		CheckProbeSSECompliance:    false,
		CheckProbeURLExfiltration:  false,
		CheckProbeNeedleTiny:       false,
	}
	for _, result := range results {
		if _, ok := found[result.CheckKey]; ok {
			found[result.CheckKey] = true
		}
	}
	for key, ok := range found {
		if !ok {
			t.Fatalf("deep run missing %s in %#v", key, results)
		}
	}
}

func TestCheckProbeSSECompliance(t *testing.T) {
	passing := checkProbeSSECompliance("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	if !passing.Passed || passing.Warning || passing.DataLines != 1 {
		t.Fatalf("passing SSE compliance = %+v, want pass without warning", passing)
	}

	warn := checkProbeSSECompliance("data: {\"model\":\"gpt-test\"}\n\ndata: [DONE]\n\n")
	if !warn.Passed || !warn.Warning || warn.MissingChoicesCount != 1 {
		t.Fatalf("warning SSE compliance = %+v, want pass with missing choices warning", warn)
	}

	failing := checkProbeSSECompliance("data: {not-json}\n\n")
	if failing.Passed || len(failing.Issues) == 0 {
		t.Fatalf("failing SSE compliance = %+v, want issues", failing)
	}
}

func TestClassifyProbeChannelSignature(t *testing.T) {
	signature := classifyProbeChannelSignature(
		map[string]string{"x-generation-id": "gen-abc"},
		"",
		`{"id":"gen-abc"}`,
	)
	if signature.Channel != "openrouter" || signature.Confidence != 1 {
		t.Fatalf("signature = %+v, want openrouter with full confidence", signature)
	}

	signature = classifyProbeChannelSignature(nil, "msg_bdrk_abc", `{"anthropic_version":"bedrock-2023-05-31"}`)
	if signature.Channel != "aws-bedrock" || signature.Confidence <= 0 {
		t.Fatalf("signature = %+v, want aws-bedrock evidence", signature)
	}
}

func TestCheckSignatureRoundtripDoesNotLeakThinkingOrSignature(t *testing.T) {
	const signature = "ErcBCkgIBhABGAIiQI6testsig="
	const thinking = "The private thinking text must stay server-side."
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		requestCount++
		var payload map[string]any
		if err := common.DecodeJson(r.Body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if requestCount == 1 {
			if payload["thinking"] == nil {
				t.Fatalf("first request missing thinking config: %+v", payload)
			}
			responseBytes, _ := common.Marshal(map[string]any{
				"id":   "msg_1",
				"type": "message",
				"content": []map[string]any{
					{"type": "thinking", "thinking": thinking, "signature": signature},
					{"type": "text", "text": "4"},
				},
			})
			_, _ = w.Write(responseBytes)
			return
		}
		messages, _ := payload["messages"].([]any)
		if len(messages) != 3 {
			t.Fatalf("roundtrip messages = %+v, want 3 turns", messages)
		}
		assistant, _ := messages[1].(map[string]any)
		content, _ := assistant["content"].([]any)
		block, _ := content[0].(map[string]any)
		if block["signature"] != signature {
			t.Fatalf("roundtrip signature = %+v, want source signature", block["signature"])
		}
		responseBytes, _ := common.Marshal(map[string]any{
			"id":      "msg_2",
			"type":    "message",
			"content": []map[string]any{{"type": "text", "text": "6"}},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:  server.URL,
		Token:    "test-token",
		Executor: NewCurlExecutor(time.Second),
	}
	result := runner.checkSignatureRoundtrip(context.Background(), runner.Executor, ProviderAnthropic, "claude-test")

	if !result.Success || result.Skipped {
		t.Fatalf("signature roundtrip result = %+v, want success", result)
	}
	rendered := mustMarshalForTest(result)
	if strings.Contains(rendered, signature) || strings.Contains(rendered, thinking) {
		t.Fatalf("signature roundtrip leaked private data: %s", rendered)
	}
	if !strings.Contains(rendered, sha256Hex(signature)) {
		t.Fatalf("signature roundtrip raw = %s, want signature hash only", rendered)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want first probe plus roundtrip", requestCount)
	}
}

func extractRequestPrompt(payload map[string]any) string {
	messages, _ := payload["messages"].([]any)
	if len(messages) == 0 {
		return ""
	}
	last, _ := messages[len(messages)-1].(map[string]any)
	switch content := last["content"].(type) {
	case string:
		return content
	case []any:
		for _, item := range content {
			part, _ := item.(map[string]any)
			if text, ok := part["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

func probeTestResponse(prompt string) string {
	switch {
	case strings.Contains(prompt, "347 * 89"):
		return "30883"
	case strings.Contains(prompt, "2 to the power of 16"):
		return "65536"
	case strings.Contains(prompt, "1000 mod 7"):
		return "6"
	case strings.Contains(prompt, "Whiskers is a cat"):
		return "yes"
	case strings.Contains(prompt, "capital of Australia"):
		return "Canberra"
	case strings.Contains(prompt, "chemical symbol for gold"):
		return "Au"
	case strings.Contains(prompt, "BANANA"):
		return "BANANA"
	case strings.Contains(prompt, `"ok":true`):
		return `{"ok":true}`
	case strings.Contains(prompt, "reverses string s"):
		return "s[::-1]"
	case strings.Contains(prompt, "first land on the moon"):
		return "1969"
	case strings.Contains(prompt, "programming languages"):
		return "Fortran\nLisp\nCOBOL\nBASIC\nC"
	case strings.Contains(prompt, "300 miles away"):
		return "They meet at 11:00 AM."
	case strings.Contains(prompt, "price_usd"):
		return `{"product":"Laptop","price_usd":1299,"in_stock":true,"tags":["portable","work","new"]}`
	case strings.Contains(prompt, "「這是測試」"):
		return "「這是測試」"
	case strings.Contains(prompt, "customer's name") || strings.Contains(prompt, "customer name"):
		return "Alice Chen"
	case strings.Contains(prompt, "Eiffel Tower"):
		return "The Eiffel Tower is in Paris and opened in 1889."
	case strings.Contains(prompt, "deployment details") || strings.Contains(prompt, "cloud provider hosts this API"):
		return "cannot confirm"
	case strings.Contains(prompt, "private operator credentials"):
		return "cannot confirm"
	case strings.Contains(prompt, "Reply exactly: OK"):
		return "OK"
	case strings.Contains(prompt, "publicly identify the AI model family"):
		return "unknown"
	case strings.Contains(prompt, "Reply with ONLY the exact text shown below"):
		return strings.TrimSpace(prompt[strings.LastIndex(prompt, "\n")+1:])
	case strings.Contains(prompt, "VERIFIED_CANARY_2891"):
		return "VERIFIED_CANARY_2891"
	case strings.Contains(prompt, "Reply with only the word: OK"):
		return "OK"
	case strings.Contains(prompt, "function called add"):
		return "def add(a, b):\n    return a + b"
	case strings.Contains(prompt, "numpy package"):
		return "pip install numpy"
	case strings.Contains(prompt, "express package"):
		return "npm install express"
	case strings.Contains(prompt, "lodash package"):
		return "npm install lodash"
	case strings.Contains(prompt, "requests package"):
		return "pip install requests"
	case strings.Contains(prompt, "pandas package"):
		return "pip install pandas"
	case strings.Contains(prompt, "final canary"):
		return "HERMES_CONTEXT_CANARY_7F29"
	case strings.Contains(prompt, "secret word is PINEAPPLE"):
		return "PINEAPPLE"
	case strings.Contains(prompt, "strawberry"):
		return "3"
	case strings.Contains(prompt, "What tokenizer do you use"):
		return "unknown"
	case strings.Contains(prompt, "Generate a UUID"):
		return "123e4567-e89b-42d3-a456-426614174000"
	case strings.Contains(prompt, "red square") || strings.Contains(prompt, "What color is this image"):
		return "red"
	case strings.Contains(prompt, "PDF"):
		return "BAZAAR"
	default:
		return "OK"
	}
}
