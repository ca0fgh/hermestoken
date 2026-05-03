package token_verifier

import (
	"strings"
	"testing"
)

func TestDetectToolSchemaFamily(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "openai tool_calls present",
			body: map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"tool_calls": []any{
								map[string]any{"id": "call_x", "type": "function", "function": map[string]any{"name": "get_current_time"}},
							},
						},
					},
				},
			},
			want: ToolSchemaFamilyOpenAI,
		},
		{
			name: "openai finish_reason=tool_calls without inline tool_calls array",
			body: map[string]any{
				"choices": []any{
					map[string]any{
						"finish_reason": "tool_calls",
						"message":       map[string]any{},
					},
				},
			},
			want: ToolSchemaFamilyOpenAI,
		},
		{
			name: "anthropic content[].type=tool_use",
			body: map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "..."},
					map[string]any{"type": "tool_use", "name": "get_current_time"},
				},
			},
			want: ToolSchemaFamilyAnthropic,
		},
		{
			name: "anthropic stop_reason=tool_use",
			body: map[string]any{
				"content":     []any{},
				"stop_reason": "tool_use",
			},
			want: ToolSchemaFamilyAnthropic,
		},
		{
			name: "ambiguous: both signatures present (relay confusion)",
			body: map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"tool_calls": []any{map[string]any{"id": "x"}},
						},
					},
				},
				"content": []any{map[string]any{"type": "tool_use"}},
			},
			want: ToolSchemaFamilyAmbiguous,
		},
		{
			name: "openai shape but model returned text only",
			body: map[string]any{
				"choices": []any{
					map[string]any{"message": map[string]any{"content": "It is 2pm UTC."}, "finish_reason": "stop"},
				},
			},
			want: ToolSchemaFamilyNone,
		},
		{
			name: "anthropic shape but model returned text only",
			body: map[string]any{
				"content":     []any{map[string]any{"type": "text", "text": "It is 2pm UTC."}},
				"stop_reason": "end_turn",
			},
			want: ToolSchemaFamilyNone,
		},
		{
			name: "completely unrecognised envelope",
			body: map[string]any{"weird_field": 1},
			want: ToolSchemaFamilyUnknown,
		},
		{
			name: "nil",
			body: nil,
			want: ToolSchemaFamilyUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectToolSchemaFamily(tc.body); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyToolCallResult(t *testing.T) {
	cases := []struct {
		name             string
		expected         string
		detected         string
		wantSuccess      bool
		wantSkipped      bool
		wantSchemaMatch  bool
		messageContains  string
	}{
		{
			name: "match openai", expected: ToolSchemaFamilyOpenAI, detected: ToolSchemaFamilyOpenAI,
			wantSuccess: true, wantSkipped: false, wantSchemaMatch: true,
			messageContains: "一致",
		},
		{
			name: "cross-family mismatch", expected: ToolSchemaFamilyAnthropic, detected: ToolSchemaFamilyOpenAI,
			wantSuccess: false, wantSkipped: false, wantSchemaMatch: false,
			messageContains: "不一致",
		},
		{
			name: "model didn't call tool", expected: ToolSchemaFamilyOpenAI, detected: ToolSchemaFamilyNone,
			wantSuccess: true, wantSkipped: true, wantSchemaMatch: false,
			messageContains: "未调用",
		},
		{
			name: "ambiguous", expected: ToolSchemaFamilyOpenAI, detected: ToolSchemaFamilyAmbiguous,
			wantSuccess: false, wantSkipped: false, wantSchemaMatch: false,
			messageContains: "同时",
		},
		{
			name: "unknown shape", expected: ToolSchemaFamilyOpenAI, detected: ToolSchemaFamilyUnknown,
			wantSuccess: true, wantSkipped: true, wantSchemaMatch: false,
			messageContains: "不可识别",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := classifyToolCallResult("openai", "gpt-4o", tc.expected, tc.detected, 100, nil)
			if res.Success != tc.wantSuccess {
				t.Errorf("success: got %v, want %v", res.Success, tc.wantSuccess)
			}
			if res.Skipped != tc.wantSkipped {
				t.Errorf("skipped: got %v, want %v", res.Skipped, tc.wantSkipped)
			}
			if res.SchemaMatches != tc.wantSchemaMatch {
				t.Errorf("schema_matches: got %v, want %v", res.SchemaMatches, tc.wantSchemaMatch)
			}
			if !strings.Contains(res.Message, tc.messageContains) {
				t.Errorf("message %q does not contain %q", res.Message, tc.messageContains)
			}
		})
	}
}

func TestBuildReportToolCallSection(t *testing.T) {
	results := []CheckResult{
		// Healthy openai with matching schema — adds to checklist + tool_calls section.
		{Provider: "openai", CheckKey: CheckModelAccess, ModelName: "gpt-4o", Success: true, LatencyMs: 800},
		{
			Provider:             "openai",
			CheckKey:             CheckToolCall,
			ModelName:            "gpt-4o",
			ExpectedSchemaFamily: ToolSchemaFamilyOpenAI,
			ToolSchemaFamily:     ToolSchemaFamilyOpenAI,
			SchemaMatches:        true,
			Success:              true,
			Score:                100,
			Message:              "工具调用 schema 与请求 provider 一致",
		},
		// Cross-family swap on a separate model — must produce a risk.
		{Provider: "anthropic", CheckKey: CheckModelAccess, ModelName: "claude-opus-4-5", Success: true, LatencyMs: 900},
		{
			Provider:             "anthropic",
			CheckKey:             CheckToolCall,
			ModelName:            "claude-opus-4-5",
			ExpectedSchemaFamily: ToolSchemaFamilyAnthropic,
			ToolSchemaFamily:     ToolSchemaFamilyOpenAI,
			SchemaMatches:        false,
			Success:              false,
			Score:                0,
			Message:              "工具调用 schema 与请求 provider 不一致，疑似跨家族模型替换",
		},
		// Skipped (model didn't call tool) — must not pull stability down.
		{Provider: "openai", CheckKey: CheckModelAccess, ModelName: "gpt-3.5-turbo", Success: true, LatencyMs: 700},
		{
			Provider:             "openai",
			CheckKey:             CheckToolCall,
			ModelName:            "gpt-3.5-turbo",
			ExpectedSchemaFamily: ToolSchemaFamilyOpenAI,
			ToolSchemaFamily:     ToolSchemaFamilyNone,
			Skipped:              true,
			Success:              true,
		},
	}

	report := BuildReport(results)

	if len(report.ToolCalls) != 3 {
		t.Fatalf("tool_calls entries: got %d, want 3", len(report.ToolCalls))
	}

	hasRisk := false
	for _, r := range report.Risks {
		if strings.Contains(r, "跨家族") {
			hasRisk = true
			break
		}
	}
	if !hasRisk {
		t.Errorf("expected cross-family risk in report.Risks, got %v", report.Risks)
	}

	// Skipped tool-call must not contribute to stability denominator. With 3
	// model_access (all pass) + 2 tool_call (1 pass, 1 fail) we have 4/5 = 80%
	// stability ratio → dim 12 (int(0.8 * 15)).
	if got := report.Dimensions["stability"]; got != 12 {
		t.Errorf("stability dimension: got %d, want 12 (4 successes / 5 non-skipped checks * 15)", got)
	}
}

func TestBuildReportToolCallNoRiskWhenSchemaMatches(t *testing.T) {
	results := []CheckResult{
		{Provider: "openai", CheckKey: CheckModelAccess, ModelName: "gpt-4o", Success: true, LatencyMs: 800},
		{
			Provider:             "openai",
			CheckKey:             CheckToolCall,
			ModelName:            "gpt-4o",
			ExpectedSchemaFamily: ToolSchemaFamilyOpenAI,
			ToolSchemaFamily:     ToolSchemaFamilyOpenAI,
			SchemaMatches:        true,
			Success:              true,
		},
	}
	report := BuildReport(results)
	for _, r := range report.Risks {
		if strings.Contains(r, "跨家族") {
			t.Errorf("matching schema should not produce a cross-family risk, got %q", r)
		}
	}
}
