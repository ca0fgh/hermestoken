package controller

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
)

var subscriptionCreemCheckoutLinkGenerator = genCreemLinkWithUnits

func SubscriptionRequestCreemPay(c *gin.Context) {
	var req dto.SubscriptionPaymentRequest

	// Keep body for debugging consistency (like RequestCreemPay)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("read subscription creem pay req body err: %v", err)
		c.JSON(200, gin.H{"message": "error", "data": "read query error"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		c.JSON(200, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if !isCreemTopupSwitchEnabled() {
		common.ApiErrorMsg(c, "当前管理员未开启 Creem 支付")
		return
	}

	quantity, err := req.GetQuantity()
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if !requireActiveSubscriptionReferral(c) {
		return
	}

	if setting.CreemWebhookSecret == "" && !setting.CreemTestMode {
		common.ApiErrorMsg(c, "Creem Webhook 未配置")
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

	reference := "sub-creem-ref-" + randstr.String(6)
	referenceId := "sub_ref_" + common.Sha1([]byte(reference+time.Now().String()+user.Username))

	plan, err := createPendingSubscriptionOrder(
		userId,
		req.PlanId,
		quantity,
		referenceId,
		PaymentMethodCreem,
		func(plan *model.SubscriptionPlan) error {
			if !plan.Enabled {
				return fmt.Errorf("套餐未启用")
			}
			if plan.CreemProductId == "" {
				return fmt.Errorf("该套餐未配置 CreemProductId")
			}
			return nil
		},
		model.PaymentProviderCreem,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Reuse Creem checkout generator by building a lightweight product reference.
	currency := "USD"
	switch operation_setting.GetGeneralSetting().QuotaDisplayType {
	case operation_setting.QuotaDisplayTypeCNY:
		currency = "CNY"
	case operation_setting.QuotaDisplayTypeUSD:
		currency = "USD"
	default:
		currency = "USD"
	}
	product := &CreemProduct{
		ProductId: plan.CreemProductId,
		Name:      plan.Title,
		Price:     plan.PriceAmount,
		Currency:  currency,
		Quota:     0,
	}

	checkoutUrl, err := subscriptionCreemCheckoutLinkGenerator(referenceId, product, user.Email, user.Username, quantity)
	if err != nil {
		_ = model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderCreem)
		log.Printf("获取Creem支付链接失败: %v", err)
		c.JSON(200, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url": checkoutUrl,
			"order_id":     referenceId,
		},
	})
}
