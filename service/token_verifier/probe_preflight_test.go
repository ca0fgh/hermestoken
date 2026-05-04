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
	result := httpFailedResult(CheckModelAccess, "gpt-test", 401, []byte(`{"error":{"message":"bad key"}}`), 42)

	if result.ErrorCode != "authentication_failed" {
		t.Fatalf("error code = %q, want authentication_failed", result.ErrorCode)
	}
	if result.Message == "" {
		t.Fatal("expected user-facing preflight message")
	}
}
