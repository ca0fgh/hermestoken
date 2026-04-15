package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

	if err := db.AutoMigrate(&model.PricingGroup{}, &model.PricingGroupAlias{}); err != nil {
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

	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("failed to set usable groups: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}

	t.Cleanup(func() {
		if err := setting.UpdateUserUsableGroupsByJSONString(originalUsable); err != nil {
			t.Fatalf("failed to restore usable groups: %v", err)
		}
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
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

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/group/admin/consistency", nil, 1)
	GetPricingGroupConsistencyReport(ctx)

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

	for _, unresolved := range report.UnresolvedLegacyReferences {
		if unresolved.Value == "legacy-missing" {
			return
		}
	}
	t.Fatalf("expected legacy-missing to be reported, got %#v", report.UnresolvedLegacyReferences)
}
