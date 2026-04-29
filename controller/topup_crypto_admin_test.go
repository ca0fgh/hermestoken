package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminListCryptoTopUpOrders(t *testing.T) {
	setupCryptoControllerTest(t)
	_, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                801,
		Network:               model.CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/crypto/topup/orders", nil)

	AdminListCryptoTopUpOrders(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "crypto_usdt")
}

func TestAdminCompleteCryptoTopUpRequiresEvidence(t *testing.T) {
	setupCryptoControllerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "trade_no", Value: "missing"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/crypto/topup/orders/missing/complete", nil)

	AdminCompleteCryptoTopUp(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "链上证据")
}

func TestAdminCompleteCryptoTopUpWithEvidence(t *testing.T) {
	setupCryptoControllerTest(t)
	order, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                801,
		Network:               model.CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
	})
	require.NoError(t, err)
	body := `{"network":"tron_trc20","tx_hash":"0xadmin-complete","log_index":0,"to_address":"` + order.ReceiveAddress + `","token_contract":"` + order.TokenContract + `","amount_base_units":"` + order.PayAmountBaseUnits + `","confirmations":20,"reason":"manual verified"}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 99)
	c.Params = gin.Params{{Key: "trade_no", Value: order.TradeNo}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/crypto/topup/orders/"+order.TradeNo+"/complete", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")

	AdminCompleteCryptoTopUp(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)
	assert.Equal(t, "success", model.GetTopUpByTradeNo(order.TradeNo).Status)
}
