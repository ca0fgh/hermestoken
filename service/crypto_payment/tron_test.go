package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeTronTransferEvent(t *testing.T) {
	event := tronGridEvent{
		TransactionID:  "abc123",
		BlockNumber:    100,
		BlockTimestamp: 1710000000000,
		EventIndex:     2,
		Result: map[string]string{
			"from":  "TFromAddress1111111111111111111111111",
			"to":    "TToAddress111111111111111111111111111",
			"value": "10003721",
		},
	}
	transfer, err := decodeTronTransferEvent(event, 6)
	require.NoError(t, err)
	assert.Equal(t, "abc123", transfer.TxHash)
	assert.Equal(t, 2, transfer.LogIndex)
	assert.Equal(t, "10003721", transfer.AmountBaseUnits)
	assert.EqualValues(t, 100, transfer.BlockNumber)
}
