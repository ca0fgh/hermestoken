package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupReferralTemplateDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	originalCommonGroupCol := commonGroupCol

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	commonGroupCol = "`group`"

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(
		&ReferralTemplate{},
		&ReferralTemplateBinding{},
		&ReferralInviteeShareOverride{},
		&ReferralEngineRoute{},
		&ReferralSettlementBatch{},
		&ReferralSettlementRecord{},
	); err != nil {
		t.Fatalf("failed to migrate referral template tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
		commonGroupCol = originalCommonGroupCol
	})

	return db
}

func TestReferralTemplateRejectsInvalidSubscriptionRules(t *testing.T) {
	db := setupReferralTemplateDB(t)

	testCases := []struct {
		name     string
		template ReferralTemplate
		wantErr  string
	}{
		{
			name: "rejects unsupported level type",
			template: ReferralTemplate{
				Name:           "invalid-level-type",
				ReferralType:   ReferralTypeSubscription,
				Group:          "starter",
				LevelType:      "matrix",
				DirectCapBps:   1000,
				TeamCapBps:     2000,
				TeamDecayRatio: 0.5,
				TeamMaxDepth:   3,
			},
			wantErr: "level type",
		},
		{
			name: "rejects caps outside subscription bounds",
			template: ReferralTemplate{
				Name:           "invalid-caps",
				ReferralType:   ReferralTypeSubscription,
				Group:          "starter",
				LevelType:      ReferralLevelTypeDirect,
				DirectCapBps:   2500,
				TeamCapBps:     2000,
				TeamDecayRatio: 0.5,
				TeamMaxDepth:   2,
			},
			wantErr: "cap bps",
		},
		{
			name: "rejects decay ratio outside range",
			template: ReferralTemplate{
				Name:           "invalid-decay",
				ReferralType:   ReferralTypeSubscription,
				Group:          "starter",
				LevelType:      ReferralLevelTypeTeam,
				DirectCapBps:   1000,
				TeamCapBps:     2000,
				TeamDecayRatio: 0,
				TeamMaxDepth:   2,
			},
			wantErr: "decay ratio",
		},
		{
			name: "rejects team depth below one",
			template: ReferralTemplate{
				Name:           "invalid-depth",
				ReferralType:   ReferralTypeSubscription,
				Group:          "starter",
				LevelType:      ReferralLevelTypeTeam,
				DirectCapBps:   1000,
				TeamCapBps:     2000,
				TeamDecayRatio: 0.5,
				TeamMaxDepth:   0,
			},
			wantErr: "max depth",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.Create(&tc.template).Error
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.wantErr) {
				t.Fatalf("Create() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestReferralTemplateBindingRejectsCrossDimensionTemplate(t *testing.T) {
	db := setupReferralTemplateDB(t)

	template := &ReferralTemplate{
		Name:           "starter-template",
		ReferralType:   ReferralTypeSubscription,
		Group:          "starter",
		LevelType:      ReferralLevelTypeDirect,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
		TeamDecayRatio: 0.5,
		TeamMaxDepth:   2,
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	binding := &ReferralTemplateBinding{
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		TemplateId:   template.Id,
	}
	err := db.Create(binding).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "group") {
		t.Fatalf("Create(binding) error = %v, want group mismatch error", err)
	}
}

func TestResolveReferralEngineModeDefaultsToLegacy(t *testing.T) {
	setupReferralTemplateDB(t)

	mode, err := ResolveReferralEngineMode(ReferralTypeSubscription, "starter")
	if err != nil {
		t.Fatalf("ResolveReferralEngineMode() error = %v", err)
	}
	if mode != ReferralEngineModeLegacy {
		t.Fatalf("ResolveReferralEngineMode() = %q, want %q", mode, ReferralEngineModeLegacy)
	}
}
