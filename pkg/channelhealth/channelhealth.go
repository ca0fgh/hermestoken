// Package channelhealth implements a lightweight, in-memory per-channel circuit
// breaker used by channel selection to route around upstreams that are currently
// failing (e.g. a provider returning 504/524 in a burst).
//
// Design goals:
//   - Zero false-failover risk: a channel is only "open" (skipped) after it has
//     actually produced `threshold` health failures within `window`.
//   - Self-healing: after `cooldown` the channel goes half-open (one trial allowed);
//     a single success clears it immediately.
//   - Decoupled & testable: holds no project config and imports nothing from the
//     app. Callers push config via Configure and decide whether to consult it.
//   - Selection always keeps a fallback: callers must never let the breaker skip
//     the last remaining candidate (handled in the selection layer), so a total
//     outage still attempts every channel.
package channelhealth

import (
	"sync"
	"time"
)

type config struct {
	threshold int
	window    time.Duration
	cooldown  time.Duration
}

type breakerState struct {
	failures  []int64 // unix-nano timestamps of recent failures within the window
	openUntil int64   // unix-nano; >0 means the breaker is open until this instant
}

var (
	mu     sync.Mutex
	cfg    config
	states = make(map[int]*breakerState)
	// nowFn is a seam for deterministic tests; production uses time.Now.
	nowFn = time.Now
)

// Configure sets the breaker thresholds. threshold <= 0 disables the breaker
// entirely (IsOpen always false, RecordFailure is a no-op so the state map does
// not grow while the feature is off). Safe to call repeatedly on config reload.
func Configure(threshold int, window, cooldown time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	cfg = config{threshold: threshold, window: window, cooldown: cooldown}
}

// RecordFailure registers one health failure for the channel. Once `threshold`
// failures accumulate within `window`, the breaker trips open for `cooldown`.
func RecordFailure(channelID int) {
	mu.Lock()
	defer mu.Unlock()
	if cfg.threshold <= 0 {
		return
	}
	now := nowFn().UnixNano()
	st := states[channelID]
	if st == nil {
		st = &breakerState{}
		states[channelID] = st
	}
	// Drop failures that have aged out of the window (in-place filter).
	cutoff := now - int64(cfg.window)
	kept := st.failures[:0]
	for _, t := range st.failures {
		if t >= cutoff {
			kept = append(kept, t)
		}
	}
	st.failures = append(kept, now)
	if len(st.failures) >= cfg.threshold {
		st.openUntil = now + int64(cfg.cooldown)
		st.failures = st.failures[:0]
	}
}

// RecordSuccess clears any failure history and closes the breaker for the channel.
func RecordSuccess(channelID int) {
	mu.Lock()
	defer mu.Unlock()
	st := states[channelID]
	if st == nil {
		return
	}
	st.failures = st.failures[:0]
	st.openUntil = 0
}

// IsOpen reports whether the channel is currently tripped open and should be
// skipped by selection. After the cooldown elapses the breaker is half-opened
// (returns false, allowing one trial) so a recovered channel can resume.
func IsOpen(channelID int) bool {
	mu.Lock()
	defer mu.Unlock()
	if cfg.threshold <= 0 {
		return false
	}
	st := states[channelID]
	if st == nil || st.openUntil == 0 {
		return false
	}
	if nowFn().UnixNano() >= st.openUntil {
		st.openUntil = 0 // half-open: allow a trial
		return false
	}
	return true
}
