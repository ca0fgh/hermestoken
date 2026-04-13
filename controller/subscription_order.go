package controller

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var errSubscriptionPurchaseLimitReached = errors.New("已达到该套餐购买上限")

func createPendingSubscriptionOrder(
	userId int,
	planId int,
	tradeNo string,
	paymentMethod string,
	validate func(plan *model.SubscriptionPlan) error,
) (*model.SubscriptionPlan, error) {
	var lockedPlan model.SubscriptionPlan

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", planId).
			First(&lockedPlan).Error; err != nil {
			return err
		}

		if validate != nil {
			if err := validate(&lockedPlan); err != nil {
				return err
			}
		}

		if lockedPlan.MaxPurchasePerUser > 0 {
			var count int64
			if err := tx.Model(&model.UserSubscription{}).
				Where("user_id = ? AND plan_id = ?", userId, lockedPlan.Id).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(lockedPlan.MaxPurchasePerUser) {
				return errSubscriptionPurchaseLimitReached
			}
		}
		if err := model.ReserveSubscriptionPlanStockForPendingOrderTx(tx, &lockedPlan); err != nil {
			return err
		}

		stockReserved := 0
		if lockedPlan.HasStockLimit() {
			stockReserved = 1
		}

		order := &model.SubscriptionOrder{
			UserId:        userId,
			PlanId:        lockedPlan.Id,
			Money:         lockedPlan.PriceAmount,
			TradeNo:       tradeNo,
			PaymentMethod: paymentMethod,
			CreateTime:    time.Now().Unix(),
			Status:        "pending",
			StockReserved: stockReserved,
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &lockedPlan, nil
}
