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

func TestCompleteCryptoTopUpCreditsQuotaOnce(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	insertUserForPaymentGuardTest(t, 901, 0)
	order := seedCryptoOrderForCompletion(t, 901, 10, "10003721")
	evidence := CryptoTxEvidence{
		Network:         order.Network,
		TxHash:          "0xcomplete1",
		LogIndex:        0,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   int64(order.RequiredConfirmations),
		BlockNumber:     100,
		BlockTimestamp:  time.Now().Unix(),
		FromAddress:     "sender",
		RawPayload:      `{"ok":true}`,
	}

	require.NoError(t, CompleteCryptoTopUp(order.TradeNo, evidence))
	require.NoError(t, CompleteCryptoTopUp(order.TradeNo, evidence))
	assert.Equal(t, 5000000, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, order.TradeNo))
}

func TestCompleteCryptoTopUpRejectsAmountMismatch(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 902, 0)
	order := seedCryptoOrderForCompletion(t, 902, 10, "10003721")
	err := CompleteCryptoTopUp(order.TradeNo, CryptoTxEvidence{
		Network:         order.Network,
		TxHash:          "0xmismatch",
		LogIndex:        0,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		AmountBaseUnits: "10000000",
		Confirmations:   int64(order.RequiredConfirmations),
	})
	require.ErrorIs(t, err, ErrCryptoTransactionMismatch)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 902))
}

func TestCompleteCryptoTopUpRejectsReusedTransaction(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 903, 0)
	insertUserForPaymentGuardTest(t, 904, 0)
	firstOrder := seedCryptoOrderForCompletion(t, 903, 10, "10003721")
	secondOrder := seedCryptoOrderForCompletion(t, 904, 10, "10003721")
	require.NoError(t, DB.Create(&CryptoPaymentTransaction{
		Network:         firstOrder.Network,
		TxHash:          "0xreused",
		LogIndex:        0,
		ToAddress:       firstOrder.ReceiveAddress,
		TokenContract:   firstOrder.TokenContract,
		TokenSymbol:     CryptoTokenUSDT,
		TokenDecimals:   firstOrder.TokenDecimals,
		Amount:          firstOrder.PayAmount,
		AmountBaseUnits: firstOrder.PayAmountBaseUnits,
		Confirmations:   20,
		Status:          CryptoTransactionStatusConfirmed,
		MatchedOrderId:  firstOrder.Id,
		CreateTime:      time.Now().Unix(),
		UpdateTime:      time.Now().Unix(),
	}).Error)

	err := CompleteCryptoTopUp(secondOrder.TradeNo, CryptoTxEvidence{
		Network:         secondOrder.Network,
		TxHash:          "0xreused",
		LogIndex:        0,
		ToAddress:       secondOrder.ReceiveAddress,
		TokenContract:   secondOrder.TokenContract,
		AmountBaseUnits: secondOrder.PayAmountBaseUnits,
		Confirmations:   int64(secondOrder.RequiredConfirmations),
	})
	require.ErrorIs(t, err, ErrCryptoTransactionMismatch)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 904))
}

func seedCryptoOrderForCompletion(t *testing.T, userID int, amount int64, payUnits string) *CryptoPaymentOrder {
	t.Helper()
	topUp := &TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         10.003721,
		TradeNo:       "crypto-complete-" + time.Now().Format("150405.000000000"),
		PaymentMethod: PaymentMethodCryptoUSDT,
		Currency:      CryptoTokenUSDT,
		Status:        common.TopUpStatusPending,
		CreateTime:    time.Now().Unix(),
	}
	require.NoError(t, DB.Create(topUp).Error)
	order := &CryptoPaymentOrder{
		TopUpId:               topUp.Id,
		TradeNo:               topUp.TradeNo,
		UserId:                userID,
		Network:               CryptoNetworkTronTRC20,
		TokenSymbol:           CryptoTokenUSDT,
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		BaseAmount:            "10.000000",
		PayAmount:             "10.003721",
		PayAmountBaseUnits:    payUnits,
		UniqueSuffix:          3721,
		ExpiresAt:             time.Now().Add(10 * time.Minute).Unix(),
		RequiredConfirmations: 20,
		Status:                CryptoPaymentStatusConfirmed,
		MatchedLogIndex:       -1,
		CreateTime:            time.Now().Unix(),
		UpdateTime:            time.Now().Unix(),
	}
	require.NoError(t, DB.Create(order).Error)
	return order
}

func TestRecordCryptoTransferMatchesPendingOrder(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 1001, 0)
	order := seedCryptoOrderForCompletion(t, 1001, 10, "10003721")
	order.Status = CryptoPaymentStatusPending
	require.NoError(t, DB.Save(order).Error)

	tx, matched, err := RecordCryptoTransfer(CryptoObservedTransfer{
		Network:         order.Network,
		TxHash:          "0xseen1",
		LogIndex:        0,
		BlockNumber:     123,
		FromAddress:     "sender",
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		TokenDecimals:   order.TokenDecimals,
		Amount:          order.PayAmount,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   1,
		ObservedAt:      time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, matched)
	assert.Equal(t, order.Id, matched.Id)
	assert.Equal(t, CryptoPaymentStatusDetected, GetCryptoPaymentOrderByTradeNo(order.TradeNo).Status)
}

func TestRecordCryptoTransferMarksLatePaidForExpiredExactAmount(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 1002, 0)
	order := seedCryptoOrderForCompletion(t, 1002, 10, "10003721")
	order.Status = CryptoPaymentStatusExpired
	order.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, DB.Save(order).Error)

	_, matched, err := RecordCryptoTransfer(CryptoObservedTransfer{
		Network:         order.Network,
		TxHash:          "0xlate1",
		LogIndex:        0,
		BlockNumber:     124,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		TokenDecimals:   order.TokenDecimals,
		Amount:          order.PayAmount,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   1,
		ObservedAt:      time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, matched)
	assert.Equal(t, CryptoPaymentStatusLatePaid, GetCryptoPaymentOrderByTradeNo(order.TradeNo).Status)
}

func TestCryptoScannerStateUpsert(t *testing.T) {
	truncateTables(t)
	require.NoError(t, UpsertCryptoScannerState(CryptoNetworkBSCERC20, 100, 85))
	state, err := GetCryptoScannerState(CryptoNetworkBSCERC20)
	require.NoError(t, err)
	assert.EqualValues(t, 100, state.LastScannedBlock)
	assert.EqualValues(t, 85, state.LastFinalizedBlock)
}

func TestCompleteReadyCryptoOrders(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	insertUserForPaymentGuardTest(t, 1101, 0)
	order := seedCryptoOrderForCompletion(t, 1101, 10, "10003721")
	order.Status = CryptoPaymentStatusDetected
	order.MatchedTxHash = "0xready"
	order.MatchedLogIndex = 0
	require.NoError(t, DB.Save(order).Error)
	require.NoError(t, DB.Create(&CryptoPaymentTransaction{
		Network:         order.Network,
		TxHash:          "0xready",
		LogIndex:        0,
		BlockNumber:     100,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		TokenSymbol:     CryptoTokenUSDT,
		TokenDecimals:   order.TokenDecimals,
		Amount:          order.PayAmount,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   20,
		Status:          CryptoTransactionStatusConfirmed,
		MatchedOrderId:  order.Id,
		CreateTime:      time.Now().Unix(),
		UpdateTime:      time.Now().Unix(),
	}).Error)

	completed, err := CompleteReadyCryptoOrders(order.Network)
	require.NoError(t, err)
	assert.Equal(t, 1, completed)
	assert.Equal(t, 5000000, getUserQuotaForPaymentGuardTest(t, 1101))
}

func TestRecordCryptoTransferDoesNotReassignMatchedTransaction(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 1201, 0)
	insertUserForPaymentGuardTest(t, 1202, 0)
	firstOrder := seedCryptoOrderForCompletion(t, 1201, 10, "10003721")
	secondOrder := seedCryptoOrderForCompletion(t, 1202, 10, "10003721")
	secondOrder.Status = CryptoPaymentStatusPending
	require.NoError(t, DB.Save(secondOrder).Error)
	require.NoError(t, DB.Create(&CryptoPaymentTransaction{
		Network:         firstOrder.Network,
		TxHash:          "0xalready-matched",
		LogIndex:        0,
		ToAddress:       firstOrder.ReceiveAddress,
		TokenContract:   firstOrder.TokenContract,
		TokenSymbol:     CryptoTokenUSDT,
		TokenDecimals:   firstOrder.TokenDecimals,
		Amount:          firstOrder.PayAmount,
		AmountBaseUnits: firstOrder.PayAmountBaseUnits,
		Confirmations:   1,
		Status:          CryptoTransactionStatusSeen,
		MatchedOrderId:  firstOrder.Id,
		CreateTime:      time.Now().Unix(),
		UpdateTime:      time.Now().Unix(),
	}).Error)

	_, matched, err := RecordCryptoTransfer(CryptoObservedTransfer{
		Network:         secondOrder.Network,
		TxHash:          "0xalready-matched",
		LogIndex:        0,
		BlockNumber:     125,
		ToAddress:       secondOrder.ReceiveAddress,
		TokenContract:   secondOrder.TokenContract,
		TokenDecimals:   secondOrder.TokenDecimals,
		Amount:          secondOrder.PayAmount,
		AmountBaseUnits: secondOrder.PayAmountBaseUnits,
		Confirmations:   2,
		ObservedAt:      time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, matched)
	assert.Equal(t, firstOrder.Id, matched.Id)

	var tx CryptoPaymentTransaction
	require.NoError(t, DB.Where("tx_hash = ?", "0xalready-matched").First(&tx).Error)
	assert.Equal(t, firstOrder.Id, tx.MatchedOrderId)
	assert.Equal(t, CryptoPaymentStatusPending, GetCryptoPaymentOrderByTradeNo(secondOrder.TradeNo).Status)
}
