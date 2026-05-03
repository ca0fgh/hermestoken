package controller

import (
	"context"
	"strconv"
	"strings"

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
		ctx, cancel := context.WithTimeout(context.Background(), tokenverifier.TaskTimeout())
		defer cancel()
		if err := tokenverifier.RunTask(ctx, task.ID); err != nil {
			common.SysLog("token verification task failed: " + err.Error())
		}
	})
	common.ApiSuccess(c, toTokenVerificationTaskView(task))
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
