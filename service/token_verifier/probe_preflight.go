package token_verifier

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

const (
	preflightOutcomeOK    = "ok"
	preflightOutcomeAbort = "abort"
	preflightOutcomeWarn  = "warn"
)

type preflightResult struct {
	Outcome string
	Code    string
	Reason  string
}

func classifyPreflightResult(status int, rawBody []byte) preflightResult {
	if status >= 200 && status < 300 {
		return preflightResult{Outcome: preflightOutcomeOK}
	}
	code, message, errorType := extractPreflightError(rawBody)
	display := firstNonEmptyString(message, fmt.Sprintf("HTTP %d", status))

	switch status {
	case 401:
		return preflightResult{Outcome: preflightOutcomeAbort, Code: "authentication_failed", Reason: "认证失败（401）：" + display}
	case 403:
		return preflightResult{Outcome: preflightOutcomeAbort, Code: "permission_denied", Reason: "权限不足（403）：" + display}
	case 429:
		return preflightResult{Outcome: preflightOutcomeWarn, Code: "rate_limit", Reason: "Rate limit（429）：" + display}
	}

	if errorType != "" {
		switch errorType {
		case "authentication_error", "not_found_error", "permission_error", "invalid_request_error":
			return preflightResult{Outcome: preflightOutcomeAbort, Code: errorType, Reason: errorType + "：" + display}
		}
	}
	if isPreflightModelNotFound(code, message) {
		return preflightResult{Outcome: preflightOutcomeAbort, Code: "model_not_found", Reason: "模型不存在：" + display}
	}
	return preflightResult{Outcome: preflightOutcomeWarn, Code: fmt.Sprintf("http_%d", status), Reason: fmt.Sprintf("端点返回 %d：%s", status, display)}
}

func extractPreflightError(rawBody []byte) (string, string, string) {
	var decoded map[string]any
	if err := common.Unmarshal(rawBody, &decoded); err != nil {
		return "", strings.TrimSpace(truncate(string(rawBody), 200)), ""
	}
	code := ""
	message := ""
	errorType := ""
	if errorValue, ok := decoded["error"].(map[string]any); ok {
		if value, ok := errorValue["code"].(string); ok {
			code = value
		}
		if value, ok := errorValue["message"].(string); ok {
			message = value
		}
		if value, ok := errorValue["type"].(string); ok {
			errorType = value
		}
	}
	if decoded["type"] == "error" {
		if nested, ok := decoded["error"].(map[string]any); ok {
			if value, ok := nested["type"].(string); ok {
				errorType = value
			}
			if value, ok := nested["message"].(string); ok && message == "" {
				message = value
			}
		}
	}
	return code, message, errorType
}

func isPreflightModelNotFound(code string, message string) bool {
	haystack := strings.ToLower(code + " " + message)
	for _, pattern := range []string{"model_not_found", "no available channel", "model not found"} {
		if strings.Contains(haystack, pattern) {
			return true
		}
	}
	return regexp.MustCompile(`(?i)\bmodel\b.*\bnot\s+found\b`).MatchString(haystack)
}
