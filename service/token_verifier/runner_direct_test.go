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

func isLLMProbeResultKey(checkKey CheckKey) bool {
	value := string(checkKey)
	return strings.HasPrefix(value, "probe_") || strings.HasPrefix(value, "canary_")
}
