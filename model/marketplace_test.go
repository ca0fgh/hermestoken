package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMarketplaceModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	InitColumnMetadata()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	originalDB := DB
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestMarketplaceMigrationModelsAutoMigrate(t *testing.T) {
	db := setupMarketplaceModelTestDB(t)

	require.NoError(t, db.AutoMigrate(marketplaceMigrationModels()...))

	require.True(t, db.Migrator().HasTable(&MarketplaceCredential{}))
	require.True(t, db.Migrator().HasTable(&MarketplaceCredentialStats{}))
	require.False(t, db.Migrator().HasTable("marketplace_credential_events"))
	require.True(t, db.Migrator().HasTable(&MarketplaceFixedOrder{}))
	require.True(t, db.Migrator().HasTable(&MarketplaceFixedOrderFill{}))
	require.True(t, db.Migrator().HasTable(&MarketplacePoolFill{}))
	require.True(t, db.Migrator().HasTable(&MarketplaceSettlement{}))
	require.True(t, db.Migrator().HasColumn(&MarketplaceCredential{}, "encrypted_api_key"))
}

func TestMarketplaceCredentialDoesNotMarshalEncryptedAPIKey(t *testing.T) {
	credential := MarketplaceCredential{
		ID:                 1,
		SellerUserID:       10,
		VendorType:         constant.ChannelTypeOpenAI,
		VendorNameSnapshot: "OpenAI",
		EncryptedAPIKey:    "v1:encrypted-secret",
		KeyFingerprint:     "hmac-sha256:fingerprint",
		Models:             "gpt-4o-mini",
		QuotaMode:          MarketplaceQuotaModeUnlimited,
		Multiplier:         1.2,
		ConcurrencyLimit:   3,
		ListingStatus:      MarketplaceListingStatusListed,
		ServiceStatus:      MarketplaceServiceStatusEnabled,
		HealthStatus:       MarketplaceHealthStatusHealthy,
		CapacityStatus:     MarketplaceCapacityStatusAvailable,
		RiskStatus:         MarketplaceRiskStatusNormal,
	}

	payload, err := json.Marshal(credential)
	require.NoError(t, err)

	assert.NotContains(t, string(payload), "encrypted_api_key")
	assert.NotContains(t, string(payload), "v1:encrypted-secret")
	assert.Contains(t, string(payload), "hmac-sha256:fingerprint")
}
