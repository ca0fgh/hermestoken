package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/types"
)

func TestChannelTestRecordsConsumeLog(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	withChannelGroupRatios(t, `{"default":1,"veo-福利渠道":1,"cc-opus-福利渠道":1}`)

	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate channel test log tables: %v", err)
	}

	originalLogConsumeEnabled := common.LogConsumeEnabled
	originalDataExportEnabled := common.DataExportEnabled
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	service.InitHttpClient()
	t.Cleanup(func() {
		common.LogConsumeEnabled = originalLogConsumeEnabled
		common.DataExportEnabled = originalDataExportEnabled
	})

	user := &model.User{
		Id:       1,
		Username: "ca0fgh",
		Password: "password123",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "veo-福利渠道",
	}
	user.SetSetting(dto.UserSetting{AcceptUnsetRatioModel: true})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create channel-test user: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1710000000,
			"model":"claude-opus-4-6",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}
		}`))
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Id:       99,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "sk-test",
		Status:   common.ChannelStatusEnabled,
		Name:     "test-openai-channel",
		BaseURL:  common.GetPointer(upstream.URL),
		Models:   "claude-opus-4-6",
		Group:    "default,cc-opus-福利渠道",
		Priority: common.GetPointer(int64(0)),
		Weight:   common.GetPointer(uint(0)),
	}
	if err := channel.AddAbilities(db); err != nil {
		t.Fatalf("failed to add channel abilities: %v", err)
	}

	result := testChannel(channel, "claude-opus-4-6", "", false)
	if result.localErr != nil {
		t.Fatalf("testChannel returned local error: %v", result.localErr)
	}
	if result.hermesTokenError != nil {
		t.Fatalf("testChannel returned api error: %v", result.hermesTokenError)
	}

	var logEntry model.Log
	if err := db.Where("type = ? AND token_name = ?", model.LogTypeConsume, "模型测试").
		Order("id desc").
		First(&logEntry).Error; err != nil {
		t.Fatalf("expected channel test to record consume log: %v", err)
	}
	if logEntry.ChannelId != channel.Id {
		t.Fatalf("expected consume log channel_id=%d, got %d", channel.Id, logEntry.ChannelId)
	}
	if logEntry.ModelName != "claude-opus-4-6" {
		t.Fatalf("expected consume log model claude-opus-4-6, got %s", logEntry.ModelName)
	}
	if logEntry.UserId != user.Id {
		t.Fatalf("expected consume log user_id=%d, got %d", user.Id, logEntry.UserId)
	}
	if logEntry.Group != "default,cc-opus-福利渠道" {
		t.Fatalf("expected consume log group to use channel test groups, got %q", logEntry.Group)
	}
}

func TestChannelTestErrorLogUsesTestTokenAndGroups(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	withChannelGroupRatios(t, `{"default":1,"veo-福利渠道":1,"cc-opus-福利渠道":1}`)

	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate channel test error log tables: %v", err)
	}

	originalLogConsumeEnabled := common.LogConsumeEnabled
	originalDataExportEnabled := common.DataExportEnabled
	originalErrorLogEnabled := constant.ErrorLogEnabled
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	constant.ErrorLogEnabled = true
	service.InitHttpClient()
	t.Cleanup(func() {
		common.LogConsumeEnabled = originalLogConsumeEnabled
		common.DataExportEnabled = originalDataExportEnabled
		constant.ErrorLogEnabled = originalErrorLogEnabled
	})

	user := &model.User{
		Id:       1,
		Username: "ca0fgh",
		Password: "password123",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "veo-福利渠道",
	}
	user.SetSetting(dto.UserSetting{AcceptUnsetRatioModel: true})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create channel-test user: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1710000000,
			"model":"claude-opus-4-6",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}
		}`))
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Id:       99,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "sk-test",
		Status:   common.ChannelStatusEnabled,
		Name:     "test-openai-channel",
		BaseURL:  common.GetPointer(upstream.URL),
		Models:   "claude-opus-4-6",
		Group:    "default,cc-opus-福利渠道",
		Priority: common.GetPointer(int64(0)),
		Weight:   common.GetPointer(uint(0)),
	}
	if err := channel.AddAbilities(db); err != nil {
		t.Fatalf("failed to add channel abilities: %v", err)
	}

	result := testChannel(channel, "claude-opus-4-6", "", false)
	if result.localErr != nil {
		t.Fatalf("testChannel returned local error: %v", result.localErr)
	}
	if result.hermesTokenError != nil {
		t.Fatalf("testChannel returned api error: %v", result.hermesTokenError)
	}

	apiErr := types.NewOpenAIError(
		fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", 7.89, 5.00),
		types.ErrorCodeChannelResponseTimeExceeded,
		http.StatusRequestTimeout,
	)
	processChannelError(result.context, channel.Id, apiErr)

	var logEntry model.Log
	if err := db.Where("type = ?", model.LogTypeError).
		Order("id desc").
		First(&logEntry).Error; err != nil {
		t.Fatalf("expected channel test error log: %v", err)
	}
	if logEntry.TokenName != channelTestTokenName {
		t.Fatalf("expected error log token_name=%q, got %q", channelTestTokenName, logEntry.TokenName)
	}
	if logEntry.Group != "default,cc-opus-福利渠道" {
		t.Fatalf("expected error log group to use channel test groups, got %q", logEntry.Group)
	}

	var other struct {
		ChannelTest       bool     `json:"channel_test"`
		ChannelTestGroups []string `json:"channel_test_groups"`
		AdminInfo         struct {
			ChannelTestGroups     []string `json:"channel_test_groups"`
			ChannelTestUsingGroup string   `json:"channel_test_using_group"`
		} `json:"admin_info"`
	}
	if err := common.Unmarshal([]byte(logEntry.Other), &other); err != nil {
		t.Fatalf("failed to unmarshal error log other: %v", err)
	}
	if !other.ChannelTest {
		t.Fatalf("expected error log other.channel_test=true, got false")
	}
	if got := fmt.Sprint(other.ChannelTestGroups); got != "[default cc-opus-福利渠道]" {
		t.Fatalf("expected error log channel_test_groups, got %s", got)
	}
	if got := fmt.Sprint(other.AdminInfo.ChannelTestGroups); got != "[default cc-opus-福利渠道]" {
		t.Fatalf("expected error log admin_info.channel_test_groups, got %s", got)
	}
	if other.AdminInfo.ChannelTestUsingGroup != "default" {
		t.Fatalf("expected error log channel_test_using_group=default, got %q", other.AdminInfo.ChannelTestUsingGroup)
	}
}
