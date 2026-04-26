package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelCacheTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroup2Model2Channels := group2model2channels
	originalChannelsIDM := channelsIDM

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
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
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
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

	enabled, err := GetRandomSatisfiedChannel("default", "enabled-model", 0)
	require.NoError(t, err)
	require.NotNil(t, enabled)
	require.Equal(t, channel.Id, enabled.Id)

	disabled, err := GetRandomSatisfiedChannel("default", "disabled-model", 0)
	require.NoError(t, err)
	require.Nil(t, disabled)
}

func TestCacheDisableChannelModelRemovesOnlyThatModel(t *testing.T) {
	setupChannelCacheTestDB(t)

	priority := int64(2)
	weight := uint(0)
	baseURL := "https://cache-disable-test.example.com"
	channel := &Channel{
		Id:       1002,
		Name:     "cache-disable-test",
		Type:     14,
		Key:      "sk-cache-disable-test",
		Status:   common.ChannelStatusEnabled,
		BaseURL:  &baseURL,
		Group:    "default",
		Models:   "kept-model,removed-model",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, channel.Insert())
	InitChannelCache()

	CacheDisableChannelModel(channel.Id, "removed-model")

	kept, err := GetRandomSatisfiedChannel("default", "kept-model", 0)
	require.NoError(t, err)
	require.NotNil(t, kept)
	require.Equal(t, channel.Id, kept.Id)

	removed, err := GetRandomSatisfiedChannel("default", "removed-model", 0)
	require.NoError(t, err)
	require.Nil(t, removed)
}
