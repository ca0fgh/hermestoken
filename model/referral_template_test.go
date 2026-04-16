package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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
		&User{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
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

func TestUpsertReferralInviteeShareOverrideRequiresActiveBinding(t *testing.T) {
	db := setupReferralTemplateDB(t)
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("failed to migrate users: %v", err)
	}

	inviter := &User{
		Username: "referral_inviter",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "referral_inviter_code",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}

	invitee := &User{
		Username:  "referral_invitee",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		InviterId: inviter.Id,
		AffCode:   "referral_invitee_code",
	}
	if err := db.Create(invitee).Error; err != nil {
		t.Fatalf("failed to create invitee: %v", err)
	}

	_, err := UpsertReferralInviteeShareOverride(inviter.Id, invitee.Id, ReferralTypeSubscription, "vip", 1200, inviter.Id)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "active binding") {
		t.Fatalf("UpsertReferralInviteeShareOverride() error = %v, want active binding validation", err)
	}

	template := &ReferralTemplate{
		Name:                   "active-template",
		ReferralType:           ReferralTypeSubscription,
		Group:                  "vip",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1000,
		TeamCapBps:             2000,
		TeamDecayRatio:         0.5,
		TeamMaxDepth:           2,
		InviteeShareDefaultBps: 500,
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	binding := &ReferralTemplateBinding{
		UserId:       inviter.Id,
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		TemplateId:   template.Id,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("failed to create binding: %v", err)
	}

	override, err := UpsertReferralInviteeShareOverride(inviter.Id, invitee.Id, ReferralTypeSubscription, "vip", 1200, inviter.Id)
	if err != nil {
		t.Fatalf("UpsertReferralInviteeShareOverride() with active binding error = %v", err)
	}
	if override.InviteeShareBps != 1200 {
		t.Fatalf("InviteeShareBps = %d, want 1200", override.InviteeShareBps)
	}
}

func TestListLegacySubscriptionReferralSeedRowsReturnsOverrideAndInviteeSeeds(t *testing.T) {
	db := setupReferralTemplateDB(t)
	if err := db.AutoMigrate(&User{}, &SubscriptionReferralOverride{}, &SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("failed to migrate legacy referral tables: %v", err)
	}

	inviter := &User{
		Username: "legacy_seed_inviter",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "legacy_seed_inviter_code",
	}
	inviter.SetSetting(dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 900},
	})
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}

	invitee := &User{
		Username:  "legacy_seed_invitee",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		InviterId: inviter.Id,
		AffCode:   "legacy_seed_invitee_code",
	}
	if err := db.Create(invitee).Error; err != nil {
		t.Fatalf("failed to create invitee: %v", err)
	}

	override := &SubscriptionReferralOverride{
		UserId:       inviter.Id,
		Group:        "vip",
		TotalRateBps: 3200,
		CreatedBy:    inviter.Id,
		UpdatedBy:    inviter.Id,
	}
	if err := db.Create(override).Error; err != nil {
		t.Fatalf("failed to create legacy override: %v", err)
	}

	inviteeOverride := &SubscriptionReferralInviteeOverride{
		InviterUserId:  inviter.Id,
		InviteeUserId:  invitee.Id,
		Group:          "vip",
		InviteeRateBps: 700,
		CreatedBy:      inviter.Id,
		UpdatedBy:      inviter.Id,
	}
	if err := db.Create(inviteeOverride).Error; err != nil {
		t.Fatalf("failed to create legacy invitee override: %v", err)
	}

	seeds, err := ListLegacySubscriptionReferralSeedRows("vip")
	if err != nil {
		t.Fatalf("ListLegacySubscriptionReferralSeedRows() error = %v", err)
	}
	if seeds == nil {
		t.Fatal("ListLegacySubscriptionReferralSeedRows() returned nil")
	}
	if len(seeds.OverrideSeeds) != 1 {
		t.Fatalf("OverrideSeeds length = %d, want 1", len(seeds.OverrideSeeds))
	}
	if len(seeds.InviteeOverrideSeeds) != 1 {
		t.Fatalf("InviteeOverrideSeeds length = %d, want 1", len(seeds.InviteeOverrideSeeds))
	}
	if got := seeds.InviteeRateSeeds[inviter.Id]; got != 900 {
		t.Fatalf("InviteeRateSeeds[%d] = %d, want 900", inviter.Id, got)
	}
}
