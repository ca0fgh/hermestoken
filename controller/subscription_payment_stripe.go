package controller

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/thanhpk/randstr"
)

var subscriptionStripeCheckoutLinkGenerator = genStripeSubscriptionLink

func SubscriptionRequestStripePay(c *gin.Context) {
	var req dto.SubscriptionPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	quantity, err := req.GetQuantity()
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
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
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	payLink, err := subscriptionStripeCheckoutLinkGenerator(referenceId, user.StripeCustomer, user.Email, plan.StripePriceId, int64(quantity))
	if err != nil {
		_ = model.ExpireSubscriptionOrder(referenceId)
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
