package token_verifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	defaultVerifierModel       = "gpt-4o-mini"
	defaultAnthropicModel      = "claude-3-5-haiku-latest"
	defaultVerifierHTTPTimeout = 40 * time.Second

	reproducibilitySeed   = 42
	reproducibilityPrompt = "Reply with this exact string and nothing else: STABLE_PING_9F3"
)

type Runner struct {
	BaseURL   string
	Token     string
	Models    []string
	Providers []string
	Executor  *CurlExecutor
}

func RunTask(ctx context.Context, taskID int64) error {
	task, err := model.GetTokenVerificationTaskByID(taskID)
	if err != nil {
		return err
	}
	token, err := model.GetTokenByIds(task.TokenID, task.UserID)
	if err != nil {
		_ = model.FailTokenVerificationTask(taskID, err.Error())
		return err
	}
	if err := model.UpdateTokenVerificationTaskRunning(taskID); err != nil {
		_ = model.FailTokenVerificationTask(taskID, err.Error())
		return err
	}

	runner := Runner{
		BaseURL:   resolveBaseURL(),
		Token:     token.GetFullKey(),
		Models:    resolveModels(task.GetModels(), token),
		Providers: resolveProviders(task.GetProviders()),
		Executor:  NewCurlExecutor(defaultVerifierHTTPTimeout),
	}
	results, err := runner.Run(ctx)
	if len(results) > 0 {
		if saveErr := model.AddTokenVerificationResults(toModelResults(taskID, results)); saveErr != nil && err == nil {
			err = saveErr
		}
	}
	if err != nil {
		_ = model.FailTokenVerificationTask(taskID, err.Error())
		return err
	}

	report := BuildReport(results)
	summaryBytes, _ := common.Marshal(report)
	if err := model.UpsertTokenVerificationReport(&model.TokenVerificationReport{
		TaskID:         taskID,
		UserID:         task.UserID,
		Summary:        string(summaryBytes),
		ScoringVersion: report.ScoringVersion,
		BaselineSource: report.BaselineSource,
	}); err != nil {
		_ = model.FailTokenVerificationTask(taskID, err.Error())
		return err
	}
	return model.CompleteTokenVerificationTask(taskID, report.Score, report.Grade)
}

func (r Runner) Run(ctx context.Context) ([]CheckResult, error) {
	if strings.TrimSpace(r.BaseURL) == "" {
		return nil, errors.New("token verifier base url is empty")
	}
	if strings.TrimSpace(r.Token) == "" {
		return nil, errors.New("token is empty")
	}
	models := normalizeModels(r.Models)
	if len(models) == 0 {
		models = []string{defaultVerifierModel}
	}
	executor := r.Executor
	if executor == nil {
		executor = NewCurlExecutor(defaultVerifierHTTPTimeout)
	}

	providers := resolveProviders(r.Providers)
	results := make([]CheckResult, 0, len(providers)*(2+len(models)*4))
	for _, provider := range providers {
		providerModels := models
		if len(providerModels) == 1 && provider == ProviderAnthropic && providerModels[0] == defaultVerifierModel {
			providerModels = []string{defaultAnthropicModel}
		}
		results = append(results, withProvider(r.checkModelsList(ctx, executor, provider), provider))
		for i, modelName := range providerModels {
			chatResult := r.checkModelAccess(ctx, executor, provider, modelName)
			if i == 0 {
				availability := chatResult
				availability.CheckKey = CheckAvailability
				results = append(results, availability)
			}
			results = append(results, chatResult)
			results = append(results, buildModelIdentityResult(provider, modelName, chatResult))
			results = append(results, withProvider(r.checkStream(ctx, executor, provider, modelName), provider))
			results = append(results, r.checkJSON(ctx, executor, provider, modelName))
			results = append(results, r.checkReproducibility(ctx, executor, provider, modelName))
		}
	}
	return results, nil
}

func (r Runner) checkModelsList(ctx context.Context, executor *CurlExecutor, provider string) CheckResult {
	resp, err := executor.Do(ctx, "GET", r.endpoint("/v1/models"), providerHeaders(provider, r.Token), nil)
	if err != nil {
		return failedResult(CheckModelsList, "", err, 0)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpFailedResult(CheckModelsList, "", resp.StatusCode, resp.Body, resp.LatencyMs)
	}
	var decoded map[string]any
	_ = common.Unmarshal(resp.Body, &decoded)
	return CheckResult{
		CheckKey:  CheckModelsList,
		Success:   true,
		Score:     100,
		LatencyMs: resp.LatencyMs,
		TTFTMs:    resp.TTFTMs,
		Message:   "模型列表接口可访问",
		Raw:       compactRaw(decoded),
	}
}

func (r Runner) checkModelAccess(ctx context.Context, executor *CurlExecutor, provider string, modelName string) CheckResult {
	switch provider {
	case ProviderAnthropic:
		return withProvider(r.checkAnthropicMessage(ctx, executor, modelName, CheckModelAccess, "Reply with exactly: ok", false), provider)
	default:
		return withProvider(r.checkChat(ctx, executor, modelName, CheckModelAccess, "Reply with exactly: ok", false, false), ProviderOpenAI)
	}
}

func (r Runner) checkJSON(ctx context.Context, executor *CurlExecutor, provider string, modelName string) CheckResult {
	switch provider {
	case ProviderAnthropic:
		return withProvider(r.checkAnthropicMessage(ctx, executor, modelName, CheckJSON, "Return a valid JSON object with one boolean field named ok.", false), provider)
	default:
		return withProvider(r.checkChat(ctx, executor, modelName, CheckJSON, "Return a valid JSON object with one boolean field named ok.", false, true), ProviderOpenAI)
	}
}

func (r Runner) checkChat(ctx context.Context, executor *CurlExecutor, modelName string, checkKey CheckKey, prompt string, stream bool, jsonMode bool) CheckResult {
	body := map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 64,
		"stream":     stream,
	}
	if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	payload, _ := common.Marshal(body)
	headers := providerHeaders(ProviderOpenAI, r.Token)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", r.endpoint("/v1/chat/completions"), headers, payload)
	if err != nil {
		return failedResult(checkKey, modelName, err, 0)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpFailedResult(checkKey, modelName, resp.StatusCode, resp.Body, resp.LatencyMs)
	}
	var decoded map[string]any
	if err := common.Unmarshal(resp.Body, &decoded); err != nil {
		return CheckResult{
			CheckKey:  checkKey,
			ModelName: modelName,
			Success:   false,
			Score:     0,
			LatencyMs: resp.LatencyMs,
			TTFTMs:    resp.TTFTMs,
			ErrorCode: "invalid_json_response",
			Message:   err.Error(),
		}
	}
	if message := extractErrorMessage(decoded); message != "" {
		return CheckResult{
			CheckKey:  checkKey,
			ModelName: modelName,
			Success:   false,
			Score:     0,
			LatencyMs: resp.LatencyMs,
			TTFTMs:    resp.TTFTMs,
			ErrorCode: "upstream_error",
			Message:   message,
			Raw:       compactRaw(decoded),
		}
	}
	return CheckResult{
		CheckKey:      checkKey,
		ModelName:     modelName,
		ClaimedModel:  modelName,
		ObservedModel: extractObservedModel(decoded),
		Success:       true,
		Score:         100,
		LatencyMs:     resp.LatencyMs,
		TTFTMs:        resp.TTFTMs,
		Message:       "请求成功",
		Raw:           compactRaw(decoded),
	}
}

func (r Runner) checkAnthropicMessage(ctx context.Context, executor *CurlExecutor, modelName string, checkKey CheckKey, prompt string, stream bool) CheckResult {
	body := map[string]any{
		"model":      modelName,
		"max_tokens": 64,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	if stream {
		body["stream"] = true
	}
	payload, _ := common.Marshal(body)
	headers := providerHeaders(ProviderAnthropic, r.Token)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", r.endpoint("/v1/messages"), headers, payload)
	if err != nil {
		return failedResult(checkKey, modelName, err, 0)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpFailedResult(checkKey, modelName, resp.StatusCode, resp.Body, resp.LatencyMs)
	}
	var decoded map[string]any
	if err := common.Unmarshal(resp.Body, &decoded); err != nil {
		return CheckResult{
			CheckKey:  checkKey,
			ModelName: modelName,
			Success:   false,
			Score:     0,
			LatencyMs: resp.LatencyMs,
			TTFTMs:    resp.TTFTMs,
			ErrorCode: "invalid_json_response",
			Message:   err.Error(),
		}
	}
	if message := extractErrorMessage(decoded); message != "" {
		return CheckResult{
			CheckKey:  checkKey,
			ModelName: modelName,
			Success:   false,
			Score:     0,
			LatencyMs: resp.LatencyMs,
			TTFTMs:    resp.TTFTMs,
			ErrorCode: "upstream_error",
			Message:   message,
			Raw:       compactRaw(decoded),
		}
	}
	return CheckResult{
		CheckKey:      checkKey,
		ModelName:     modelName,
		ClaimedModel:  modelName,
		ObservedModel: extractObservedModel(decoded),
		Success:       true,
		Score:         100,
		LatencyMs:     resp.LatencyMs,
		TTFTMs:        resp.TTFTMs,
		Message:       "请求成功",
		Raw:           compactRaw(decoded),
	}
}

func (r Runner) checkStream(ctx context.Context, executor *CurlExecutor, provider string, modelName string) CheckResult {
	if provider == ProviderAnthropic {
		return r.checkAnthropicStream(ctx, executor, modelName)
	}
	body := map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "Count from 1 to 20 separated by spaces."},
		},
		"max_tokens": 64,
		"stream":     true,
	}
	payload, _ := common.Marshal(body)
	headers := providerHeaders(ProviderOpenAI, r.Token)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", r.endpoint("/v1/chat/completions"), headers, payload)
	if err != nil {
		return failedResult(CheckStream, modelName, err, 0)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpFailedResult(CheckStream, modelName, resp.StatusCode, resp.Body, resp.LatencyMs)
	}
	return analyzeStreamBody(resp.Body, modelName, resp.TTFTMs, resp.LatencyMs)
}

func (r Runner) checkAnthropicStream(ctx context.Context, executor *CurlExecutor, modelName string) CheckResult {
	body := map[string]any{
		"model":      modelName,
		"max_tokens": 64,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": "Count from 1 to 20 separated by spaces."},
		},
	}
	payload, _ := common.Marshal(body)
	headers := providerHeaders(ProviderAnthropic, r.Token)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", r.endpoint("/v1/messages"), headers, payload)
	if err != nil {
		return failedResult(CheckStream, modelName, err, 0)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpFailedResult(CheckStream, modelName, resp.StatusCode, resp.Body, resp.LatencyMs)
	}
	return analyzeStreamBody(resp.Body, modelName, resp.TTFTMs, resp.LatencyMs)
}

func (r Runner) endpoint(path string) string {
	base := strings.TrimRight(strings.TrimSpace(r.BaseURL), "/")
	if base == "" {
		return path
	}
	return base + path
}

func resolveBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_BASE_URL")); value != "" {
		return value
	}
	if value := strings.TrimSpace(system_setting.ServerAddress); value != "" {
		return value
	}
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "3000"
	}
	return "http://127.0.0.1:" + port
}

func resolveModels(taskModels []string, token *model.Token) []string {
	models := normalizeModels(taskModels)
	if len(models) > 0 {
		return models
	}
	if token != nil && token.ModelLimitsEnabled {
		models = normalizeModels(strings.Split(token.ModelLimits, ","))
		if len(models) > 0 {
			return models[:min(len(models), 5)]
		}
	}
	return []string{defaultVerifierModel}
}

func resolveProviders(providers []string) []string {
	seen := make(map[string]struct{}, len(providers))
	out := make([]string, 0, len(providers))
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider != ProviderOpenAI && provider != ProviderAnthropic {
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		out = append(out, provider)
	}
	if len(out) == 0 {
		return []string{ProviderOpenAI}
	}
	return out
}

func normalizeModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		out = append(out, modelName)
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func buildModelIdentityResult(provider string, claimedModel string, accessResult CheckResult) CheckResult {
	observedModel := strings.TrimSpace(accessResult.ObservedModel)
	if observedModel == "" && accessResult.Raw != nil {
		if value, ok := accessResult.Raw["model"].(string); ok {
			observedModel = strings.TrimSpace(value)
		}
	}
	if !accessResult.Success {
		return CheckResult{
			Provider:           provider,
			CheckKey:           CheckModelIdentity,
			ModelName:          claimedModel,
			ClaimedModel:       claimedModel,
			ObservedModel:      observedModel,
			IdentityConfidence: 0,
			Success:            false,
			Score:              0,
			ErrorCode:          "model_access_failed",
			Message:            "模型请求失败，无法判断真实模型身份",
		}
	}
	confidence, suspectedDowngrade, message := evaluateModelIdentity(provider, claimedModel, observedModel)
	return CheckResult{
		Provider:           provider,
		CheckKey:           CheckModelIdentity,
		ModelName:          claimedModel,
		ClaimedModel:       claimedModel,
		ObservedModel:      observedModel,
		IdentityConfidence: confidence,
		SuspectedDowngrade: suspectedDowngrade,
		Success:            confidence >= 70 && !suspectedDowngrade,
		Score:              confidence,
		Message:            message,
		Raw: map[string]any{
			"claimed_model":       claimedModel,
			"observed_model":      observedModel,
			"identity_confidence": confidence,
			"suspected_downgrade": suspectedDowngrade,
			"identity_method":     "response_model_consistency",
		},
	}
}

func evaluateModelIdentity(provider string, claimedModel string, observedModel string) (int, bool, string) {
	rawClaimed := normalizeRawModelName(claimedModel)
	rawObserved := normalizeRawModelName(observedModel)
	claimed := canonicalModelName(claimedModel)
	observed := canonicalModelName(observedModel)
	if claimed == "" {
		return 0, false, "缺少声明模型，无法判断模型身份"
	}
	if observed == "" {
		return 50, false, "响应未返回模型名，只能确认请求成功，无法确认真实模型身份"
	}
	if rawClaimed == rawObserved {
		return 95, false, "响应模型名与声明模型一致"
	}
	if claimed == observed || modelAliasMatches(claimed, observed) {
		return 90, false, "响应模型名与声明模型属于同一官方别名或日期版本"
	}

	claimedFamily, claimedTier := modelFamilyAndTier(provider, claimed)
	observedFamily, observedTier := modelFamilyAndTier(provider, observed)
	if claimedFamily != "" && claimedFamily == observedFamily {
		if observedTier > 0 && claimedTier > 0 && observedTier < claimedTier {
			return 35, true, "响应模型低于声明模型档位，疑似被降级"
		}
		if observedTier == claimedTier {
			return 80, false, "响应模型与声明模型属于同系列同档位，存在命名差异"
		}
		if observedTier > claimedTier {
			return 85, false, "响应模型档位不低于声明模型，存在命名差异"
		}
	}
	return 25, true, "响应模型名与声明模型不一致，存在身份不符风险"
}

func normalizeRawModelName(modelName string) string {
	value := strings.ToLower(strings.TrimSpace(modelName))
	return strings.ReplaceAll(value, "_", "-")
}

func canonicalModelName(modelName string) string {
	value := normalizeRawModelName(modelName)
	value = strings.TrimSuffix(value, "-latest")
	value = strings.TrimSuffix(value, "-preview")
	parts := strings.Split(value, "-")
	for len(parts) > 0 {
		switch {
		case hasCompactDateSuffix(parts):
			parts = parts[:len(parts)-1]
		case hasDashedDateSuffix(parts):
			parts = parts[:len(parts)-3]
		default:
			return strings.Join(parts, "-")
		}
	}
	return strings.Join(parts, "-")
}

func hasCompactDateSuffix(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	return len(last) == 8 && allDigits(last)
}

func hasDashedDateSuffix(parts []string) bool {
	if len(parts) < 3 {
		return false
	}
	a := parts[len(parts)-3]
	b := parts[len(parts)-2]
	c := parts[len(parts)-1]
	return len(a) == 4 && len(b) == 2 && len(c) == 2 && allDigits(a) && allDigits(b) && allDigits(c)
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func modelAliasMatches(claimed string, observed string) bool {
	if claimed == observed {
		return true
	}
	aliasGroups := [][]string{
		{"gpt-4o-mini", "chatgpt-4o-mini"},
		{"gpt-4o", "chatgpt-4o"},
		{"claude-3-5-haiku", "claude-3.5-haiku"},
		{"claude-3-5-sonnet", "claude-3.5-sonnet"},
		{"claude-3-7-sonnet", "claude-3.7-sonnet"},
	}
	for _, group := range aliasGroups {
		claimedHit := false
		observedHit := false
		for _, alias := range group {
			if claimed == alias {
				claimedHit = true
			}
			if observed == alias {
				observedHit = true
			}
		}
		if claimedHit && observedHit {
			return true
		}
	}
	return false
}

func modelFamilyAndTier(provider string, modelName string) (string, int) {
	value := canonicalModelName(modelName)
	if provider == ProviderAnthropic || strings.HasPrefix(value, "claude-") {
		switch {
		case strings.Contains(value, "opus"):
			return "claude", 40
		case strings.Contains(value, "sonnet"):
			return "claude", 30
		case strings.Contains(value, "haiku"):
			return "claude", 20
		default:
			return "claude", 10
		}
	}
	switch {
	case strings.Contains(value, "gpt-5"):
		return "openai", 50
	case strings.Contains(value, "gpt-4.5"):
		return "openai", 45
	case strings.Contains(value, "gpt-4.1"):
		return "openai", 42
	case strings.Contains(value, "gpt-4o") && strings.Contains(value, "mini"):
		return "openai", 32
	case strings.Contains(value, "gpt-4o") && strings.Contains(value, "nano"):
		return "openai", 28
	case strings.Contains(value, "gpt-4o"):
		return "openai", 38
	case strings.Contains(value, "gpt-4") && strings.Contains(value, "mini"):
		return "openai", 30
	case strings.Contains(value, "gpt-4"):
		return "openai", 36
	case strings.Contains(value, "gpt-3.5"):
		return "openai", 15
	default:
		return "", 0
	}
}

func toModelResults(taskID int64, results []CheckResult) []*model.TokenVerificationResult {
	out := make([]*model.TokenVerificationResult, 0, len(results))
	for _, result := range results {
		rawBytes, _ := common.Marshal(result.Raw)
		out = append(out, &model.TokenVerificationResult{
			TaskID:             taskID,
			Provider:           result.Provider,
			CheckKey:           string(result.CheckKey),
			ModelName:          result.ModelName,
			ClaimedModel:       result.ClaimedModel,
			ObservedModel:      result.ObservedModel,
			IdentityConfidence: result.IdentityConfidence,
			SuspectedDowngrade: result.SuspectedDowngrade,
			Success:            result.Success,
			Score:              result.Score,
			LatencyMs:          result.LatencyMs,
			TTFTMs:             result.TTFTMs,
			TokensPS:           result.TokensPS,
			ErrorCode:          result.ErrorCode,
			Message:            result.Message,
			Raw:                string(rawBytes),
		})
	}
	return out
}

func withProvider(result CheckResult, provider string) CheckResult {
	result.Provider = provider
	return result
}

func providerHeaders(provider string, token string) map[string]string {
	headers := make(map[string]string)
	switch provider {
	case ProviderAnthropic:
		headers["x-api-key"] = token
		headers["anthropic-version"] = "2023-06-01"
	default:
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

func analyzeStreamBody(body []byte, modelName string, ttftMs int64, latencyMs int64) CheckResult {
	outputChars := 0
	for _, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		outputChars += len(payload)
		if message := extractErrorMessageFromBytes([]byte(payload)); message != "" {
			return CheckResult{
				CheckKey:  CheckStream,
				ModelName: modelName,
				Success:   false,
				LatencyMs: latencyMs,
				TTFTMs:    ttftMs,
				ErrorCode: "stream_error",
				Message:   message,
			}
		}
	}
	if outputChars == 0 {
		return CheckResult{
			CheckKey:  CheckStream,
			ModelName: modelName,
			Success:   false,
			LatencyMs: latencyMs,
			TTFTMs:    ttftMs,
			ErrorCode: "empty_stream",
			Message:   "stream response has no data chunks",
		}
	}
	tokensPS := 0.0
	if latencyMs > 0 {
		estimatedTokens := float64(outputChars) / 4.0
		tokensPS = estimatedTokens / (float64(latencyMs) / 1000.0)
	}
	return CheckResult{
		CheckKey:  CheckStream,
		ModelName: modelName,
		Success:   true,
		Score:     100,
		LatencyMs: latencyMs,
		TTFTMs:    ttftMs,
		TokensPS:  tokensPS,
		Message:   "流式输出可用",
	}
}

func failedResult(checkKey CheckKey, modelName string, err error, latencyMs int64) CheckResult {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return CheckResult{
		CheckKey:  checkKey,
		ModelName: modelName,
		Success:   false,
		Score:     0,
		LatencyMs: latencyMs,
		ErrorCode: "request_failed",
		Message:   message,
	}
}

func httpFailedResult(checkKey CheckKey, modelName string, statusCode int, body []byte, latencyMs int64) CheckResult {
	message := extractErrorMessageFromBytes(body)
	if message == "" {
		message = fmt.Sprintf("HTTP %d", statusCode)
	}
	return CheckResult{
		CheckKey:  checkKey,
		ModelName: modelName,
		Success:   false,
		Score:     0,
		LatencyMs: latencyMs,
		ErrorCode: fmt.Sprintf("http_%d", statusCode),
		Message:   message,
	}
}

func extractErrorMessageFromBytes(body []byte) string {
	var decoded map[string]any
	if err := common.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	return extractErrorMessage(decoded)
}

func extractObservedModel(decoded map[string]any) string {
	if decoded == nil {
		return ""
	}
	if value, ok := decoded["model"].(string); ok {
		return strings.TrimSpace(value)
	}
	if value, ok := decoded["model_name"].(string); ok {
		return strings.TrimSpace(value)
	}
	if value, ok := decoded["id"].(string); ok && strings.HasPrefix(strings.ToLower(value), "model-") {
		return strings.TrimSpace(value)
	}
	return ""
}

func extractErrorMessage(decoded map[string]any) string {
	if decoded == nil {
		return ""
	}
	errValue, ok := decoded["error"]
	if !ok || errValue == nil {
		return ""
	}
	switch value := errValue.(type) {
	case string:
		return value
	case map[string]any:
		if message, ok := value["message"].(string); ok && message != "" {
			return message
		}
		if nested, ok := value["error"].(map[string]any); ok {
			if message, ok := nested["message"].(string); ok && message != "" {
				return message
			}
		}
	}
	data, _ := common.Marshal(errValue)
	return string(data)
}

func compactRaw(decoded map[string]any) map[string]any {
	if decoded == nil {
		return nil
	}
	out := make(map[string]any)
	for _, key := range []string{"id", "object", "model", "usage"} {
		if value, ok := decoded[key]; ok {
			out[key] = value
		}
	}
	if data, ok := decoded["data"].([]any); ok {
		out["data_count"] = len(data)
		sample := make([]any, 0, min(len(data), 5))
		for i := 0; i < len(data) && i < 5; i++ {
			if item, ok := data[i].(map[string]any); ok {
				if id, ok := item["id"]; ok {
					sample = append(sample, map[string]any{"id": id})
					continue
				}
			}
			sample = append(sample, data[i])
		}
		out["data_sample"] = sample
	}
	if choices, ok := decoded["choices"].([]any); ok {
		out["choices_count"] = len(choices)
	}
	return out
}

func ValidateBaseURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("base url is empty")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("base url must start with http or https")
	}
	if parsed.Host == "" {
		return errors.New("base url host is empty")
	}
	return nil
}

// checkReproducibility issues two identical seeded probes against the same model and
// returns a CheckResult signalling whether the upstream produced the same response twice.
// Anthropic is skipped (Messages API does not expose a `seed` parameter).
func (r Runner) checkReproducibility(ctx context.Context, executor *CurlExecutor, provider string, modelName string) CheckResult {
	if provider == ProviderAnthropic {
		return CheckResult{
			Provider:  provider,
			CheckKey:  CheckReproducibility,
			ModelName: modelName,
			Skipped:   true,
			Success:   true,
			Score:     0,
			ErrorCode: "skipped",
			Message:   "Anthropic Messages API 不支持 seed 参数，跳过复现性检查",
		}
	}

	fp1, hash1, err := r.runSeededProbe(ctx, executor, modelName)
	if err != nil {
		return failedResult(CheckReproducibility, modelName, err, 0)
	}
	fp2, hash2, err := r.runSeededProbe(ctx, executor, modelName)
	if err != nil {
		return failedResult(CheckReproducibility, modelName, err, 0)
	}

	consistent, method := decideReproducibility(fp1, fp2, hash1, hash2)
	score := 100
	message := "两次相同 seed/temperature=0 请求结果一致"
	switch {
	case consistent && method == ConsistencyMethodSystemFingerprint:
		message = "两次请求 system_fingerprint 一致"
	case consistent && method == ConsistencyMethodContentHash:
		message = "system_fingerprint 缺失，但两次响应内容哈希一致"
	case !consistent && method == ConsistencyMethodSystemFingerprintChanged:
		score = 30
		message = "两次请求 system_fingerprint 不一致，疑似路由或模型变更"
	case !consistent && method == ConsistencyMethodContentDiverged:
		score = 30
		message = "两次响应内容不一致"
	case !consistent && method == ConsistencyMethodInsufficientData:
		score = 50
		message = "复现性数据不足，无法判定（无 system_fingerprint 且响应为空）"
	}

	return CheckResult{
		Provider:          provider,
		CheckKey:          CheckReproducibility,
		ModelName:         modelName,
		Consistent:        consistent,
		ConsistencyMethod: method,
		Success:           consistent,
		Score:             score,
		Message:           message,
		Raw: map[string]any{
			"fingerprint_1":  fp1,
			"fingerprint_2":  fp2,
			"content_hash_1": hash1,
			"content_hash_2": hash2,
			"method":         method,
			"prompt":         reproducibilityPrompt,
			"seed":           reproducibilitySeed,
		},
	}
}

// runSeededProbe sends a single non-stream chat completion with temperature=0 and a fixed seed,
// returning system_fingerprint (when present) and a sha256 hex of the assistant content.
func (r Runner) runSeededProbe(ctx context.Context, executor *CurlExecutor, modelName string) (string, string, error) {
	body := map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": reproducibilityPrompt},
		},
		"max_tokens":  32,
		"temperature": 0,
		"seed":        reproducibilitySeed,
		"stream":      false,
	}
	payload, _ := common.Marshal(body)
	headers := providerHeaders(ProviderOpenAI, r.Token)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", r.endpoint("/v1/chat/completions"), headers, payload)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(resp.Body), 128))
	}
	var decoded map[string]any
	if err := common.Unmarshal(resp.Body, &decoded); err != nil {
		return "", "", err
	}
	fingerprint, _ := decoded["system_fingerprint"].(string)
	content := extractAssistantContent(decoded)
	hash := ""
	if strings.TrimSpace(content) != "" {
		hash = sha256Hex(content)
	}
	return strings.TrimSpace(fingerprint), hash, nil
}

// decideReproducibility picks the strongest available consistency signal.
// system_fingerprint matches/differences trump content hash because the fingerprint is
// emitted by the upstream and is hard to spoof at a relay layer.
func decideReproducibility(fp1, fp2, hash1, hash2 string) (bool, string) {
	switch {
	case fp1 != "" && fp2 != "" && fp1 == fp2:
		return true, ConsistencyMethodSystemFingerprint
	case fp1 != "" && fp2 != "" && fp1 != fp2:
		return false, ConsistencyMethodSystemFingerprintChanged
	case hash1 != "" && hash2 != "" && hash1 == hash2:
		return true, ConsistencyMethodContentHash
	case hash1 != "" && hash2 != "" && hash1 != hash2:
		return false, ConsistencyMethodContentDiverged
	default:
		return false, ConsistencyMethodInsufficientData
	}
}

func extractAssistantContent(decoded map[string]any) string {
	if decoded == nil {
		return ""
	}
	choices, ok := decoded["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	first, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	if message, ok := first["message"].(map[string]any); ok {
		if content, ok := message["content"].(string); ok {
			return content
		}
	}
	if content, ok := first["text"].(string); ok {
		return content
	}
	return ""
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func init() {
	if err := ValidateBaseURL(resolveBaseURL()); err != nil {
		common.SysLog("token verifier base url warning: " + err.Error())
	}
}
