package controller

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/price"
	"github.com/thanhpk/randstr"
)

var subscriptionStripeCheckoutLinkGenerator = genStripeSubscriptionLink
var subscriptionStripeUnitAmountResolver = fetchStripeUnitAmount

func SubscriptionRequestStripePay(c *gin.Context) {
	var req dto.SubscriptionPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if !isStripeTopupSwitchEnabled() {
		common.ApiErrorMsg(c, "当前管理员未开启 Stripe 支付")
		return
	}

	quantity, err := req.GetQuantity()
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if !requireActiveSubscriptionReferral(c) {
		return
	}

	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}
	if setting.StripeWebhookSecret == "" {
		common.ApiErrorMsg(c, "Stripe Webhook 未配置")
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
	planPreview, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if planPreview == nil {
		common.ApiErrorMsg(c, "套餐不存在")
		return
	}
	if planPreview.StripePriceId == "" {
		common.ApiErrorMsg(c, "该套餐未配置 StripePriceId")
		return
	}
	unitAmount, err := subscriptionStripeUnitAmountResolver(planPreview.StripePriceId)
	if err != nil {
		common.ApiErrorMsg(c, "Stripe PriceId 配置错误")
		return
	}
	actualUnitMoney := moneyStringFromMinorUnits(unitAmount, "USD")
	expectedUnitMoney := fmt.Sprintf("%.2f", planPreview.PriceAmount)
	if actualUnitMoney != expectedUnitMoney {
		common.ApiErrorMsg(c, "Stripe PriceId 金额与套餐价格不一致")
		return
	}

	reference := fmt.Sprintf("sub-stripe-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "sub_ref_" + common.Sha1([]byte(reference))

	plan, err := createPendingSubscriptionOrder(
		userId,
		req.PlanId,
		quantity,
		referenceId,
		PaymentMethodStripe,
		func(plan *model.SubscriptionPlan) error {
			if !plan.Enabled {
				return fmt.Errorf("套餐未启用")
			}
			if plan.StripePriceId == "" {
				return fmt.Errorf("该套餐未配置 StripePriceId")
			}
			return nil
		},
		model.PaymentProviderStripe,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	payLink, err := subscriptionStripeCheckoutLinkGenerator(referenceId, user.StripeCustomer, user.Email, plan.StripePriceId, int64(quantity))
	if err != nil {
		_ = model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe)
		log.Println("获取Stripe Checkout支付链接失败", err)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

func fetchStripeUnitAmount(priceID string) (int64, error) {
	if priceID == "" {
		return 0, fmt.Errorf("stripe price id is empty")
	}
	stripe.Key = setting.StripeApiSecret
	result, err := price.Get(priceID, nil)
	if err != nil {
		return 0, err
	}
	if result == nil || result.UnitAmount <= 0 {
		return 0, fmt.Errorf("invalid stripe unit amount")
	}
	return result.UnitAmount, nil
}

func genStripeSubscriptionLink(referenceId string, customerId string, email string, priceId string, quantity int64) (string, error) {
	stripe.Key = setting.StripeApiSecret
	if quantity <= 0 {
		quantity = 1
	}

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(system_setting.ServerAddress + "/console/topup"),
		CancelURL:         stripe.String(system_setting.ServerAddress + "/console/topup"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceId),
				Quantity: stripe.Int64(quantity),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}
		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := session.New(params)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
