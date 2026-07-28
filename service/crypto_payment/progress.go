package crypto_payment

import (
	"fmt"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
)

const (
	// A scan runs every 10 seconds. Three minutes of a cursor that will not move
	// while the chain keeps producing blocks is not a slow provider, it is a stuck
	// scanner.
	stalledTicksBeforeAlarm = 18
	// Then keep saying so. The Polygon scanner was dead for two months and nothing
	// ever said a word, which is the only reason it lasted two months.
	stalledAlarmInterval = 30 * time.Minute
	// One minute of a stuck cursor is long enough to stop trusting the transport's
	// accumulated state — whichever endpoint is current, whatever span was
	// negotiated — and start over from the operator's configuration. Alarming
	// without self-repair is what let a pinned endpoint hold BSC for five days
	// while the alarm dutifully fired every half hour.
	stalledTicksBeforeTransportReset = 6
)

type scannerProgress struct {
	cursor       int64
	stalledTicks int
	lastAlarm    time.Time
}

var (
	scannerProgressMutex sync.Mutex
	scannerProgressState = map[string]*scannerProgress{}
)

// reportScannerProgress watches the one number that says whether a scanner is doing
// its job, and complains when it stops moving.
//
// lastScanned is where the cursor ended up, maxSafe is where it should have been
// able to reach this tick. A cursor short of maxSafe that is also no further along
// than it was last tick means this network is not scanning — every deposit landing
// now is going unseen — and that is the state that has to be loud.
//
// The return value asks the caller to reset its transport — re-audition RPC
// endpoints from the operator's first choice and re-open span bidding. It turns
// true after a minute of stall and then once per further minute, so a scanner
// pinned to a poisoned endpoint or a bogus negotiated span heals itself instead
// of waiting for a restart.
func reportScannerProgress(network string, lastScanned int64, maxSafe int64, currentBlock int64) bool {
	scannerProgressMutex.Lock()
	state, ok := scannerProgressState[network]
	if !ok {
		state = &scannerProgress{cursor: -1}
		scannerProgressState[network] = state
	}
	advanced := lastScanned > state.cursor
	caughtUp := lastScanned >= maxSafe
	if advanced || caughtUp {
		state.cursor = lastScanned
		state.stalledTicks = 0
		scannerProgressMutex.Unlock()
		return false
	}
	state.stalledTicks++
	stalledTicks := state.stalledTicks
	shouldAlarm := stalledTicks >= stalledTicksBeforeAlarm && time.Since(state.lastAlarm) >= stalledAlarmInterval
	if shouldAlarm {
		state.lastAlarm = time.Now()
	}
	scannerProgressMutex.Unlock()

	shouldResetTransport := stalledTicks%stalledTicksBeforeTransportReset == 0
	if shouldResetTransport {
		common.SysLog(fmt.Sprintf(
			"crypto scanner %s is stuck at block %d, re-auditioning RPC endpoints from the configured primary",
			network, lastScanned))
	}
	if !shouldAlarm {
		return shouldResetTransport
	}
	message := fmt.Sprintf(
		"crypto scanner %s has not advanced past block %d for %s while the chain is at %d (%d blocks behind): incoming deposits on this network are not being seen",
		network, lastScanned, time.Duration(stalledTicks)*10*time.Second, currentBlock, currentBlock-lastScanned)
	common.SysLog(message)
	model.RecordLog(0, model.LogTypeSystem, message)
	return shouldResetTransport
}
