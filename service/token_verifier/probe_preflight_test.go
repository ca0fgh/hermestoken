package token_verifier

import "testing"

func TestClassifyPreflightResult(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		outcome string
		code    string
	}{
		{
			name:    "auth failed",
			status:  401,
			body:    `{"error":{"message":"bad key","code":"invalid_api_key"}}`,
			outcome: preflightOutcomeAbort,
			code:    "authentication_failed",
		},
		{
			name:    "anthropic nested not found",
			status:  400,
			body:    `{"type":"error","error":{"type":"not_found_error","message":"model missing"}}`,
			outcome: preflightOutcomeAbort,
			code:    "not_found_error",
		},
		{
			name:    "rate limit warns",
			status:  429,
			body:    `{"error":{"message":"too many requests"}}`,
			outcome: preflightOutcomeWarn,
			code:    "rate_limit",
		},
		{
			name:    "ok proceeds",
			status:  200,
			body:    `{}`,
			outcome: preflightOutcomeOK,
			code:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyPreflightResult(tt.status, []byte(tt.body))
			if got.Outcome != tt.outcome || got.Code != tt.code {
				t.Fatalf("preflight = %+v, want outcome=%s code=%s", got, tt.outcome, tt.code)
			}
		})
	}
}

func TestHTTPFailedResultUsesPreflightClassification(t *testing.T) {
	result := httpFailedResult(CheckProbeInstructionFollow, "gpt-test", 401, []byte(`{"error":{"message":"bad key"}}`), 42)

	if result.ErrorCode != "authentication_failed" {
		t.Fatalf("error code = %q, want authentication_failed", result.ErrorCode)
	}
	if result.Message == "" {
		t.Fatal("expected user-facing preflight message")
	}
}

func TestHTTPFailedResultTreatsTransientEndpointErrorsAsUnscored(t *testing.T) {
	result := httpFailedResult(CheckProbeNPMRegistry, "claude-test", 502, []byte(`HTTP 502`), 1104)

	if result.Success {
		t.Fatal("transient endpoint error should not pass")
	}
	if !result.Skipped {
		t.Fatalf("skipped = %v, want true for unscored endpoint error", result.Skipped)
	}
	if result.ErrorCode != "http_502" {
		t.Fatalf("error code = %q, want http_502", result.ErrorCode)
	}
	if result.RiskLevel != "unknown" {
		t.Fatalf("risk level = %q, want unknown", result.RiskLevel)
	}
	if len(result.Evidence) == 0 {
		t.Fatal("expected evidence explaining endpoint error was not scored as model behavior")
	}
}

func TestFailedDirectProbeSuiteResultsAreUnscoredWhenPreflightAborts(t *testing.T) {
	preflight := preflightResult{Outcome: preflightOutcomeAbort, Code: "authentication_failed", Reason: "认证失败（401）：bad key"}
	results := failedDirectProbeSuiteResults(ProviderAnthropic, "claude-test", ProbeProfileStandard, nil, preflight, 42)

	if len(results) == 0 {
		t.Fatal("expected generated probe results")
	}
	for _, result := range results {
		if result.Success {
			t.Fatalf("preflight abort result %s passed unexpectedly", result.CheckKey)
		}
		if !result.Skipped {
			t.Fatalf("preflight abort result %s skipped = false, want unscored", result.CheckKey)
		}
		if result.RiskLevel != "unknown" {
			t.Fatalf("preflight abort result %s risk = %q, want unknown", result.CheckKey, result.RiskLevel)
		}
	}
}
