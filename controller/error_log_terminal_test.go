package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
)

// 验证错误日志降噪：可重试的中间尝试不落 type=5，只有终态失败才落库。
// 背景：一个最终成功的请求（如 11->34->61）此前会在后台留下多条红色"错误"，
// 与真·终态失败混淆。详见 processChannelError 注释。
func setupErrorLogTestContext(t *testing.T) (*gin.Context, func() int64) {
	t.Helper()
	db := setupChannelControllerTestDB(t)
	if err := db.AutoMigrate(&model.Log{}, &model.User{}); err != nil {
		t.Fatalf("migrate logs/users: %v", err)
	}

	originalErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() { constant.ErrorLogEnabled = originalErrorLogEnabled })

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 1)
	c.Set("original_model", "claude-opus-4-6")
	c.Set("token_name", "tok")

	countErrorLogs := func() int64 {
		var n int64
		if err := db.Model(&model.Log{}).Where("type = ?", model.LogTypeError).Count(&n).Error; err != nil {
			t.Fatalf("count error logs: %v", err)
		}
		return n
	}
	return c, countErrorLogs
}

func TestProcessChannelError_SkipsDBWriteWhenWillRetry(t *testing.T) {
	c, countErrorLogs := setupErrorLogTestContext(t)

	// 中间可重试尝试：should NOT persist a type=5 log
	setRetryAdminInfo(c, retryAdminInfo{Index: 0, Remaining: 12, WillRetry: true, Reason: retryReasonSelectedChannelUnavailable})
	err := types.NewOpenAIError(errors.New("令牌分组 claude-windsurf-discount 已被禁用"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden)
	processChannelError(c, 34, err)

	if got := countErrorLogs(); got != 0 {
		t.Fatalf("中间可重试尝试不应落 type=5，期望 0 条，实得 %d 条", got)
	}
}

func TestProcessChannelError_RecordsTerminalFailure(t *testing.T) {
	c, countErrorLogs := setupErrorLogTestContext(t)

	// 终态失败（不再重试）：SHOULD persist exactly one type=5 log
	setRetryAdminInfo(c, retryAdminInfo{Index: 4, Remaining: 0, WillRetry: false, Reason: retryReasonStatusCodeNotRetryable})
	err := types.NewOpenAIError(errors.New("custom.input_schema invalid"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	processChannelError(c, 11, err)

	if got := countErrorLogs(); got != 1 {
		t.Fatalf("终态失败应落 1 条 type=5，实得 %d 条", got)
	}
}
