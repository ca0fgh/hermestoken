package token_verifier

import (
	"strings"
	"testing"
)

func TestBuildLabeledProbeCorpusDraftFromResults(t *testing.T) {
	promptTokens := 51
	corpus := BuildLabeledProbeCorpusDraftFromResults("real endpoint capture", []CheckResult{
		{
			Provider:            ProviderOpenAI,
			CheckKey:            CheckProbeInstructionFollow,
			ModelName:           "gpt-test",
			Success:             true,
			PrivateResponseText: "Fortran\nLisp\nCOBOL\nBASIC\nC",
			Raw:                 map[string]any{"response_sample": "truncated sample must not win"},
		},
		{
			Provider:     ProviderOpenAI,
			CheckKey:     CheckProbeTokenInflation,
			ModelName:    "gpt-test",
			Success:      false,
			ErrorCode:    "token_inflation",
			InputTokens:  &promptTokens,
			OutputTokens: intPtrForTest(1),
			Raw: map[string]any{
				"usage": map[string]any{"prompt_tokens": 51, "completion_tokens": 1},
			},
		},
		{
			Provider:  ProviderOpenAI,
			CheckKey:  CheckProbeCacheDetection,
			ModelName: "gpt-test",
			Success:   false,
			ErrorCode: "cache_header_hit",
			Raw: map[string]any{
				"header_key":   "x-cache",
				"header_value": "HIT",
			},
		},
	})

	if corpus.Description != "real endpoint capture" {
		t.Fatalf("description = %q, want custom description", corpus.Description)
	}
	if corpus.ManualReview.Status != corpusManualReviewStatusDraft || corpus.ManualReview.Source != corpusSourceDetectorGeneratedDraft {
		t.Fatalf("manual review = %+v, want detector-generated draft metadata", corpus.ManualReview)
	}
	if len(corpus.Cases) != 3 {
		t.Fatalf("case count = %d, want 3: %+v", len(corpus.Cases), corpus.Cases)
	}

	textCase := corpus.Cases[0]
	if textCase.Name != "gpt_test_probe_instruction_follow_1" || textCase.CheckKey != CheckProbeInstructionFollow {
		t.Fatalf("text case identity = %+v", textCase)
	}
	if textCase.Source.Provider != ProviderOpenAI || textCase.Source.Model != "gpt-test" || textCase.Source.CheckKey != CheckProbeInstructionFollow {
		t.Fatalf("text case source = %+v, want provider/model/check key from capture", textCase.Source)
	}
	if textCase.ResponseText != "Fortran\nLisp\nCOBOL\nBASIC\nC" {
		t.Fatalf("response_text = %q, want private response", textCase.ResponseText)
	}
	if !textCase.WantPassed {
		t.Fatalf("want_passed = false, want true for successful result")
	}

	usageCase := corpus.Cases[1]
	if usageCase.ResponseText != "" {
		t.Fatalf("token usage response_text = %q, want empty", usageCase.ResponseText)
	}
	usage, ok := usageCase.Decoded["usage"].(map[string]any)
	if !ok || usage["prompt_tokens"] != 51 || usage["completion_tokens"] != 1 {
		t.Fatalf("usage decoded = %+v, want prompt/completion token usage", usageCase.Decoded)
	}
	if usageCase.WantPassed || usageCase.WantErrorCode != "" {
		t.Fatalf("token usage labels = passed:%v error:%q, want failure label without locked error code", usageCase.WantPassed, usageCase.WantErrorCode)
	}

	cacheCase := corpus.Cases[2]
	if cacheCase.CacheHeader != "HIT" || cacheCase.WantPassed {
		t.Fatalf("cache case = %+v, want HIT failure", cacheCase)
	}
}

func TestBuildLabeledProbeCorpusDraftHandlesSpecialProbeFields(t *testing.T) {
	corpus := BuildLabeledProbeCorpusDraftFromResults("", []CheckResult{
		{
			CheckKey: CheckProbeSSECompliance,
			Success:  true,
			Raw: map[string]any{
				"raw_sse": "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n",
			},
		},
		{
			CheckKey: CheckProbeThinkingBlock,
			Success:  true,
			Raw: map[string]any{
				"raw_sse": "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n",
			},
		},
		{
			CheckKey: CheckProbeConsistencyCache,
			Success:  true,
			Raw: map[string]any{
				"first_response":  "18342",
				"second_response": "927451",
			},
		},
		{
			CheckKey:  CheckProbeAdaptiveInjection,
			Success:   false,
			ErrorCode: "adaptive_probe_diverged",
			Raw: map[string]any{
				"neutral_response": "apple banana cherry date elderberry",
				"trigger_response": "apple banana aws_secret_access_key date elderberry",
			},
		},
		{
			CheckKey: CheckProbeContextLength,
			Success:  true,
			Raw: map[string]any{
				"length_results": []any{
					map[string]any{"chars": 4000, "found_canaries": 5, "total_canaries": 5},
					map[string]any{"chars": 16000, "found_canaries": 4.0, "total_canaries": 5.0},
				},
			},
		},
	})

	if len(corpus.Cases) != 5 {
		t.Fatalf("case count = %d, want 5", len(corpus.Cases))
	}
	if corpus.Cases[0].RawSSE == "" {
		t.Fatalf("SSE case = %+v, want raw_sse", corpus.Cases[0])
	}
	if corpus.Cases[1].RawSSE == "" {
		t.Fatalf("thinking case = %+v, want raw_sse", corpus.Cases[1])
	}
	if corpus.Cases[2].First != "18342" || corpus.Cases[2].Second != "927451" {
		t.Fatalf("consistency case = %+v, want first/second responses", corpus.Cases[2])
	}
	if corpus.Cases[3].Neutral == "" || corpus.Cases[3].Trigger == "" || corpus.Cases[3].WantErrorCode != "" {
		t.Fatalf("adaptive case = %+v, want neutral/trigger without locked error", corpus.Cases[3])
	}
	if levels := corpus.Cases[4].ContextLevels; len(levels) != 2 || levels[0].Chars != 4000 || levels[1].Found != 4 {
		t.Fatalf("context levels = %+v, want converted length results", levels)
	}
}

func TestBuildLabeledProbeCorpusDraftRedactsSensitiveResponses(t *testing.T) {
	const secret = "sk-live-secret"
	corpus := BuildLabeledProbeCorpusDraftFromResults("", []CheckResult{
		{
			CheckKey:            CheckProbeURLExfiltration,
			ModelName:           "secret/model",
			Success:             false,
			ErrorCode:           "leaked_secret",
			PrivateResponseText: "leaked " + secret,
			Raw: map[string]any{
				"response_sample": "leaked " + secret,
			},
		},
		{
			CheckKey:  CheckProbeInstructionFollow,
			ModelName: "safe-model",
			Success:   true,
			Raw: map[string]any{
				"response_sample": "Bearer " + secret,
			},
		},
	}, secret)

	rendered, err := MarshalLabeledProbeCorpusDraft(corpus)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if strings.Contains(string(rendered), secret) {
		t.Fatalf("draft leaked secret: %s", string(rendered))
	}
	sensitiveCase := corpus.Cases[0]
	if sensitiveCase.ResponseText != "leaked [REDACTED]" || sensitiveCase.WantErrorCode != "" || sensitiveCase.WantPassed {
		t.Fatalf("sensitive case = %+v, want redacted response text with failure label but no locked error", sensitiveCase)
	}
	probe := verifierProbe{
		Key:            CheckProbeURLExfiltration,
		PassIfContains: []string{"VERIFIED_CANARY_2891"},
		FailIfContains: []string{"[REDACTED]"},
	}
	if result := scoreVerifierProbeDetailed(probe, sensitiveCase.ResponseText, nil); result.Passed {
		t.Fatalf("sensitive response replay result = %+v, want redacted text to remain scoreable as failure", result)
	}
	normalCase := corpus.Cases[1]
	if normalCase.ResponseText != "Bearer [REDACTED]" || !normalCase.WantPassed {
		t.Fatalf("normal case = %+v, want redacted response sample", normalCase)
	}
}

func TestBuildLabeledProbeCorpusDraftPreservesSensitiveCommandOutputs(t *testing.T) {
	corpus := BuildLabeledProbeCorpusDraftFromResults("", []CheckResult{
		{
			CheckKey:            CheckProbePipGitURL,
			ModelName:           "cmd-model",
			Success:             true,
			PrivateResponseText: "pip install httpx",
			Raw: map[string]any{
				"response_hash":     "hash-only",
				"response_redacted": true,
			},
		},
	})

	if len(corpus.Cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(corpus.Cases))
	}
	if corpus.Cases[0].ResponseText != "pip install httpx" || !corpus.Cases[0].WantPassed {
		t.Fatalf("command corpus case = %+v, want scoreable command output", corpus.Cases[0])
	}
}

func TestBuildLabeledProbeCorpusDraftSkipsUnscoredResults(t *testing.T) {
	corpus := BuildLabeledProbeCorpusDraftFromResults("", []CheckResult{
		{
			CheckKey:  CheckProbeInstructionFollow,
			Skipped:   true,
			Success:   false,
			Score:     0,
			Raw:       map[string]any{"response_sample": "endpoint error"},
			RiskLevel: "unknown",
		},
		{
			CheckKey:  CheckProbeInstructionFollow,
			Success:   true,
			Score:     100,
			Raw:       map[string]any{"response_sample": "Fortran"},
			RiskLevel: "low",
		},
	})

	if len(corpus.Cases) != 1 {
		t.Fatalf("case count = %d, want only scored result", len(corpus.Cases))
	}
	if corpus.Cases[0].ResponseText != "Fortran" {
		t.Fatalf("case = %+v, want scored response sample", corpus.Cases[0])
	}
}

func TestBuildLabeledProbeCorpusDraftSkipsNeutralIdentityResults(t *testing.T) {
	corpus := BuildLabeledProbeCorpusDraftFromResults("", []CheckResult{
		{
			CheckKey:            CheckProbeIdentitySelfKnowledge,
			Group:               probeGroupIdentity,
			ModelName:           "gpt-test",
			Neutral:             true,
			Success:             true,
			PrivateResponseText: "I am ChatGPT, a model created by OpenAI.",
		},
		{
			CheckKey:            CheckProbeInstructionFollow,
			ModelName:           "gpt-test",
			Success:             true,
			PrivateResponseText: "Fortran",
		},
	})

	if len(corpus.Cases) != 1 {
		t.Fatalf("case count = %d, want only pass/fail result", len(corpus.Cases))
	}
	if corpus.Cases[0].CheckKey != CheckProbeInstructionFollow {
		t.Fatalf("case = %+v, want non-neutral pass/fail probe", corpus.Cases[0])
	}
}

func TestBuildIdentityAssessmentCorpusDraftFromDirectProbeResponse(t *testing.T) {
	response := DirectProbeResponse{
		Provider:     ProviderOpenAI,
		Model:        "gpt-5.5",
		ProbeProfile: ProbeProfileFull,
		Results: []CheckResult{
			{
				Provider:            ProviderOpenAI,
				Group:               probeGroupIdentity,
				CheckKey:            CheckProbeIdentitySelfKnowledge,
				ModelName:           "gpt-5.5",
				Neutral:             true,
				Success:             true,
				PrivateResponseText: "I am ChatGPT, a model created by OpenAI.",
			},
			{
				Provider:            ProviderOpenAI,
				Group:               probeGroupIdentity,
				CheckKey:            CheckProbeIdentityRefusalPattern,
				ModelName:           "gpt-5.5",
				Neutral:             true,
				Success:             true,
				PrivateResponseText: "I'm sorry, but I cannot help with harmful instructions.",
			},
		},
	}
	response.Report = BuildReport(response.Results)

	corpus := BuildIdentityAssessmentCorpusDraftFromDirectProbeResponse("identity capture", response)

	if corpus.Description != "identity capture" {
		t.Fatalf("description = %q, want custom description", corpus.Description)
	}
	if corpus.ManualReview.Status != corpusManualReviewStatusDraft || corpus.ManualReview.Source != corpusSourceDetectorGeneratedDraft {
		t.Fatalf("manual review = %+v, want detector-generated draft metadata", corpus.ManualReview)
	}
	if len(corpus.Cases) != 1 {
		t.Fatalf("case count = %d, want one report-level identity case", len(corpus.Cases))
	}
	item := corpus.Cases[0]
	if item.WantIdentityStatus != "match" || item.WantVerdictStatus != "clean_match" || item.WantPredictedFamily != "openai" {
		t.Fatalf("identity labels = %+v, want current report outcome copied for manual review", item)
	}
	if item.Source.Provider != ProviderOpenAI || item.Source.Model != "gpt-5.5" || item.Source.CheckKey != CheckProbeIdentitySelfKnowledge {
		t.Fatalf("identity source = %+v, want source from first identity result", item.Source)
	}
	if len(item.Results) != 2 || item.Results[0].PrivateResponseText == "" {
		t.Fatalf("identity result evidence = %+v, want private response text for review", item.Results)
	}
}

func TestBuildIdentityAssessmentCorpusDraftRedactsSecrets(t *testing.T) {
	const secret = "sk-live-secret"
	response := DirectProbeResponse{
		Provider: ProviderOpenAI,
		Model:    "gpt-5.5",
		Results: []CheckResult{
			{
				Provider:            ProviderOpenAI,
				Group:               probeGroupIdentity,
				CheckKey:            CheckProbeIdentitySelfKnowledge,
				ModelName:           "gpt-5.5",
				Neutral:             true,
				Success:             true,
				PrivateResponseText: "I am ChatGPT. token=" + secret,
			},
		},
	}
	response.Report = BuildReport(response.Results)

	corpus := BuildIdentityAssessmentCorpusDraftFromDirectProbeResponse("", response, secret)
	rendered, err := MarshalIdentityAssessmentCorpusDraft(corpus)
	if err != nil {
		t.Fatalf("marshal identity draft: %v", err)
	}
	if strings.Contains(string(rendered), secret) {
		t.Fatalf("identity draft leaked secret: %s", string(rendered))
	}
	if len(corpus.Cases) != 1 || !strings.Contains(corpus.Cases[0].Results[0].PrivateResponseText, "[REDACTED]") {
		t.Fatalf("corpus = %+v, want redacted private response", corpus)
	}
}

func TestBuildInformationalProbeCorpusDraftFromResults(t *testing.T) {
	corpus := BuildInformationalProbeCorpusDraftFromResults("informational capture", []CheckResult{
		{
			CheckKey:  CheckProbeChannelSignature,
			ModelName: "gpt-test",
			Neutral:   true,
			Success:   true,
			Raw: map[string]any{
				"channel":    "openrouter",
				"confidence": 1,
				"headers": map[string]any{
					"x-generation-id": "gen-test",
				},
				"message_id": "gen-test",
				"raw_body":   `{"id":"gen-test"}`,
			},
		},
		{
			CheckKey:  CheckProbeSignatureRoundtrip,
			ModelName: "claude-test",
			Neutral:   true,
			Success:   false,
			ErrorCode: "signature_rejected",
			Raw: map[string]any{
				"thinking_present": true,
				"roundtrip_status": 400,
				"raw_body":         `{"error":{"message":"invalid thinking signature"}}`,
			},
		},
		{
			CheckKey:  CheckProbeInstructionFollow,
			ModelName: "gpt-test",
			Success:   true,
			Raw:       map[string]any{"response_sample": "Fortran"},
		},
	})

	if corpus.Description != "informational capture" {
		t.Fatalf("description = %q, want custom description", corpus.Description)
	}
	if corpus.ManualReview.Status != corpusManualReviewStatusDraft || corpus.ManualReview.Source != corpusSourceDetectorGeneratedDraft {
		t.Fatalf("manual review = %+v, want detector-generated draft metadata", corpus.ManualReview)
	}
	if len(corpus.Cases) != 2 {
		t.Fatalf("case count = %d, want informational cases only", len(corpus.Cases))
	}
	channel := corpus.Cases[0]
	if channel.CheckKey != CheckProbeChannelSignature || channel.WantChannel != "openrouter" || !channel.WantPassed || channel.Headers["x-generation-id"] != "gen-test" {
		t.Fatalf("channel case = %+v, want openrouter informational evidence", channel)
	}
	if channel.Source.Provider != "" || channel.Source.Model != "gpt-test" || channel.Source.CheckKey != CheckProbeChannelSignature {
		t.Fatalf("channel source = %+v, want model/check key from capture", channel.Source)
	}
	signature := corpus.Cases[1]
	if signature.CheckKey != CheckProbeSignatureRoundtrip || signature.WantPassed || signature.WantErrorCode != "" || signature.RoundtripStatus != 400 {
		t.Fatalf("signature case = %+v, want rejected roundtrip evidence", signature)
	}
}

func TestBuildInformationalProbeCorpusDraftRedactsSecrets(t *testing.T) {
	const secret = "sk-live-secret"
	corpus := BuildInformationalProbeCorpusDraftFromResults("", []CheckResult{
		{
			CheckKey: CheckProbeChannelSignature,
			Neutral:  true,
			Success:  true,
			Raw: map[string]any{
				"channel": "one-api",
				"headers": map[string]any{
					"x-oneapi-request-id": "req-" + secret,
				},
				"raw_body": "body " + secret,
			},
		},
	}, secret)

	rendered, err := MarshalInformationalProbeCorpusDraft(corpus)
	if err != nil {
		t.Fatalf("marshal informational draft: %v", err)
	}
	if strings.Contains(string(rendered), secret) {
		t.Fatalf("informational draft leaked secret: %s", string(rendered))
	}
}

func TestBuildProbeBaselineDraftFromResults(t *testing.T) {
	corpus := BuildProbeBaselineDraftFromResults("baseline capture", []CheckResult{
		{
			CheckKey:            CheckProbeZHReasoning,
			ModelName:           "gpt-test",
			Success:             true,
			Skipped:             true,
			ErrorCode:           "judge_unconfigured",
			PrivateResponseText: "繁中推理参考答案",
		},
		{
			CheckKey:  CheckProbeCodeGeneration,
			ModelName: "gpt-test",
			Success:   true,
			Raw: map[string]any{
				"response_sample": "def binary_search(items, target): ...",
			},
		},
		{
			CheckKey:            CheckProbeENReasoning,
			ModelName:           "gpt-test",
			Success:             true,
			PrivateResponseText: "Concurrency is about managing multiple tasks...",
		},
		{
			CheckKey:            CheckProbeHallucination,
			ModelName:           "gpt-test",
			Success:             true,
			PrivateResponseText: "This local review-only probe should not need a judge baseline.",
		},
	})

	if corpus.Description != "baseline capture" {
		t.Fatalf("description = %q, want custom description", corpus.Description)
	}
	if len(corpus.Probes) != 3 {
		t.Fatalf("baseline probe count = %d, want 3: %+v", len(corpus.Probes), corpus.Probes)
	}
	wantIDs := []string{"zh_reasoning", "code_gen", "en_reasoning"}
	for i, want := range wantIDs {
		if corpus.Probes[i].ProbeID != want {
			t.Fatalf("probe[%d].probeId = %q, want %q", i, corpus.Probes[i].ProbeID, want)
		}
		if strings.TrimSpace(corpus.Probes[i].ResponseText) == "" {
			t.Fatalf("probe[%d] response text is empty: %+v", i, corpus.Probes[i])
		}
	}
	rendered, err := MarshalProbeBaselineDraft(corpus)
	if err != nil {
		t.Fatalf("marshal baseline draft: %v", err)
	}
	if !strings.Contains(string(rendered), `"probeId": "zh_reasoning"`) || !strings.Contains(string(rendered), `"responseText"`) {
		t.Fatalf("baseline draft = %s, want parseProbeBaselineJSON-compatible shape", string(rendered))
	}
}

func intPtrForTest(value int) *int {
	return &value
}
