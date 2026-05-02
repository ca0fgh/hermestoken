package token_verifier

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	aaRefreshFetchTimeout = 30 * time.Second
)

var (
	aaRefreshOnce    sync.Once
	aaRefreshRunning atomic.Bool
)

// StartAABaselineAutoRefreshTask boots the periodic AA baseline sync.
// No-ops on non-master nodes or when AA_API_KEY is unset.
func StartAABaselineAutoRefreshTask() {
	aaRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !AABaselineEnabled() {
			logger.LogInfo(context.Background(), "AA baseline auto-refresh skipped: AA_API_KEY not configured")
			return
		}

		interval := AARefreshInterval()
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("AA baseline auto-refresh task started: interval=%s", interval))

			runAABaselineRefreshOnce()

			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				runAABaselineRefreshOnce()
			}
		})
	})
}

func runAABaselineRefreshOnce() {
	if !aaRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer aaRefreshRunning.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), aaRefreshFetchTimeout)
	defer cancel()

	snap, err := FetchAABaseline(ctx)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("AA baseline refresh failed: %v", err))
		return
	}
	if err := StoreAABaselineSnapshot(snap); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("AA baseline store failed: %v", err))
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf("AA baseline refreshed: models=%d", len(snap.Models)))
}
