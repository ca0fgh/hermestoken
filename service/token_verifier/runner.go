package token_verifier

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting/system_setting"
	"github.com/cespare/xxhash/v2"
	"github.com/google/uuid"
)

const (
	defaultVerifierModel       = "gpt-4o-mini"
	defaultAnthropicModel      = "claude-3-5-haiku-latest"
	defaultVerifierHTTPTimeout = 40 * time.Second

	ClientProfileClaudeCode = "claude_code"

	claudeCodeCLIUserAgent                        = "claude-cli/2.1.92 (external, cli)"
	claudeCodeCLIVersion                          = "2.1.92"
	claudeCodeAnthropicBeta                       = "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,prompt-caching-scope-2026-01-05,effort-2025-11-24,redact-thinking-2026-02-12,context-management-2025-06-27,extended-cache-ttl-2025-04-11"
	claudeCodeSystemPrompt                        = "You are Claude Code, Anthropic's official CLI for Claude."
	claudeCodeMetadataDeviceIDLength              = 64
	claudeCodeMetadataDeviceIDAllowedChars        = "0123456789abcdef"
	claudeCodeFingerprintSalt                     = "59cf53e54c78"
	claudeCodeBillingCCHSeed               uint64 = 0x6E52736AC806831E
)

var claudeCodeCCHPlaceholderPattern = regexp.MustCompile(`(x-anthropic-billing-header:[^"]*?\bcch=)(00000)(;)`)

type Runner struct {
	BaseURL       string
	Token         string
	Models        []string
	Providers     []string
	ProbeProfile  string
	ClientProfile string
	sessionID     string
	ProbeJudge    *ProbeJudgeConfig
	ProbeBaseline BaselineMap
	Executor      *CurlExecutor
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
		BaseURL:       resolveBaseURL(),
		Token:         token.GetFullKey(),
		Models:        resolveModels(task.GetModels(), token),
		Providers:     resolveProviders(task.GetProviders()),
		ProbeJudge:    probeJudgeConfigFromEnv(),
		ProbeBaseline: probeBaselineFromEnv(),
		Executor:      NewCurlExecutor(defaultVerifierHTTPTimeout),
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
	clientProfile := normalizeClientProfile(input.ClientProfile)
	sessionID := ""
	if clientProfile == ClientProfileClaudeCode {
		sessionID = uuid.NewString()
	}
	runner := Runner{
		BaseURL:       strings.TrimSpace(input.BaseURL),
		Token:         apiKey,
		Models:        models,
		Providers:     providers,
		ProbeProfile:  normalizeProbeProfile(input.ProbeProfile),
		ClientProfile: clientProfile,
		sessionID:     sessionID,
		ProbeJudge:    probeJudgeConfigFromEnv(),
		ProbeBaseline: probeBaselineFromEnv(),
		Executor:      NewCurlExecutor(defaultVerifierHTTPTimeout),
	}
	results, err := runner.RunProbeSuiteOnly(ctx)
	if err != nil {
		return nil, err
	}
	return RedactDirectProbeResponse(&DirectProbeResponse{
		BaseURL:       runner.BaseURL,
		Provider:      providers[0],
		Model:         models[0],
		ProbeProfile:  runner.ProbeProfile,
		ClientProfile: runner.ClientProfile,
		Results:       results,
		Report:        BuildProbeReportWithOptions(ctx, results, DefaultReportOptionsFromEnv()),
	}, apiKey), nil
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
		if r.isClaudeCodeClientProfile(provider) {
			body["stream"] = true
		}
		body = r.applyAnthropicClientProfileBody(provider, body)
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

	payload, _ := r.marshalRequestBody(provider, body)
	headers := r.requestHeadersForBody(provider, body)
	headers["Content-Type"] = "application/json"
	resp, err := executor.Do(ctx, "POST", target, headers, payload)
	if err != nil {
		return preflightResult{Outcome: preflightOutcomeWarn, Code: "preflight_request_failed", Reason: err.Error()}, 0, false
	}
	return classifyPreflightResult(resp.StatusCode, resp.Body), resp.LatencyMs, true
}

func failedDirectProbeSuiteResults(provider string, modelName string, profile string, preflight preflightResult, latencyMs int64) []CheckResult {
	probes := probeSuiteDefinitions(profile)
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

func normalizeClientProfile(profile string) string {
	if strings.EqualFold(strings.TrimSpace(profile), ClientProfileClaudeCode) {
		return ClientProfileClaudeCode
	}
	return ""
}

func (r Runner) isClaudeCodeClientProfile(provider string) bool {
	return provider == ProviderAnthropic && normalizeClientProfile(r.ClientProfile) == ClientProfileClaudeCode
}

func (r Runner) requestHeaders(provider string) map[string]string {
	return r.requestHeadersForBody(provider, nil)
}

func (r Runner) requestHeadersForBody(provider string, body map[string]any) map[string]string {
	headers := providerHeaders(provider, r.Token)
	if r.isClaudeCodeClientProfile(provider) {
		applyClaudeCodeClientProfileHeaders(headers, claudeCodeSessionIDFromBody(body, r.claudeCodeSessionID()), boolValue(body["stream"]))
	}
	return headers
}

func (r Runner) claudeCodeSessionID() string {
	if strings.TrimSpace(r.sessionID) != "" {
		return r.sessionID
	}
	return uuid.NewString()
}

func applyClaudeCodeClientProfileHeaders(headers map[string]string, sessionID string, stream bool) {
	if headers == nil {
		return
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = uuid.NewString()
	}
	headers["User-Agent"] = claudeCodeCLIUserAgent
	headers["X-Stainless-Lang"] = "js"
	headers["X-Stainless-Package-Version"] = "0.70.0"
	headers["X-Stainless-OS"] = "Linux"
	headers["X-Stainless-Arch"] = "arm64"
	headers["X-Stainless-Runtime"] = "node"
	headers["X-Stainless-Runtime-Version"] = "v24.13.0"
	headers["X-Stainless-Retry-Count"] = "0"
	headers["X-Stainless-Timeout"] = "600"
	headers["X-App"] = "cli"
	headers["Anthropic-Dangerous-Direct-Browser-Access"] = "true"
	headers["Accept"] = "application/json"
	headers["anthropic-beta"] = mergeHeaderCSV(claudeCodeAnthropicBeta, headers["anthropic-beta"])
	if stream {
		headers["x-stainless-helper-method"] = "stream"
	}
	if strings.TrimSpace(headers["x-client-request-id"]) == "" {
		headers["x-client-request-id"] = uuid.NewString()
	}
	if strings.TrimSpace(headers["X-Claude-Code-Session-Id"]) == "" {
		headers["X-Claude-Code-Session-Id"] = sessionID
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func mergeHeaderCSV(required string, existing string) string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	add(required)
	add(existing)
	return strings.Join(out, ",")
}

func (r Runner) marshalRequestBody(provider string, body map[string]any) ([]byte, error) {
	var payload []byte
	var err error
	if r.isClaudeCodeClientProfile(provider) {
		payload, err = marshalClaudeCodeRequestBody(body)
	} else {
		payload, err = common.Marshal(body)
	}
	if err != nil {
		return nil, err
	}
	if r.isClaudeCodeClientProfile(provider) {
		payload = signClaudeCodeBillingHeaderCCH(payload)
	}
	return payload, nil
}

func marshalClaudeCodeRequestBody(body map[string]any) ([]byte, error) {
	if body == nil {
		return []byte("{}"), nil
	}
	orderedKeys := []string{
		"model",
		"max_tokens",
		"stream",
		"messages",
		"system",
		"metadata",
		"thinking",
		"tools",
		"tool_choice",
		"stop_sequences",
	}
	seen := make(map[string]struct{}, len(orderedKeys))
	parts := make([]string, 0, len(body))
	for _, key := range orderedKeys {
		value, ok := body[key]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		keyBytes, _ := json.Marshal(key)
		parts = append(parts, string(keyBytes)+":"+string(encoded))
		seen[key] = struct{}{}
	}
	for key, value := range body {
		if _, ok := seen[key]; ok {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		keyBytes, _ := json.Marshal(key)
		parts = append(parts, string(keyBytes)+":"+string(encoded))
	}
	return []byte("{" + strings.Join(parts, ",") + "}"), nil
}

func (r Runner) applyAnthropicClientProfileBody(provider string, body map[string]any) map[string]any {
	if !r.isClaudeCodeClientProfile(provider) || body == nil {
		return body
	}
	out := make(map[string]any, len(body)+2)
	for key, value := range body {
		out[key] = value
	}
	fingerprintMessages := out["messages"]
	systemText := extractAnthropicSystemText(out["system"])
	if shouldMoveSystemTextToMessages(systemText) {
		out["messages"] = prependClaudeCodeSystemInstructionMessages(out["messages"], systemText)
	}
	out["system"] = buildClaudeCodeSystemBlocks(fingerprintMessages)
	out["metadata"] = mergeClaudeCodeMetadata(out["metadata"], r.claudeCodeSessionID())
	return out
}

func claudeCodeSessionIDFromBody(body map[string]any, fallback string) string {
	metadata, ok := body["metadata"].(map[string]any)
	if !ok {
		return fallback
	}
	userID := strings.TrimSpace(common.Interface2String(metadata["user_id"]))
	if userID == "" {
		return fallback
	}
	const marker = "_session_"
	idx := strings.LastIndex(userID, marker)
	if idx < 0 {
		return fallback
	}
	sessionID := strings.TrimSpace(userID[idx+len(marker):])
	if sessionID == "" {
		return fallback
	}
	return sessionID
}

func buildClaudeCodeSystemBlocks(fingerprintMessages any) []map[string]any {
	return []map[string]any{
		buildClaudeCodeBillingAttributionBlock(fingerprintMessages),
		buildClaudeCodeSystemPromptBlock(),
	}
}

func buildClaudeCodeBillingAttributionBlock(fingerprintMessages any) map[string]any {
	fingerprint := computeClaudeCodeFingerprint(fingerprintMessages, claudeCodeCLIVersion)
	return map[string]any{
		"type": "text",
		"text": fmt.Sprintf(
			"x-anthropic-billing-header: cc_version=%s.%s; cc_entrypoint=cli; cch=00000;",
			claudeCodeCLIVersion,
			fingerprint,
		),
	}
}

func buildClaudeCodeSystemPromptBlock() map[string]any {
	systemPromptBlock := map[string]any{
		"type": "text",
		"text": claudeCodeSystemPrompt,
		"cache_control": map[string]any{
			"type": "ephemeral",
			"ttl":  "5m",
		},
	}
	return systemPromptBlock
}

func extractAnthropicSystemText(system any) string {
	switch typed := system.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []map[string]any:
		parts := make([]string, 0, len(typed))
		for _, block := range typed {
			if text := strings.TrimSpace(common.Interface2String(block["text"])); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text := strings.TrimSpace(common.Interface2String(block["text"])); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func shouldMoveSystemTextToMessages(systemText string) bool {
	value := strings.TrimSpace(systemText)
	return value != "" &&
		value != claudeCodeSystemPrompt &&
		!strings.HasPrefix(value, "x-anthropic-billing-header") &&
		!strings.HasPrefix(value, claudeCodeSystemPrompt)
}

func prependClaudeCodeSystemInstructionMessages(messages any, systemText string) []map[string]any {
	out := []map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "[System Instructions]\n" + strings.TrimSpace(systemText)},
			},
		},
		{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "Understood. I will follow these instructions."},
			},
		},
	}
	switch typed := messages.(type) {
	case []map[string]any:
		out = append(out, typed...)
	case []map[string]string:
		for _, item := range typed {
			msg := make(map[string]any, len(item))
			for key, value := range item {
				msg[key] = value
			}
			out = append(out, msg)
		}
	case []any:
		for _, item := range typed {
			msg, ok := item.(map[string]any)
			if ok {
				out = append(out, msg)
			}
		}
	}
	return out
}

func computeClaudeCodeFingerprint(messages any, version string) string {
	firstText := extractFirstClaudeCodeUserText(messages)
	indices := []int{4, 7, 20}
	chars := make([]byte, 0, len(indices))
	for _, idx := range indices {
		if idx < len(firstText) {
			chars = append(chars, firstText[idx])
			continue
		}
		chars = append(chars, '0')
	}
	sum := sha256.Sum256([]byte(claudeCodeFingerprintSalt + string(chars) + version))
	return hex.EncodeToString(sum[:])[:3]
}

func extractFirstClaudeCodeUserText(messages any) string {
	switch typed := messages.(type) {
	case []map[string]any:
		for _, msg := range typed {
			if msg["role"] != "user" {
				continue
			}
			return extractFirstClaudeCodeTextContent(msg["content"])
		}
	case []map[string]string:
		for _, msg := range typed {
			if msg["role"] == "user" {
				return msg["content"]
			}
		}
	case []any:
		for _, item := range typed {
			msg, ok := item.(map[string]any)
			if !ok || msg["role"] != "user" {
				continue
			}
			return extractFirstClaudeCodeTextContent(msg["content"])
		}
	}
	return ""
}

func extractFirstClaudeCodeTextContent(content any) string {
	switch typed := content.(type) {
	case string:
		return typed
	case []map[string]any:
		for _, block := range typed {
			if block["type"] == "text" {
				return common.Interface2String(block["text"])
			}
		}
	case []any:
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if ok && block["type"] == "text" {
				return common.Interface2String(block["text"])
			}
		}
	}
	return ""
}

func signClaudeCodeBillingHeaderCCH(payload []byte) []byte {
	if !claudeCodeCCHPlaceholderPattern.Match(payload) {
		return payload
	}
	cch := fmt.Sprintf("%05x", xxHash64Seeded(payload, claudeCodeBillingCCHSeed)&0xFFFFF)
	return claudeCodeCCHPlaceholderPattern.ReplaceAll(payload, []byte("${1}"+cch+"${3}"))
}

func xxHash64Seeded(data []byte, seed uint64) uint64 {
	d := xxhash.NewWithSeed(seed)
	_, _ = d.Write(data)
	return d.Sum64()
}

func mergeClaudeCodeMetadata(metadata any, sessionID string) map[string]any {
	out := make(map[string]any)
	if existing, ok := metadata.(map[string]any); ok {
		for key, value := range existing {
			out[key] = value
		}
	}
	if strings.TrimSpace(common.Interface2String(out["user_id"])) == "" {
		out["user_id"] = buildClaudeCodeMetadataUserID(sessionID)
	}
	return out
}

func buildClaudeCodeMetadataUserID(sessionID string) string {
	deviceID := randomHexString(claudeCodeMetadataDeviceIDLength)
	if strings.TrimSpace(sessionID) == "" {
		sessionID = uuid.NewString()
	}
	return "user_" + deviceID + "_account__session_" + sessionID
}

func randomHexString(length int) string {
	if length <= 0 {
		return ""
	}
	out := make([]byte, length)
	max := big.NewInt(int64(len(claudeCodeMetadataDeviceIDAllowedChars)))
	for i := range out {
		n, err := crand.Int(crand.Reader, max)
		if err != nil {
			out[i] = claudeCodeMetadataDeviceIDAllowedChars[i%len(claudeCodeMetadataDeviceIDAllowedChars)]
			continue
		}
		out[i] = claudeCodeMetadataDeviceIDAllowedChars[n.Int64()]
	}
	return string(out)
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
