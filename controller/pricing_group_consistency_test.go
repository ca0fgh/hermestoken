package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type pricingGroupConsistencyResponse struct {
	UnresolvedLegacyReferences []pricingGroupLegacyReference `json:"unresolved_legacy_references"`
}

type pricingGroupLegacyReference struct {
	Scope string `json:"scope"`
	Value string `json:"value"`
}

func setupPricingGroupConsistencyControllerTestDB(t *testing.T) *gorm.DB {
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

	if err := db.AutoMigrate(&model.User{}, &model.PricingGroup{}, &model.PricingGroupAlias{}); err != nil {
		t.Fatalf("failed to migrate pricing group tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func withConsistencyGroupSettings(t *testing.T, usableJSON string, ratioJSON string) {
	t.Helper()

	originalUsable := setting.UserUsableGroups2JSONString()
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}
	if err := setting.UpdateAutoGroupsByJsonString(`["default","legacy-default","auto-missing"]`); err != nil {
		t.Fatalf("failed to set auto groups: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
		if err := setting.UpdateAutoGroupsByJsonString(originalAutoGroups); err != nil {
			t.Fatalf("failed to restore auto groups: %v", err)
		}
	})
}

func TestGetPricingGroupConsistencyReportIncludesUnknownLegacyReferences(t *testing.T) {
	db := setupPricingGroupConsistencyControllerTestDB(t)
	withConsistencyGroupSettings(
		t,
		`{"default":"Default","legacy-default":"Legacy alias","legacy-missing":"Missing"}`,
		`{"default":1,"legacy-default":1,"legacy-missing":1}`,
	)

	group := model.PricingGroup{GroupKey: "default", DisplayName: "Default"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("failed to create canonical pricing group: %v", err)
	}
	alias := model.PricingGroupAlias{AliasKey: "legacy-default", GroupId: group.Id, Reason: "migration"}
	if err := db.Create(&alias).Error; err != nil {
		t.Fatalf("failed to create pricing group alias: %v", err)
	}

	rootToken := "root-token"
	rootUser := model.User{
		Id:          1,
		Username:    "root",
		Password:    "password123",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "root-aff",
		AccessToken: &rootToken,
	}
	if err := db.Create(&rootUser).Error; err != nil {
		t.Fatalf("failed to create root user: %v", err)
	}

	adminToken := "admin-token"
	adminUser := model.User{
		Id:          2,
		Username:    "admin",
		Password:    "password123",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "admin-aff",
		AccessToken: &adminToken,
	}
	if err := db.Create(&adminUser).Error; err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	engine := gin.New()
	store := cookie.NewStore([]byte(common.SessionSecret))
	engine.Use(sessions.Sessions("session", store))
	apiRouter := engine.Group("/api")
	groupAdminRoute := apiRouter.Group("/group/admin")
	groupAdminRoute.Use(middleware.RootAuth())
	groupAdminRoute.GET("/consistency", GetPricingGroupConsistencyReport)

	adminRequest := httptest.NewRequest(http.MethodGet, "/api/group/admin/consistency", nil)
	adminRequest.Header.Set("Authorization", adminToken)
	adminRequest.Header.Set("New-Api-User", "2")
	adminRecorder := httptest.NewRecorder()
	engine.ServeHTTP(adminRecorder, adminRequest)

	adminResponse := decodeAPIResponse(t, adminRecorder)
	if adminResponse.Success {
		t.Fatalf("expected admin request to be rejected by root-only route, got %#v", adminResponse)
	}

	rootRequest := httptest.NewRequest(http.MethodGet, "/api/group/admin/consistency", nil)
	rootRequest.Header.Set("Authorization", rootToken)
	rootRequest.Header.Set("New-Api-User", "1")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, rootRequest)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var report pricingGroupConsistencyResponse
	if err := common.Unmarshal(response.Data, &report); err != nil {
		t.Fatalf("failed to decode consistency report: %v", err)
	}

	if len(report.UnresolvedLegacyReferences) == 0 {
		t.Fatal("expected unresolved legacy references to be reported")
	}

	reported := make(map[string]string, len(report.UnresolvedLegacyReferences))
	for _, unresolved := range report.UnresolvedLegacyReferences {
		reported[unresolved.Scope+"|"+unresolved.Value] = unresolved.Value
	}
	if _, ok := reported["group_ratio|legacy-missing"]; !ok {
		t.Fatalf("expected legacy-missing to be reported, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["user_usable_groups|legacy-missing"]; !ok {
		t.Fatalf("expected user-usable legacy-missing to be reported, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["auto_groups|auto-missing"]; !ok {
		t.Fatalf("expected auto-missing auto group to be reported, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["group_ratio|legacy-default"]; ok {
		t.Fatalf("expected alias-backed legacy-default to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["user_usable_groups|legacy-default"]; ok {
		t.Fatalf("expected alias-backed user-usable legacy-default to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["auto_groups|legacy-default"]; ok {
		t.Fatalf("expected alias-backed auto group to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["group_ratio|default"]; ok {
		t.Fatalf("expected canonical group_ratio value default to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["user_usable_groups|default"]; ok {
		t.Fatalf("expected canonical user-usable value default to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
	if _, ok := reported["auto_groups|default"]; ok {
		t.Fatalf("expected canonical auto group value default to stay resolved, got %#v", report.UnresolvedLegacyReferences)
	}
}
