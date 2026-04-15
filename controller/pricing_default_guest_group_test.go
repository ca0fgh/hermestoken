package controller

import (
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

type pricingAPIResponse struct {
	Success    bool              `json:"success"`
	Data       []model.Pricing   `json:"data"`
	GroupRatio map[string]float64 `json:"group_ratio"`
	UsableGroup map[string]string `json:"usable_group"`
	AutoGroups []string          `json:"auto_groups"`
}

func setupPricingControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnMetadata()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(
		&model.User{},
		&model.Channel{},
		&model.Ability{},
		&model.Model{},
		&model.Vendor{},
		&model.UserSubscription{},
	); err != nil {
		t.Fatalf("failed to migrate pricing tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func withPricingGuestSettings(t *testing.T, usableJSON string, ratioJSON string, groupGroupRatioJSON string) {
	t.Helper()

	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	originalSpecial := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}
	if err := ratio_setting.UpdateGroupGroupRatioByJSONString(groupGroupRatioJSON); err != nil {
		t.Fatalf("failed to set group-group ratios: %v", err)
	}
	if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, `{}`); err != nil {
		t.Fatalf("failed to clear special usable groups: %v", err)
	}
	if err := setting.UpdateAutoGroupsByJsonString(`["default"]`); err != nil {
		t.Fatalf("failed to set auto groups: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
		if err := ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatios); err != nil {
			t.Fatalf("failed to restore group-group ratios: %v", err)
		}
		if err := types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, originalSpecial); err != nil {
			t.Fatalf("failed to restore special usable groups: %v", err)
		}
		if err := setting.UpdateAutoGroupsByJsonString(originalAutoGroups); err != nil {
			t.Fatalf("failed to restore auto groups: %v", err)
		}
	})
}

func seedPricingAbility(t *testing.T, db *gorm.DB, group string, modelName string) {
	t.Helper()

	channel := &model.Channel{
		Id:     len(modelName) + len(group) + 1,
		Name:   group + "-channel-" + modelName,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Type:   constant.ChannelTypeOpenAI,
		Group:  group,
		Models: modelName,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create pricing channel: %v", err)
	}

	ability := &model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
	}
	if err := db.Create(ability).Error; err != nil {
		t.Fatalf("failed to create pricing ability: %v", err)
	}
}

func newGuestPricingContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	return ctx, recorder
}

func decodePricingResponse(t *testing.T, recorder *httptest.ResponseRecorder) pricingAPIResponse {
	t.Helper()

	var response pricingAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode pricing response: %v", err)
	}
	return response
}

func TestGetPricingUsesDefaultGroupForGuests(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withPricingGuestSettings(
		t,
		`{}`,
		`{"default":1,"vip":2}`,
		`{"default":{"default":0.75}}`,
	)
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingAbility(t, db, "vip", "gpt-vip")
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodePricingResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response for guest pricing")
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected exactly one default-group model for guest, got %d", len(response.Data))
	}
	if response.Data[0].ModelName != "gpt-default" {
		t.Fatalf("expected guest to see default model, got %q", response.Data[0].ModelName)
	}
	if len(response.UsableGroup) != 1 || response.UsableGroup["default"] != "用户分组" {
		t.Fatalf("expected guest usable groups to resolve to default, got %#v", response.UsableGroup)
	}
	if len(response.GroupRatio) != 1 || response.GroupRatio["default"] != 0.75 {
		t.Fatalf("expected guest group ratio override for default, got %#v", response.GroupRatio)
	}
	if len(response.AutoGroups) != 1 || response.AutoGroups[0] != "default" {
		t.Fatalf("expected guest auto groups to include default, got %#v", response.AutoGroups)
	}
}

func TestGetPricingReturnsEmptyListWhenDefaultGroupHasNoModels(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withPricingGuestSettings(
		t,
		`{}`,
		`{"default":1,"vip":2}`,
		`{"default":{"default":0.75}}`,
	)
	seedPricingAbility(t, db, "vip", "gpt-vip-only")
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodePricingResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response when default group has no pricing")
	}
	if len(response.Data) != 0 {
		t.Fatalf("expected empty pricing list for guest without default-group models, got %d items", len(response.Data))
	}
	if len(response.UsableGroup) != 1 || response.UsableGroup["default"] != "用户分组" {
		t.Fatalf("expected guest usable groups to still resolve to default, got %#v", response.UsableGroup)
	}
}
