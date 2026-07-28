package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A scanner whose cursor stops moving gets its transport reset once per minute of
// stall — pinned endpoint, poisoned span, whatever it was — instead of alarming
// for five days while nothing tries anything different.
func TestReportScannerProgressAsksForATransportResetOncePerMinuteOfStall(t *testing.T) {
	network := "progress_test_stall"

	require.False(t, reportScannerProgress(network, 100, 200, 210), "first sighting of a cursor is not a stall")

	resetTicks := []int{}
	for tick := 1; tick <= 2*stalledTicksBeforeTransportReset; tick++ {
		if reportScannerProgress(network, 100, 200, 210) {
			resetTicks = append(resetTicks, tick)
		}
	}
	assert.Equal(t, []int{stalledTicksBeforeTransportReset, 2 * stalledTicksBeforeTransportReset}, resetTicks)

	// The moment the cursor moves, the stall is over and the counter starts fresh.
	require.False(t, reportScannerProgress(network, 150, 200, 210))
	require.False(t, reportScannerProgress(network, 150, 200, 210), "one stalled tick after progress must not reset")
}

// A caught-up scanner sitting at maxSafe is idle, not stuck.
func TestReportScannerProgressDoesNotResetACaughtUpScanner(t *testing.T) {
	network := "progress_test_caught_up"

	for tick := 0; tick <= 2*stalledTicksBeforeTransportReset; tick++ {
		assert.False(t, reportScannerProgress(network, 200, 200, 210))
	}
}
