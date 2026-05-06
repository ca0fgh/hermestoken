package controller

import (
	"context"
	"errors"
	"net"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

type createTokenVerificationTaskRequest struct {
	TokenID   int      `json:"token_id"`
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
}

type createTokenVerificationProbeRequest struct {
	URL           string `json:"url"`
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key"`
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	ProbeProfile  string `json:"probe_profile"`
	ClientProfile string `json:"client_profile"`
}

type tokenVerificationTaskView struct {
	ID         int64    `json:"id"`
	UserID     int      `json:"user_id"`
	TokenID    int      `json:"token_id"`
	TokenName  string   `json:"token_name"`
	Models     []string `json:"models"`
	Providers  []string `json:"providers"`
	Status     string   `json:"status"`
	Score      int      `json:"score"`
	Grade      string   `json:"grade"`
	FailReason string   `json:"fail_reason"`
	CreatedAt  int64    `json:"created_at"`
	StartedAt  int64    `json:"started_at"`
	FinishedAt int64    `json:"finished_at"`
}

type tokenVerificationDetailView struct {
	Task    *tokenVerificationTaskView       `json:"task"`
	Results []*model.TokenVerificationResult `json:"results"`
	Report  any                              `json:"report,omitempty"`
}

var runDirectTokenVerificationProbe = tokenverifier.RunDirectProbe

func CreateTokenVerificationTask(c *gin.Context) {
	userID := c.GetInt("id")
	var req createTokenVerificationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.TokenID <= 0 {
		common.ApiErrorMsg(c, "请选择要检测的 Token")
		return
	}
	token, err := model.GetTokenByIds(req.TokenID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	models := resolveRequestedVerificationModels(req.Models, token)
	providers := normalizeVerificationProviders(req.Providers)
	task, err := model.CreateTokenVerificationTask(userID, token, models, providers)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	gopool.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := tokenverifier.RunTask(ctx, task.ID); err != nil {
			common.SysLog("token verification task failed: " + err.Error())
		}
	})
	common.ApiSuccess(c, toTokenVerificationTaskView(task))
}

func CreateTokenVerificationProbe(c *gin.Context) {
	var req createTokenVerificationProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	baseURL, err := normalizeTokenVerificationProbeBaseURL(firstNonEmptyString(req.URL, req.BaseURL))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	apiKey, err := normalizeTokenVerificationProbeAPIKey(req.APIKey)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	modelName, err := normalizeTokenVerificationProbeModel(req.Model)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	provider, err := normalizeTokenVerificationProbeProvider(req.Provider)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	probeProfile, err := normalizeTokenVerificationProbeProfile(req.ProbeProfile, c.GetInt("role"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	clientProfile, err := normalizeTokenVerificationProbeClientProfile(req.ClientProfile, provider)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), tokenVerificationProbeTimeout(probeProfile))
	defer cancel()
	result, err := runDirectTokenVerificationProbe(ctx, tokenverifier.DirectProbeRequest{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Model:         modelName,
		Provider:      provider,
		ProbeProfile:  probeProfile,
		ClientProfile: clientProfile,
	})
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if strings.TrimSpace(result.CapturedAt) == "" {
		result.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(result.SourceTaskID) == "" {
		result.SourceTaskID = "direct-probe-" + result.CapturedAt
	}
	common.ApiSuccess(c, tokenverifier.RedactDirectProbeResponse(result, apiKey))
}

func ListTokenVerificationTasks(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tasks, err := model.ListUserTokenVerificationTasks(userID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, err := model.CountUserTokenVerificationTasks(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]*tokenVerificationTaskView, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, toTokenVerificationTaskView(task))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetTokenVerificationTask(c *gin.Context) {
	task, ok := getUserVerificationTaskFromParam(c)
	if !ok {
		return
	}
	results, err := model.GetTokenVerificationResults(task.ID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	report := loadTokenVerificationReportSummary(task.ID)
	common.ApiSuccess(c, tokenVerificationDetailView{
		Task:    toTokenVerificationTaskView(task),
		Results: results,
		Report:  report,
	})
}

func GetTokenVerificationReport(c *gin.Context) {
	task, ok := getUserVerificationTaskFromParam(c)
	if !ok {
		return
	}
	report := loadTokenVerificationReportSummary(task.ID)
	if report == nil {
		common.ApiErrorMsg(c, "检测报告尚未生成")
		return
	}
	common.ApiSuccess(c, gin.H{
		"task":   toTokenVerificationTaskView(task),
		"report": report,
	})
}

func normalizeTokenVerificationProbeBaseURL(rawURL string) (string, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return "", errors.New("请输入检测 URL")
	}
	if len(value) > 2048 {
		return "", errors.New("检测 URL 过长")
	}
	parsed, err := neturl.Parse(value)
	if err != nil || parsed == nil || parsed.Host == "" {
		return "", errors.New("请输入有效的检测 URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("检测 URL 必须以 http:// 或 https:// 开头")
	}
	if parsed.User != nil {
		return "", errors.New("检测 URL 不能包含用户名或密码")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", errors.New("检测 URL 缺少主机名")
	}
	if !allowPrivateTokenVerificationProbeURL() {
		if err := rejectPrivateTokenVerificationProbeHost(host); err != nil {
			return "", err
		}
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if strings.EqualFold(parsed.Path, "/v1") {
		parsed.Path = ""
	} else if strings.HasSuffix(strings.ToLower(parsed.Path), "/v1") {
		parsed.Path = parsed.Path[:len(parsed.Path)-len("/v1")]
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeTokenVerificationProbeAPIKey(rawAPIKey string) (string, error) {
	value := strings.TrimSpace(rawAPIKey)
	if value == "" {
		return "", errors.New("请输入 API Key")
	}
	if len(value) > 8192 {
		return "", errors.New("API Key 过长")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("API Key 不能包含换行")
	}
	return value, nil
}

func normalizeTokenVerificationProbeModel(rawModel string) (string, error) {
	value := strings.TrimSpace(rawModel)
	if value == "" {
		return "", errors.New("请输入检测模型")
	}
	if len(value) > 191 {
		return "", errors.New("检测模型名称过长")
	}
	if strings.ContainsAny(value, "\r\n,，") {
		return "", errors.New("一次检测只支持一个模型")
	}
	return value, nil
}

func normalizeTokenVerificationProbeProvider(rawProvider string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(rawProvider))
	if value == "" {
		return tokenverifier.ProviderOpenAI, nil
	}
	if value != tokenverifier.ProviderOpenAI && value != tokenverifier.ProviderAnthropic {
		return "", errors.New("检测协议仅支持 OpenAI 或 Anthropic")
	}
	return value, nil
}

func normalizeTokenVerificationProbeProfile(rawProfile string, role int) (string, error) {
	value := strings.ToLower(strings.TrimSpace(rawProfile))
	if value == "" || value == tokenverifier.ProbeProfileStandard {
		return tokenverifier.ProbeProfileStandard, nil
	}
	if value != tokenverifier.ProbeProfileDeep && value != tokenverifier.ProbeProfileFull {
		return "", errors.New("检测深度仅支持 standard、deep 或 full")
	}
	if value == tokenverifier.ProbeProfileFull && role < common.RoleAdminUser {
		return "", errors.New("完整检测仅管理员可用")
	}
	return value, nil
}

func normalizeTokenVerificationProbeClientProfile(rawProfile string, provider string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(rawProfile))
	if value == "" || value == "default" {
		return "", nil
	}
	if value != tokenverifier.ClientProfileClaudeCode {
		return "", errors.New("客户端模式仅支持 default 或 claude_code")
	}
	if provider != tokenverifier.ProviderAnthropic {
		return "", errors.New("Claude Code 客户端模式仅支持 Anthropic 协议")
	}
	return value, nil
}

func tokenVerificationProbeTimeout(profile string) time.Duration {
	switch normalizeProbeProfileForTimeout(profile) {
	case tokenverifier.ProbeProfileFull:
		return 15 * time.Minute
	case tokenverifier.ProbeProfileDeep:
		return 8 * time.Minute
	default:
		return 5 * time.Minute
	}
}

func normalizeProbeProfileForTimeout(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case tokenverifier.ProbeProfileFull:
		return tokenverifier.ProbeProfileFull
	case tokenverifier.ProbeProfileDeep:
		return tokenverifier.ProbeProfileDeep
	default:
		return tokenverifier.ProbeProfileStandard
	}
}

func rejectPrivateTokenVerificationProbeHost(host string) error {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return errors.New("检测 URL 不能指向本机地址")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeTokenVerificationProbeIP(ip) {
			return errors.New("检测 URL 不能指向内网或本机地址")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return errors.New("检测 URL 无法解析")
	}
	for _, address := range addresses {
		if isUnsafeTokenVerificationProbeIP(address.IP) {
			return errors.New("检测 URL 不能解析到内网或本机地址")
		}
	}
	return nil
}

func isUnsafeTokenVerificationProbeIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

func allowPrivateTokenVerificationProbeURL() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_ALLOW_PRIVATE_URLS")), "true")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func getUserVerificationTaskFromParam(c *gin.Context) (*model.TokenVerificationTask, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的检测任务ID")
		return nil, false
	}
	task, err := model.GetUserTokenVerificationTaskByID(c.GetInt("id"), id)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	return task, true
}

func loadTokenVerificationReportSummary(taskID int64) any {
	report, err := model.GetTokenVerificationReportByTaskID(taskID)
	if err != nil || report == nil || strings.TrimSpace(report.Summary) == "" {
		return nil
	}
	var summary any
	if err := common.UnmarshalJsonStr(report.Summary, &summary); err != nil {
		return report.Summary
	}
	return summary
}

func toTokenVerificationTaskView(task *model.TokenVerificationTask) *tokenVerificationTaskView {
	if task == nil {
		return nil
	}
	return &tokenVerificationTaskView{
		ID:         task.ID,
		UserID:     task.UserID,
		TokenID:    task.TokenID,
		TokenName:  task.TokenName,
		Models:     task.GetModels(),
		Providers:  task.GetProviders(),
		Status:     task.Status,
		Score:      task.Score,
		Grade:      task.Grade,
		FailReason: task.FailReason,
		CreatedAt:  task.CreatedAt,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
	}
}

func normalizeVerificationProviders(providers []string) []string {
	seen := make(map[string]struct{}, len(providers))
	normalized := make([]string, 0, len(providers))
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		if provider != tokenverifier.ProviderOpenAI && provider != tokenverifier.ProviderAnthropic {
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		normalized = append(normalized, provider)
	}
	if len(normalized) == 0 {
		return []string{tokenverifier.ProviderOpenAI}
	}
	return normalized
}

func normalizeVerificationModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		normalized = append(normalized, modelName)
		if len(normalized) >= 10 {
			break
		}
	}
	return normalized
}

func resolveRequestedVerificationModels(models []string, token *model.Token) []string {
	normalized := normalizeVerificationModels(models)
	if len(normalized) > 0 {
		return normalized
	}
	if token != nil && token.ModelLimitsEnabled {
		normalized = normalizeVerificationModels(strings.Split(token.ModelLimits, ","))
		if len(normalized) > 0 {
			if len(normalized) > 5 {
				return normalized[:5]
			}
			return normalized
		}
	}
	return []string{"gpt-4o-mini"}
}
