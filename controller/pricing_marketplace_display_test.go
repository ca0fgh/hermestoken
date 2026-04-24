package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type marketplacePricingResponse struct {
	Success       bool               `json:"success"`
	Data          []model.Pricing    `json:"data"`
	DisplayGroups map[string]string  `json:"display_groups"`
	GroupRatio    map[string]float64 `json:"group_ratio"`
}

func withHeaderNavModulesOption(t *testing.T, raw string) {
	t.Helper()

	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	original, existed := common.OptionMap["HeaderNavModules"]
	common.OptionMap["HeaderNavModules"] = raw
	t.Cleanup(func() {
		if !existed {
			delete(common.OptionMap, "HeaderNavModules")
			return
		}
		common.OptionMap["HeaderNavModules"] = original
	})
}

func seedPricingModelMeta(t *testing.T, db *gorm.DB, modelName string, status int) {
	t.Helper()

	record := &model.Model{
		ModelName:    modelName,
		Status:       status,
		SyncOfficial: 1,
		NameRule:     model.NameRuleExact,
	}
	if err := record.Insert(); err != nil {
		t.Fatalf("failed to create model meta: %v", err)
	}
}

func decodeMarketplacePricingResponse(t *testing.T, recorder *httptest.ResponseRecorder) marketplacePricingResponse {
	t.Helper()

	var response marketplacePricingResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode marketplace pricing response: %v", err)
	}
	return response
}

func newAuthenticatedPricingContext(t *testing.T, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", userID)
	return ctx, recorder
}

func TestGetPricingReturnsEmptyMarketplaceDataWhenMarketplaceDisabled(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withHeaderNavModulesOption(t, `{"home":true,"pricing":{"enabled":false,"requireAuth":false}}`)
	withPricingGuestSettings(
		t,
		`{"vip":"vip分组"}`,
		`{"default":1,"vip":2}`,
		`{"default":{"default":0.75}}`,
	)
	seedPricingModelMeta(t, db, "gpt-default", 1)
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingModelMeta(t, db, "gpt-vip", 1)
	seedPricingAbility(t, db, "vip", "gpt-vip")
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response when marketplace is disabled")
	}
	if len(response.Data) != 0 {
		t.Fatalf("expected empty data when marketplace is disabled, got %d items", len(response.Data))
	}
	if len(response.DisplayGroups) != 0 {
		t.Fatalf("expected empty display groups when marketplace is disabled, got %#v", response.DisplayGroups)
	}
	if len(response.GroupRatio) != 0 {
		t.Fatalf("expected empty group ratio when marketplace is disabled, got %#v", response.GroupRatio)
	}
	serverTiming := recorder.Header().Get("Server-Timing")
	if serverTiming == "" {
		t.Fatal("expected Server-Timing header when marketplace is disabled")
	}
	if !strings.Contains(serverTiming, "pricing_total") {
		t.Fatalf("expected Server-Timing header %q to contain pricing_total", serverTiming)
	}
}

func TestGetPricingShowsOnlyDefaultMarketplaceModelsToGuests(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withHeaderNavModulesOption(t, `{"home":true,"pricing":{"enabled":true,"requireAuth":false}}`)
	withPricingGuestSettings(
		t,
		`{"vip":"vip分组"}`,
		`{"default":1,"vip":2}`,
		`{"default":{"default":0.75}}`,
	)
	seedPricingModelMeta(t, db, "gpt-default", 1)
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingModelMeta(t, db, "gpt-vip", 1)
	seedPricingAbility(t, db, "vip", "gpt-vip")
	seedPricingModelMeta(t, db, "gpt-hidden-default", 0)
	seedPricingAbility(t, db, "default", "gpt-hidden-default")
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response for guest pricing")
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected exactly one default-group model for guest, got %d", len(response.Data))
	}
	if response.Data[0].ModelName != "gpt-default" {
		t.Fatalf("expected guest to see default model, got %q", response.Data[0].ModelName)
	}
	if len(response.DisplayGroups) != 1 || response.DisplayGroups["default"] != "default" {
		t.Fatalf("expected guest display groups to contain only default, got %#v", response.DisplayGroups)
	}
	if len(response.GroupRatio) != 1 || response.GroupRatio["default"] != 0.75 {
		t.Fatalf("expected guest group ratio override for default, got %#v", response.GroupRatio)
	}
}

func TestGetPricingPreservesAllGroupModelsForGuests(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withHeaderNavModulesOption(t, `{"home":true,"pricing":{"enabled":true,"requireAuth":false}}`)
	withPricingGuestSettings(
		t,
		`{"vip":"vip分组"}`,
		`{"default":1,"vip":2}`,
		`{"default":{"default":0.75}}`,
	)
	seedPricingModelMeta(t, db, "gpt-all", 1)
	seedPricingAbility(t, db, "all", "gpt-all")
	seedPricingModelMeta(t, db, "gpt-default", 1)
	seedPricingAbility(t, db, "default", "gpt-default")
	model.RefreshPricing()

	ctx, recorder := newGuestPricingContext(t)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response for guest pricing")
	}
	if len(response.Data) != 2 {
		t.Fatalf("expected guest to see all-group and default models, got %d", len(response.Data))
	}
	if len(response.DisplayGroups) != 2 {
		t.Fatalf("expected guest display groups to include all and default, got %#v", response.DisplayGroups)
	}
	if response.DisplayGroups["all"] != "all" || response.DisplayGroups["default"] != "default" {
		t.Fatalf("expected guest display groups to preserve all semantics, got %#v", response.DisplayGroups)
	}
	if len(response.GroupRatio) != 1 || response.GroupRatio["default"] != 0.75 {
		t.Fatalf("expected guest group ratio to remain filtered to visible ratio groups, got %#v", response.GroupRatio)
	}
}

func TestGetPricingShowsAllDisplayModelsToAuthenticatedUsers(t *testing.T) {
	db := setupPricingControllerTestDB(t)
	withHeaderNavModulesOption(t, `{"home":true,"pricing":{"enabled":true,"requireAuth":false}}`)
	originalAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		if err := setting.UpdateAutoGroupsByJsonString(originalAutoGroups); err != nil {
			t.Fatalf("failed to restore auto groups: %v", err)
		}
	})
	withPricingGuestSettings(
		t,
		`{"vip":"vip分组"}`,
		`{"default":1,"vip":2}`,
		`{"vip":{"default":0.75,"vip":2}}`,
	)
	if err := setting.UpdateAutoGroupsByJsonString(`[]`); err != nil {
		t.Fatalf("failed to disable auto groups for authenticated test: %v", err)
	}
	seedPricingModelMeta(t, db, "gpt-default", 1)
	seedPricingAbility(t, db, "default", "gpt-default")
	seedPricingModelMeta(t, db, "gpt-vip", 1)
	seedPricingAbility(t, db, "vip", "gpt-vip")
	seedPricingModelMeta(t, db, "gpt-hidden-vip", 0)
	seedPricingAbility(t, db, "vip", "gpt-hidden-vip")
	if err := db.Create(&model.User{
		Id:       101,
		Username: "vip-user",
		Password: "password123",
		Group:    "vip",
		Status:   1,
	}).Error; err != nil {
		t.Fatalf("failed to create authenticated user: %v", err)
	}
	model.RefreshPricing()

	ctx, recorder := newAuthenticatedPricingContext(t, 101)
	GetPricing(ctx)

	response := decodeMarketplacePricingResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response for authenticated pricing")
	}
	if len(response.Data) != 2 {
		t.Fatalf("expected authenticated users to see all display models, got %d", len(response.Data))
	}
	for _, item := range response.Data {
		if item.ModelName == "gpt-hidden-vip" {
			t.Fatalf("expected hidden model to be excluded from pricing response, got %#v", response.Data)
		}
	}
	if len(response.DisplayGroups) != 2 {
		t.Fatalf("expected authenticated display groups to include all visible groups, got %#v", response.DisplayGroups)
	}
	if response.DisplayGroups["default"] != "default" || response.DisplayGroups["vip"] != "vip分组" {
		t.Fatalf("expected authenticated display groups to include default and vip, got %#v", response.DisplayGroups)
	}
	if len(response.GroupRatio) != 2 {
		t.Fatalf("expected group ratio to be filtered to displayed groups, got %#v", response.GroupRatio)
	}
	if response.GroupRatio["default"] != 0.75 || response.GroupRatio["vip"] != 2 {
		t.Fatalf("expected authenticated group ratios to include display groups only, got %#v", response.GroupRatio)
	}
}
