package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
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
