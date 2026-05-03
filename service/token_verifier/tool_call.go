package token_verifier

import (
	"context"

	"github.com/ca0fgh/hermestoken/common"
)

// Tool-call protocol fingerprint check. Sends one chat completion that
// defines a single zero-argument tool, then classifies the response by
// schema family. A relay performing cross-family substitution
// (e.g. claiming claude-opus but routing to gpt-4o) almost always exposes
// itself here because the tool_calls / content[].type=tool_use shapes are
// fundamentally incompatible.

const (
	toolCallProbeName        = "get_current_time"
	toolCallProbeDescription = "Returns the current ISO timestamp in UTC."
	toolCallProbePrompt      = "Use the get_current_time tool to look up the current UTC ISO timestamp, then reply with just the timestamp."
)

func (r Runner) checkToolCall(ctx context.Context, executor *CurlExecutor, provider, modelName string) CheckResult {
	expected := expectedSchemaFamily(provider)
	body := buildToolCallRequestBody(provider, modelName)
	headers := providerHeaders(provider, r.Token)
	headers["Content-Type"] = "application/json"

	payload, _ := common.Marshal(body)
	endpoint := r.endpoint(toolCallEndpoint(provider))

	resp, err := executor.Do(ctx, "POST", endpoint, headers, payload)
	if err != nil {
		return withProvider(failedResult(CheckToolCall, modelName, err, 0), provider)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return withProvider(httpFailedResult(CheckToolCall, modelName, resp.StatusCode, resp.Body, resp.LatencyMs), provider)
	}

	var decoded map[string]any
	if err := common.Unmarshal(resp.Body, &decoded); err != nil {
		return CheckResult{
			Provider:  provider,
			CheckKey:  CheckToolCall,
			ModelName: modelName,
			Success:   false,
			Score:     0,
			LatencyMs: resp.LatencyMs,
			ErrorCode: "invalid_json_response",
			Message:   err.Error(),
		}
	}
	if msg := extractErrorMessage(decoded); msg != "" {
		return CheckResult{
			Provider:  provider,
			CheckKey:  CheckToolCall,
			ModelName: modelName,
			Success:   false,
			Score:     0,
			LatencyMs: resp.LatencyMs,
			ErrorCode: "upstream_error",
			Message:   msg,
			Raw:       compactRaw(decoded),
		}
	}

	detected := detectToolSchemaFamily(decoded)
	return classifyToolCallResult(provider, modelName, expected, detected, resp.LatencyMs, decoded)
}

func classifyToolCallResult(provider, modelName, expected, detected string, latencyMs int64, decoded map[string]any) CheckResult {
	res := CheckResult{
		Provider:             provider,
		CheckKey:             CheckToolCall,
		ModelName:            modelName,
		ExpectedSchemaFamily: expected,
		ToolSchemaFamily:     detected,
		LatencyMs:            latencyMs,
		Raw:                  compactRaw(decoded),
	}
	switch {
	case detected == ToolSchemaFamilyNone:
		// Model didn't call the tool. Inconclusive — neither punish nor credit.
		res.Skipped = true
		res.Success = true
		res.Score = 0
		res.Message = "模型未调用工具，无法判定 schema 家族（结果不计入稳定性）"
	case detected == ToolSchemaFamilyAmbiguous:
		res.SchemaMatches = false
		res.Success = false
		res.Score = 30
		res.Message = "响应同时包含 OpenAI 与 Anthropic 两种 schema 信号，疑似中转层混合或异常"
	case detected == ToolSchemaFamilyUnknown:
		// Response shape doesn't match either family. Could be a custom relay
		// or a non-standard model. Leave as informational.
		res.Skipped = true
		res.Success = true
		res.Score = 0
		res.Message = "响应 schema 不可识别，无法判定（结果不计入稳定性）"
	case detected == expected:
		res.SchemaMatches = true
		res.Success = true
		res.Score = 100
		res.Message = "工具调用 schema 与请求 provider 一致"
	default:
		res.SchemaMatches = false
		res.Success = false
		res.Score = 0
		res.Message = "工具调用 schema 与请求 provider 不一致，疑似跨家族模型替换"
	}
	return res
}

func expectedSchemaFamily(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return ToolSchemaFamilyAnthropic
	default:
		return ToolSchemaFamilyOpenAI
	}
}

func toolCallEndpoint(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return "/v1/messages"
	default:
		return "/v1/chat/completions"
	}
}

func buildToolCallRequestBody(provider, modelName string) map[string]any {
	if provider == ProviderAnthropic {
		return map[string]any{
			"model":      modelName,
			"max_tokens": 64,
			"tools": []map[string]any{
				{
					"name":        toolCallProbeName,
					"description": toolCallProbeDescription,
					"input_schema": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
						"required":   []string{},
					},
				},
			},
			"messages": []map[string]string{
				{"role": "user", "content": toolCallProbePrompt},
			},
		}
	}
	return map[string]any{
		"model":      modelName,
		"max_tokens": 64,
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        toolCallProbeName,
					"description": toolCallProbeDescription,
					"parameters": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
						"required":   []string{},
					},
				},
			},
		},
		"tool_choice": "auto",
		"messages": []map[string]string{
			{"role": "user", "content": toolCallProbePrompt},
		},
	}
}

// detectToolSchemaFamily classifies the response shape by tool-call signature.
// Pure function for testability.
//
// OpenAI signature (any of):
//   - choices[0].message.tool_calls is a non-empty array
//   - choices[0].finish_reason == "tool_calls"
//
// Anthropic signature (any of):
//   - any element of top-level content[].type == "tool_use"
//   - top-level stop_reason == "tool_use"
//
// Ambiguous: both signatures present in the same response (relay confusion).
// None: neither, but the response otherwise looks like a valid completion
// (model returned text without calling the tool). Inconclusive.
// Unknown: response shape doesn't match a recognised provider envelope.
func detectToolSchemaFamily(decoded map[string]any) string {
	if decoded == nil {
		return ToolSchemaFamilyUnknown
	}
	openaiHit := false
	anthropicHit := false

	if choices, ok := decoded["choices"].([]any); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]any); ok {
			if msg, ok := first["message"].(map[string]any); ok {
				if calls, ok := msg["tool_calls"].([]any); ok && len(calls) > 0 {
					openaiHit = true
				}
			}
			if reason, _ := first["finish_reason"].(string); reason == "tool_calls" {
				openaiHit = true
			}
		}
	}
	if content, ok := decoded["content"].([]any); ok {
		for _, item := range content {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "tool_use" {
					anthropicHit = true
					break
				}
			}
		}
	}
	if reason, _ := decoded["stop_reason"].(string); reason == "tool_use" {
		anthropicHit = true
	}

	switch {
	case openaiHit && anthropicHit:
		return ToolSchemaFamilyAmbiguous
	case openaiHit:
		return ToolSchemaFamilyOpenAI
	case anthropicHit:
		return ToolSchemaFamilyAnthropic
	}

	// Neither family-specific signature present. If the response otherwise
	// has the right envelope, the model just declined to call the tool.
	if _, ok := decoded["choices"].([]any); ok {
		return ToolSchemaFamilyNone
	}
	if _, ok := decoded["content"].([]any); ok {
		return ToolSchemaFamilyNone
	}
	return ToolSchemaFamilyUnknown
}
