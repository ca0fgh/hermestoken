package token_verifier

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

type probeJudgeScore struct {
	Score  int
	Reason string
}

type probeJudgeResult struct {
	Passed *bool
	Reason string
}

func parseProbeJudgeScore(text string) *probeJudgeScore {
	candidates := make([]string, 0, 3)
	if fenced := firstRegexSubmatch(text, "(?s)```(?:json)?\\s*(.*?)```"); fenced != "" {
		candidates = append(candidates, strings.TrimSpace(fenced))
	}
	if braced := firstRegexSubmatch(text, `(?s)(\{.*?"score".*?\})`); braced != "" {
		candidates = append(candidates, strings.TrimSpace(braced))
	}
	candidates = append(candidates, strings.TrimSpace(text))

	for _, raw := range candidates {
		var parsed map[string]any
		if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
			continue
		}
		score, ok := probeJudgeScoreFromAny(parsed["score"])
		if !ok || score < 1 || score > 10 {
			continue
		}
		reason := ""
		if parsed["reason"] != nil {
			reason = fmt.Sprint(parsed["reason"])
		}
		return &probeJudgeScore{
			Score:  int(math.Round(score)),
			Reason: reason,
		}
	}
	return nil
}

func probeJudgeScoreFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed)
	case float32:
		score := float64(typed)
		return score, !math.IsNaN(score)
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		score, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return score, err == nil && !math.IsNaN(score)
	default:
		return 0, false
	}
}

func runProbeJudgeWithBaseline(ctx context.Context, executor *CurlExecutor, config ProbeJudgeConfig, probe verifierProbe, candidateResponse string, baselineResponse string) probeJudgeResult {
	if executor == nil {
		executor = NewCurlExecutor(0)
	}
	threshold := config.Threshold
	if threshold <= 0 {
		threshold = 7
	}
	judgePrompt := `You are evaluating whether two AI responses are from the same underlying model.

Original probe question: "` + sourceJSStringPrefix(probe.Prompt, 300) + `"

Baseline response (from official API):
` + sourceJSStringPrefix(baselineResponse, 2000) + `

Candidate response (under test):
` + sourceJSStringPrefix(candidateResponse, 2000) + `

Score how similar these responses are on a scale of 1-10, where:
- 10: Identical or near-identical content, style, and capability level
- 7-9: Very similar, minor variations in phrasing
- 4-6: Somewhat similar, possible different capability level or style
- 1-3: Very different, likely a different model family or capability tier

Respond ONLY with valid JSON: {"score": <number 1-10>, "reason": "<one sentence>"}`

	body := map[string]any{
		"model": config.ModelID,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a strict JSON-only evaluator. Respond ONLY with the JSON object requested.",
			},
			{"role": "user", "content": judgePrompt},
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
	if err != nil {
		return probeJudgeResult{Reason: sanitizeProbeJudgeReason(config, "Judge call failed: "+sourceJSStringPrefix(err.Error(), 100))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return probeJudgeResult{Reason: fmt.Sprintf("Judge API error: HTTP %d", resp.StatusCode)}
	}
	var decoded map[string]any
	if err := common.Unmarshal(resp.Body, &decoded); err != nil {
		return probeJudgeResult{Reason: sanitizeProbeJudgeReason(config, "Judge returned unparseable response: "+sourceJSStringPrefix(string(resp.Body), 100))}
	}
	text := extractAssistantContent(decoded)
	scored := parseProbeJudgeScore(text)
	if scored == nil {
		return probeJudgeResult{Reason: sanitizeProbeJudgeReason(config, "Judge returned unparseable response: "+sourceJSStringPrefix(text, 100))}
	}
	passed := scored.Score >= threshold
	return probeJudgeResult{
		Passed: &passed,
		Reason: sanitizeProbeJudgeReason(config, fmt.Sprintf("Similarity score: %d/10 (threshold: %d) — %s", scored.Score, threshold, scored.Reason)),
	}
}

func sanitizeProbeJudgeReason(config ProbeJudgeConfig, reason string) string {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return reason
	}
	return strings.ReplaceAll(reason, apiKey, "[REDACTED]")
}

func probeJudgeConfigFromEnv() *ProbeJudgeConfig {
	config := &ProbeJudgeConfig{
		BaseURL: strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_PROBE_JUDGE_BASE_URL")),
		APIKey:  strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_PROBE_JUDGE_API_KEY")),
		ModelID: strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_PROBE_JUDGE_MODEL")),
	}
	if config.BaseURL == "" || config.APIKey == "" || config.ModelID == "" {
		return nil
	}
	if threshold, err := strconv.Atoi(strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_PROBE_JUDGE_THRESHOLD"))); err == nil {
		config.Threshold = threshold
	}
	return config
}

func probeBaselineFromEnv() BaselineMap {
	if raw := strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_PROBE_BASELINE_JSON")); raw != "" {
		return parseProbeBaselineJSON([]byte(raw))
	}
	if path := strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_PROBE_BASELINE_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		return parseProbeBaselineJSON(data)
	}
	return nil
}

func parseProbeBaselineJSON(data []byte) BaselineMap {
	var raw struct {
		Probes []struct {
			ProbeID      string `json:"probeId"`
			ResponseText string `json:"responseText"`
		} `json:"probes"`
	}
	if err := common.Unmarshal(data, &raw); err == nil && len(raw.Probes) > 0 {
		out := make(BaselineMap, len(raw.Probes))
		for _, probe := range raw.Probes {
			probeID := strings.TrimSpace(probe.ProbeID)
			if probeID == "" {
				continue
			}
			out[probeID] = probe.ResponseText
		}
		if len(out) > 0 {
			return out
		}
	}

	var direct map[string]string
	if err := common.Unmarshal(data, &direct); err != nil || len(direct) == 0 {
		return nil
	}
	out := make(BaselineMap, len(direct))
	for probeID, response := range direct {
		probeID = strings.TrimSpace(probeID)
		if probeID != "" {
			out[probeID] = response
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
