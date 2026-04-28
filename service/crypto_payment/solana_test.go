package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeSolanaTokenTransfersMatchesReceiveOwner(t *testing.T) {
	const receiveOwner = "7YttLkHDoWJYNNe7U2s1owz8FC6xk4kZqGSPdU2ovbYW"
	const tokenAccount = "H3q1wZ6JgXJZ7t9w2a1x2y3z4a5b6c7d8e9fGhijkLmN"
	const usdtMint = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"

	tx := solanaTransactionResult{
		Slot:      12345,
		BlockTime: 1710000000,
		Transaction: solanaTransaction{
			Message: solanaMessage{
				AccountKeys: []solanaAccountKey{
					{Pubkey: "Sender11111111111111111111111111111111111"},
					{Pubkey: tokenAccount},
				},
				Instructions: []solanaInstruction{
					{
						Program: "spl-token",
						Parsed: solanaParsedInstruction{
							Type: "transferChecked",
							Info: map[string]any{
								"authority":   "Sender11111111111111111111111111111111111",
								"destination": tokenAccount,
								"mint":        usdtMint,
								"tokenAmount": map[string]any{"amount": "10003721"},
							},
						},
					},
				},
			},
		},
		Meta: solanaTransactionMeta{
			PostTokenBalances: []solanaTokenBalance{
				{
					AccountIndex: 1,
					Mint:         usdtMint,
					Owner:        receiveOwner,
					UITokenAmount: solanaTokenAmount{
						Amount: "10003721",
					},
				},
			},
		},
	}

	transfers, err := decodeSolanaTokenTransfers("5TxSignature", tx, receiveOwner, usdtMint, 6, 12380)
	require.NoError(t, err)
	require.Len(t, transfers, 1)
	assert.Equal(t, "5TxSignature", transfers[0].TxHash)
	assert.Equal(t, receiveOwner, transfers[0].ToAddress)
	assert.Equal(t, usdtMint, transfers[0].TokenContract)
	assert.Equal(t, "10003721", transfers[0].AmountBaseUnits)
	assert.EqualValues(t, 36, transfers[0].Confirmations)
}
