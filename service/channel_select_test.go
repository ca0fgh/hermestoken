package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

func setupChannelSelectTestDB(t *testing.T) *gorm.DB {
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
	common.MemoryCacheEnabled = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	model.InitColumnMetadata()

	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

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

func insertChannelForRetryTest(t *testing.T, id int, name string, priority int64) {
	t.Helper()

	weight := uint(0)
	baseURL := fmt.Sprintf("https://%s.example.com", name)
	channel := &model.Channel{
		Id:       id,
		Name:     name,
		Type:     14,
		Key:      fmt.Sprintf("sk-%s", name),
		Status:   common.ChannelStatusEnabled,
		BaseURL:  &baseURL,
		Group:    "cc-opus-福利渠道",
		Models:   "claude-opus-4-7",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, channel.Insert())
}

func TestCacheGetRandomSatisfiedChannel_ExhaustsSamePriorityBeforeDowngrade(t *testing.T) {
	setupChannelSelectTestDB(t)

	insertChannelForRetryTest(t, 12, "high-a", 2)
	insertChannelForRetryTest(t, 13, "high-b", 2)
	insertChannelForRetryTest(t, 11, "mid", 1)
	insertChannelForRetryTest(t, 1, "low", 0)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	retry := 0
	param := &RetryParam{
		Ctx:        ctx,
		TokenGroup: "cc-opus-福利渠道",
		ModelName:  "claude-opus-4-7",
		Retry:      &retry,
	}

	first, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.Equal(t, "cc-opus-福利渠道", group)
	require.NotNil(t, first)
	require.Contains(t, []int{12, 13}, first.Id)

	param.IncreaseRetry()
	second, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Contains(t, []int{12, 13}, second.Id)
	require.NotEqual(t, first.Id, second.Id, "should try the other same-priority channel before downgrading")

	param.IncreaseRetry()
	third, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, third)
	require.Equal(t, 11, third.Id, "should downgrade only after all priority-2 channels are exhausted")

	param.IncreaseRetry()
	fourth, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, fourth)
	require.Equal(t, 1, fourth.Id)
}

func TestCacheGetRandomSatisfiedChannel_ReturnsNilAfterAllCandidateChannelsAreTried(t *testing.T) {
	setupChannelSelectTestDB(t)

	insertChannelForRetryTest(t, 12, "high-a", 2)
	insertChannelForRetryTest(t, 13, "high-b", 2)
	insertChannelForRetryTest(t, 1, "low", 0)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	retry := 0
	param := &RetryParam{
		Ctx:        ctx,
		TokenGroup: "cc-opus-福利渠道",
		ModelName:  "claude-opus-4-7",
		Retry:      &retry,
	}

	for i := 0; i < 3; i++ {
		channel, _, err := CacheGetRandomSatisfiedChannel(param)
		require.NoError(t, err)
		require.NotNil(t, channel)
		param.IncreaseRetry()
	}

	channel, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.Nil(t, channel, "should stop retrying once every candidate channel has already been attempted")
}

func TestRetryParamSeedSelectedChannel_AvoidsReusingInitialChannelOnFirstRetry(t *testing.T) {
	setupChannelSelectTestDB(t)

	insertChannelForRetryTest(t, 12, "high-a", 2)
	insertChannelForRetryTest(t, 13, "high-b", 2)
	insertChannelForRetryTest(t, 11, "mid", 1)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	param := &RetryParam{
		Ctx:        ctx,
		TokenGroup: "cc-opus-福利渠道",
		ModelName:  "claude-opus-4-7",
		Retry:      common.GetPointer(1),
	}

	require.NoError(t, param.SeedSelectedChannel("cc-opus-福利渠道", 12))

	channel, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 13, channel.Id, "first retry should exhaust same-priority siblings before downgrading or reusing the initial channel")
}
