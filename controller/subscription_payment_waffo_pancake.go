package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

func SubscriptionRequestWaffoPancakePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req dto.SubscriptionPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// Plan targets its own Pancake product, so we only require credentials
	// here — not the gateway-level WaffoPancakeProductID.
	if strings.TrimSpace(setting.WaffoPancakeMerchantID) == "" ||
		strings.TrimSpace(setting.WaffoPancakePrivateKey) == "" {
		common.ApiErrorMsg(c, "Waffo Pancake 未配置或密钥无效")
		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	quantity, err := req.GetQuantity()
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// WAFFO_PANCAKE_SUB- prefix (vs. wallet's WAFFO_PANCAKE-) drives webhook
	// dispatch in WaffoPancakeWebhook.
	tradeNo := fmt.Sprintf("WAFFO_PANCAKE_SUB-%d-%d-%s", userId, time.Now().UnixMilli(), randstr.String(6))

	plan, err := createPendingSubscriptionOrder(
		userId,
		req.PlanId,
		quantity,
		tradeNo,
		model.PaymentMethodWaffoPancake,
		func(plan *model.SubscriptionPlan) error {
			if !plan.Enabled {
				return fmt.Errorf("套餐未启用")
			}
			if strings.TrimSpace(plan.WaffoPancakeProductId) == "" {
				return fmt.Errorf("该套餐未配置 WaffoPancakeProductId")
			}
			return nil
		},
		model.PaymentProviderWaffoPancake,
	)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Waffo Pancake 订阅订单创建失败 user_id=%d plan_id=%d trade_no=%s error=%q", userId, req.PlanId, tradeNo, err.Error()))
		common.ApiError(c, err)
		return
	}

	expiresInSeconds := 45 * 60
	payMoney := getSubscriptionOrderTotal(plan.PriceAmount, quantity)
	session, err := service.CreateWaffoPancakeCheckoutSession(c.Request.Context(), &service.WaffoPancakeCreateSessionParams{
		ProductID:     plan.WaffoPancakeProductId,
		LocalTradeNo:  tradeNo,
		OrderType:     service.WaffoPancakeOrderTypeSubscription,
		BuyerIdentity: service.WaffoPancakeBuyerIdentityFromUserID(user.Id),
		PriceSnapshot: &service.WaffoPancakePriceSnapshot{
			Amount:      decimal.NewFromFloat(payMoney).StringFixed(2),
			TaxCategory: "saas",
		},
		BuyerEmail:       getWaffoPancakeBuyerEmail(user),
		ExpiresInSeconds: &expiresInSeconds,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Waffo Pancake 订阅结账会话创建失败 user_id=%d plan_id=%d trade_no=%s error=%q", userId, plan.Id, tradeNo, err.Error()))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentProviderWaffoPancake)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Waffo Pancake 订阅订单创建成功 user_id=%d plan_id=%d trade_no=%s session_id=%s money=%.2f quantity=%d", userId, plan.Id, tradeNo, session.SessionID, payMoney, quantity))

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url":     session.CheckoutURL,
			"session_id":       session.SessionID,
			"expires_at":       session.ExpiresAt,
			"order_id":         tradeNo,
			"token":            session.Token,
			"token_expires_at": session.TokenExpiresAt,
		},
	})
}
