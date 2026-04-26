package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCryptoPayAmountFromSuffix_TRONSixDecimals(t *testing.T) {
	pay, units, err := CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 6, 3721)
	require.NoError(t, err)
	assert.Equal(t, "10.003721", pay)
	assert.Equal(t, "10003721", units)
}

func TestCryptoPayAmountFromSuffix_BSCUnitsWith18Decimals(t *testing.T) {
	pay, units, err := CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 18, 3721)
	require.NoError(t, err)
	assert.Equal(t, "10.003721", pay)
	assert.Equal(t, "10003721000000000000", units)
}

func TestCryptoPayAmountFromSuffixRejectsInvalidSuffix(t *testing.T) {
	_, _, err := CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 6, 0)
	require.Error(t, err)
	_, _, err = CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 6, 10000)
	require.Error(t, err)
}

func TestCryptoOrderActiveStatus(t *testing.T) {
	assert.True(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusPending))
	assert.True(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusDetected))
	assert.False(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusSuccess))
	assert.False(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusExpired))
}

func TestCryptoOrderIsExpired(t *testing.T) {
	order := &CryptoPaymentOrder{ExpiresAt: time.Now().Add(-time.Second).Unix()}
	assert.True(t, order.IsExpired(time.Now()))
}

func TestCryptoPaymentMethodConstant(t *testing.T) {
	assert.Equal(t, "crypto_usdt", PaymentMethodCryptoUSDT)
	assert.Equal(t, common.TopUpStatusPending, CryptoTopUpInitialStatus())
}

func TestCryptoTablesAutoMigrate(t *testing.T) {
	truncateTables(t)
	require.True(t, DB.Migrator().HasTable(&CryptoPaymentOrder{}))
	require.True(t, DB.Migrator().HasTable(&CryptoPaymentTransaction{}))
	require.True(t, DB.Migrator().HasTable(&CryptoScannerState{}))
}

func TestCreateCryptoTopUpOrderCreatesTopUpAndCryptoOrder(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	insertUserForPaymentGuardTest(t, 701, 0)

	input := CreateCryptoTopUpOrderInput{
		UserID:                701,
		Network:               CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
		Now:                   time.Unix(1000, 0),
		SuffixGenerator: func(max int) int {
			return 3721
		},
	}

	order, err := CreateCryptoTopUpOrder(input)
	require.NoError(t, err)
	assert.Equal(t, "10.003721", order.PayAmount)
	assert.Equal(t, "10003721", order.PayAmountBaseUnits)
	assert.Equal(t, CryptoPaymentStatusPending, order.Status)
	assert.Equal(t, int64(1600), order.ExpiresAt)

	topUp := GetTopUpByTradeNo(order.TradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, PaymentMethodCryptoUSDT, topUp.PaymentMethod)
	assert.Equal(t, "USDT", topUp.Currency)
	assert.Equal(t, int64(10), topUp.Amount)
	assert.Equal(t, 10.003721, topUp.Money)
}

func TestCreateCryptoTopUpOrderRejectsActiveAmountCollision(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 702, 0)

	input := CreateCryptoTopUpOrderInput{
		UserID:                702,
		Network:               CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
		Now:                   time.Unix(1000, 0),
		SuffixGenerator: func(max int) int {
			return 3721
		},
	}
	_, err := CreateCryptoTopUpOrder(input)
	require.NoError(t, err)
	_, err = CreateCryptoTopUpOrder(input)
	require.ErrorIs(t, err, ErrCryptoAmountCollision)
}
