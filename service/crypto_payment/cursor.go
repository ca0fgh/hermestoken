package crypto_payment

import (
	"fmt"

	"github.com/ca0fgh/hermestoken/common"
)

// resumeFromBlock decides which block a scan resumes at, and refuses to trust a
// cursor that cannot have come from the chain it is now looking at.
//
// A cursor above the chain tip is not a cursor for this chain. It is what an
// endpoint repointed at a different network leaves behind: production's Solana RPC
// was an Ankr *devnet* endpoint, and devnet's slot height runs tens of millions
// ahead of mainnet's. Left alone, fromBlock stays above maxSafe on every tick and
// ScanOnce returns nil immediately — the scanner reports no errors, holds its lock,
// logs nothing, and scans nothing. A silent healthy-looking no-op is the worst
// failure this subsystem can have, so it cold-starts loudly instead.
func resumeFromBlock(network string, lastScannedBlock int64, currentBlock int64, coldStartBlock int64) int64 {
	if lastScannedBlock > currentBlock {
		common.SysLog(fmt.Sprintf(
			"crypto scanner cursor for %s is %d but the chain tip is %d: this cursor is from a different chain, cold-starting at %d",
			network, lastScannedBlock, currentBlock, coldStartBlock))
		return coldStartBlock
	}
	if lastScannedBlock <= 0 {
		return coldStartBlock
	}
	return lastScannedBlock + 1
}
