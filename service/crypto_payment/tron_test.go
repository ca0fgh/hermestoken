package crypto_payment

import (
	"testing"

	"github.com/ca0fgh/hermestoken/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real TRON mainnet values; the hex forms were read back off-chain from an actual
// USDT transfer's transaction info. The previous test asserted invented base58
// strings ("TToAddress1111..."), so it happily confirmed a comparison that can
// never match what a node actually returns.
const (
	tronMainnetUSDT      = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	tronMainnetUSDTHex   = "a614f803b6fd780986a42c78ec9c7f77e6ded13c"
	tronReceiveAddress   = "TDrQ1DQsmNV69ezyFE9We1vX6ozDQNAwze"
	tronReceiveHex       = "2a96c6a6ccacf972d136903ec0aa6e771279fe93"
	tronSenderAddress    = "TRWe8ehhfZU1aswY33PePSBXS6wLk5cDvS"
	tronSenderHex        = "aa7bc281547e8b7e5c7ff0c58a0a34ecbf0ed18d"
	tronTransferTopicHex = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

func TestTronAddressRoundTripsAgainstMainnetValues(t *testing.T) {
	hexForm, err := tronHexFromAddress(tronMainnetUSDT)
	require.NoError(t, err)
	assert.Equal(t, tronMainnetUSDTHex, hexForm)

	base58Form, err := tronAddressFromHex(tronMainnetUSDTHex)
	require.NoError(t, err)
	assert.Equal(t, tronMainnetUSDT, base58Form)

	// An event topic carries the address right-aligned in a 32-byte word.
	fromTopic, err := tronAddressFromHex("000000000000000000000000" + tronReceiveHex)
	require.NoError(t, err)
	assert.Equal(t, tronReceiveAddress, fromTopic)
}

func TestTronAddressRejectsCorruptedInput(t *testing.T) {
	_, err := tronHexFromAddress("TDrQ1DQsmNV69ezyFE9We1vX6ozDQNAwzf")
	assert.Error(t, err, "one changed character must fail the checksum, not decode to another wallet")

	_, err = tronHexFromAddress("0xaeaf7ECc0337c06911bc01C227135e3646B77580")
	assert.Error(t, err, "an EVM address is not a TRON address")
}

func tronTestScanner(t *testing.T) *TronScanner {
	t.Helper()
	scanner := NewTronScanner(setting.CryptoPaymentNetworkConfig{
		Network:        "tron_trc20",
		Contract:       tronMainnetUSDT,
		ReceiveAddress: tronReceiveAddress,
		Decimals:       6,
		Confirmations:  20,
	})
	require.NoError(t, scanner.configErr)
	return scanner
}

func tronTransferLog(toHex string, amountHex string) tronContractLog {
	return tronContractLog{
		Address: tronMainnetUSDTHex,
		Topics: []string{
			tronTransferTopicHex,
			"000000000000000000000000" + tronSenderHex,
			"000000000000000000000000" + toHex,
		},
		Data: amountHex,
	}
}

// TronGrid reports log addresses as bare hex while the config holds base58. The
// old scanner compared the two forms directly, so an incoming deposit was never
// recognised on any amount.
func TestTronScannerDecodesIncomingTransferFromHexLog(t *testing.T) {
	scanner := tronTestScanner(t)
	info := tronTransactionInfo{
		BlockNumber:    84417516,
		BlockTimeStamp: 1783900000000,
		Log: []tronContractLog{
			// 0xf5045 = 1003589 base units = 1.003589 USDT, the shape a generated
			// pay amount actually takes (base amount plus a unique suffix).
			tronTransferLog(tronReceiveHex, "00000000000000000000000000000000000000000000000000000000000f5045"),
		},
	}

	transfers := scanner.decodeIncomingTransfers("abc123", info, 84417536)
	require.Len(t, transfers, 1)

	transfer := transfers[0]
	assert.Equal(t, "abc123", transfer.TxHash)
	assert.Equal(t, 0, transfer.LogIndex)
	assert.EqualValues(t, 84417516, transfer.BlockNumber)
	assert.EqualValues(t, 1783900000, transfer.BlockTimestamp, "TRON reports milliseconds")
	assert.Equal(t, "1003589", transfer.AmountBaseUnits)
	assert.Equal(t, tronReceiveAddress, transfer.ToAddress, "stored in the base58 form the order carries")
	assert.Equal(t, tronSenderAddress, transfer.FromAddress)
	assert.Equal(t, tronMainnetUSDT, transfer.TokenContract)
	assert.EqualValues(t, 21, transfer.Confirmations)
}

func TestTronScannerIgnoresTransfersToOtherAddresses(t *testing.T) {
	scanner := tronTestScanner(t)
	info := tronTransactionInfo{
		BlockNumber: 84417516,
		Log:         []tronContractLog{tronTransferLog(tronSenderHex, "0000000000000000000000000000000000000000000000000000000000000064")},
	}
	assert.Empty(t, scanner.decodeIncomingTransfers("abc123", info, 84417536))
}

func TestTronScannerIgnoresOtherTokenContracts(t *testing.T) {
	scanner := tronTestScanner(t)
	log := tronTransferLog(tronReceiveHex, "0000000000000000000000000000000000000000000000000000000000000064")
	log.Address = "ea51342dabbb928ae1e576bd39eff8aaf070a8c6"
	info := tronTransactionInfo{BlockNumber: 84417516, Log: []tronContractLog{log}}
	assert.Empty(t, scanner.decodeIncomingTransfers("abc123", info, 84417536))
}

// The transfer's position among the transaction's logs is its log index, and it is
// what keeps two transfers in one transaction apart on the (network, tx_hash,
// log_index) key.
func TestTronScannerUsesTheRealLogIndex(t *testing.T) {
	scanner := tronTestScanner(t)
	info := tronTransactionInfo{
		BlockNumber: 84417516,
		Log: []tronContractLog{
			tronTransferLog(tronSenderHex, "0000000000000000000000000000000000000000000000000000000000000001"),
			tronTransferLog(tronReceiveHex, "0000000000000000000000000000000000000000000000000000000000000002"),
		},
	}
	transfers := scanner.decodeIncomingTransfers("abc123", info, 84417536)
	require.Len(t, transfers, 1)
	assert.Equal(t, 1, transfers[0].LogIndex)
}

// One unreadable log must not wedge the scanner: the old decode path returned an
// error, which aborted ScanOnce before the cursor was written, so the very same
// malformed event was re-fetched and re-failed every 10 seconds forever.
func TestTronScannerSkipsUnreadableLogWithoutFailingTheScan(t *testing.T) {
	scanner := tronTestScanner(t)
	info := tronTransactionInfo{
		BlockNumber: 84417516,
		Log: []tronContractLog{
			tronTransferLog(tronReceiveHex, ""),
			tronTransferLog(tronReceiveHex, "0000000000000000000000000000000000000000000000000000000000000064"),
		},
	}
	transfers := scanner.decodeIncomingTransfers("abc123", info, 84417536)
	require.Len(t, transfers, 1, "the readable transfer still lands")
	assert.Equal(t, "100", transfers[0].AmountBaseUnits)
	assert.Equal(t, 1, transfers[0].LogIndex)
}

// The old default pointed at a contract that does not exist on TRON mainnet, so
// the scanner watched a contract that emits nothing and orders recorded a
// token_contract no real deposit could ever match.
func TestTronDefaultContractIsMainnetUSDT(t *testing.T) {
	assert.Equal(t, tronMainnetUSDT, setting.CryptoTronUSDTContract)
}

func TestNewTronScannerReportsUnusableConfigInsteadOfScanningNothing(t *testing.T) {
	scanner := NewTronScanner(setting.CryptoPaymentNetworkConfig{
		Contract:       "not-an-address",
		ReceiveAddress: tronReceiveAddress,
	})
	require.Error(t, scanner.configErr)
	assert.ErrorContains(t, scanner.ScanOnce(nil), "invalid TRON token contract")
}
