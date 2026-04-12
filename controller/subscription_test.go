package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSubscriptionControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
	); err != nil {
		t.Fatalf("failed to migrate subscription tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
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
		t.Fatalf("expected plan to be deleted, found %d rows", count)
	}
}

func TestAdminDeleteSubscriptionPlanRejectsReferencedOrder(t *testing.T) {
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
	if response.Success {
		t.Fatalf("expected delete to be rejected for referenced order")
	}

	var count int64
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count subscription plans: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected plan to remain after failed delete, found %d rows", count)
	}
}

func TestAdminDeleteSubscriptionPlanRejectsReferencedUserSubscription(t *testing.T) {
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
	if response.Success {
		t.Fatalf("expected delete to be rejected for referenced user subscription")
	}

	var count int64
	if err := db.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count subscription plans: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected plan to remain after failed delete, found %d rows", count)
	}
}

func TestCreatePendingSubscriptionOrderCreatesOrder(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	plan := seedSubscriptionPlan(t, db, "checkout-plan")

	lockedPlan, err := createPendingSubscriptionOrder(
		1,
		plan.Id,
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
