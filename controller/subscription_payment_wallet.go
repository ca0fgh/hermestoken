package controller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-gonic/gin"
)

func SubscriptionRequestWalletPay(c *gin.Context) {
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

	userId := c.GetInt("id")
	if !requireActiveSubscriptionReferral(c) {
		return
	}
	randomPart := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo := fmt.Sprintf("SUBWALUSR%dNO%s", userId, randomPart)
	result, err := model.PurchaseSubscriptionWithWallet(userId, req.PlanId, quantity, tradeNo)
	if err != nil {
		if errors.Is(err, model.ErrSubscriptionWalletInsufficientBalance) ||
			errors.Is(err, model.ErrSubscriptionPlanOutOfStock) ||
			errors.Is(err, model.ErrSubscriptionWalletQuotaInvalid) {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data":    result,
	})
}
