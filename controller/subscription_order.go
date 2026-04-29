package controller

import (
	"errors"
	"time"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var errSubscriptionPurchaseLimitReached = errors.New("已达到该套餐购买上限")

func createPendingSubscriptionOrder(
	userId int,
	planId int,
	quantity int,
	tradeNo string,
	paymentMethod string,
	validate func(plan *model.SubscriptionPlan) error,
	paymentProvider ...string,
) (*model.SubscriptionPlan, error) {
	if quantity <= 0 {
		return nil, errors.New("invalid quantity")
	}

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
			if count+int64(quantity) > int64(lockedPlan.MaxPurchasePerUser) {
				return errSubscriptionPurchaseLimitReached
			}
		}
		if err := model.ReserveSubscriptionPlanStockForPendingOrderTx(tx, &lockedPlan, quantity); err != nil {
			return err
		}

		stockReserved := 0
		if lockedPlan.HasStockLimit() {
			stockReserved = quantity
		}

		orderTotal := getSubscriptionOrderTotal(lockedPlan.PriceAmount, quantity)
		provider := ""
		if len(paymentProvider) > 0 {
			provider = paymentProvider[0]
		}
		order := &model.SubscriptionOrder{
			UserId:          userId,
			PlanId:          lockedPlan.Id,
			Money:           orderTotal,
			PaymentMoney:    orderTotal,
			PaymentCurrency: lockedPlan.Currency,
			Quantity:        quantity,
			TradeNo:         tradeNo,
			PaymentMethod:   paymentMethod,
			PaymentProvider: provider,
			CreateTime:      time.Now().Unix(),
			Status:          "pending",
			StockReserved:   stockReserved,
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

func getSubscriptionOrderTotal(unitPrice float64, quantity int) float64 {
	if quantity <= 0 {
		return 0
	}
	return decimal.NewFromFloat(unitPrice).
		Mul(decimal.NewFromInt(int64(quantity))).
		Round(2).
		InexactFloat64()
}
