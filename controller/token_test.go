package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type tokenAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenPageResponse struct {
	Items []tokenResponseItem `json:"items"`
}

type tokenResponseItem struct {
	ID                            int      `json:"id"`
	Name                          string   `json:"name"`
	Key                           string   `json:"key"`
	Status                        int      `json:"status"`
	MarketplaceFixedOrderID       int      `json:"marketplace_fixed_order_id"`
	MarketplaceFixedOrderIDs      []int    `json:"marketplace_fixed_order_ids"`
	MarketplaceRouteOrder         []string `json:"marketplace_route_order"`
	MarketplaceRouteEnabled       []string `json:"marketplace_route_enabled"`
	MarketplacePoolFiltersEnabled bool     `json:"marketplace_pool_filters_enabled"`
	MarketplacePoolFilters        struct {
		Model         string  `json:"model"`
		MaxMultiplier float64 `json:"max_multiplier"`
	} `json:"marketplace_pool_filters"`
}

type tokenKeyResponse struct {
	Key string `json:"key"`
}

func setupTokenControllerTestDB(t *testing.T) *gorm.DB {
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
		t.Fatalf("failed to migrate token table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUser(t *testing.T, db *gorm.DB, userID int, group string) *model.User {
	t.Helper()

	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("user_%d", userID),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    group,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func withTokenGroupSettings(t *testing.T, usableJSON string, specialJSON string) {
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

func withTokenGroupSettingsAndRatios(t *testing.T, usableJSON string, specialJSON string, ratioJSON string) {
	t.Helper()

	originalRatios := ratio_setting.GroupRatio2JSONString()
	withTokenGroupSettings(t, usableJSON, specialJSON)

	if err := ratio_setting.UpdateGroupRatioByJSONString(ratioJSON); err != nil {
		t.Fatalf("failed to set group ratios: %v", err)
	}

	t.Cleanup(func() {
		if err := ratio_setting.UpdateGroupRatioByJSONString(originalRatios); err != nil {
			t.Fatalf("failed to restore group ratios: %v", err)
		}
	})
}

func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func seedMarketplaceFixedOrderForTokenTest(t *testing.T, db *gorm.DB, buyerUserID int) *model.MarketplaceFixedOrder {
	t.Helper()

	if err := db.AutoMigrate(&model.MarketplaceFixedOrder{}); err != nil {
		t.Fatalf("failed to migrate marketplace fixed order table: %v", err)
	}
	order := &model.MarketplaceFixedOrder{
		BuyerUserID:        buyerUserID,
		SellerUserID:       10,
		CredentialID:       1,
		PurchasedQuota:     1000,
		RemainingQuota:     1000,
		MultiplierSnapshot: 1,
		Status:             model.MarketplaceFixedOrderStatusActive,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create marketplace fixed order: %v", err)
	}
	return order
}

func newAuthenticatedContext(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func TestGetAllTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "list-token", "abcd1234efgh5678")
	seedToken(t, db, 2, "other-user-token", "zzzz1234yyyy5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("list response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestSearchTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "searchable-token", "ijkl1234mnop5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=searchable-token&p=1&size=10", nil, 1)
	SearchTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one search result, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked search key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("search response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "detail-token", "qrst1234uvwx5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked detail key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("detail response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenReturnsMarketplacePoolFilterSavedState(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "pool-filter-detail-token", "pooldetail123456")
	token.MarketplacePoolFiltersEnabled = true
	token.MarketplacePoolFilters = model.NewMarketplacePoolFilters(model.MarketplacePoolFilterValues{
		Model:         "gpt-4o-mini",
		MaxMultiplier: 1.2,
	})
	if err := token.Update(); err != nil {
		t.Fatalf("failed to seed token pool filters: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if !detail.MarketplacePoolFiltersEnabled {
		t.Fatalf("expected saved pool filter flag in token detail response")
	}
	if detail.MarketplacePoolFilters.Model != "gpt-4o-mini" {
		t.Fatalf("expected saved pool filter model, got %q", detail.MarketplacePoolFilters.Model)
	}
	if detail.MarketplacePoolFilters.MaxMultiplier != 1.2 {
		t.Fatalf("expected saved pool filter max multiplier, got %v", detail.MarketplacePoolFilters.MaxMultiplier)
	}
}

func TestUpdateTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)
	token := seedToken(t, db, 1, "editable-token", "yzab1234cdef5678")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "standard",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked update key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenStoresMarketplaceRouteSettings(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)
	token := seedToken(t, db, 1, "route-edit-token", "route1234edit5678")

	body := map[string]any{
		"id":                        token.Id,
		"name":                      "route-edit-token-updated",
		"expired_time":              -1,
		"remain_quota":              100,
		"unlimited_quota":           true,
		"model_limits_enabled":      false,
		"model_limits":              "",
		"group":                     "standard",
		"cross_group_retry":         false,
		"marketplace_route_order":   []string{"group", "fixed_order", "pool"},
		"marketplace_route_enabled": []string{"group", "fixed_order"},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token update to store route settings, got %q", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	expectedOrder := []string{
		model.MarketplaceRouteGroup,
		model.MarketplaceRouteFixedOrder,
		model.MarketplaceRoutePool,
	}
	expectedEnabled := []string{
		model.MarketplaceRouteGroup,
		model.MarketplaceRouteFixedOrder,
	}
	if fmt.Sprint(detail.MarketplaceRouteOrder) != fmt.Sprint(expectedOrder) {
		t.Fatalf("expected response route order %v, got %v", expectedOrder, detail.MarketplaceRouteOrder)
	}
	if fmt.Sprint(detail.MarketplaceRouteEnabled) != fmt.Sprint(expectedEnabled) {
		t.Fatalf("expected response enabled routes %v, got %v", expectedEnabled, detail.MarketplaceRouteEnabled)
	}

	var stored model.Token
	if err := db.First(&stored, "id = ? AND user_id = ?", token.Id, 1).Error; err != nil {
		t.Fatalf("failed to reload updated token: %v", err)
	}
	if got := stored.MarketplaceRouteOrderList(); fmt.Sprint(got) != fmt.Sprint(expectedOrder) {
		t.Fatalf("expected stored route order %v, got %v", expectedOrder, got)
	}
	if got := stored.MarketplaceRouteEnabledList(); fmt.Sprint(got) != fmt.Sprint(expectedEnabled) {
		t.Fatalf("expected stored enabled routes %v, got %v", expectedEnabled, got)
	}
}

func TestUpdateTokenAllowsRouteUpdateWithUnchangedExpiredMarketplaceFixedOrderBinding(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)
	order := seedMarketplaceFixedOrderForTokenTest(t, db, 1)
	order.Status = model.MarketplaceFixedOrderStatusExpired
	if err := db.Save(order).Error; err != nil {
		t.Fatalf("failed to expire marketplace fixed order: %v", err)
	}
	token := seedToken(t, db, 1, "route-toggle-token", "route1234toggle5678")
	token.SetMarketplaceFixedOrderIDList([]int{order.ID})
	token.MarketplaceRouteOrder = model.NewMarketplaceRouteOrder([]string{
		model.MarketplaceRouteFixedOrder,
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	})
	token.MarketplaceRouteEnabled = model.NewMarketplaceRouteEnabled([]string{
		model.MarketplaceRouteFixedOrder,
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	})
	if err := db.Save(token).Error; err != nil {
		t.Fatalf("failed to bind expired marketplace fixed order to token: %v", err)
	}

	body := map[string]any{
		"id":                         token.Id,
		"name":                       "route-toggle-token",
		"expired_time":               -1,
		"remain_quota":               100,
		"unlimited_quota":            true,
		"model_limits_enabled":       false,
		"model_limits":               "",
		"group":                      "standard",
		"cross_group_retry":          false,
		"marketplace_fixed_order_id": order.ID,
		"marketplace_fixed_order_ids": []int{
			order.ID,
		},
		"marketplace_route_order": []string{
			model.MarketplaceRouteFixedOrder,
			model.MarketplaceRouteGroup,
			model.MarketplaceRoutePool,
		},
		"marketplace_route_enabled": []string{
			model.MarketplaceRouteFixedOrder,
			model.MarketplaceRouteGroup,
		},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token update to allow unchanged expired fixed order binding, got %q", response.Message)
	}

	var stored model.Token
	if err := db.First(&stored, "id = ? AND user_id = ?", token.Id, 1).Error; err != nil {
		t.Fatalf("failed to reload updated token: %v", err)
	}
	if got := stored.MarketplaceFixedOrderIDList(); fmt.Sprint(got) != fmt.Sprint([]int{order.ID}) {
		t.Fatalf("expected unchanged fixed order binding %v, got %v", []int{order.ID}, got)
	}
	expectedEnabled := []string{
		model.MarketplaceRouteFixedOrder,
		model.MarketplaceRouteGroup,
	}
	if got := stored.MarketplaceRouteEnabledList(); fmt.Sprint(got) != fmt.Sprint(expectedEnabled) {
		t.Fatalf("expected enabled routes %v, got %v", expectedEnabled, got)
	}
}

func TestAddTokenAllowsBlankGroupWhenAssignedDefaultGroupIsNotSelectable(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "default")
	withTokenGroupSettings(t, `{"standard":"标准价格"}`, `{}`)

	body := map[string]any{
		"name":                 "new-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to allow blank group, got %q", response.Message)
	}
}

func TestAddTokenAllowsBlankGroupWhenAssignedDefaultGroupIsExplicitlySelectable(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "default")
	withTokenGroupSettings(t, `{"standard":"标准价格","default":"默认分组"}`, `{}`)

	body := map[string]any{
		"name":                 "new-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to allow blank group, got %q", response.Message)
	}
}

func TestAddTokenAllowsBlankGroupWhenAssignedNonDefaultGroupIsCurrentUserGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "cc-opus-福利渠道")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1,"cc-opus-福利渠道":1}`,
	)

	body := map[string]any{
		"name":                 "new-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to succeed when assigned non-default group is current user group, got %q", response.Message)
	}
}

func TestAddTokenAllowsExplicitAssignedNonDefaultGroupSelection(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "cc-opus-福利渠道")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1,"cc-opus-福利渠道":1}`,
	)

	body := map[string]any{
		"name":                 "new-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "cc-opus-福利渠道",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to succeed when explicitly selecting assigned non-default group, got %q", response.Message)
	}
}

func TestAddTokenAllowsOwnedMarketplaceFixedOrderBinding(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	order := seedMarketplaceFixedOrderForTokenTest(t, db, 1)
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)

	body := map[string]any{
		"name":                       "market-token",
		"expired_time":               -1,
		"remain_quota":               100,
		"unlimited_quota":            true,
		"model_limits_enabled":       false,
		"model_limits":               "",
		"group":                      "standard",
		"cross_group_retry":          false,
		"marketplace_fixed_order_id": order.ID,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to bind owned marketplace order, got %q", response.Message)
	}

	var token model.Token
	if err := db.Where("user_id = ? AND name = ?", 1, "market-token").First(&token).Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	if token.MarketplaceFixedOrderID != order.ID {
		t.Fatalf("expected marketplace fixed order binding %d, got %d", order.ID, token.MarketplaceFixedOrderID)
	}
}

func TestAddTokenNormalizesMarketplaceRouteOrder(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)

	body := map[string]any{
		"name":                    "route-order-token",
		"expired_time":            -1,
		"remain_quota":            100,
		"unlimited_quota":         true,
		"model_limits_enabled":    false,
		"model_limits":            "",
		"group":                   "standard",
		"cross_group_retry":       false,
		"marketplace_route_order": []string{"group", "pool", "fixed_order"},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to accept route order, got %q", response.Message)
	}

	var token model.Token
	if err := db.Where("user_id = ? AND name = ?", 1, "route-order-token").First(&token).Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	expected := []string{model.MarketplaceRouteGroup, model.MarketplaceRoutePool, model.MarketplaceRouteFixedOrder}
	if got := token.MarketplaceRouteOrderList(); fmt.Sprint(got) != fmt.Sprint(expected) {
		t.Fatalf("expected route order %v, got %v", expected, got)
	}
}

func TestAddTokenStoresMarketplaceRouteEnabled(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)

	body := map[string]any{
		"name":                      "route-enabled-token",
		"expired_time":              -1,
		"remain_quota":              100,
		"unlimited_quota":           true,
		"model_limits_enabled":      false,
		"model_limits":              "",
		"group":                     "standard",
		"cross_group_retry":         false,
		"marketplace_route_order":   []string{"fixed_order", "group", "pool"},
		"marketplace_route_enabled": []string{"group", "pool"},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token creation to accept route enabled set, got %q", response.Message)
	}

	var token model.Token
	if err := db.Where("user_id = ? AND name = ?", 1, "route-enabled-token").First(&token).Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	expected := []string{model.MarketplaceRouteGroup, model.MarketplaceRoutePool}
	if got := token.MarketplaceRouteEnabledList(); fmt.Sprint(got) != fmt.Sprint(expected) {
		t.Fatalf("expected enabled routes %v, got %v", expected, got)
	}
}

func TestAddTokenRejectsUnownedMarketplaceFixedOrderBinding(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedUser(t, db, 1, "standard")
	order := seedMarketplaceFixedOrderForTokenTest(t, db, 2)
	withTokenGroupSettingsAndRatios(
		t,
		`{"standard":"标准价格","default":"默认分组"}`,
		`{}`,
		`{"default":1,"standard":1}`,
	)

	body := map[string]any{
		"name":                       "market-token",
		"expired_time":               -1,
		"remain_quota":               100,
		"unlimited_quota":            true,
		"model_limits_enabled":       false,
		"model_limits":               "",
		"group":                      "standard",
		"cross_group_retry":          false,
		"marketplace_fixed_order_id": order.ID,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected token creation to reject unowned marketplace order binding")
	}
}

func TestGetTokenKeyRequiresOwnershipAndReturnsFullKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(authorizedCtx)

	authorizedResponse := decodeAPIResponse(t, authorizedRecorder)
	if !authorizedResponse.Success {
		t.Fatalf("expected authorized key fetch to succeed, got message: %s", authorizedResponse.Message)
	}

	var keyData tokenKeyResponse
	if err := common.Unmarshal(authorizedResponse.Data, &keyData); err != nil {
		t.Fatalf("failed to decode token key response: %v", err)
	}
	if keyData.Key != token.GetFullKey() {
		t.Fatalf("expected full key %q, got %q", token.GetFullKey(), keyData.Key)
	}

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(unauthorizedCtx)

	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	if unauthorizedResponse.Success {
		t.Fatalf("expected unauthorized key fetch to fail")
	}
	if strings.Contains(unauthorizedRecorder.Body.String(), token.Key) {
		t.Fatalf("unauthorized key response leaked raw token key: %s", unauthorizedRecorder.Body.String())
	}
}
