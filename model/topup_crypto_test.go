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
