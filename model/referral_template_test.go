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
					DirectCapBps:   10001,
					TeamCapBps:     0,
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

func TestReferralTemplateRejectsDuplicateNameAcrossScopes(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:           "shared-template-name",
		ReferralType:   ReferralTypeSubscription,
		Group:          "vip",
		LevelType:      ReferralLevelTypeDirect,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
	}
	if err := db.Create(firstTemplate).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}

	secondTemplate := &ReferralTemplate{
		Name:           "shared-template-name",
		ReferralType:   ReferralTypeSubscription,
		Group:          "default",
		LevelType:      ReferralLevelTypeTeam,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
	}
	err := db.Create(secondTemplate).Error
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "template name already exists") {
		t.Fatalf("Create(secondTemplate) error = %v, want duplicate name error", err)
	}
}

func TestReferralTemplateRejectsRenameToDuplicateName(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:           "unique-template-a",
		ReferralType:   ReferralTypeSubscription,
		Group:          "vip",
		LevelType:      ReferralLevelTypeDirect,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
	}
	secondTemplate := &ReferralTemplate{
		Name:           "unique-template-b",
		ReferralType:   ReferralTypeSubscription,
		Group:          "vip",
		LevelType:      ReferralLevelTypeTeam,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
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
		Name:           "starter-template",
		ReferralType:   ReferralTypeSubscription,
		Group:          "starter",
		LevelType:      ReferralLevelTypeDirect,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
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

func TestReferralTemplateBindingNormalizesPersistedScopeFromTemplate(t *testing.T) {
	db := setupReferralTemplateDB(t)

	template := &ReferralTemplate{
		Name:           "vip-template",
		ReferralType:   ReferralTypeSubscription,
		Group:          "vip",
		LevelType:      ReferralLevelTypeDirect,
		DirectCapBps:   1000,
		TeamCapBps:     2000,
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
