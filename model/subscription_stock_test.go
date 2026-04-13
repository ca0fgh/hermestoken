package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"gorm.io/gorm"
)

func TestSubscriptionPlanPopulateStockAvailableUsesLockedAndSold(t *testing.T) {
	plan := &SubscriptionPlan{
		StockTotal:  10,
		StockLocked: 2,
		StockSold:   3,
	}

	plan.PopulateStockAvailable()

	if plan.StockAvailable != 5 {
		t.Fatalf("expected stock_available=5, got %d", plan.StockAvailable)
	}
}

func TestReserveSubscriptionPlanStockTxRejectsSoldOutPlan(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  1,
		"stock_locked": 1,
		"stock_sold":   0,
	}).Error; err != nil {
		t.Fatalf("failed to seed stock counters: %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var locked SubscriptionPlan
		if err := tx.Where("id = ?", plan.Id).First(&locked).Error; err != nil {
			return err
		}
		return reserveSubscriptionPlanStockTx(tx, &locked, 1)
	})
	if err == nil || err.Error() != "库存不足" {
		t.Fatalf("expected 库存不足, got %v", err)
	}
}

func TestReserveSubscriptionPlanStockTxReloadsLatestPlanState(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  2,
		"stock_locked": 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}

	stalePlan := &SubscriptionPlan{
		Id:         plan.Id,
		StockTotal: 2,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return reserveSubscriptionPlanStockTx(tx, stalePlan, 1)
	}); err != nil {
		t.Fatalf("reserveSubscriptionPlanStockTx() error = %v", err)
	}

	var afterPlan SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&afterPlan).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if afterPlan.StockLocked != 2 {
		t.Fatalf("expected stock_locked=2 after reserving against fresh state, got %d", afterPlan.StockLocked)
	}
}

func TestReserveSubscriptionPlanStockTxUsesFreshLimitWhenCallerLooksUnlimited(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  1,
		"stock_locked": 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}

	stalePlan := &SubscriptionPlan{Id: plan.Id}
	err := db.Transaction(func(tx *gorm.DB) error {
		return reserveSubscriptionPlanStockTx(tx, stalePlan, 1)
	})
	if err != ErrSubscriptionPlanOutOfStock {
		t.Fatalf("expected ErrSubscriptionPlanOutOfStock, got %v", err)
	}
}

func TestReleaseReservedSubscriptionOrderStockTxClearsReservedCount(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	order := seedPendingReferralOrder(t, db, 1, plan.Id, "stock-release-order", 9.9)

	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  5,
		"stock_locked": 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}
	if err := db.Model(&SubscriptionOrder{}).Where("id = ?", order.Id).Update("stock_reserved", 1).Error; err != nil {
		t.Fatalf("failed to seed order stock_reserved: %v", err)
	}

	if err := ExpireSubscriptionOrder(order.TradeNo); err != nil {
		t.Fatalf("ExpireSubscriptionOrder() error = %v", err)
	}

	var afterPlan SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&afterPlan).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if afterPlan.StockLocked != 0 {
		t.Fatalf("expected stock_locked=0, got %d", afterPlan.StockLocked)
	}

	var afterOrder SubscriptionOrder
	if err := db.Where("id = ?", order.Id).First(&afterOrder).Error; err != nil {
		t.Fatalf("failed to reload order: %v", err)
	}
	if afterOrder.StockReserved != 0 || afterOrder.Status != common.TopUpStatusExpired {
		t.Fatalf("expected expired order with stock_reserved=0, got status=%s reserved=%d", afterOrder.Status, afterOrder.StockReserved)
	}
}

func TestReleaseReservedSubscriptionOrderStockTxRejectsInvariantMismatch(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	order := seedPendingReferralOrder(t, db, 1, plan.Id, "stock-release-invariant-order", 9.9)

	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  5,
		"stock_locked": 0,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}
	if err := db.Model(&SubscriptionOrder{}).Where("id = ?", order.Id).Update("stock_reserved", 1).Error; err != nil {
		t.Fatalf("failed to seed order stock_reserved: %v", err)
	}
	order.StockReserved = 1

	err := db.Transaction(func(tx *gorm.DB) error {
		return releaseReservedSubscriptionOrderStockTx(tx, order, plan)
	})
	if err != ErrSubscriptionPlanStockInvariant {
		t.Fatalf("expected ErrSubscriptionPlanStockInvariant, got %v", err)
	}
}

func TestConsumeReservedSubscriptionOrderStockTxUsesFreshPlanState(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	order := seedPendingReferralOrder(t, db, 1, plan.Id, "stock-consume-reserved-order", 9.9)

	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  5,
		"stock_locked": 1,
		"stock_sold":   0,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}
	if err := db.Model(&SubscriptionOrder{}).Where("id = ?", order.Id).Update("stock_reserved", 1).Error; err != nil {
		t.Fatalf("failed to seed order stock_reserved: %v", err)
	}
	order.StockReserved = 1
	stalePlan := &SubscriptionPlan{Id: plan.Id}

	err := db.Transaction(func(tx *gorm.DB) error {
		return consumeReservedSubscriptionOrderStockTx(tx, order, stalePlan)
	})
	if err != nil {
		t.Fatalf("consumeReservedSubscriptionOrderStockTx() error = %v", err)
	}

	var afterPlan SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&afterPlan).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if afterPlan.StockLocked != 0 || afterPlan.StockSold != 1 {
		t.Fatalf("expected locked=0 sold=1, got locked=%d sold=%d", afterPlan.StockLocked, afterPlan.StockSold)
	}
}

func TestConsumeSubscriptionPlanStockDirectTxUsesFreshLimitWhenCallerLooksUnlimited(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 9.9)
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total": 1,
		"stock_sold":  1,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}

	stalePlan := &SubscriptionPlan{Id: plan.Id}
	err := db.Transaction(func(tx *gorm.DB) error {
		return consumeSubscriptionPlanStockDirectTx(tx, stalePlan, 1)
	})
	if err != ErrSubscriptionPlanOutOfStock {
		t.Fatalf("expected ErrSubscriptionPlanOutOfStock, got %v", err)
	}
}

func TestCompleteSubscriptionOrderMovesReservedStockToSold(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	user := seedReferralUser(t, db, "stock-complete-user", 0, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 18)
	order := seedPendingReferralOrder(t, db, user.Id, plan.Id, "trade-stock-complete", 18)

	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"stock_total":  5,
		"stock_locked": 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed plan stock: %v", err)
	}
	if err := db.Model(&SubscriptionOrder{}).Where("id = ?", order.Id).Update("stock_reserved", 1).Error; err != nil {
		t.Fatalf("failed to seed order stock_reserved: %v", err)
	}

	if err := CompleteSubscriptionOrder(order.TradeNo, `{"ok":true}`); err != nil {
		t.Fatalf("CompleteSubscriptionOrder() error = %v", err)
	}

	var afterPlan SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&afterPlan).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if afterPlan.StockLocked != 0 || afterPlan.StockSold != 1 {
		t.Fatalf("expected locked=0 sold=1, got locked=%d sold=%d", afterPlan.StockLocked, afterPlan.StockSold)
	}

	var afterOrder SubscriptionOrder
	if err := db.Where("id = ?", order.Id).First(&afterOrder).Error; err != nil {
		t.Fatalf("failed to reload order: %v", err)
	}
	if afterOrder.StockReserved != 0 || afterOrder.Status != common.TopUpStatusSuccess {
		t.Fatalf("expected success order with stock_reserved=0, got status=%s reserved=%d", afterOrder.Status, afterOrder.StockReserved)
	}
}

func TestAdminBindSubscriptionConsumesStockDirectly(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	user := seedReferralUser(t, db, "stock-admin-bind-user", 0, dto.UserSetting{})
	plan := seedReferralPlan(t, db, 18)
	if err := db.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Update("stock_total", 2).Error; err != nil {
		t.Fatalf("failed to seed stock_total: %v", err)
	}

	if _, err := AdminBindSubscription(user.Id, plan.Id, ""); err != nil {
		t.Fatalf("AdminBindSubscription() error = %v", err)
	}

	var afterPlan SubscriptionPlan
	if err := db.Where("id = ?", plan.Id).First(&afterPlan).Error; err != nil {
		t.Fatalf("failed to reload plan: %v", err)
	}
	if afterPlan.StockSold != 1 || afterPlan.StockLocked != 0 {
		t.Fatalf("expected sold=1 locked=0, got sold=%d locked=%d", afterPlan.StockSold, afterPlan.StockLocked)
	}
}
