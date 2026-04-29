package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCryptoControllerTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnMetadata()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.Log{},
		&model.CryptoPaymentOrder{},
		&model.CryptoPaymentTransaction{},
		&model.CryptoScannerState{},
	))
	require.NoError(t, db.Create(&model.User{
		Id:       801,
		Username: "crypto_controller_user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Quota:    0,
		Group:    "default",
	}).Error)

	setting.CryptoPaymentEnabled = true
	setting.CryptoTronEnabled = true
	setting.CryptoTronReceiveAddress = "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"
	setting.CryptoTronUSDTContract = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"
	setting.CryptoTronConfirmations = 20
	setting.CryptoOrderExpireMinutes = 10
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestGetCryptoTopUpConfig(t *testing.T) {
	setupCryptoControllerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)

	GetCryptoTopUpConfig(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tron_trc20")
	assert.Contains(t, w.Body.String(), "USDT")
}

func TestCreateCryptoTopUpOrder(t *testing.T) {
	setupCryptoControllerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/crypto/topup/order", bytes.NewReader([]byte(`{"network":"tron_trc20","amount":10}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateCryptoTopUpOrder(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "pay_amount")
	assert.Contains(t, w.Body.String(), setting.CryptoTronReceiveAddress)
	assert.Contains(t, w.Body.String(), "pending")
}

func TestCreateCryptoTopUpOrderRejectsDisabledNetwork(t *testing.T) {
	setupCryptoControllerTest(t)
	setting.CryptoTronEnabled = false
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/crypto/topup/order", bytes.NewReader([]byte(`{"network":"tron_trc20","amount":10}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateCryptoTopUpOrder(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "不可用")
}

func TestGetCryptoTopUpOrderExpiresPendingOrder(t *testing.T) {
	setupCryptoControllerTest(t)
	order, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                801,
		Network:               model.CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        setting.CryptoTronReceiveAddress,
		TokenContract:         setting.CryptoTronUSDTContract,
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.CryptoPaymentOrder{}).Where("id = ?", order.Id).Update("expires_at", int64(1)).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)
	c.Params = gin.Params{{Key: "trade_no", Value: order.TradeNo}}

	GetCryptoTopUpOrder(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}
