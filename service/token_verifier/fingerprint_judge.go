package token_verifier

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

type judgeIdentityResult struct {
	Family     string
	Confidence float64
	Reasons    []string
}

func judgeFingerprint(ctx context.Context, executor *CurlExecutor, responses map[string]string, config IdentityJudgeConfig) ([]IdentityCandidateSummary, *judgeIdentityResult) {
	if executor == nil {
		executor = NewCurlExecutor(0)
	}
	if strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.ModelID) == "" || len(responses) == 0 {
		return nil, nil
	}
	body := map[string]any{
		"model": config.ModelID,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a strict JSON-only model fingerprinting expert. Respond with exactly one JSON object and nothing else.",
			},
			{"role": "user", "content": buildJudgeIdentityPrompt(responses)},
		},
		"stream":      false,
		"max_tokens":  256,
		"temperature": 0,
	}
	payload, _ := common.Marshal(body)
	headers := map[string]string{
		"Authorization": "Bearer " + config.APIKey,
		"Content-Type":  "application/json",
	}
	resp, err := executor.Do(ctx, "POST", endpointWithSuffix(config.BaseURL, "/chat/completions"), headers, payload)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil
	}
	var decoded map[string]any
	if err := common.Unmarshal(resp.Body, &decoded); err != nil {
		return nil, nil
	}
	result := parseJudgeIdentityResult(extractAssistantContent(decoded))
	if result == nil {
		return nil, nil
	}
	candidate := IdentityCandidateSummary{
		Family:  result.Family,
		Model:   identityFamilyDisplayName(result.Family),
		Score:   sourceCandidateScore(result.Confidence),
		Reasons: firstNStrings(result.Reasons, 5),
	}
	return []IdentityCandidateSummary{candidate}, result
}

func buildJudgeIdentityPrompt(responses map[string]string) string {
	keys := sortedStringKeys(responses)
	sections := make([]string, 0, len(keys))
	for _, key := range keys {
		text := truncateRunes(responses[key], 600)
		sections = append(sections, "["+key+"]\n"+text)
	}
	return "You are a model fingerprinting expert. Analyze the following AI assistant probe responses and identify which model family produced them.\n\n" +
		"Known families: anthropic, openai, google, qwen, meta, mistral, deepseek\n\n" +
		"Look for: self-identification claims, refusal phrasing, writing style, formatting preferences (bold headers, numbered lists, emoji), JSON discipline, and reasoning patterns.\n\n" +
		"PROBE RESPONSES:\n" + strings.Join(sections, "\n\n---\n\n") + "\n\n" +
		`Reply with ONLY a JSON object:
{"family": "<family>", "confidence": <0.0-1.0>, "reasons": ["<evidence 1>", "<evidence 2>", "<evidence 3>"]}`
}

func parseJudgeIdentityResult(text string) *judgeIdentityResult {
	candidates := make([]string, 0, 3)
	if fenced := firstRegexSubmatch(text, "(?s)```(?:json)?\\s*(.*?)```"); fenced != "" {
		candidates = append(candidates, strings.TrimSpace(fenced))
	}
	if braced := firstRegexSubmatch(text, `(?s)(\{.*"family".*\})`); braced != "" {
		candidates = append(candidates, strings.TrimSpace(braced))
	}
	candidates = append(candidates, strings.TrimSpace(text))

	for _, raw := range candidates {
		var parsed struct {
			Family     string   `json:"family"`
			Confidence float64  `json:"confidence"`
			Reasons    []string `json:"reasons"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			continue
		}
		family := strings.ToLower(strings.TrimSpace(parsed.Family))
		if !knownIdentityFamily(family) {
			continue
		}
		return &judgeIdentityResult{
			Family:     family,
			Confidence: sourceCandidateScore(parsed.Confidence),
			Reasons:    firstNStrings(parsed.Reasons, 5),
		}
	}
	return nil
}

func knownIdentityFamily(family string) bool {
	switch family {
	case "anthropic", "openai", "google", "qwen", "meta", "mistral", "deepseek":
		return true
	default:
		return false
	}
}
