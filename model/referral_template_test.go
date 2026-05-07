package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
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
	originalSubscriptionReferralTeamDecayRatio := subscriptionReferralTeamDecayRatio
	originalSubscriptionReferralTeamMaxDepth := subscriptionReferralTeamMaxDepth
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}

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
		&Option{},
		&User{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&ReferralTemplate{},
		&ReferralTemplateBinding{},
		&ReferralInviteeShareOverride{},
		&ReferralSettlementBatch{},
		&ReferralSettlementRecord{},
	); err != nil {
		t.Fatalf("failed to migrate referral template tables: %v", err)
	}
	InitOptionMap()

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
		subscriptionReferralTeamDecayRatio = originalSubscriptionReferralTeamDecayRatio
		subscriptionReferralTeamMaxDepth = originalSubscriptionReferralTeamMaxDepth
		common.OptionMap = originalOptionMap
	})

	return db
}

func sqliteSchemaVersion(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var version int64
	row := db.Raw("PRAGMA schema_version").Row()
	if err := row.Scan(&version); err != nil {
		t.Fatalf("lookup sqlite schema version: %v", err)
	}
	return version
}

func sqliteIndexRowID(t *testing.T, db *gorm.DB, indexName string) int64 {
	t.Helper()

	var rowID int64
	row := db.Raw("SELECT rowid FROM sqlite_master WHERE type = ? AND name = ?", "index", indexName).Row()
	if err := row.Scan(&rowID); err != nil {
		t.Fatalf("lookup sqlite_master rowid for index %s: %v", indexName, err)
	}
	return rowID
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
				Name:         "invalid-level-type",
				ReferralType: ReferralTypeSubscription,
				Group:        "starter",
				LevelType:    "matrix",
				DirectCapBps: 1000,
				TeamCapBps:   2000,
			},
			wantErr: "level type",
		},
		{
			name: "rejects caps outside subscription bounds",
			template: ReferralTemplate{
				Name:         "invalid-caps",
				ReferralType: ReferralTypeSubscription,
				Group:        "starter",
				LevelType:    ReferralLevelTypeDirect,
				DirectCapBps: 10001,
				TeamCapBps:   0,
			},
			wantErr: "cap bps",
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

func TestReferralTemplateBindingDerivesScopeFromTemplate(t *testing.T) {
	db := setupReferralTemplateDB(t)

	template := &ReferralTemplate{
		Name:                   "binding-derived-scope-template",
		ReferralType:           ReferralTypeSubscription,
		Group:                  "vip",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1000,
		InviteeShareDefaultBps: 600,
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	user := &User{
		Username: "binding-derived-scope-user",
		Password: "password",
		AffCode:  "binding_derived_scope",
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	binding, err := UpsertReferralTemplateBinding(&ReferralTemplateBinding{
		UserId:       user.Id,
		ReferralType: "",
		Group:        "",
		TemplateId:   template.Id,
		CreatedBy:    1,
		UpdatedBy:    1,
	})
	if err != nil {
		t.Fatalf("UpsertReferralTemplateBinding() error = %v", err)
	}

	if binding.ReferralType != ReferralTypeSubscription {
		t.Fatalf("ReferralType = %q, want %q", binding.ReferralType, ReferralTypeSubscription)
	}
	if binding.Group != "vip" {
		t.Fatalf("Group = %q, want vip", binding.Group)
	}
}

func TestAssignLowestSubscriptionReferralTemplateForInvitedUserRequiresInviterActiveBinding(t *testing.T) {
	db := setupReferralTemplateDB(t)

	lowestRows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"starter", "pro"},
		Name:                   "lowest-cap-template",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           800,
		InviteeShareDefaultBps: 200,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle(lowest) error = %v", err)
	}
	higherRows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"starter", "pro"},
		Name:                   "higher-cap-template",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1500,
		InviteeShareDefaultBps: 200,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle(higher) error = %v", err)
	}

	inviter := &User{
		Username: "sub-ref-inviter",
		Password: "password",
		AffCode:  "sub_ref_inviter",
		Group:    "default",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}
	if _, err := UpsertReferralTemplateBindingBundleForUser(inviter.Id, ReferralTypeSubscription, higherRows[0].Id, nil, 1); err != nil {
		t.Fatalf("failed to bind inviter: %v", err)
	}

	invitee := &User{
		Username: "sub-ref-invitee",
		Password: "password123",
		Group:    "default",
	}
	if err := invitee.Insert(inviter.Id); err != nil {
		t.Fatalf("Insert(invited user) error = %v", err)
	}

	bundles, err := ListReferralTemplateBindingBundlesByUser(invitee.Id, ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingBundlesByUser(invitee) error = %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected invitee to receive one subscription referral bundle, got %d", len(bundles))
	}
	if bundles[0].BundleKey != lowestRows[0].BundleKey {
		t.Fatalf("expected invitee bundle %q, got %q", lowestRows[0].BundleKey, bundles[0].BundleKey)
	}
	if len(bundles[0].TemplateIDs) != len(lowestRows) {
		t.Fatalf("expected invitee to bind all lowest bundle rows, got template ids %v", bundles[0].TemplateIDs)
	}
}

func TestAssignLowestSubscriptionReferralTemplateForInvitedUserSkipsInviterWithoutActiveBinding(t *testing.T) {
	db := setupReferralTemplateDB(t)

	_, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"starter"},
		Name:                   "available-template",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           800,
		InviteeShareDefaultBps: 200,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}

	inviter := &User{
		Username: "no-sub-ref-inviter",
		Password: "password",
		AffCode:  "no_sub_ref_inviter",
		Group:    "default",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}

	invitee := &User{
		Username: "no-sub-ref-invitee",
		Password: "password123",
		Group:    "default",
	}
	if err := invitee.Insert(inviter.Id); err != nil {
		t.Fatalf("Insert(invited user) error = %v", err)
	}

	bundles, err := ListReferralTemplateBindingBundlesByUser(invitee.Id, ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingBundlesByUser(invitee) error = %v", err)
	}
	if len(bundles) != 0 {
		t.Fatalf("expected invitee to receive no subscription referral bundle, got %d", len(bundles))
	}
}

func TestAssignLowestSubscriptionReferralTemplateForInvitedUserSortsByCapBeforeType(t *testing.T) {
	db := setupReferralTemplateDB(t)

	directRows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"starter"},
		Name:                   "direct-higher-cap-template",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 200,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle(direct) error = %v", err)
	}
	teamRows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"starter"},
		Name:                   "team-lower-cap-template",
		LevelType:              ReferralLevelTypeTeam,
		Enabled:                true,
		TeamCapBps:             500,
		InviteeShareDefaultBps: 200,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle(team) error = %v", err)
	}

	inviter := &User{
		Username: "sort-cap-inviter",
		Password: "password",
		AffCode:  "sort_cap_inviter",
		Group:    "default",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}
	if _, err := UpsertReferralTemplateBindingBundleForUser(inviter.Id, ReferralTypeSubscription, directRows[0].Id, nil, 1); err != nil {
		t.Fatalf("failed to bind inviter: %v", err)
	}

	invitee := &User{
		Username: "sort-cap-invitee",
		Password: "password123",
		Group:    "default",
	}
	if err := invitee.Insert(inviter.Id); err != nil {
		t.Fatalf("Insert(invited user) error = %v", err)
	}

	bundles, err := ListReferralTemplateBindingBundlesByUser(invitee.Id, ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingBundlesByUser(invitee) error = %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected invitee to receive one subscription referral bundle, got %d", len(bundles))
	}
	if bundles[0].BundleKey != teamRows[0].BundleKey {
		t.Fatalf("expected cap-sorted bundle %q, got %q", teamRows[0].BundleKey, bundles[0].BundleKey)
	}
}

func TestAssignLowestSubscriptionReferralTemplateForInvitedUserRunsInsideInsertWithTx(t *testing.T) {
	db := setupReferralTemplateDB(t)

	rows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"starter"},
		Name:                   "tx-lowest-cap-template",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           700,
		InviteeShareDefaultBps: 200,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}

	inviter := &User{
		Username: "tx-sub-ref-inviter",
		Password: "password",
		AffCode:  "tx_sub_ref_inviter",
		Group:    "default",
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}
	if _, err := UpsertReferralTemplateBindingBundleForUser(inviter.Id, ReferralTypeSubscription, rows[0].Id, nil, 1); err != nil {
		t.Fatalf("failed to bind inviter: %v", err)
	}

	invitee := &User{
		Username: "tx-sub-ref-invitee",
		Password: "password123",
		Group:    "default",
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("failed to begin transaction: %v", tx.Error)
	}
	if err := invitee.InsertWithTx(tx, inviter.Id); err != nil {
		_ = tx.Rollback()
		t.Fatalf("InsertWithTx(invited user) error = %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	bundles, err := ListReferralTemplateBindingBundlesByUser(invitee.Id, ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingBundlesByUser(invitee) error = %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected tx invitee to receive one subscription referral bundle, got %d", len(bundles))
	}
	if bundles[0].BundleKey != rows[0].BundleKey {
		t.Fatalf("expected tx invitee bundle %q, got %q", rows[0].BundleKey, bundles[0].BundleKey)
	}
}

func TestResolveBindingInviteeShareDefaultUsesTemplateDefaultOnly(t *testing.T) {
	view := ReferralTemplateBindingView{
		Template: ReferralTemplate{
			InviteeShareDefaultBps: 700,
		},
	}

	if got := ResolveBindingInviteeShareDefault(view); got != 700 {
		t.Fatalf("ResolveBindingInviteeShareDefault() = %d, want 700", got)
	}
}

func TestSubscriptionReferralGlobalSettingDefaults(t *testing.T) {
	setupReferralTemplateDB(t)

	setting := GetSubscriptionReferralGlobalSetting()
	if setting.TeamDecayRatio != DefaultSubscriptionReferralTeamDecayRatio {
		t.Fatalf("TeamDecayRatio = %v, want %v", setting.TeamDecayRatio, DefaultSubscriptionReferralTeamDecayRatio)
	}
	if setting.TeamMaxDepth != DefaultSubscriptionReferralTeamMaxDepth {
		t.Fatalf("TeamMaxDepth = %d, want %d", setting.TeamMaxDepth, DefaultSubscriptionReferralTeamMaxDepth)
	}
}

func TestSubscriptionReferralGlobalSettingRejectsInvalidValues(t *testing.T) {
	setupReferralTemplateDB(t)

	err := UpdateSubscriptionReferralGlobalSetting(SubscriptionReferralGlobalSetting{
		TeamDecayRatio: 0,
		TeamMaxDepth:   0,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "decay ratio") {
		t.Fatalf("UpdateSubscriptionReferralGlobalSetting() error = %v, want decay ratio validation", err)
	}

	err = UpdateSubscriptionReferralGlobalSetting(SubscriptionReferralGlobalSetting{
		TeamDecayRatio: 0.5,
		TeamMaxDepth:   -1,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "max depth") {
		t.Fatalf("UpdateSubscriptionReferralGlobalSetting() error = %v, want max depth validation", err)
	}
}

func TestReferralTemplateRejectsEmptyGroupScope(t *testing.T) {
	db := setupReferralTemplateDB(t)

	template := &ReferralTemplate{
		Name:                   "missing-group-template",
		ReferralType:           ReferralTypeSubscription,
		Group:                  "",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1000,
		TeamCapBps:             2500,
		InviteeShareDefaultBps: 0,
	}
	err := db.Create(template).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "group") {
		t.Fatalf("Create(template) error = %v, want group required error", err)
	}
}

func TestReferralTemplateAllowsDuplicateNameAcrossDifferentGroups(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
		TeamCapBps:   2000,
	}
	if err := db.Create(firstTemplate).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}

	secondTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "default",
		LevelType:    ReferralLevelTypeTeam,
		DirectCapBps: 1000,
		TeamCapBps:   2000,
	}
	if err := db.Create(secondTemplate).Error; err != nil {
		t.Fatalf("Create(secondTemplate) error = %v, want success across groups", err)
	}
}

func TestReferralTemplateAllowsDuplicateNameAcrossDifferentReferralTypes(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
	}
	if err := db.Create(firstTemplate).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}

	secondTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: "commission_referral",
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(secondTemplate).Error; err != nil {
		t.Fatalf("Create(secondTemplate) error = %v, want success across referral types", err)
	}
}

func TestReferralTemplateRejectsDuplicateNameWithinSameScope(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
	}
	if err := db.Create(firstTemplate).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}

	secondTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeTeam,
		TeamCapBps:   2000,
	}
	err := normalizeReferralTemplatePersistenceError(db.Session(&gorm.Session{SkipHooks: true}).Create(secondTemplate).Error)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "template name already exists") {
		t.Fatalf("Create(secondTemplate) error = %v, want duplicate name error", err)
	}
}

func TestCreateReferralTemplateBundleCreatesOneRowPerGroup(t *testing.T) {
	setupReferralTemplateDB(t)

	rows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"vip", "default"},
		Name:                   "starter-direct",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 300,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("bundle row count = %d, want 2", len(rows))
	}
	if rows[0].BundleKey == "" || rows[1].BundleKey == "" || rows[0].BundleKey != rows[1].BundleKey {
		t.Fatalf("expected a shared bundle key, got %#v", rows)
	}
	if rows[0].Group != "default" || rows[1].Group != "vip" {
		t.Fatalf("unexpected groups = [%q %q], want [default vip]", rows[0].Group, rows[1].Group)
	}
}

func TestListReferralTemplateBundlesAggregatesRowsByBundleKey(t *testing.T) {
	setupReferralTemplateDB(t)

	created, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType: ReferralTypeSubscription,
		Groups:       []string{"default", "vip"},
		Name:         "bundle-team",
		LevelType:    ReferralLevelTypeTeam,
		Enabled:      true,
		TeamCapBps:   2600,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}

	bundles, err := ListReferralTemplateBundles(ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBundles() error = %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundle count = %d, want 1", len(bundles))
	}
	if bundles[0].BundleKey != created[0].BundleKey {
		t.Fatalf("BundleKey = %q, want %q", bundles[0].BundleKey, created[0].BundleKey)
	}
	if strings.Join(bundles[0].Groups, ",") != "default,vip" {
		t.Fatalf("Groups = %#v, want [default vip]", bundles[0].Groups)
	}
	if len(bundles[0].TemplateIDs) != 2 {
		t.Fatalf("TemplateIDs count = %d, want 2", len(bundles[0].TemplateIDs))
	}
}

func TestBackfillReferralTemplateBundleKeysPopulatesLegacyRows(t *testing.T) {
	db := setupReferralTemplateDB(t)

	legacy := &ReferralTemplate{
		ReferralType: ReferralTypeSubscription,
		Group:        "legacy",
		Name:         "legacy-row",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy template: %v", err)
	}
	if err := db.Model(&ReferralTemplate{}).Where("id = ?", legacy.Id).UpdateColumn("bundle_key", "").Error; err != nil {
		t.Fatalf("clear bundle key: %v", err)
	}

	if err := BackfillReferralTemplateBundleKeys(); err != nil {
		t.Fatalf("BackfillReferralTemplateBundleKeys() error = %v", err)
	}

	reloaded, err := GetReferralTemplateByID(legacy.Id)
	if err != nil {
		t.Fatalf("reload legacy template: %v", err)
	}
	wantBundleKey := fmt.Sprintf("template:%d", legacy.Id)
	if reloaded.BundleKey != wantBundleKey {
		t.Fatalf("BundleKey = %q, want %q", reloaded.BundleKey, wantBundleKey)
	}
}

func TestEnsureReferralTemplateSchemaCreatesBundleKeyIndex(t *testing.T) {
	db := setupReferralTemplateDB(t)

	const bundleKeyIndex = "idx_referral_templates_bundle_key"
	if !db.Migrator().HasIndex(&ReferralTemplate{}, bundleKeyIndex) {
		t.Fatalf("expected %s to exist after automigrate", bundleKeyIndex)
	}
	if err := db.Migrator().DropIndex(&ReferralTemplate{}, bundleKeyIndex); err != nil {
		t.Fatalf("DropIndex(%s) error = %v", bundleKeyIndex, err)
	}
	if db.Migrator().HasIndex(&ReferralTemplate{}, bundleKeyIndex) {
		t.Fatalf("expected %s to be dropped before ensureReferralTemplateSchema", bundleKeyIndex)
	}

	if err := ensureReferralTemplateSchema(); err != nil {
		t.Fatalf("ensureReferralTemplateSchema() error = %v", err)
	}
	if !db.Migrator().HasIndex(&ReferralTemplate{}, bundleKeyIndex) {
		t.Fatalf("expected %s to be recreated", bundleKeyIndex)
	}
}

func TestEnsureReferralTemplateSchemaIsIdempotent(t *testing.T) {
	db := setupReferralTemplateDB(t)

	if err := ensureReferralTemplateSchema(); err != nil {
		t.Fatalf("first ensureReferralTemplateSchema() error = %v", err)
	}
	firstSchemaVersion := sqliteSchemaVersion(t, db)
	firstScopedIndexRowID := sqliteIndexRowID(t, db, "uk_referral_template_scope_name")
	if err := ensureReferralTemplateSchema(); err != nil {
		t.Fatalf("second ensureReferralTemplateSchema() error = %v", err)
	}

	if !db.Migrator().HasIndex(&ReferralTemplate{}, "uk_referral_template_scope_name") {
		t.Fatal("expected unique scope+name index to remain present after repeated schema ensure")
	}
	secondScopedIndexRowID := sqliteIndexRowID(t, db, "uk_referral_template_scope_name")
	if secondScopedIndexRowID != firstScopedIndexRowID {
		t.Fatalf("expected repeated schema ensure to keep existing scoped unique index row, got rowid %d then %d", firstScopedIndexRowID, secondScopedIndexRowID)
	}
	secondSchemaVersion := sqliteSchemaVersion(t, db)
	if secondSchemaVersion != firstSchemaVersion {
		t.Fatalf("expected repeated schema ensure to avoid schema changes, got schema_version %d then %d", firstSchemaVersion, secondSchemaVersion)
	}
	if db.Migrator().HasIndex(&ReferralTemplate{}, "uk_referral_template_name") {
		t.Fatal("expected legacy unique name index to stay absent after repeated schema ensure")
	}
}

func TestEnsureReferralTemplateSchemaDropsLegacyGlobalNameIndex(t *testing.T) {
	db := setupReferralTemplateDB(t)

	if err := db.Exec("CREATE UNIQUE INDEX idx_referral_templates_name ON referral_templates(name)").Error; err != nil {
		t.Fatalf("create legacy global name index: %v", err)
	}
	if !db.Migrator().HasIndex(&ReferralTemplate{}, "idx_referral_templates_name") {
		t.Fatal("expected legacy global name index to exist before ensureReferralTemplateSchema")
	}

	if err := ensureReferralTemplateSchema(); err != nil {
		t.Fatalf("ensureReferralTemplateSchema() error = %v", err)
	}
	if db.Migrator().HasIndex(&ReferralTemplate{}, "idx_referral_templates_name") {
		t.Fatal("expected ensureReferralTemplateSchema to drop legacy global name index")
	}
}

func TestNormalizeReferralTemplatePersistenceErrorHandlesLegacyGlobalNameIndex(t *testing.T) {
	err := normalizeReferralTemplatePersistenceError(errors.New(`ERROR: duplicate key value violates unique constraint "idx_referral_templates_name" (SQLSTATE 23505)`))
	if err == nil {
		t.Fatal("expected normalized error, got nil")
	}
	if err.Error() != "template name already exists" {
		t.Fatalf("normalized error = %q, want %q", err.Error(), "template name already exists")
	}
}

func TestReferralTemplateRejectsRenameToDuplicateName(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:         "unique-template-a",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
		TeamCapBps:   2000,
	}
	secondTemplate := &ReferralTemplate{
		Name:         "unique-template-b",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeTeam,
		DirectCapBps: 1000,
		TeamCapBps:   2000,
	}
	if err := db.Create(firstTemplate).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}
	if err := db.Create(secondTemplate).Error; err != nil {
		t.Fatalf("failed to create second template: %v", err)
	}

	secondTemplate.Name = "unique-template-a"
	err := db.Save(secondTemplate).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "template name already exists") {
		t.Fatalf("Save(secondTemplate) error = %v, want duplicate name error", err)
	}
}

func TestReferralTemplateBindingUsesTemplateScopeWhenCreating(t *testing.T) {
	db := setupReferralTemplateDB(t)

	template := &ReferralTemplate{
		Name:         "starter-template",
		ReferralType: ReferralTypeSubscription,
		Group:        "starter",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
		TeamCapBps:   2000,
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	user := &User{
		Username: "binding_scope_template_user",
		Password: "password",
		AffCode:  "binding_scope_template_user_code",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	binding := &ReferralTemplateBinding{
		UserId:       user.Id,
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		TemplateId:   template.Id,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("Create(binding) error = %v", err)
	}
	if binding.Group != "starter" {
		t.Fatalf("binding.Group = %q, want starter", binding.Group)
	}
}

func TestReferralTemplateBindingIgnoresMissingTemplateRows(t *testing.T) {
	db := setupReferralTemplateDB(t)

	user := &User{
		Username: "stale-binding-user",
		Password: "password",
		AffCode:  "stale_binding_user",
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	activeTemplate := &ReferralTemplate{
		Name:                   "active-stale-binding-template",
		ReferralType:           ReferralTypeSubscription,
		Group:                  "vip",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1000,
		InviteeShareDefaultBps: 500,
	}
	if err := db.Create(activeTemplate).Error; err != nil {
		t.Fatalf("failed to create active template: %v", err)
	}

	staleBinding := &ReferralTemplateBinding{
		UserId:       user.Id,
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		TemplateId:   activeTemplate.Id,
		CreatedBy:    1,
		UpdatedBy:    1,
	}
	if err := db.Create(staleBinding).Error; err != nil {
		t.Fatalf("failed to create binding: %v", err)
	}

	if err := db.Delete(&ReferralTemplate{}, activeTemplate.Id).Error; err != nil {
		t.Fatalf("failed to delete bound template: %v", err)
	}

	active, binding, err := HasActiveReferralTemplateBinding(user.Id, ReferralTypeSubscription, "vip")
	if err != nil {
		t.Fatalf("HasActiveReferralTemplateBinding() error = %v", err)
	}
	if active {
		t.Fatal("expected missing template binding to be inactive")
	}
	if binding == nil {
		t.Fatal("expected binding metadata even when template row is missing")
	}

	views, err := ListReferralTemplateBindingsByUser(user.Id, ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBindingsByUser() error = %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("binding view count = %d, want 0 when template row is missing", len(views))
	}

	view, err := GetReferralTemplateBindingViewByUserAndScope(user.Id, ReferralTypeSubscription, "vip")
	if err != nil {
		t.Fatalf("GetReferralTemplateBindingViewByUserAndScope() error = %v", err)
	}
	if view != nil {
		t.Fatal("expected nil binding view when template row is missing")
	}
}

func TestReferralTemplateBindingNormalizesPersistedScopeFromTemplate(t *testing.T) {
	db := setupReferralTemplateDB(t)

	template := &ReferralTemplate{
		Name:         "vip-template",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
		TeamCapBps:   2000,
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	user := &User{
		Username: "binding_scope_persisted_user",
		Password: "password",
		AffCode:  "binding_scope_persisted_user_code",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	binding := &ReferralTemplateBinding{
		UserId:       user.Id,
		ReferralType: ReferralTypeSubscription,
		Group:        "default",
		TemplateId:   template.Id,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatalf("Create(binding) error = %v", err)
	}

	view, err := GetReferralTemplateBindingViewByUserAndScope(user.Id, ReferralTypeSubscription, "vip")
	if err != nil {
		t.Fatalf("GetReferralTemplateBindingViewByUserAndScope() error = %v", err)
	}
	if view == nil {
		t.Fatal("expected binding view")
	}
	if view.Binding.Group != "vip" {
		t.Fatalf("binding.Group = %q, want vip", view.Binding.Group)
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
