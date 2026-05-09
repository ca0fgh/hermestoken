package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSubscriptionControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}
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

	if err := db.AutoMigrate(
		&model.User{},
		&model.Option{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.ReferralTemplate{},
		&model.ReferralTemplateBinding{},
		&model.ReferralInviteeShareOverride{},
		&model.ReferralSettlementBatch{},
		&model.ReferralSettlementRecord{},
		&model.TopUp{},
		&model.Log{},
	); err != nil {
		t.Fatalf("failed to migrate subscription tables: %v", err)
	}
	model.InitOptionMap()

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		common.OptionMap = originalOptionMap
	})

	return db
}

func seedSubscriptionPlan(t *testing.T, db *gorm.DB, title string) *model.SubscriptionPlan {
	t.Helper()

	plan := &model.SubscriptionPlan{
		Title:         title,
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create subscription plan: %v", err)
	}
	return plan
}

func TestGetSubscriptionPlansHidesPlansWithoutActiveSubscriptionReferralBindingWhenOpenToAllDisabled(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "plan-hidden@example.com", "plan_hidden", "")
	seedSubscriptionPlan(t, db, "hidden-plan")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/subscription/plans", nil, user.Id)
	GetSubscriptionPlans(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var plans []SubscriptionPlanDTO
	if err := common.Unmarshal(response.Data, &plans); err != nil {
		t.Fatalf("failed to decode plan response: %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("expected no subscription plans for user without active subscription referral binding, got %d", len(plans))
	}
}

func TestGetSubscriptionPlansReturnsEnabledPlansWithoutActiveSubscriptionReferralBindingWhenOpenToAllEnabled(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := model.UpdateSubscriptionReferralGlobalSetting(model.SubscriptionReferralGlobalSetting{
		TeamDecayRatio:            model.DefaultSubscriptionReferralTeamDecayRatio,
		TeamMaxDepth:              model.DefaultSubscriptionReferralTeamMaxDepth,
		AutoAssignInviteeTemplate: model.DefaultSubscriptionReferralAutoAssignInviteeTemplate,
		PlanOpenToAllUsers:        true,
	}); err != nil {
		t.Fatalf("failed to enable subscription plans for all users: %v", err)
	}
	user := seedSubscriptionPaymentUser(t, db, 1, "plan-public@example.com", "plan_public", "")
	plan := seedSubscriptionPlan(t, db, "public-plan")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/subscription/plans", nil, user.Id)
	GetSubscriptionPlans(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var plans []SubscriptionPlanDTO
	if err := common.Unmarshal(response.Data, &plans); err != nil {
		t.Fatalf("failed to decode plan response: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected subscription plans for user without active subscription referral binding when open to all is enabled, got %d", len(plans))
	}
	if plans[0].Plan.Id != plan.Id {
		t.Fatalf("expected returned plan id %d, got %d", plan.Id, plans[0].Plan.Id)
	}
}

func TestAdminDeleteSubscriptionPlanDeletesUnreferencedPlan(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "deletable-plan")

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/plans/"+strconv.Itoa(plan.Id),
		nil,
		1,
	)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminDeleteSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count subscription plans: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected plan to be hidden after soft delete, found %d visible rows", count)
	}

	var deleted model.SubscriptionPlan
	if err := db.Unscoped().Where("id = ?", plan.Id).First(&deleted).Error; err != nil {
		t.Fatalf("expected soft-deleted plan to remain in database: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatalf("expected plan deleted_at to be set")
	}
}

func TestAdminDeleteSubscriptionPlanSoftDeletesReferencedOrder(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "plan-with-order")

	order := &model.SubscriptionOrder{
		UserId:        1,
		PlanId:        plan.Id,
		Money:         99,
		TradeNo:       "trade-order-1",
		PaymentMethod: "epay",
		Status:        common.TopUpStatusSuccess,
		CreateTime:    1,
		CompleteTime:  1,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to create subscription order: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/plans/"+strconv.Itoa(plan.Id),
		nil,
		1,
	)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminDeleteSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected delete to soft-delete referenced order plan, got message: %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count subscription plans: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected plan to disappear from default queries after soft delete, found %d rows", count)
	}

	var deleted model.SubscriptionPlan
	if err := db.Unscoped().Where("id = ?", plan.Id).First(&deleted).Error; err != nil {
		t.Fatalf("expected soft-deleted referenced plan to remain in database: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatalf("expected referenced plan deleted_at to be set")
	}
}

func TestAdminDeleteSubscriptionPlanSoftDeletesReferencedUserSubscription(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "plan-with-subscription")

	subscription := &model.UserSubscription{
		UserId:      1,
		PlanId:      plan.Id,
		AmountTotal: 1000,
		AmountUsed:  10,
		StartTime:   1,
		EndTime:     9999999999,
		Status:      "active",
		Source:      "admin",
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to create user subscription: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodDelete,
		"/api/subscription/admin/plans/"+strconv.Itoa(plan.Id),
		nil,
		1,
	)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminDeleteSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected delete to soft-delete referenced user subscription plan, got message: %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count subscription plans: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected plan to disappear from default queries after soft delete, found %d rows", count)
	}

	var deleted model.SubscriptionPlan
	if err := db.Unscoped().Where("id = ?", plan.Id).First(&deleted).Error; err != nil {
		t.Fatalf("expected soft-deleted referenced plan to remain in database: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatalf("expected referenced plan deleted_at to be set")
	}
}

func TestCreatePendingSubscriptionOrderCreatesOrder(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "checkout-plan")

	lockedPlan, err := createPendingSubscriptionOrder(
		1,
		plan.Id,
		1,
		"trade-create-order",
		"epay",
		func(currentPlan *model.SubscriptionPlan) error {
			if !currentPlan.Enabled {
				return errors.New("套餐未启用")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected order creation to succeed, got error: %v", err)
	}
	if lockedPlan == nil || lockedPlan.Id != plan.Id {
		t.Fatalf("expected locked plan %d, got %+v", plan.Id, lockedPlan)
	}

	var orderCount int64
	if err := db.Model(&model.SubscriptionOrder{}).Where("trade_no = ?", "trade-create-order").Count(&orderCount).Error; err != nil {
		t.Fatalf("failed to count created orders: %v", err)
	}
	if orderCount != 1 {
		t.Fatalf("expected one pending order, found %d", orderCount)
	}
}

func TestCreatePendingSubscriptionOrderRejectsDisabledPlan(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "disabled-checkout-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error; err != nil {
		t.Fatalf("failed to disable plan: %v", err)
	}

	_, err := createPendingSubscriptionOrder(
		1,
		plan.Id,
		1,
		"trade-disabled-order",
		"epay",
		func(currentPlan *model.SubscriptionPlan) error {
			if !currentPlan.Enabled {
				return errors.New("套餐未启用")
			}
			return nil
		},
	)
	if err == nil {
		t.Fatalf("expected disabled plan to be rejected")
	}

	var orderCount int64
	if err := db.Model(&model.SubscriptionOrder{}).Where("trade_no = ?", "trade-disabled-order").Count(&orderCount).Error; err != nil {
		t.Fatalf("failed to count created orders: %v", err)
	}
	if orderCount != 0 {
		t.Fatalf("expected no order for disabled plan, found %d", orderCount)
	}
}

func TestCreatePendingSubscriptionOrderReservesStock(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "stocked-checkout-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("stock_total", 2).Error; err != nil {
		t.Fatalf("failed to seed stock_total: %v", err)
	}

	_, err := createPendingSubscriptionOrder(
		1,
		plan.Id,
		1,
		"trade-stock-lock",
		"epay",
		func(currentPlan *model.SubscriptionPlan) error {
			if !currentPlan.Enabled {
				return errors.New("套餐未启用")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected order creation to succeed, got %v", err)
	}

	var locked model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&locked).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if locked.StockLocked != 1 {
		t.Fatalf("expected stock_locked=1, got %d", locked.StockLocked)
	}

	var order model.SubscriptionOrder
	if err := db.Where("trade_no = ?", "trade-stock-lock").First(&order).Error; err != nil {
		t.Fatalf("failed to reload order: %v", err)
	}
	if order.StockReserved != 1 {
		t.Fatalf("expected stock_reserved=1, got %d", order.StockReserved)
	}
}

func TestCreatePendingSubscriptionOrderRejectsSoldOutPlan(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "sold-out-checkout-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  1,
		"stock_locked": 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed stock state: %v", err)
	}

	_, err := createPendingSubscriptionOrder(
		1,
		plan.Id,
		1,
		"trade-stock-sold-out",
		"epay",
		func(currentPlan *model.SubscriptionPlan) error {
			return nil
		},
	)
	if !errors.Is(err, model.ErrSubscriptionPlanOutOfStock) {
		t.Fatalf("expected ErrSubscriptionPlanOutOfStock, got %v", err)
	}
}

func TestCreatePendingSubscriptionOrderRejectsQuantityBeyondRemainingUserLimit(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "limited-checkout-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("max_purchase_per_user", 2).Error; err != nil {
		t.Fatalf("failed to set max_purchase_per_user: %v", err)
	}
	existingSubscription := &model.UserSubscription{
		UserId:      1,
		PlanId:      plan.Id,
		AmountTotal: 1000,
		StartTime:   1,
		EndTime:     9999999999,
		Status:      "active",
		Source:      "order",
	}
	if err := db.Create(existingSubscription).Error; err != nil {
		t.Fatalf("failed to create existing subscription: %v", err)
	}

	_, err := createPendingSubscriptionOrder(
		1,
		plan.Id,
		2,
		"trade-user-limit-overflow",
		"epay",
		func(currentPlan *model.SubscriptionPlan) error {
			return nil
		},
	)
	if !errors.Is(err, errSubscriptionPurchaseLimitReached) {
		t.Fatalf("expected errSubscriptionPurchaseLimitReached, got %v", err)
	}

	var orderCount int64
	if err := db.Model(&model.SubscriptionOrder{}).Where("trade_no = ?", "trade-user-limit-overflow").Count(&orderCount).Error; err != nil {
		t.Fatalf("failed to count created orders: %v", err)
	}
	if orderCount != 0 {
		t.Fatalf("expected no order for quantity beyond user limit, found %d", orderCount)
	}
}

func TestGetSubscriptionOrderTotalRoundsToTwoDecimals(t *testing.T) {
	total := getSubscriptionOrderTotal(10.015, 3)

	if total != 30.05 {
		t.Fatalf("expected rounded total 30.05, got %.6f", total)
	}
}

func TestAdminCreateSubscriptionPlanRejectsNegativeStock(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	body := AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{
			Title:         "negative-stock-plan",
			PriceAmount:   9.9,
			Currency:      "USD",
			DurationUnit:  model.SubscriptionDurationMonth,
			DurationValue: 1,
			StockTotal:    -1,
		},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/admin/plans", body, 1)

	AdminCreateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success || response.Message != "库存不能为负数" {
		t.Fatalf("expected 库存不能为负数, got success=%v message=%s", response.Success, response.Message)
	}
}

func TestAdminUpdateSubscriptionPlanResetsStockCycleWhenEnablingFromZero(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "stock-cycle-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  0,
		"stock_locked": 3,
		"stock_sold":   4,
	}).Error; err != nil {
		t.Fatalf("failed to seed stock counters: %v", err)
	}

	body := AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{
			Title:         "stock-cycle-plan",
			PriceAmount:   9.9,
			Currency:      "USD",
			DurationUnit:  model.SubscriptionDurationMonth,
			DurationValue: 1,
			Enabled:       true,
			StockTotal:    20,
		},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var after model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&after).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if after.StockTotal != 20 || after.StockLocked != 0 || after.StockSold != 0 {
		t.Fatalf("expected total=20 locked=0 sold=0, got total=%d locked=%d sold=%d", after.StockTotal, after.StockLocked, after.StockSold)
	}
}

func TestAdminUpdateSubscriptionPlanRejectsDisablingStockWhenReservedOrderExists(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "stock-block-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  5,
		"stock_locked": 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed stock counters: %v", err)
	}
	order := &model.SubscriptionOrder{
		UserId:        1,
		PlanId:        plan.Id,
		Money:         9.9,
		TradeNo:       "trade-stock-block",
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    1,
		StockReserved: 1,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to seed reserved order: %v", err)
	}

	body := AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{
			Title:         "stock-block-plan",
			PriceAmount:   9.9,
			Currency:      "USD",
			DurationUnit:  model.SubscriptionDurationMonth,
			DurationValue: 1,
			Enabled:       true,
			StockTotal:    0,
		},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success || response.Message != "存在待支付订单，暂不允许切换库存周期" {
		t.Fatalf("expected blocked stock transition, got success=%v message=%s", response.Success, response.Message)
	}
}

func TestAdminUpdateSubscriptionPlanAllowsIncreasingStockWithReservedOrders(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "stock-grow-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  30,
		"stock_locked": 8,
		"stock_sold":   7,
	}).Error; err != nil {
		t.Fatalf("failed to seed stock counters: %v", err)
	}
	order := &model.SubscriptionOrder{
		UserId:        1,
		PlanId:        plan.Id,
		Money:         79.2,
		Quantity:      8,
		TradeNo:       "trade-stock-grow",
		PaymentMethod: "epay",
		Status:        common.TopUpStatusPending,
		CreateTime:    1,
		StockReserved: 8,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("failed to seed reserved order: %v", err)
	}

	body := AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{
			Title:         "stock-grow-plan",
			PriceAmount:   9.9,
			Currency:      "USD",
			DurationUnit:  model.SubscriptionDurationMonth,
			DurationValue: 1,
			Enabled:       true,
			StockTotal:    40,
		},
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var after model.SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&after).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if after.StockTotal != 40 || after.StockLocked != 8 || after.StockSold != 7 {
		t.Fatalf("expected total=40 locked=8 sold=7, got total=%d locked=%d sold=%d", after.StockTotal, after.StockLocked, after.StockSold)
	}
}

func TestGetSubscriptionPlansIncludesStockAvailable(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stock-api@example.com", "stock_api", "")
	seedActiveSubscriptionReferralBinding(t, user.Id, "default", model.ReferralLevelTypeDirect, 0)
	plan := seedSubscriptionPlan(t, db, "stock-api-plan")
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  10,
		"stock_locked": 2,
		"stock_sold":   3,
	}).Error; err != nil {
		t.Fatalf("failed to seed stock counters: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/subscription/plans", nil, user.Id)
	GetSubscriptionPlans(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var plans []SubscriptionPlanDTO
	if err := common.Unmarshal(response.Data, &plans); err != nil {
		t.Fatalf("failed to decode plan response: %v", err)
	}
	if len(plans) == 0 {
		t.Fatal("expected at least one subscription plan")
	}
	if plans[0].Plan.StockAvailable != 5 {
		t.Fatalf("expected stock_available=5, got %d", plans[0].Plan.StockAvailable)
	}
}
