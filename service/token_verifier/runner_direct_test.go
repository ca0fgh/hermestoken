package token_verifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

func TestRunProbeSuiteOnlyDirectPathUsesLLMProbeResultsOnly(t *testing.T) {
	modelsListCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			modelsListCalled = true
			http.NotFound(w, r)
			return
		}
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
			"id":     "chatcmpl-direct-test",
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
		BaseURL:      server.URL,
		Token:        "test-token",
		Models:       []string{"gpt-test"},
		Providers:    []string{ProviderOpenAI},
		ProbeProfile: ProbeProfileStandard,
		Executor:     NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if modelsListCalled {
		t.Fatal("direct probe suite must not call the legacy models list check")
	}
	if len(results) == 0 {
		t.Fatal("direct probe suite returned no results")
	}

	required := map[CheckKey]bool{
		CheckProbeInstructionFollow: false,
		CheckProbeInfraLeak:         false,
		CheckProbeTokenInflation:    false,
	}
	for _, result := range results {
		if !isLLMProbeResultKey(result.CheckKey) {
			t.Fatalf("direct probe suite included non-LLMprobe check %s in %#v", result.CheckKey, results)
		}
		if _, ok := required[result.CheckKey]; ok {
			required[result.CheckKey] = true
		}
		if result.Provider != ProviderOpenAI {
			t.Fatalf("result provider = %q, want %q", result.Provider, ProviderOpenAI)
		}
	}
	for key, ok := range required {
		if !ok {
			t.Fatalf("direct probe suite missing %s in %#v", key, results)
		}
	}
}

func TestRunProbeSuiteOnlyCanTargetSpecificCheckKeys(t *testing.T) {
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
			"id":     "chatcmpl-targeted-test",
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
		BaseURL:      server.URL,
		Token:        "test-token",
		Models:       []string{"gpt-test"},
		Providers:    []string{ProviderOpenAI},
		ProbeProfile: ProbeProfileFull,
		CheckKeys:    []CheckKey{CheckProbeInstructionFollow, CheckProbeJSONOutput},
		Executor:     NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d, want targeted checks only: %+v", len(results), results)
	}
	if results[0].CheckKey != CheckProbeInstructionFollow || results[1].CheckKey != CheckProbeJSONOutput {
		t.Fatalf("results = %+v, want requested check keys in suite order", results)
	}
}

func TestRunProbeSuiteOnlyCanTargetLegacyCheckKeysOutsideRuntimeProfile(t *testing.T) {
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
			"id":     "chatcmpl-targeted-legacy-test",
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
		BaseURL:      server.URL,
		Token:        "test-token",
		Models:       []string{"gpt-test"},
		Providers:    []string{ProviderOpenAI},
		ProbeProfile: ProbeProfileFull,
		CheckKeys:    []CheckKey{CheckProbePipBundledExtra, CheckCanaryMathMul},
		Executor:     NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d, want targeted legacy checks only: %+v", len(results), results)
	}
	if results[0].CheckKey != CheckProbePipBundledExtra || results[1].CheckKey != CheckCanaryMathMul {
		t.Fatalf("results = %+v, want requested legacy check keys in definition order", results)
	}
}

func TestRunDirectProbeIncludesSourceMetadataForCorpusExport(t *testing.T) {
	var requestMu sync.Mutex
	var firstRequestStartedAt time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		requestMu.Lock()
		if firstRequestStartedAt.IsZero() {
			firstRequestStartedAt = time.Now().UTC()
			requestMu.Unlock()
			time.Sleep(1200 * time.Millisecond)
		} else {
			requestMu.Unlock()
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
			"id":     "chatcmpl-direct-capture-test",
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

	startedAt := time.Now().UTC()
	response, err := RunDirectProbe(context.Background(), DirectProbeRequest{
		BaseURL:      server.URL,
		APIKey:       "test-token",
		Model:        "gpt-test",
		Provider:     ProviderOpenAI,
		ProbeProfile: ProbeProfileStandard,
	})
	finishedAt := time.Now().UTC()
	if err != nil {
		t.Fatalf("RunDirectProbe returned error: %v", err)
	}
	if strings.TrimSpace(response.CapturedAt) == "" {
		t.Fatal("CapturedAt is empty; direct probe evidence exports need capture time")
	}
	if !strings.HasPrefix(response.SourceTaskID, "direct-probe-") {
		t.Fatalf("SourceTaskID = %q, want direct-probe prefixed capture id", response.SourceTaskID)
	}
	capturedAt, err := time.Parse(time.RFC3339, response.CapturedAt)
	if err != nil {
		t.Fatalf("CapturedAt = %q, want RFC3339: %v", response.CapturedAt, err)
	}
	if capturedAt.Before(startedAt.Truncate(time.Second)) || capturedAt.After(finishedAt.Add(time.Second)) {
		t.Fatalf("CapturedAt = %s, want within direct probe request window %s..%s", capturedAt, startedAt, finishedAt)
	}
	requestMu.Lock()
	firstRequestAt := firstRequestStartedAt
	requestMu.Unlock()
	if firstRequestAt.IsZero() {
		t.Fatal("test server did not observe a probe request")
	}
	if capturedAt.After(firstRequestAt) {
		t.Fatalf("CapturedAt = %s, want capture start no later than first upstream request at %s", capturedAt, firstRequestAt)
	}
}

func TestRunProbeSuiteOnlyUsesLLMProbeResultsOnly(t *testing.T) {
	modelsListCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			modelsListCalled = true
			http.NotFound(w, r)
			return
		}
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
			"id":     "chatcmpl-task-test",
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
		BaseURL:      server.URL,
		Token:        "test-token",
		Models:       []string{"gpt-test"},
		Providers:    []string{ProviderOpenAI},
		ProbeProfile: ProbeProfileStandard,
		Executor:     NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if modelsListCalled {
		t.Fatal("probe-only task path must not call the legacy models list check")
	}
	if len(results) == 0 {
		t.Fatal("probe-only task path returned no results")
	}

	for _, result := range results {
		if !isLLMProbeResultKey(result.CheckKey) {
			t.Fatalf("probe-only task path included non-probe check %s", result.CheckKey)
		}
	}
}

func TestRunProbeSuiteOnlyPreflightAbortReturnsProbeFailuresOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			t.Fatal("direct preflight abort must not call the legacy models list check")
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key","type":"authentication_error"}}`))
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:      server.URL,
		Token:        "bad-token",
		Models:       []string{"gpt-test"},
		Providers:    []string{ProviderOpenAI},
		ProbeProfile: ProbeProfileStandard,
		Executor:     NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("preflight abort should return failed probe-suite results")
	}
	for _, result := range results {
		if !isLLMProbeResultKey(result.CheckKey) {
			t.Fatalf("preflight abort included non-LLMprobe check %s", result.CheckKey)
		}
		if result.Success {
			t.Fatalf("preflight abort result %s succeeded unexpectedly", result.CheckKey)
		}
		if result.ErrorCode != "authentication_failed" {
			t.Fatalf("preflight abort error code = %q, want authentication_failed", result.ErrorCode)
		}
	}
}

func TestToModelResultsPersistsProbeClassificationFields(t *testing.T) {
	results := toModelResults(123, []CheckResult{
		{
			Provider:            ProviderAnthropic,
			Group:               probeGroupSecurity,
			CheckKey:            CheckProbeNPMRegistry,
			ModelName:           "claude-opus-4-7",
			Neutral:             false,
			Skipped:             true,
			Success:             false,
			Score:               0,
			ErrorCode:           "http_502",
			Message:             "端点返回 502：HTTP 502",
			RiskLevel:           "unknown",
			Evidence:            []string{"端点错误，本探针未评分"},
			LatencyMs:           1104,
			PrivateResponseText: "private response must not persist",
			Raw: map[string]any{
				"response_redacted": true,
			},
		},
	})

	if len(results) != 1 {
		t.Fatalf("model result count = %d, want 1", len(results))
	}
	result := results[0]
	if !result.Skipped {
		t.Fatal("Skipped was not persisted")
	}
	if result.RiskLevel != "unknown" {
		t.Fatalf("RiskLevel = %q, want unknown", result.RiskLevel)
	}
	if len(result.Evidence) != 1 || result.Evidence[0] != "端点错误，本探针未评分" {
		t.Fatalf("Evidence = %#v, want persisted evidence", result.Evidence)
	}
	if strings.Contains(result.Raw, "private response") {
		t.Fatalf("Raw leaked private response text: %s", result.Raw)
	}
}

func isLLMProbeResultKey(checkKey CheckKey) bool {
	value := string(checkKey)
	return strings.HasPrefix(value, "probe_") || strings.HasPrefix(value, "canary_")
}

func TestAnthropicClaudeCodeClientProfileAddsClientFingerprint(t *testing.T) {
	var sawClaudeCodeRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("User-Agent"); !regexp.MustCompile(`^claude-cli/\d+\.\d+\.\d+`).MatchString(got) {
			t.Fatalf("User-Agent = %q, want claude-cli semver", got)
		}
		if got := r.Header.Get("X-App"); got != "cli" {
			t.Fatalf("X-App = %q, want cli", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("anthropic-version = %q, want 2023-06-01", got)
		}
		beta := r.Header.Get("anthropic-beta")
		for _, token := range []string{"claude-code-20250219", "interleaved-thinking-2025-05-14"} {
			if !strings.Contains(beta, token) {
				t.Fatalf("anthropic-beta = %q, missing %q", beta, token)
			}
		}
		if got := r.Header.Get("X-Claude-Code-Session-Id"); got == "" {
			t.Fatal("X-Claude-Code-Session-Id must be set")
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("x-stainless-helper-method"); got != "stream" {
			t.Fatalf("x-stainless-helper-method = %q, want stream", got)
		}
		if got := r.Header.Get("x-client-request-id"); !regexp.MustCompile(`^[0-9a-f-]{36}$`).MatchString(got) {
			t.Fatalf("x-client-request-id = %q, want UUID", got)
		}

		var payload map[string]any
		if err := common.DecodeJson(r.Body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		system, ok := payload["system"].([]any)
		if !ok || len(system) != 2 {
			t.Fatalf("payload system = %#v, want billing + Claude Code prompt blocks", payload["system"])
		}
		billingBlock, _ := system[0].(map[string]any)
		billingText := stringFromAnyForTest(billingBlock["text"])
		if !regexp.MustCompile(`^x-anthropic-billing-header: cc_version=\d+\.\d+\.\d+\.[0-9a-f]{3}; cc_entrypoint=cli; cch=[0-9a-f]{5};$`).MatchString(billingText) {
			t.Fatalf("billing system block = %q, want signed Claude Code billing attribution", billingText)
		}
		if strings.Contains(billingText, "cch=00000") {
			t.Fatalf("billing system block was not CCH signed: %q", billingText)
		}
		claudePromptBlock, _ := system[1].(map[string]any)
		if !strings.HasPrefix(strings.TrimSpace(stringFromAnyForTest(claudePromptBlock["text"])), claudeCodeSystemPrompt) {
			t.Fatalf("Claude Code system block = %#v, want Claude Code system prompt", claudePromptBlock)
		}
		metadata, ok := payload["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("metadata = %#v, want object", payload["metadata"])
		}
		userID, ok := metadata["user_id"].(string)
		if !ok || !regexp.MustCompile(`^user_[0-9a-f]{64}_account__session_[0-9a-f-]{36}$`).MatchString(userID) {
			t.Fatalf("metadata.user_id = %#v, want Claude Code legacy metadata", metadata["user_id"])
		}
		if !strings.HasSuffix(userID, "_session_"+r.Header.Get("X-Claude-Code-Session-Id")) {
			t.Fatalf("metadata.user_id = %q and X-Claude-Code-Session-Id = %q should use the same session", userID, r.Header.Get("X-Claude-Code-Session-Id"))
		}
		if payload["stream"] != true {
			t.Fatalf("payload stream = %#v, want true for Claude Code profile", payload["stream"])
		}
		sawClaudeCodeRequest = true

		prompt := extractAnthropicRequestPrompt(payload)
		responseBytes, _ := common.Marshal(map[string]any{
			"id":    "msg-claude-code-profile-test",
			"type":  "message",
			"model": payload["model"],
			"content": []map[string]any{
				{"type": "text", "text": probeTestResponse(prompt)},
			},
			"usage": map[string]any{"input_tokens": 32, "output_tokens": 4},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:       server.URL,
		Token:         "test-token",
		Models:        []string{"claude-test"},
		Providers:     []string{ProviderAnthropic},
		ProbeProfile:  ProbeProfileStandard,
		ClientProfile: ClientProfileClaudeCode,
		Executor:      NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if !sawClaudeCodeRequest {
		t.Fatal("server did not observe a Claude Code-profile request")
	}
	if len(results) == 0 {
		t.Fatal("direct probe suite returned no results")
	}
}

func TestAnthropicClaudeCodeClientProfileUsesCLICompatibleBodyOrder(t *testing.T) {
	runner := Runner{
		ClientProfile: ClientProfileClaudeCode,
		sessionID:     "11111111-1111-4111-8111-111111111111",
	}
	body := runner.applyAnthropicClientProfileBody(ProviderAnthropic, map[string]any{
		"model":      "claude-test",
		"max_tokens": 16,
		"stream":     true,
		"messages": []map[string]any{
			{"role": "user", "content": "Reply OK."},
		},
	})
	payload, err := runner.marshalRequestBody(ProviderAnthropic, body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	rendered := string(payload)
	order := []string{`"model"`, `"max_tokens"`, `"stream"`, `"messages"`, `"system"`, `"metadata"`}
	last := -1
	for _, key := range order {
		idx := strings.Index(rendered, key)
		if idx < 0 {
			t.Fatalf("payload missing %s: %s", key, rendered)
		}
		if idx <= last {
			t.Fatalf("payload key %s is out of Claude Code-compatible order: %s", key, rendered)
		}
		last = idx
	}
	if strings.Contains(rendered, "cch=00000") {
		t.Fatalf("payload CCH placeholder was not signed: %s", rendered)
	}
}

func TestAnthropicClaudeCodeClientProfilePassesGatewayStyleRestriction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := common.DecodeJson(r.Body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !isHermesProxyClaudeCodeClientForTest(r, payload) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"No available accounts: this group only allows Claude Code clients"}}`))
			return
		}
		prompt := extractAnthropicRequestPrompt(payload)
		responseBytes, _ := common.Marshal(map[string]any{
			"id":    "msg-claude-code-gateway-test",
			"type":  "message",
			"model": payload["model"],
			"content": []map[string]any{
				{"type": "text", "text": probeTestResponse(prompt)},
			},
			"usage": map[string]any{"input_tokens": 32, "output_tokens": 4},
		})
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:       server.URL,
		Token:         "test-token",
		Models:        []string{"claude-test"},
		Providers:     []string{ProviderAnthropic},
		ProbeProfile:  ProbeProfileStandard,
		ClientProfile: ClientProfileClaudeCode,
		Executor:      NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("direct probe suite returned no results")
	}
	for _, result := range results {
		if strings.Contains(result.Message, "only allows Claude Code clients") {
			t.Fatalf("request failed Claude Code-only gateway validation: %#v", result)
		}
	}
}

func TestAnthropicClaudeCodeClientProfilePassesStrictMimicryGateway(t *testing.T) {
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
		if !isStrictClaudeCodeOAuthMimicryForTest(r, payload) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream rejected non-Claude-Code request fingerprint"}}`))
			return
		}
		prompt := extractAnthropicRequestPrompt(payload)
		content := probeTestResponse(prompt)
		if content == "" {
			content = "OK"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"message_start","message":{"id":"msg_strict_mimicry","type":"message","model":"claude-test","usage":{"input_tokens":32}}}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n"))
		chunkBytes, _ := common.Marshal(map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": content},
		})
		_, _ = w.Write([]byte("data: " + string(chunkBytes) + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_delta","usage":{"output_tokens":4}}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_stop"}` + "\n\n"))
	}))
	defer server.Close()

	runner := Runner{
		BaseURL:       server.URL,
		Token:         "test-token",
		Models:        []string{"claude-test"},
		Providers:     []string{ProviderAnthropic},
		ProbeProfile:  ProbeProfileStandard,
		ClientProfile: ClientProfileClaudeCode,
		sessionID:     "11111111-1111-4111-8111-111111111111",
		Executor:      NewCurlExecutor(time.Second),
	}
	results, err := runner.RunProbeSuiteOnly(context.Background())
	if err != nil {
		t.Fatalf("RunProbeSuiteOnly returned error: %v", err)
	}
	if requestCount == 0 {
		t.Fatal("strict gateway did not receive any requests")
	}
	if len(results) == 0 {
		t.Fatal("direct probe suite returned no results")
	}
	for _, result := range results {
		if strings.Contains(result.Message, "non-Claude-Code request fingerprint") {
			t.Fatalf("request failed strict Claude Code gateway validation: %#v", result)
		}
		if result.CheckKey == CheckProbeInstructionFollow && !result.Success {
			t.Fatalf("instruction probe should parse Anthropic SSE and pass, got %#v", result)
		}
	}
}

func isStrictClaudeCodeOAuthMimicryForTest(r *http.Request, payload map[string]any) bool {
	if r.Header.Get("User-Agent") != claudeCodeCLIUserAgent {
		return false
	}
	beta := r.Header.Get("anthropic-beta")
	for _, required := range []string{
		"claude-code-20250219",
		"oauth-2025-04-20",
		"interleaved-thinking-2025-05-14",
		"prompt-caching-scope-2026-01-05",
		"effort-2025-11-24",
		"redact-thinking-2026-02-12",
		"context-management-2025-06-27",
		"extended-cache-ttl-2025-04-11",
	} {
		if !strings.Contains(beta, required) {
			return false
		}
	}
	if payload["stream"] != true {
		return false
	}
	system, ok := payload["system"].([]any)
	if !ok || len(system) != 2 {
		return false
	}
	billingBlock, _ := system[0].(map[string]any)
	billingText := stringFromAnyForTest(billingBlock["text"])
	if !strings.Contains(billingText, "cc_version="+claudeCodeCLIVersion+".") || strings.Contains(billingText, "cch=00000") {
		return false
	}
	return isHermesProxyClaudeCodeClientForTest(r, payload)
}

func isHermesProxyClaudeCodeClientForTest(r *http.Request, payload map[string]any) bool {
	if !regexp.MustCompile(`(?i)^claude-cli/\d+\.\d+\.\d+`).MatchString(r.Header.Get("User-Agent")) {
		return false
	}
	if r.Header.Get("X-App") == "" || r.Header.Get("anthropic-beta") == "" || r.Header.Get("anthropic-version") == "" {
		return false
	}
	system, ok := payload["system"].([]any)
	if !ok {
		return false
	}
	hasPrompt := false
	for _, item := range system {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringFromAnyForTest(block["text"])) == claudeCodeSystemPrompt {
			hasPrompt = true
			break
		}
	}
	if !hasPrompt {
		return false
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		return false
	}
	userID := stringFromAnyForTest(metadata["user_id"])
	if !regexp.MustCompile(`^user_[0-9a-f]{64}_account__session_[0-9a-f-]{36}$`).MatchString(userID) {
		return false
	}
	return strings.HasSuffix(userID, "_session_"+r.Header.Get("X-Claude-Code-Session-Id"))
}

func TestAnthropicClaudeCodeClientProfileMovesCustomSystemToMessages(t *testing.T) {
	runner := Runner{ClientProfile: ClientProfileClaudeCode}
	body := runner.applyAnthropicClientProfileBody(ProviderAnthropic, map[string]any{
		"model":      "claude-test",
		"max_tokens": 32,
		"system":     "You are a route-specific evaluator.",
		"messages": []map[string]any{
			{"role": "user", "content": "Return OK."},
		},
	})

	system := body["system"].([]map[string]any)
	if len(system) != 2 {
		t.Fatalf("system blocks = %#v, want billing + Claude Code prompt", system)
	}
	if text := stringFromAnyForTest(system[0]["text"]); !strings.HasPrefix(text, "x-anthropic-billing-header:") {
		t.Fatalf("first system block = %q, want billing attribution", text)
	}
	if text := stringFromAnyForTest(system[1]["text"]); text != claudeCodeSystemPrompt {
		t.Fatalf("second system block = %q, want Claude Code prompt", text)
	}

	messages := body["messages"].([]map[string]any)
	if roles := []string{messages[0]["role"].(string), messages[1]["role"].(string), messages[2]["role"].(string)}; !slices.Equal(roles, []string{"user", "assistant", "user"}) {
		t.Fatalf("message roles = %#v, want system-instruction pair before original user", roles)
	}
	firstContent := messages[0]["content"].([]map[string]any)
	if text := stringFromAnyForTest(firstContent[0]["text"]); text != "[System Instructions]\nYou are a route-specific evaluator." {
		t.Fatalf("first injected message text = %q", text)
	}
	ackContent := messages[1]["content"].([]map[string]any)
	if text := stringFromAnyForTest(ackContent[0]["text"]); text != "Understood. I will follow these instructions." {
		t.Fatalf("ack message text = %q", text)
	}
}

func extractAnthropicRequestPrompt(payload map[string]any) string {
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) == 0 {
		return ""
	}
	for _, item := range messages {
		msg, ok := item.(map[string]any)
		if !ok || msg["role"] != "user" {
			continue
		}
		text := stringFromAnyForTest(msg["content"])
		if strings.HasPrefix(text, "[System Instructions]") {
			continue
		}
		return text
	}
	return ""
}

func stringFromAnyForTest(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var parts []string
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := block["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}
