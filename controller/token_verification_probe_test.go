package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestNormalizeTokenVerificationProbeProfile(t *testing.T) {
	profile, err := normalizeTokenVerificationProbeProfile("", common.RoleCommonUser)
	if err != nil {
		t.Fatalf("expected empty profile to use full, got %v", err)
	}
	if profile != tokenverifier.ProbeProfileFull {
		t.Fatalf("expected full profile, got %q", profile)
	}

	profile, err = normalizeTokenVerificationProbeProfile(" full ", common.RoleCommonUser)
	if err != nil {
		t.Fatalf("expected common user full profile to pass, got %v", err)
	}
	if profile != tokenverifier.ProbeProfileFull {
		t.Fatalf("expected full profile, got %q", profile)
	}

	if _, err = normalizeTokenVerificationProbeProfile("standard", common.RoleAdminUser); err == nil {
		t.Fatal("expected standard profile to fail")
	}
	if _, err = normalizeTokenVerificationProbeProfile("deep", common.RoleAdminUser); err == nil {
		t.Fatal("expected deep profile to fail")
	}
	if _, err = normalizeTokenVerificationProbeProfile("ultimate", common.RoleAdminUser); err == nil {
		t.Fatal("expected unsupported profile to fail")
	}
}

func TestNormalizeTokenVerificationProbeClientProfile(t *testing.T) {
	profile, err := normalizeTokenVerificationProbeClientProfile("", tokenverifier.ProviderAnthropic)
	if err != nil {
		t.Fatalf("expected empty client profile to pass, got %v", err)
	}
	if profile != "" {
		t.Fatalf("empty client profile normalized to %q, want empty", profile)
	}

	profile, err = normalizeTokenVerificationProbeClientProfile(" Claude_Code ", tokenverifier.ProviderAnthropic)
	if err != nil {
		t.Fatalf("expected Claude Code profile to pass for Anthropic, got %v", err)
	}
	if profile != tokenverifier.ClientProfileClaudeCode {
		t.Fatalf("client profile = %q, want %q", profile, tokenverifier.ClientProfileClaudeCode)
	}

	if _, err := normalizeTokenVerificationProbeClientProfile("claude_code", tokenverifier.ProviderOpenAI); err == nil {
		t.Fatal("expected Claude Code profile to fail for OpenAI provider")
	}
	if _, err := normalizeTokenVerificationProbeClientProfile("browser", tokenverifier.ProviderAnthropic); err == nil {
		t.Fatal("expected unsupported client profile to fail")
	}
}

func TestCreateTokenVerificationProbeDoesNotReturnAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const secret = "sk-test-secret-value"
	const requestedProfile = tokenverifier.ProbeProfileFull
	const requestedClientProfile = tokenverifier.ClientProfileClaudeCode
	originalRunner := runDirectTokenVerificationProbe
	runDirectTokenVerificationProbe = func(ctx context.Context, input tokenverifier.DirectProbeRequest) (*tokenverifier.DirectProbeResponse, error) {
		if input.APIKey != secret {
			t.Fatalf("expected API key to be passed to runner")
		}
		if input.ProbeProfile != requestedProfile {
			t.Fatalf("expected probe profile %q, got %q", requestedProfile, input.ProbeProfile)
		}
		if input.ClientProfile != requestedClientProfile {
			t.Fatalf("expected client profile %q, got %q", requestedClientProfile, input.ClientProfile)
		}
		return &tokenverifier.DirectProbeResponse{
			BaseURL:       input.BaseURL,
			Provider:      input.Provider,
			Model:         input.Model,
			ProbeProfile:  input.ProbeProfile,
			ClientProfile: input.ClientProfile,
			Results: []tokenverifier.CheckResult{
				{
					Provider:  input.Provider,
					CheckKey:  tokenverifier.CheckProbeInstructionFollow,
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

	payload := []byte(`{"url":"https://8.8.8.8/v1","api_key":"` + secret + `","model":"claude-opus-4-7","provider":"anthropic","probe_profile":"` + requestedProfile + `","client_profile":"` + requestedClientProfile + `"}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleAdminUser)
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
	if data["probe_profile"] != requestedProfile {
		t.Fatalf("response probe_profile = %v, want %q", data["probe_profile"], requestedProfile)
	}
	if data["client_profile"] != requestedClientProfile {
		t.Fatalf("response client_profile = %v, want %q", data["client_profile"], requestedClientProfile)
	}
	sourceTaskID, ok := data["source_task_id"].(string)
	if !ok || strings.TrimSpace(sourceTaskID) == "" {
		t.Fatalf("response source_task_id = %#v, want exportable source id", data["source_task_id"])
	}
	capturedAt, ok := data["captured_at"].(string)
	if !ok || strings.TrimSpace(capturedAt) == "" {
		t.Fatalf("response captured_at = %#v, want RFC3339 capture time", data["captured_at"])
	}
	if _, err := time.Parse(time.RFC3339, capturedAt); err != nil {
		t.Fatalf("response captured_at = %q, want RFC3339: %v", capturedAt, err)
	}
}
