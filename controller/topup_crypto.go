package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
)

type cryptoTopUpOrderRequest struct {
	Network string  `json:"network"`
	Amount  float64 `json:"amount"`
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

func AdminListCryptoTopUpOrders(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	orders, total, err := model.ListCryptoPaymentOrders(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		items = append(items, gin.H{
			"id":                     order.Id,
			"topup_id":               order.TopUpId,
			"trade_no":               order.TradeNo,
			"user_id":                order.UserId,
			"payment_method":         model.PaymentMethodCryptoUSDT,
			"network":                order.Network,
			"token_symbol":           order.TokenSymbol,
			"receive_address":        order.ReceiveAddress,
			"base_amount":            order.BaseAmount,
			"pay_amount":             order.PayAmount,
			"pay_amount_base_units":  order.PayAmountBaseUnits,
			"required_confirmations": order.RequiredConfirmations,
			"status":                 order.Status,
			"matched_tx_hash":        order.MatchedTxHash,
			"matched_log_index":      order.MatchedLogIndex,
			"expires_at":             order.ExpiresAt,
			"create_time":            order.CreateTime,
			"update_time":            order.UpdateTime,
		})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListCryptoTransactions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	transactions, total, err := model.ListCryptoPaymentTransactions(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(transactions)
	common.ApiSuccess(c, pageInfo)
}

type adminCompleteCryptoTopUpRequest struct {
	Network         string `json:"network"`
	TxHash          string `json:"tx_hash"`
	LogIndex        int    `json:"log_index"`
	ToAddress       string `json:"to_address"`
	TokenContract   string `json:"token_contract"`
	AmountBaseUnits string `json:"amount_base_units"`
	Confirmations   int64  `json:"confirmations"`
	Reason          string `json:"reason"`
}

func AdminCompleteCryptoTopUp(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	var req adminCompleteCryptoTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil || tradeNo == "" || strings.TrimSpace(req.TxHash) == "" || strings.TrimSpace(req.Reason) == "" {
		common.ApiErrorMsg(c, "链上证据不完整")
		return
	}
	order := model.GetCryptoPaymentOrderByTradeNo(tradeNo)
	if order == nil {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	err := model.CompleteCryptoTopUp(tradeNo, model.CryptoTxEvidence{
		Network:         req.Network,
		TxHash:          req.TxHash,
		LogIndex:        req.LogIndex,
		ToAddress:       req.ToAddress,
		TokenContract:   req.TokenContract,
		AmountBaseUnits: req.AmountBaseUnits,
		Confirmations:   req.Confirmations,
		RawPayload:      common.GetJsonString(req),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(order.UserId, model.LogTypeTopup, "管理员USDT补单成功，订单: "+tradeNo+"，原因: "+req.Reason, map[string]interface{}{
		"admin_id": c.GetInt("id"),
		"ip":       c.ClientIP(),
		"network":  req.Network,
		"tx_hash":  req.TxHash,
		"reason":   req.Reason,
	})
	common.ApiSuccess(c, nil)
}
