package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type tokenAuthContextResponse struct {
	UsingGroup string `json:"using_group"`
	TokenGroup string `json:"token_group"`
	UserGroup  string `json:"user_group"`
	UserID     int    `json:"user_id"`
}

func setupTokenAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnMetadata()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Token{}, &model.User{}); err != nil {
		t.Fatalf("failed to migrate auth test schema: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func withTokenAuthGroupSettings(t *testing.T, usableJSON string, specialJSON string, ratioJSON string, autoJSON string) {
	t.Helper()

	originalUsable := setting.UserUsableGroups2JSONString()
	originalSpecial := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalAuto := setting.AutoGroups2JsonString()

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, specialJSON); err != nil {
		t.Fatalf("failed to set special usable groups: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}
	if err := setting.UpdateAutoGroupsByJsonString(autoJSON); err != nil {
		t.Fatalf("failed to set auto groups: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, originalSpecial); err != nil {
			t.Fatalf("failed to restore special usable groups: %v", err)
		}
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
		if err := setting.UpdateAutoGroupsByJsonString(originalAuto); err != nil {
			t.Fatalf("failed to restore auto groups: %v", err)
		}
	})
}

func seedTokenAuthUser(t *testing.T, db *gorm.DB, userID int, group string) {
	t.Helper()

	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("user_%d", userID),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    group,
		Quota:    1000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create auth test user: %v", err)
	}
}

func seedTokenAuthToken(t *testing.T, db *gorm.DB, token model.Token) {
	t.Helper()

	token.Status = common.TokenStatusEnabled
	token.CreatedTime = 1
	token.AccessedTime = 1
	token.ExpiredTime = -1
	token.RemainQuota = 100
	token.UnlimitedQuota = true
	token.Name = strings.TrimSpace(token.Name)
	if token.Name == "" {
		token.Name = fmt.Sprintf("token_%d", token.UserId)
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("failed to create auth test token: %v", err)
	}
}

func performTokenAuthRequest(t *testing.T, rawKey string) tokenAuthContextResponse {
	t.Helper()

	router := gin.New()
	router.Use(TokenAuth())
	router.GET("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group": common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"user_group":  common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"user_id":     c.GetInt("id"),
		})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-"+rawKey)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected auth request to succeed, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response tokenAuthContextResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	return response
}

func TestTokenAuthUsesAutoSelectionModeWithoutLegacyAutoGroup(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	withTokenAuthGroupSettings(
		t,
		`{"default":"Default","premium":"Premium","auto":"Auto"}`,
		`{}`,
		`{"default":1,"premium":1}`,
		`["premium","default"]`,
	)
	seedTokenAuthUser(t, db, 1001, "default")
	seedTokenAuthToken(t, db, model.Token{
		UserId:             1001,
		Name:               "auto-selection",
		Key:                "tokenautoselection123",
		SelectionMode:      "auto",
		Group:              "",
		GroupKey:           "",
		CrossGroupRetry:    true,
		ModelLimitsEnabled: false,
	})

	response := performTokenAuthRequest(t, "tokenautoselection123")

	if response.UsingGroup != "auto" {
		t.Fatalf("expected using_group=auto, got %q", response.UsingGroup)
	}
	if response.TokenGroup != "auto" {
		t.Fatalf("expected token_group=auto, got %q", response.TokenGroup)
	}
	if response.UserGroup != "default" {
		t.Fatalf("expected user_group=default, got %q", response.UserGroup)
	}
	if response.UserID != 1001 {
		t.Fatalf("expected user_id=1001, got %d", response.UserID)
	}
}

func TestTokenAuthUsesFixedSelectionModeGroupKeyWithoutLegacyGroup(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	withTokenAuthGroupSettings(
		t,
		`{"default":"Default","premium":"Premium"}`,
		`{}`,
		`{"default":1,"premium":1}`,
		`["premium","default"]`,
	)
	seedTokenAuthUser(t, db, 1002, "default")
	seedTokenAuthToken(t, db, model.Token{
		UserId:             1002,
		Name:               "fixed-selection",
		Key:                "tokenfixedselection123",
		SelectionMode:      "fixed",
		Group:              "",
		GroupKey:           "premium",
		ModelLimitsEnabled: false,
	})

	response := performTokenAuthRequest(t, "tokenfixedselection123")

	if response.UsingGroup != "premium" {
		t.Fatalf("expected using_group=premium, got %q", response.UsingGroup)
	}
	if response.TokenGroup != "premium" {
		t.Fatalf("expected token_group=premium, got %q", response.TokenGroup)
	}
	if response.UserGroup != "default" {
		t.Fatalf("expected user_group=default, got %q", response.UserGroup)
	}
}

func TestTokenAuthUsesInheritedSelectionModeWithoutExplicitTokenGroup(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	withTokenAuthGroupSettings(
		t,
		`{"default":"Default","premium":"Premium"}`,
		`{}`,
		`{"default":1,"premium":1}`,
		`["premium","default"]`,
	)
	seedTokenAuthUser(t, db, 1003, "default")
	seedTokenAuthToken(t, db, model.Token{
		UserId:             1003,
		Name:               "inherit-selection",
		Key:                "tokeninheritselection123",
		SelectionMode:      "inherit_user_default",
		Group:              "",
		GroupKey:           "",
		ModelLimitsEnabled: false,
	})

	response := performTokenAuthRequest(t, "tokeninheritselection123")

	if response.UsingGroup != "default" {
		t.Fatalf("expected using_group=default, got %q", response.UsingGroup)
	}
	if response.TokenGroup != "" {
		t.Fatalf("expected token_group to stay empty for inherited selection, got %q", response.TokenGroup)
	}
	if response.UserGroup != "default" {
		t.Fatalf("expected user_group=default, got %q", response.UserGroup)
	}
}

func TestTokenAuthSupportsLegacyFixedTokenWithoutSelectionMode(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	withTokenAuthGroupSettings(
		t,
		`{"default":"Default","premium":"Premium"}`,
		`{}`,
		`{"default":1,"premium":1}`,
		`["premium","default"]`,
	)
	seedTokenAuthUser(t, db, 1004, "default")
	seedTokenAuthToken(t, db, model.Token{
		UserId:             1004,
		Name:               "legacy-fixed-selection",
		Key:                "tokenlegacyfixed123",
		SelectionMode:      "",
		Group:              "premium",
		GroupKey:           "",
		ModelLimitsEnabled: false,
	})

	response := performTokenAuthRequest(t, "tokenlegacyfixed123")

	if response.UsingGroup != "premium" {
		t.Fatalf("expected using_group=premium, got %q", response.UsingGroup)
	}
	if response.TokenGroup != "premium" {
		t.Fatalf("expected token_group=premium, got %q", response.TokenGroup)
	}
}
