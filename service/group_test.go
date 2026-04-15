package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func withSelectableGroupSettings(t *testing.T, usableJSON string, specialJSON string) {
	t.Helper()

	originalUsable := setting.UserUsableGroups2JSONString()
	originalSpecial := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString()

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, specialJSON); err != nil {
		t.Fatalf("failed to set special usable groups: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, originalSpecial); err != nil {
			t.Fatalf("failed to restore special usable groups: %v", err)
		}
	})
}

func withSelectableGroupSettingsAndRatios(t *testing.T, usableJSON string, specialJSON string, ratioJSON string) {
	t.Helper()

	originalRatios := ratio_setting.GroupRatio2JSONString()
	withSelectableGroupSettings(t, usableJSON, specialJSON)

	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}

	t.Cleanup(func() {
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
	})
}

func ensureSubscriptionGroupTestSchema(t *testing.T) {
	t.Helper()

	migrator := model.DB.Migrator()
	requiredColumns := []struct {
		model interface{}
		name  string
	}{
		{&model.SubscriptionPlan{}, "UpgradeGroupKey"},
		{&model.SubscriptionPlan{}, "DeletedAt"},
		{&model.UserSubscription{}, "UpgradeGroupKeySnapshot"},
		{&model.UserSubscription{}, "UpgradeGroupNameSnapshot"},
	}

	for _, column := range requiredColumns {
		if migrator.HasColumn(column.model, column.name) {
			continue
		}
		if err := migrator.AddColumn(column.model, column.name); err != nil {
			t.Fatalf("failed to add %s column for group tests: %v", column.name, err)
		}
	}
}

func setupLegacyOnlySubscriptionSchemaDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	model.InitColumnMetadata()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open legacy-only sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	ddl := []string{
		`CREATE TABLE subscription_plans (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			price_amount REAL NOT NULL DEFAULT 0,
			currency TEXT NOT NULL DEFAULT 'USD',
			duration_unit TEXT NOT NULL DEFAULT 'month',
			duration_value INTEGER NOT NULL DEFAULT 1,
			enabled NUMERIC NOT NULL DEFAULT 1,
			upgrade_group TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE user_subscriptions (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			plan_id INTEGER NOT NULL,
			amount_total INTEGER NOT NULL DEFAULT 0,
			amount_used INTEGER NOT NULL DEFAULT 0,
			start_time INTEGER NOT NULL DEFAULT 0,
			end_time INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			upgrade_group TEXT NOT NULL DEFAULT '',
			prev_user_group TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("failed to create legacy-only schema: %v", err)
		}
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		model.InitColumnMetadata()
	})

	return db
}

func seedGroupTestUser(t *testing.T, id int, group string) {
	t.Helper()

	user := &model.User{
		Id:          id,
		Username:    "group_test_user",
		Password:    "password123",
		DisplayName: "group_test_user",
		Group:       group,
		Status:      1,
		Role:        1,
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
}

func seedGroupTestSubscription(t *testing.T, userID int, upgradeGroup string, status string, endTime int64) {
	t.Helper()

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:        userID,
		PlanId:        1,
		AmountTotal:   100,
		AmountUsed:    0,
		StartTime:     now,
		EndTime:       endTime,
		Status:        status,
		Source:        "test",
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: "default",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to seed subscription: %v", err)
	}
}

func TestGetUserSelectableGroupsDoesNotAutoIncludeAssignedGroup(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	groups := GetUserSelectableGroups("default")

	if _, ok := groups["default"]; ok {
		t.Fatalf("expected assigned user group to stay hidden when it is not selectable")
	}
	if got := groups["standard"]; got != "标准价格" {
		t.Fatalf("expected standard group to remain selectable, got %q", got)
	}
}

func TestGetUserSelectableGroupsAppliesPerUserOverridesWithoutAutoFallback(t *testing.T) {
	withSelectableGroupSettings(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{"vip":{"+:exclusive":"专属分组","-:default":"默认分组"}}`,
	)

	groups := GetUserSelectableGroups("vip")

	if _, ok := groups["vip"]; ok {
		t.Fatalf("expected current user group to stay hidden when not explicitly selectable")
	}
	if _, ok := groups["default"]; ok {
		t.Fatalf("expected removed default group to stay hidden")
	}
	if got := groups["exclusive"]; got != "专属分组" {
		t.Fatalf("expected exclusive group to be added, got %q", got)
	}
}

func TestValidateTokenSelectableGroupRejectsImplicitNonSelectableUserGroup(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	err := ValidateTokenSelectableGroup("default", "")

	if err == nil {
		t.Fatalf("expected blank token group to be rejected when user group is not selectable")
	}
}

func TestResolveTokenGroupForRequestRejectsBlankFallbackWhenUserGroupIsNotSelectable(t *testing.T) {
	withSelectableGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	_, err := ResolveTokenGroupForRequest("default", "")

	if err == nil {
		t.Fatalf("expected runtime fallback to be rejected when user group is not selectable")
	}
}

func TestGetUserSelectableGroupsForUserIncludesActiveSubscriptionUpgradeGroup(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"codex-5.4":1}`,
	)
	seedGroupTestUser(t, 101, "default")
	seedGroupTestSubscription(t, 101, "codex-5.4", "active", time.Now().Add(time.Hour).Unix())

	groups := GetUserSelectableGroupsForUser(101, "default")

	if got := groups["codex-5.4"]; got != "codex-5.4" {
		t.Fatalf("expected active subscription upgrade group to be selectable, got %q", got)
	}
}

func TestValidateTokenSelectableGroupForUserAllowsSubscriptionBackedFallback(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"standard":1,"codex-5.4":1}`,
	)
	seedGroupTestUser(t, 102, "codex-5.4")
	seedGroupTestSubscription(t, 102, "codex-5.4", "active", time.Now().Add(time.Hour).Unix())

	if err := ValidateTokenSelectableGroupForUser(102, "codex-5.4", ""); err != nil {
		t.Fatalf("expected implicit fallback to active subscription group to pass, got %v", err)
	}

	resolvedGroup, err := ResolveTokenGroupForUserRequest(102, "codex-5.4", "")
	if err != nil {
		t.Fatalf("expected runtime fallback to active subscription group to pass, got %v", err)
	}
	if resolvedGroup != "codex-5.4" {
		t.Fatalf("expected resolved group codex-5.4, got %q", resolvedGroup)
	}
}

func TestGetUserSelectableGroupsForUserSkipsExpiredSubscriptionUpgradeGroup(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"codex-5.4":1}`,
	)
	seedGroupTestUser(t, 103, "default")
	seedGroupTestSubscription(t, 103, "codex-5.4", "active", time.Now().Add(-time.Hour).Unix())

	groups := GetUserSelectableGroupsForUser(103, "default")

	if _, ok := groups["codex-5.4"]; ok {
		t.Fatalf("expected expired subscription upgrade group to stay hidden")
	}
}

func TestGetUserSelectableGroupsForUserFallsBackToPlanUpgradeGroupWhenSnapshotIsInvalid(t *testing.T) {
	truncate(t)
	ensureSubscriptionGroupTestSchema(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"cc-opus4.6-福利渠道":1}`,
	)
	seedGroupTestUser(t, 104, "default")
	plan := &model.SubscriptionPlan{
		Id:            204,
		Title:         "stale-upgrade-plan",
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		UpgradeGroup:  "cc-opus4.6-福利渠道",
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}
	sub := &model.UserSubscription{
		UserId:        104,
		PlanId:        plan.Id,
		AmountTotal:   100,
		AmountUsed:    0,
		StartTime:     time.Now().Unix(),
		EndTime:       time.Now().Add(time.Hour).Unix(),
		Status:        "active",
		Source:        "test",
		UpgradeGroup:  "cc-oups4.6-福利渠道",
		PrevUserGroup: "default",
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to seed stale subscription snapshot: %v", err)
	}

	groups := GetUserSelectableGroupsForUser(104, "default")

	if _, ok := groups["cc-opus4.6-福利渠道"]; !ok {
		t.Fatalf("expected current plan upgrade group to be selectable, got %#v", groups)
	}
	if _, ok := groups["cc-oups4.6-福利渠道"]; ok {
		t.Fatalf("expected stale snapshot group to stay hidden, got %#v", groups)
	}
}

func TestResolveTokenGroupForUserRequestUsesCanonicalGroupKey(t *testing.T) {
	truncate(t)
	resetPricingGroupTestTables(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"premium":"Premium"}`,
		`{}`,
		`{"default":1,"premium":1}`,
	)

	defaultGroup := seedPricingGroup(t, "default")
	premiumGroup := seedPricingGroup(t, "premium")
	seedPricingGroupAlias(t, "legacy-default", defaultGroup.Id)
	seedPricingGroupAlias(t, "legacy-premium", premiumGroup.Id)

	resolvedGroup, err := ResolveTokenGroupForUserRequest(0, "legacy-default", "legacy-premium")
	if err != nil {
		t.Fatalf("expected alias-backed token group to resolve via canonical key, got %v", err)
	}
	if resolvedGroup != "premium" {
		t.Fatalf("expected resolved group premium, got %q", resolvedGroup)
	}
}

func TestResolveTokenGroupForUserTokenUsesAutoSelectionModeWithoutLegacyAutoGroup(t *testing.T) {
	truncate(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"default":"Default","auto":"Auto","premium":"Premium"}`,
		`{}`,
		`{"default":1,"premium":1}`,
	)

	resolvedGroup, err := ResolveTokenGroupForUserToken(0, "default", &model.Token{
		SelectionMode: "auto",
		Group:         "",
		GroupKey:      "",
	})
	if err != nil {
		t.Fatalf("expected auto selection mode to resolve without legacy auto group, got %v", err)
	}
	if resolvedGroup != "auto" {
		t.Fatalf("expected resolved group auto, got %q", resolvedGroup)
	}
}

func TestValidateTokenSelectableGroupForUserUsesCanonicalGroupKey(t *testing.T) {
	truncate(t)
	resetPricingGroupTestTables(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"premium":"Premium"}`,
		`{}`,
		`{"default":1,"premium":1}`,
	)

	defaultGroup := seedPricingGroup(t, "default")
	premiumGroup := seedPricingGroup(t, "premium")
	seedPricingGroupAlias(t, "legacy-default", defaultGroup.Id)
	seedPricingGroupAlias(t, "legacy-premium", premiumGroup.Id)

	if err := ValidateTokenSelectableGroupForUser(0, "legacy-default", "legacy-premium"); err != nil {
		t.Fatalf("expected alias-backed token group validation to pass via canonical key, got %v", err)
	}
}

func TestGetUserSelectableGroupsForUserIncludesCanonicalSubscriptionUpgradeGroup(t *testing.T) {
	truncate(t)
	ensureSubscriptionGroupTestSchema(t)
	withSelectableGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格"}`,
		`{}`,
		`{"default":1,"standard":1,"premium":1}`,
	)
	seedGroupTestUser(t, 105, "default")
	plan := &model.SubscriptionPlan{
		Id:              205,
		Title:           "canonical-upgrade-plan",
		PriceAmount:     9.9,
		Currency:        "USD",
		DurationUnit:    model.SubscriptionDurationMonth,
		DurationValue:   1,
		Enabled:         true,
		UpgradeGroup:    "legacy-premium",
		UpgradeGroupKey: "premium",
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}
	sub := &model.UserSubscription{
		UserId:                   105,
		PlanId:                   plan.Id,
		AmountTotal:              100,
		AmountUsed:               0,
		StartTime:                time.Now().Unix(),
		EndTime:                  time.Now().Add(time.Hour).Unix(),
		Status:                   "active",
		Source:                   "test",
		UpgradeGroup:             "legacy-stale",
		UpgradeGroupKeySnapshot:  "premium",
		UpgradeGroupNameSnapshot: "Premium",
		PrevUserGroup:            "default",
		CreatedAt:                time.Now().Unix(),
		UpdatedAt:                time.Now().Unix(),
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to seed canonical subscription snapshot: %v", err)
	}

	groups := GetUserSelectableGroupsForUser(105, "default")

	if got := groups["premium"]; got != "premium" {
		t.Fatalf("expected canonical subscription upgrade group to be selectable, got %q from %#v", got, groups)
	}
	if _, ok := groups["legacy-stale"]; ok {
		t.Fatalf("expected legacy snapshot string to stay hidden, got %#v", groups)
	}
}

func TestGetActiveUserSubscriptionUpgradeGroupsFallsBackWithoutCanonicalColumns(t *testing.T) {
	db := setupLegacyOnlySubscriptionSchemaDB(t)

	now := time.Now().Unix()
	if err := db.Exec(
		"INSERT INTO subscription_plans (id, title, price_amount, currency, duration_unit, duration_value, enabled, upgrade_group) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		301,
		"legacy-plan",
		9.9,
		"USD",
		model.SubscriptionDurationMonth,
		1,
		true,
		"legacy-premium",
	).Error; err != nil {
		t.Fatalf("failed to seed legacy-only plan: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO user_subscriptions (id, user_id, plan_id, amount_total, amount_used, start_time, end_time, status, source, upgrade_group, prev_user_group, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		401,
		501,
		301,
		100,
		0,
		now,
		now+3600,
		"active",
		"legacy",
		"",
		"default",
		now,
		now,
	).Error; err != nil {
		t.Fatalf("failed to seed legacy-only subscription: %v", err)
	}

	groups, err := model.GetActiveUserSubscriptionUpgradeGroups(501)
	if err != nil {
		t.Fatalf("expected legacy-only schema to fall back without error, got %v", err)
	}
	if len(groups) != 1 || groups[0] != "legacy-premium" {
		t.Fatalf("expected legacy-only fallback group legacy-premium, got %#v", groups)
	}
}
