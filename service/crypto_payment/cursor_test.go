package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResumeFromBlock(t *testing.T) {
	cases := []struct {
		name             string
		lastScannedBlock int64
		currentBlock     int64
		coldStartBlock   int64
		want             int64
	}{
		{"resumes just past the cursor", 500, 1000, 900, 501},
		{"cold-starts with no cursor", 0, 1000, 900, 900},
		{"a cursor at the tip is still valid", 1000, 1000, 900, 1001},
		// Production's Solana RPC was an Ankr *devnet* endpoint, and devnet's slot
		// height ran 43 million ahead of mainnet's. Repointing it at mainnet without
		// this guard leaves fromSlot permanently above maxSafe, so ScanOnce returns
		// nil on every tick: no errors, no logs, no scanning. Silent and healthy-
		// looking is the worst way for a deposit scanner to fail.
		{"a cursor from another chain is discarded", 475912141, 432598011, 432597979, 432597979},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, resumeFromBlock("solana", testCase.lastScannedBlock, testCase.currentBlock, testCase.coldStartBlock))
		})
	}
}
