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

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting/system_setting"
)

const (
	defaultVerifierModel       = "gpt-4o-mini"
	defaultAnthropicModel      = "claude-3-5-haiku-latest"
	defaultVerifierHTTPTimeout = 40 * time.Second
)

type Runner struct {
	BaseURL      string
	Token        string
	Models       []string
	Providers    []string
	ProbeProfile string
	Executor     *CurlExecutor
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
	results, err := runner.RunProbeSuiteOnly(ctx)
	if len(results) > 0 {
		if saveErr := model.AddTokenVerificationResults(toModelResults(taskID, results)); saveErr != nil && err == nil {
			err = saveErr
		}
	}
	if err != nil {
		_ = model.FailTokenVerificationTask(taskID, err.Error())
		return err
	}

	report := BuildProbeReportWithOptions(ctx, results, DefaultReportOptionsFromEnv())
	summaryBytes, _ := common.Marshal(report)
	if err := model.UpsertTokenVerificationReport(&model.TokenVerificationReport{
		TaskID:         taskID,
		UserID:         task.UserID,
		Summary:        string(summaryBytes),
		ScoringVersion: report.ScoringVersion,
	}); err != nil {
		_ = model.FailTokenVerificationTask(taskID, err.Error())
		return err
	}
	return model.CompleteTokenVerificationTask(taskID, report.Score, report.Grade)
}

func RunDirectProbe(ctx context.Context, input DirectProbeRequest) (*DirectProbeResponse, error) {
	providers := resolveProviders([]string{strings.TrimSpace(input.Provider)})
	models := normalizeModels([]string{strings.TrimSpace(input.Model)})
	if len(models) == 0 {
		models = []string{defaultVerifierModel}
	}
	apiKey := strings.TrimSpace(input.APIKey)
	runner := Runner{
		BaseURL:      strings.TrimSpace(input.BaseURL),
		Token:        apiKey,
		Models:       models,
		Providers:    providers,
		ProbeProfile: normalizeProbeProfile(input.ProbeProfile),
		Executor:     NewCurlExecutor(defaultVerifierHTTPTimeout),
	}
	results, err := runner.RunProbeSuiteOnly(ctx)
	if err != nil {
		return nil, err
	}
	return RedactDirectProbeResponse(&DirectProbeResponse{
		BaseURL:      runner.BaseURL,
		Provider:     providers[0],
		Model:        models[0],
		ProbeProfile: runner.ProbeProfile,
		Results:      results,
		Report:       BuildProbeReportWithOptions(ctx, results, DefaultReportOptionsFromEnv()),
	}, apiKey), nil
}

func (r Runner) RunDirectProbeSuite(ctx context.Context) ([]CheckResult, error) {
	return r.RunProbeSuiteOnly(ctx)
}

func (r Runner) RunProbeSuiteOnly(ctx context.Context) ([]CheckResult, error) {
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
	results := make([]CheckResult, 0, len(providers)*len(models)*16)
	for _, provider := range providers {
		providerModels := models
		if len(providerModels) == 1 && provider == ProviderAnthropic && providerModels[0] == defaultVerifierModel {
			providerModels = []string{defaultAnthropicModel}
		}
		for _, modelName := range providerModels {
			preflight, latencyMs, ok := r.checkProbePreflight(ctx, executor, provider, modelName)
			if ok && preflight.Outcome == preflightOutcomeAbort {
				results = append(results, failedDirectProbeSuiteResults(provider, modelName, normalizeProbeProfile(r.ProbeProfile), preflight, latencyMs)...)
				continue
			}
			results = append(results, r.runProbeSuite(ctx, executor, provider, modelName)...)
		}
	}
	return results, nil
}

func (r Runner) checkProbePreflight(ctx context.Context, executor *CurlExecutor, provider string, modelName string) (preflightResult, int64, bool) {
	if executor == nil {
		executor = NewCurlExecutor(defaultVerifierHTTPTimeout)
	}

	var target string
	var body map[string]any
	switch provider {
	case ProviderAnthropic:
		target = r.endpoint("/v1/messages")
		body = map[string]any{
			"model":      modelName,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
		}
	default:
		target = r.endpoint("/v1/chat/completions")
		body = map[string]any{
			"model": modelName,
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
			"max_tokens": 1,
			"stream":     false,
		}
	}

	payload, _ := common.Marshal(body)
	headers := providerHeaders(provider, r.Token)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", target, headers, payload)
	if err != nil {
		return preflightResult{Outcome: preflightOutcomeWarn, Code: "preflight_request_failed", Reason: err.Error()}, 0, false
	}
	return classifyPreflightResult(resp.StatusCode, resp.Body), resp.LatencyMs, true
}

func failedDirectProbeSuiteResults(provider string, modelName string, profile string, preflight preflightResult, latencyMs int64) []CheckResult {
	probes := directProbeSuiteDefinitions(profile)
	results := make([]CheckResult, 0, len(probes))
	message := preflight.Reason
	if strings.TrimSpace(message) == "" {
		message = "探针预检失败"
	}
	errorCode := preflight.Code
	if strings.TrimSpace(errorCode) == "" {
		errorCode = "preflight_failed"
	}
	for _, probe := range probes {
		results = append(results, CheckResult{
			Provider:  provider,
			Group:     probe.Group,
			CheckKey:  probe.Key,
			ModelName: modelName,
			Neutral:   probe.Neutral,
			Success:   false,
			Score:     0,
			LatencyMs: latencyMs,
			ErrorCode: errorCode,
			Message:   message,
		})
	}
	return results
}

func directProbeSuiteDefinitions(profile string) []verifierProbe {
	profile = normalizeProbeProfile(profile)
	probes := make([]verifierProbe, 0, len(verifierProbeSuite(profile))+3)
	if profile == ProbeProfileDeep || profile == ProbeProfileFull {
		probes = append(probes,
			verifierProbe{Key: CheckProbeChannelSignature, Group: probeGroupSecurity, Neutral: true},
			verifierProbe{Key: CheckProbeSSECompliance, Group: probeGroupIntegrity},
		)
	}
	if profile == ProbeProfileFull {
		probes = append(probes, verifierProbe{Key: CheckProbeSignatureRoundtrip, Group: probeGroupSignature, Neutral: true})
	}
	probes = append(probes, verifierProbeSuite(profile)...)
	return probes
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

func toModelResults(taskID int64, results []CheckResult) []*model.TokenVerificationResult {
	out := make([]*model.TokenVerificationResult, 0, len(results))
	for _, result := range results {
		rawBytes, _ := common.Marshal(result.Raw)
		out = append(out, &model.TokenVerificationResult{
			TaskID:       taskID,
			Provider:     result.Provider,
			Group:        result.Group,
			CheckKey:     string(result.CheckKey),
			ModelName:    result.ModelName,
			Neutral:      result.Neutral,
			Success:      result.Success,
			Score:        result.Score,
			LatencyMs:    result.LatencyMs,
			TTFTMs:       result.TTFTMs,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			TokensPS:     result.TokensPS,
			ErrorCode:    result.ErrorCode,
			Message:      result.Message,
			Raw:          string(rawBytes),
		})
	}
	return out
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
	preflight := classifyPreflightResult(statusCode, body)
	message := preflight.Reason
	if message == "" {
		message = extractErrorMessageFromBytes(body)
	}
	if message == "" {
		message = fmt.Sprintf("HTTP %d", statusCode)
	}
	errorCode := preflight.Code
	if errorCode == "" {
		errorCode = fmt.Sprintf("http_%d", statusCode)
	}
	return CheckResult{
		CheckKey:  checkKey,
		ModelName: modelName,
		Success:   false,
		Score:     0,
		LatencyMs: latencyMs,
		ErrorCode: errorCode,
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
