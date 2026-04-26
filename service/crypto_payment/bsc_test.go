package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBSCTransferLog(t *testing.T) {
	log := bscRPCLog{
		Address: "0x55d398326f99059fF775485246999027B3197955",
		Topics: []string{
			bscTransferTopic,
			"0x0000000000000000000000001111111111111111111111111111111111111111",
			"0x0000000000000000000000002222222222222222222222222222222222222222",
		},
		Data:        "0x0000000000000000000000000000000000000000000000008ac7230489e80000",
		BlockNumber: "0x64",
		TxHash:      "0xtx",
		LogIndex:    "0x1",
	}
	transfer, err := decodeBSCTransferLog(log, 18)
	require.NoError(t, err)
	assert.Equal(t, "0x1111111111111111111111111111111111111111", transfer.FromAddress)
	assert.Equal(t, "0x2222222222222222222222222222222222222222", transfer.ToAddress)
	assert.Equal(t, "10000000000000000000", transfer.AmountBaseUnits)
	assert.Equal(t, int64(100), transfer.BlockNumber)
	assert.Equal(t, 1, transfer.LogIndex)
}
