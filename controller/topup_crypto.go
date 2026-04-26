package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type cryptoTopUpOrderRequest struct {
	Network string `json:"network"`
	Amount  int64  `json:"amount"`
}

func GetCryptoTopUpConfig(c *gin.Context) {
	networks := setting.GetEnabledCryptoPaymentNetworks()
	common.ApiSuccess(c, gin.H{
		"enabled":        setting.CryptoPaymentEnabled && len(networks) > 0,
		"networks":       networks,
		"expire_minutes": setting.CryptoOrderExpireMinutes,
	})
}

func CreateCryptoTopUpOrder(c *gin.Context) {
	var req cryptoTopUpOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "充值金额无效")
		return
	}

	config, ok := resolveCryptoNetworkConfig(req.Network)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "USDT 网络不可用"})
		return
	}

	order, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                c.GetInt("id"),
		Network:               config.Network,
		Amount:                req.Amount,
		ReceiveAddress:        config.ReceiveAddress,
		TokenContract:         config.Contract,
		TokenDecimals:         config.Decimals,
		RequiredConfirmations: config.Confirmations,
		ExpireMinutes:         setting.CryptoOrderExpireMinutes,
		SuffixMax:             setting.CryptoUniqueSuffixMax,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, cryptoOrderResponse(order))
}

func GetCryptoTopUpOrder(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	order := model.GetCryptoPaymentOrderByTradeNo(tradeNo)
	if order == nil || order.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	var err error
	order, err = model.ExpireCryptoPaymentOrderIfNeeded(order, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cryptoOrderResponse(order))
}

func resolveCryptoNetworkConfig(network string) (setting.CryptoPaymentNetworkConfig, bool) {
	for _, cfg := range setting.GetEnabledCryptoPaymentNetworks() {
		if cfg.Network == model.NormalizeCryptoNetwork(network) {
			return cfg, true
		}
	}
	return setting.CryptoPaymentNetworkConfig{}, false
}

func cryptoOrderResponse(order *model.CryptoPaymentOrder) gin.H {
	return gin.H{
		"trade_no":               order.TradeNo,
		"network":                order.Network,
		"token":                  order.TokenSymbol,
		"receive_address":        order.ReceiveAddress,
		"base_amount":            order.BaseAmount,
		"pay_amount":             order.PayAmount,
		"expires_at":             order.ExpiresAt,
		"required_confirmations": order.RequiredConfirmations,
		"status":                 order.Status,
		"tx_hash":                order.MatchedTxHash,
		"confirmations":          model.GetCryptoOrderConfirmations(order.Id),
	}
}
