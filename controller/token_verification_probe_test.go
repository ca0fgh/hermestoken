package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
	"github.com/gin-gonic/gin"
)

func TestNormalizeTokenVerificationProbeBaseURL(t *testing.T) {
	t.Setenv("TOKEN_VERIFIER_ALLOW_PRIVATE_URLS", "")

	baseURL, err := normalizeTokenVerificationProbeBaseURL("https://8.8.8.8/gateway/v1/")
	if err != nil {
		t.Fatalf("expected public url to pass, got %v", err)
	}
	if baseURL != "https://8.8.8.8/gateway" {
		t.Fatalf("expected normalized gateway url, got %q", baseURL)
	}

	if _, err := normalizeTokenVerificationProbeBaseURL("file:///tmp/key"); err == nil {
		t.Fatal("expected non-http url to fail")
	}

	if _, err := normalizeTokenVerificationProbeBaseURL("http://127.0.0.1:3000/v1"); err == nil {
		t.Fatal("expected private url to fail by default")
	}
}

func TestNormalizeTokenVerificationProbeBaseURLAllowsPrivateWhenConfigured(t *testing.T) {
	t.Setenv("TOKEN_VERIFIER_ALLOW_PRIVATE_URLS", "true")

	baseURL, err := normalizeTokenVerificationProbeBaseURL("http://127.0.0.1:3000/v1")
	if err != nil {
		t.Fatalf("expected private url to pass when configured, got %v", err)
	}
	if baseURL != "http://127.0.0.1:3000" {
		t.Fatalf("expected normalized local url, got %q", baseURL)
	}
}

func TestNormalizeTokenVerificationProbeProvider(t *testing.T) {
	provider, err := normalizeTokenVerificationProbeProvider("")
	if err != nil {
		t.Fatalf("expected empty provider to use default, got %v", err)
	}
	if provider != tokenverifier.ProviderOpenAI {
		t.Fatalf("expected default provider openai, got %q", provider)
	}

	provider, err = normalizeTokenVerificationProbeProvider(" Anthropic ")
	if err != nil {
		t.Fatalf("expected anthropic provider to pass, got %v", err)
	}
	if provider != tokenverifier.ProviderAnthropic {
		t.Fatalf("expected anthropic provider, got %q", provider)
	}

	if _, err := normalizeTokenVerificationProbeProvider("gemini"); err == nil {
		t.Fatal("expected unsupported provider to fail")
	}
}

func TestCreateTokenVerificationProbeDoesNotReturnAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const secret = "sk-test-secret-value"
	originalRunner := runDirectTokenVerificationProbe
	runDirectTokenVerificationProbe = func(ctx context.Context, input tokenverifier.DirectProbeRequest) (*tokenverifier.DirectProbeResponse, error) {
		if input.APIKey != secret {
			t.Fatalf("expected API key to be passed to runner")
		}
		return &tokenverifier.DirectProbeResponse{
			BaseURL:  input.BaseURL,
			Provider: input.Provider,
			Model:    input.Model,
			Results: []tokenverifier.CheckResult{
				{
					Provider:  input.Provider,
					CheckKey:  tokenverifier.CheckAvailability,
					ModelName: input.Model,
					Success:   true,
					Score:     100,
					Message:   "echoed " + secret,
					Raw: map[string]any{
						"echo": secret,
						"nested": []any{
							map[string]any{"value": "prefix-" + secret},
						},
					},
				},
			},
			Report: tokenverifier.ReportSummary{
				Score:      100,
				Grade:      "A",
				Conclusion: "ok " + secret,
			},
		}, nil
	}
	t.Cleanup(func() {
		runDirectTokenVerificationProbe = originalRunner
	})

	payload := []byte(`{"url":"https://8.8.8.8/v1","api_key":"` + secret + `","model":"gpt-4o-mini","provider":"openai"}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/token_verification/probe", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CreateTokenVerificationProbe(ctx)

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got %q", response.Message)
	}
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatal("response must not contain the submitted API key")
	}

	var data map[string]any
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if _, ok := data["api_key"]; ok {
		t.Fatal("response must not include api_key field")
	}
}
