package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupServiceChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL
	originalMemoryCacheEnabled := common.MemoryCacheEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.MemoryCacheEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	model.InitColumnMetadata()
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}))

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
		common.MemoryCacheEnabled = originalMemoryCacheEnabled

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestShouldDisableChannel_UpstreamNoAvailableToken(t *testing.T) {
	previous := common.AutomaticDisableChannelEnabled
	previousKeywords := append([]string(nil), operation_setting.AutomaticDisableKeywords...)
	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableKeywords = []string{"unrelated upstream failure"}
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previous
		operation_setting.AutomaticDisableKeywords = previousKeywords
	})

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "chinese compact token wording",
			message: "没有可用token（traceid: b2f0e54a96a5a9186b0f8f0f7ce4ddfd）",
		},
		{
			name:    "chinese spaced token wording",
			message: "没有可用 token",
		},
		{
			name:    "english token wording",
			message: "No available tokens for this upstream pool",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := types.NewOpenAIError(errors.New(tt.message), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
			require.True(t, ShouldDisableChannel(err))
		})
	}
}

func TestShouldDisableChannel_AnyNonSkipChannelFailure(t *testing.T) {
	previous := common.AutomaticDisableChannelEnabled
	previousKeywords := append([]string(nil), operation_setting.AutomaticDisableKeywords...)
	previousStatusCodes := append([]operation_setting.StatusCodeRange(nil), operation_setting.AutomaticDisableStatusCodeRanges...)
	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableKeywords = []string{"unrelated upstream failure"}
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previous
		operation_setting.AutomaticDisableKeywords = previousKeywords
		operation_setting.AutomaticDisableStatusCodeRanges = previousStatusCodes
	})

	err := types.NewOpenAIError(
		errors.New("upstream returned service unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	require.True(t, ShouldDisableChannel(err))
}

func TestShouldDisableChannel_SkipRetryFailureDoesNotDisable(t *testing.T) {
	previous := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previous
	})

	err := types.NewOpenAIError(
		errors.New("local request conversion failed"),
		types.ErrorCodeConvertRequestFailed,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)

	require.False(t, ShouldDisableChannel(err))
}

func TestShouldDisableChannelModelAbility_UpstreamDistributorNoAvailableModel(t *testing.T) {
	previous := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previous
	})

	err := types.NewOpenAIError(
		errors.New("No available channel for model claude-haiku-4-5-20251001 under group 自营 kiro (distributor)"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)
	require.True(t, ShouldDisableChannelModelAbility(err))

	plainErr := types.NewOpenAIError(
		errors.New("No available channel for model claude-haiku-4-5-20251001"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)
	require.False(t, ShouldDisableChannelModelAbility(plainErr))
}

func TestChannelModelAbilityAutoDisableRecoveryPreservesEnabledGroups(t *testing.T) {
	db := setupServiceChannelTestDB(t)

	baseURL := "https://ability-test.example.com"
	channel := &model.Channel{
		Id:       2101,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "sk-test",
		Status:   common.ChannelStatusEnabled,
		Name:     "ability-test",
		BaseURL:  &baseURL,
		Models:   "test-model",
		Group:    "default,vip",
		Priority: common.GetPointer(int64(0)),
		Weight:   common.GetPointer(uint(0)),
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "root",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Model(&model.Ability{}).
		Where("channel_id = ? AND model = ? AND `group` = ?", channel.Id, "test-model", "vip").
		Update("enabled", false).Error)

	DisableChannelModelAbility(types.ChannelError{
		ChannelId:   channel.Id,
		ChannelType: channel.Type,
		ChannelName: channel.Name,
		AutoBan:     true,
	}, "test-model", "status_code=503, No available channel for model test-model under group upstream (distributor)")

	groups, err := model.GetEnabledGroupsForChannelModel(channel.Id, "test-model")
	require.NoError(t, err)
	require.Empty(t, groups)

	storedChannel, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	disabledInfo := GetAutoDisabledChannelModelAbilities(storedChannel)
	require.Contains(t, disabledInfo, "test-model")
	require.Equal(t, []string{"default"}, disabledInfo["test-model"].Groups)
	require.NotZero(t, disabledInfo["test-model"].StatusTime)

	DisableChannelModelAbility(types.ChannelError{
		ChannelId:   channel.Id,
		ChannelType: channel.Type,
		ChannelName: channel.Name,
		AutoBan:     true,
	}, "test-model", "status_code=503, No available channel for model test-model under group upstream (distributor)")

	storedChannel, err = model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	disabledInfo = GetAutoDisabledChannelModelAbilities(storedChannel)
	require.Equal(t, []string{"default"}, disabledInfo["test-model"].Groups)

	EnableChannelModelAbility(channel.Id, "test-model", channel.Name)

	groups, err = model.GetEnabledGroupsForChannelModel(channel.Id, "test-model")
	require.NoError(t, err)
	require.Equal(t, []string{"default"}, groups)

	storedChannel, err = model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Empty(t, GetAutoDisabledChannelModelAbilities(storedChannel))
}
