package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

var subscriptionEpayClientProvider = GetEpayClient
var subscriptionEpayPurchase = func(client *epay.Client, args *epay.PurchaseArgs) (string, map[string]string, error) {
	return client.Purchase(args)
}
var subscriptionEpayVerify = func(client *epay.Client, params map[string]string) (*epay.VerifyRes, error) {
	return client.Verify(params)
}

func SubscriptionRequestEpay(c *gin.Context) {
	var req dto.SubscriptionEpayPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	quantity, err := req.GetQuantity()
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}

	userId := c.GetInt("id")
	callBackAddress := service.GetCallbackAddress()
	returnUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/return")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	notifyUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/notify")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}

	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("SUBUSR%dNO%s", userId, tradeNo)

	client := subscriptionEpayClientProvider()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}

	plan, err := createPendingSubscriptionOrder(
		userId,
		req.PlanId,
		quantity,
		tradeNo,
		req.PaymentMethod,
		func(plan *model.SubscriptionPlan) error {
			if !plan.Enabled {
				return fmt.Errorf("套餐未启用")
			}
			if plan.PriceAmount < 0.01 {
				return fmt.Errorf("套餐金额过低")
			}
			return nil
		},
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total := getSubscriptionOrderTotal(plan.PriceAmount, quantity)
	uri, params, err := subscriptionEpayPurchase(client, &epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("SUB:%s", plan.Title),
		Money:          strconv.FormatFloat(total, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		_ = model.ExpireSubscriptionOrder(tradeNo)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

func SubscriptionEpayNotify(c *gin.Context) {
	var params map[string]string

	if c.Request.Method == "POST" {
		// POST 请求：从 POST body 解析参数
		if err := c.Request.ParseForm(); err != nil {
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET 请求：从 URL Query 解析参数
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	client := GetEpayClient()
	if client == nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	if verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)

	if err := model.CompleteSubscriptionOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo)); err != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	_, _ = c.Writer.Write([]byte("success"))
}

// SubscriptionEpayReturn handles browser return after payment.
// It verifies the payload and completes the order, then redirects to console.
func SubscriptionEpayReturn(c *gin.Context) {
	target := subscriptionResultURL("fail")

	params, err := parseEpayParams(c)
	if err != nil || len(params) == 0 {
		renderBrowserRedirect(c, target)
		return
	}

	client := subscriptionEpayClientProvider()
	if client == nil {
		renderBrowserRedirect(c, target)
		return
	}
	verifyInfo, err := subscriptionEpayVerify(client, params)
	if err != nil || !verifyInfo.VerifyStatus {
		renderBrowserRedirect(c, target)
		return
	}
	if verifyInfo.TradeStatus == epay.StatusTradeSuccess {
		LockOrder(verifyInfo.ServiceTradeNo)
		defer UnlockOrder(verifyInfo.ServiceTradeNo)
		if err := model.CompleteSubscriptionOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo)); err != nil {
			renderBrowserRedirect(c, target)
			return
		}
		restoreSubscriptionOrderSession(c, verifyInfo.ServiceTradeNo)
		renderBrowserRedirect(c, subscriptionResultURL("success"))
		return
	}
	renderBrowserRedirect(c, subscriptionResultURL("pending"))
}

func subscriptionResultURL(status string) string {
	return topUpResultURL(status)
}

func restoreSubscriptionOrderSession(c *gin.Context, tradeNo string) {
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.UserId <= 0 {
		return
	}
	user, err := model.GetUserById(order.UserId, false)
	if err != nil || user == nil {
		return
	}
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	session.Set("group", user.Group)
	_ = session.Save()
}
