package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type userSchemaWithoutGroupDefault struct {
	Id                  int            `gorm:"column:id;primaryKey;autoIncrement"`
	Username            string         `gorm:"column:username;type:text;uniqueIndex"`
	Password            string         `gorm:"column:password;type:text;not null"`
	OriginalPassword    string         `gorm:"-:all"`
	DisplayName         string         `gorm:"column:display_name;type:text"`
	Role                int            `gorm:"column:role;type:int;default:1"`
	Status              int            `gorm:"column:status;type:int;default:1"`
	Email               string         `gorm:"column:email;type:text"`
	GitHubId            string         `gorm:"column:github_id;type:text"`
	DiscordId           string         `gorm:"column:discord_id;type:text"`
	OidcId              string         `gorm:"column:oidc_id;type:text"`
	WeChatId            string         `gorm:"column:wechat_id;type:text"`
	TelegramId          string         `gorm:"column:telegram_id;type:text"`
	AccessToken         *string        `gorm:"column:access_token;type:text"`
	Quota               int            `gorm:"column:quota;type:int;default:0"`
	WithdrawFrozenQuota int            `gorm:"column:withdraw_frozen_quota;type:int;default:0"`
	UsedQuota           int            `gorm:"column:used_quota;type:int;default:0"`
	RequestCount        int            `gorm:"column:request_count;type:int;default:0"`
	Group               string         `gorm:"column:group;type:varchar(64);not null"`
	AffCode             string         `gorm:"column:aff_code;type:text"`
	AffCount            int            `gorm:"column:aff_count;type:int;default:0"`
	AffQuota            int            `gorm:"column:aff_quota;type:int;default:0"`
	AffHistoryQuota     int            `gorm:"column:aff_history;type:int;default:0"`
	InviterId           int            `gorm:"column:inviter_id;type:int"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;index"`
	LinuxDOId           string         `gorm:"column:linux_do_id;type:text"`
	Setting             string         `gorm:"column:setting;type:text"`
	Remark              string         `gorm:"column:remark;type:text"`
	StripeCustomer      string         `gorm:"column:stripe_customer;type:text"`
}

func (userSchemaWithoutGroupDefault) TableName() string {
	return "users"
}

func setupUserDefaultGroupModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	InitColumnMetadata()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&userSchemaWithoutGroupDefault{}); err != nil {
		t.Fatalf("failed to migrate users schema without group default: %v", err)
	}

	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestInsertDefaultsBlankGroupWithoutSchemaDefault(t *testing.T) {
	db := setupUserDefaultGroupModelTestDB(t)

	t.Run("insert", func(t *testing.T) {
		user := &User{
			Username: "blank-group-insert",
			Password: "password123",
			Group:    "",
		}

		if err := user.Insert(0); err != nil {
			t.Fatalf("expected insert to succeed without schema default, got error: %v", err)
		}
		if user.Group != "default" {
			t.Fatalf("expected insert path to normalize in-memory user group to default, got %q", user.Group)
		}

		var stored User
		if err := db.Where("username = ?", user.Username).First(&stored).Error; err != nil {
			t.Fatalf("failed to load inserted user: %v", err)
		}
		if stored.Group != "default" {
			t.Fatalf("expected inserted user group to be default, got %q", stored.Group)
		}
	})

	t.Run("insert_with_tx", func(t *testing.T) {
		user := &User{
			Username: "blank-group-insert-tx",
			Password: "password123",
			Group:    "",
		}

		tx := db.Begin()
		if tx.Error != nil {
			t.Fatalf("failed to begin transaction: %v", tx.Error)
		}

		if err := user.InsertWithTx(tx, 0); err != nil {
			_ = tx.Rollback()
			t.Fatalf("expected transactional insert to succeed without schema default, got error: %v", err)
		}
		if user.Group != "default" {
			_ = tx.Rollback()
			t.Fatalf("expected tx insert path to normalize in-memory user group to default, got %q", user.Group)
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		var stored User
		if err := db.Where("username = ?", user.Username).First(&stored).Error; err != nil {
			t.Fatalf("failed to load tx-inserted user: %v", err)
		}
		if stored.Group != "default" {
			t.Fatalf("expected tx-inserted user group to be default, got %q", stored.Group)
		}
	})
}

func TestInsertInitializesInviteSidebarModuleForNewUsers(t *testing.T) {
	db := setupUserDefaultGroupModelTestDB(t)

	user := &User{
		Username: "invite-sidebar-user",
		Password: "password123",
		Group:    "default",
	}

	if err := user.Insert(0); err != nil {
		t.Fatalf("expected insert to succeed, got error: %v", err)
	}

	var stored User
	if err := db.Where("username = ?", user.Username).First(&stored).Error; err != nil {
		t.Fatalf("failed to load inserted user: %v", err)
	}

	setting := stored.GetSetting()
	if setting.SidebarModules == "" {
		t.Fatal("expected sidebar modules to be initialized for new user")
	}

	var sidebarConfig map[string]map[string]bool
	if err := json.Unmarshal([]byte(setting.SidebarModules), &sidebarConfig); err != nil {
		t.Fatalf("failed to unmarshal sidebar modules: %v", err)
	}

	inviteConfig, ok := sidebarConfig["invite"]
	if !ok {
		t.Fatal("expected invite section in new user sidebar config")
	}
	if !inviteConfig["enabled"] {
		t.Fatal("expected invite section to be enabled")
	}
	if !inviteConfig["rebate"] {
		t.Fatal("expected invite rebate module to be enabled")
	}
}

func TestInsertCountsInviteesEvenWhenInviterQuotaRewardIsDisabled(t *testing.T) {
	db := setupUserDefaultGroupModelTestDB(t)

	originalQuotaForInviter := common.QuotaForInviter
	originalQuotaForInvitee := common.QuotaForInvitee
	originalQuotaForNewUser := common.QuotaForNewUser
	t.Cleanup(func() {
		common.QuotaForInviter = originalQuotaForInviter
		common.QuotaForInvitee = originalQuotaForInvitee
		common.QuotaForNewUser = originalQuotaForNewUser
	})

	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	common.QuotaForNewUser = 0

	inviter := &userSchemaWithoutGroupDefault{
		Username: "inviter-no-quota",
		Password: strings.Repeat("h", 16),
		Group:    "default",
		AffCode:  "INVQ",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}

	invitee := &User{
		Username: "invitee-no-quota",
		Password: "password123",
		Group:    "default",
	}
	if err := invitee.Insert(inviter.Id); err != nil {
		t.Fatalf("expected invitee insert to succeed, got error: %v", err)
	}

	var storedInviter User
	if err := db.Where("id = ?", inviter.Id).First(&storedInviter).Error; err != nil {
		t.Fatalf("failed to load inviter after invitee insert: %v", err)
	}
	if storedInviter.AffCount != 1 {
		t.Fatalf("expected inviter aff_count to be 1, got %d", storedInviter.AffCount)
	}
	if storedInviter.AffQuota != 0 || storedInviter.AffHistoryQuota != 0 {
		t.Fatalf("expected inviter quotas to remain 0, got aff_quota=%d aff_history=%d", storedInviter.AffQuota, storedInviter.AffHistoryQuota)
	}
}

func TestFinalizeOAuthUserCreationCountsInviteesEvenWhenInviterQuotaRewardIsDisabled(t *testing.T) {
	db := setupUserDefaultGroupModelTestDB(t)

	originalQuotaForInviter := common.QuotaForInviter
	originalQuotaForInvitee := common.QuotaForInvitee
	originalQuotaForNewUser := common.QuotaForNewUser
	t.Cleanup(func() {
		common.QuotaForInviter = originalQuotaForInviter
		common.QuotaForInvitee = originalQuotaForInvitee
		common.QuotaForNewUser = originalQuotaForNewUser
	})

	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	common.QuotaForNewUser = 0

	inviter := &userSchemaWithoutGroupDefault{
		Username: "oauth-inviter-no-quota",
		Password: strings.Repeat("h", 16),
		Group:    "default",
		AffCode:  "OINV",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("failed to create inviter: %v", err)
	}

	oauthUser := &User{
		Username: "oauth-invitee-no-quota",
		Password: "password123",
		Group:    "default",
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("failed to begin transaction: %v", tx.Error)
	}
	if err := oauthUser.InsertWithTx(tx, inviter.Id); err != nil {
		_ = tx.Rollback()
		t.Fatalf("expected oauth invitee insert to succeed, got error: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("failed to commit oauth invitee insert: %v", err)
	}

	oauthUser.FinalizeOAuthUserCreation(inviter.Id)

	var storedInviter User
	if err := db.Where("id = ?", inviter.Id).First(&storedInviter).Error; err != nil {
		t.Fatalf("failed to load inviter after oauth finalize: %v", err)
	}
	if storedInviter.AffCount != 1 {
		t.Fatalf("expected inviter aff_count to be 1 after oauth finalize, got %d", storedInviter.AffCount)
	}
	if storedInviter.AffQuota != 0 || storedInviter.AffHistoryQuota != 0 {
		t.Fatalf("expected inviter quotas to remain 0 after oauth finalize, got aff_quota=%d aff_history=%d", storedInviter.AffQuota, storedInviter.AffHistoryQuota)
	}
}
