package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

func TestChannelTestRecordsConsumeLog(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	withChannelGroupRatios(t, `{"default":1,"veo-福利渠道":1}`)

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
		Group:    "default",
		Priority: common.GetPointer(int64(0)),
		Weight:   common.GetPointer(uint(0)),
	}

	result := testChannel(channel, "claude-opus-4-6", "", false)
	if result.localErr != nil {
		t.Fatalf("testChannel returned local error: %v", result.localErr)
	}
	if result.newAPIError != nil {
		t.Fatalf("testChannel returned api error: %v", result.newAPIError)
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
}
