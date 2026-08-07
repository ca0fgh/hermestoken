package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelCacheTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalSQLite := common.UsingMainDatabase(common.DatabaseTypeSQLite)
	originalMySQL := common.UsingMainDatabase(common.DatabaseTypeMySQL)
	originalPostgres := common.UsingMainDatabase(common.DatabaseTypePostgreSQL)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM

	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	InitColumnMetadata()

	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		if originalSQLite {
			common.SetMainDatabaseType(common.DatabaseTypeSQLite)
		}
		if originalMySQL {
			common.SetMainDatabaseType(common.DatabaseTypeMySQL)
		}
		if originalPostgres {
			common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
		}
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		group2model2channels = originalGroup2Model2Channels
		channelsIDM = originalChannelsIDM

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestInitChannelCacheUsesEnabledAbilities(t *testing.T) {
	setupChannelCacheTestDB(t)

	priority := int64(2)
	weight := uint(0)
	baseURL := "https://cache-test.example.com"
	channel := &Channel{
		Id:       1001,
		Name:     "cache-test",
		Type:     14,
		Key:      "sk-cache-test",
		Status:   common.ChannelStatusEnabled,
		BaseURL:  &baseURL,
		Group:    "default",
		Models:   "enabled-model,disabled-model",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, channel.Insert())
	require.NoError(t, DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", "default", "disabled-model", channel.Id).
		Update("enabled", false).Error)

	InitChannelCache()

	enabled, err := GetRandomSatisfiedChannel("default", "enabled-model", 0, "")
	require.NoError(t, err)
	require.NotNil(t, enabled)
	require.Equal(t, channel.Id, enabled.Id)

	disabled, err := GetRandomSatisfiedChannel("default", "disabled-model", 0, "")
	require.NoError(t, err)
	require.Nil(t, disabled)
}

// 复刻 2026-08-07 渠道 37 事故：亲和把会话钉在最低优先级渠道，该渠道 503 后，
// 旧实现单向降级直接判「无可用渠道」，而更高优先级的健康渠道全被跳过。
func TestGetNextSatisfiedChannelWrapsToHigherPriorityBuckets(t *testing.T) {
	setupChannelCacheTestDB(t)

	weight := uint(0)
	insertChannel := func(id int, name string, priority int64) {
		p := priority
		baseURL := fmt.Sprintf("https://wrap-test-%d.example.com", id)
		require.NoError(t, (&Channel{
			Id:       id,
			Name:     name,
			Type:     14,
			Key:      "sk-wrap-test",
			Status:   common.ChannelStatusEnabled,
			BaseURL:  &baseURL,
			Group:    "default",
			Models:   "wrap-model",
			Priority: &p,
			Weight:   &weight,
		}).Insert())
	}
	insertChannel(2001, "wrap-high", 2)
	insertChannel(2002, "wrap-mid", 1)
	insertChannel(2003, "wrap-low", 0)

	InitChannelCache()

	// 常规路径不受影响：从桶 0 起步仍按优先级降序选择。
	channel, idx, hasMore, err := GetNextSatisfiedChannel("default", "wrap-model", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2001, channel.Id)
	require.Equal(t, 0, idx)
	require.True(t, hasMore)

	// 亲和 seed：被钉渠道位于最低优先级桶。
	lowIdx, found, err := GetSatisfiedChannelPriorityIndex("default", "wrap-model", 2003)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 2, lowIdx)

	// 被钉渠道故障（已记入 tried）：必须回卷到最高优先级，而不是判无可用渠道。
	tried := map[int]struct{}{2003: {}}
	channel, idx, hasMore, err = GetNextSatisfiedChannel("default", "wrap-model", lowIdx, tried)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2001, channel.Id)
	require.Equal(t, 0, idx)
	require.True(t, hasMore)

	// 最高优先级也故障：继续给出中优先级。
	tried[2001] = struct{}{}
	channel, idx, hasMore, err = GetNextSatisfiedChannel("default", "wrap-model", idx, tried)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2002, channel.Id)
	require.Equal(t, 1, idx)
	require.False(t, hasMore)

	// 组内全部试过，才允许返回 nil（auto 分组据此切下一分组）。
	tried[2002] = struct{}{}
	channel, _, _, err = GetNextSatisfiedChannel("default", "wrap-model", idx, tried)
	require.NoError(t, err)
	require.Nil(t, channel)
}
