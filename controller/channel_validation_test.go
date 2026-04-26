package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupChannelControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL
	originalRedis := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnMetadata()
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Channel{}, &model.Ability{}); err != nil {
		t.Fatalf("failed to migrate channel tables: %v", err)
	}

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
		common.RedisEnabled = originalRedis

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func withChannelGroupRatios(t *testing.T, ratioJSON string) {
	t.Helper()

	originalRatios := ratio_setting.GroupRatio2JSONString()
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}

	t.Cleanup(func() {
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
	})
}

func TestValidateChannelRejectsUnknownGroup(t *testing.T) {
	withChannelGroupRatios(t, `{"default":1,"cc-opus4.6-福利渠道":1}`)

	channel := &model.Channel{
		Key:    "sk-test",
		Group:  "cc-oups4.6-福利渠道",
		Models: "claude-opus-4-6",
	}

	err := validateChannel(channel, true)
	if err == nil {
		t.Fatal("expected validateChannel to reject unknown group")
	}
	if !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("expected unknown-group error, got %v", err)
	}
}

func TestBatchSetChannelAutoBanUpdatesSelectedChannels(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	autoBanOn := 1
	autoBanOff := 0
	channels := []model.Channel{
		{Id: 1, Name: "auto-ban-off-1", Key: "sk-1", Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o", AutoBan: &autoBanOff},
		{Id: 2, Name: "auto-ban-on-2", Key: "sk-2", Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o", AutoBan: &autoBanOn},
		{Id: 3, Name: "auto-ban-off-3", Key: "sk-3", Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o", AutoBan: &autoBanOff},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatalf("failed to seed channels: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/batch/auto_ban", map[string]any{
		"ids":      []int{1, 3},
		"auto_ban": 1,
	}, 1)

	BatchSetChannelAutoBan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected batch auto-ban enable success, got %q", response.Message)
	}
	var updatedCount int
	if err := json.Unmarshal(response.Data, &updatedCount); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if updatedCount != 2 {
		t.Fatalf("expected response data 2, got %d", updatedCount)
	}

	var reloaded []model.Channel
	if err := db.Order("id").Find(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload channels: %v", err)
	}
	for _, channel := range reloaded {
		if channel.AutoBan == nil {
			t.Fatalf("channel %d auto_ban should not be nil", channel.Id)
		}
	}
	if *reloaded[0].AutoBan != 1 || *reloaded[1].AutoBan != 1 || *reloaded[2].AutoBan != 1 {
		t.Fatalf("expected selected channels enabled and untouched channel still enabled, got %d,%d,%d", *reloaded[0].AutoBan, *reloaded[1].AutoBan, *reloaded[2].AutoBan)
	}

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/channel/batch/auto_ban", map[string]any{
		"ids":      []int{1, 2},
		"auto_ban": 0,
	}, 1)

	BatchSetChannelAutoBan(ctx)

	response = decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected batch auto-ban disable success, got %q", response.Message)
	}
	reloaded = nil
	if err := db.Order("id").Find(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload channels after disable: %v", err)
	}
	if *reloaded[0].AutoBan != 0 || *reloaded[1].AutoBan != 0 || *reloaded[2].AutoBan != 1 {
		t.Fatalf("expected selected channels disabled and unselected channel unchanged, got %d,%d,%d", *reloaded[0].AutoBan, *reloaded[1].AutoBan, *reloaded[2].AutoBan)
	}
}

func TestBatchSetChannelAutoBanRejectsInvalidInput(t *testing.T) {
	setupChannelControllerTestDB(t)

	for _, tt := range []struct {
		name string
		body map[string]any
	}{
		{
			name: "empty ids",
			body: map[string]any{"ids": []int{}, "auto_ban": 1},
		},
		{
			name: "missing auto ban",
			body: map[string]any{"ids": []int{1}},
		},
		{
			name: "invalid auto ban",
			body: map[string]any{"ids": []int{1}, "auto_ban": 2},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/batch/auto_ban", tt.body, 1)

			BatchSetChannelAutoBan(ctx)

			response := decodeAPIResponse(t, recorder)
			if response.Success {
				t.Fatalf("expected invalid input to fail, got body %s", recorder.Body.String())
			}
			if response.Message != "参数错误" {
				t.Fatalf("expected 参数错误, got %q", response.Message)
			}
		})
	}
}

func TestEditTagChannelsRejectsUnknownGroupUpdate(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	withChannelGroupRatios(t, `{"default":1,"cc-opus4.6-福利渠道":1}`)

	tag := "福利渠道"
	channel := &model.Channel{
		Id:     1,
		Name:   "福利渠道",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "claude-opus-4-6",
		Tag:    &tag,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	if err := channel.AddAbilities(nil); err != nil {
		t.Fatalf("failed to seed abilities: %v", err)
	}

	payload := ChannelTag{
		Tag:    tag,
		Groups: common.GetPointer("cc-oups4.6-福利渠道"),
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/tag", payload, 1)

	EditTagChannels(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected EditTagChannels to reject invalid group update: %s", recorder.Body.String())
	}
	if !strings.Contains(response.Message, "未配置") {
		t.Fatalf("expected unknown-group message, got %q", response.Message)
	}

	reloaded, err := model.GetChannelById(channel.Id, true)
	if err != nil {
		t.Fatalf("failed to reload channel: %v", err)
	}
	if reloaded.Group != "default" {
		t.Fatalf("channel group changed unexpectedly: got %q", reloaded.Group)
	}

	var abilityCount int64
	if err := db.Model(&model.Ability{}).Where("channel_id = ?", channel.Id).Count(&abilityCount).Error; err != nil {
		t.Fatalf("failed to count abilities: %v", err)
	}
	if abilityCount != 1 {
		t.Fatalf("expected original abilities to remain intact, got %d", abilityCount)
	}
}

func TestEditTagChannelsNormalizesModelsBeforePersisting(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	withChannelGroupRatios(t, `{"default":1}`)

	tag := "normalize-models"
	channel := &model.Channel{
		Id:     2,
		Name:   "normalize-models",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "claude-opus-4-6",
		Tag:    &tag,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	if err := channel.AddAbilities(nil); err != nil {
		t.Fatalf("failed to seed abilities: %v", err)
	}

	payload := ChannelTag{
		Tag:    tag,
		Models: common.GetPointer(" claude-opus-4-6 , claude-sonnet-4-6 , claude-opus-4-6 "),
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/tag", payload, 1)

	EditTagChannels(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected EditTagChannels success, got %s", response.Message)
	}

	reloaded, err := model.GetChannelById(channel.Id, true)
	if err != nil {
		t.Fatalf("failed to reload channel: %v", err)
	}
	if reloaded.Models != "claude-opus-4-6,claude-sonnet-4-6" {
		t.Fatalf("expected normalized models, got %q", reloaded.Models)
	}
}

func TestCopyChannelRejectsLegacyInvalidGroup(t *testing.T) {
	db := setupChannelControllerTestDB(t)
	withChannelGroupRatios(t, `{"default":1,"cc-opus4.6-福利渠道":1}`)

	channel := &model.Channel{
		Id:     3,
		Name:   "legacy-invalid-group",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Group:  "cc-oups4.6-福利渠道",
		Models: "claude-opus-4-6",
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/copy/3", nil, 1)
	ctx.Params = []gin.Param{{Key: "id", Value: "3"}}

	CopyChannel(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected CopyChannel to reject invalid legacy group, got body %s", recorder.Body.String())
	}
	if !strings.Contains(response.Message, "未配置") {
		t.Fatalf("expected unknown-group error, got %q", response.Message)
	}
}
