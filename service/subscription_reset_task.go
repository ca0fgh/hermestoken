package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	subscriptionResetTickInterval       = 1 * time.Minute
	subscriptionResetBatchSize          = 300
	subscriptionPendingOrderBatchSize   = 300
	subscriptionPendingOrderExpireAfter = 30 * time.Minute
	subscriptionCleanupInterval         = 30 * time.Minute
)

var (
	subscriptionResetOnce    sync.Once
	subscriptionResetRunning atomic.Bool
	subscriptionCleanupLast  atomic.Int64
)

func StartSubscriptionQuotaResetTask() {
	subscriptionResetOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("subscription quota reset task started: tick=%s", subscriptionResetTickInterval))
			ticker := time.NewTicker(subscriptionResetTickInterval)
			defer ticker.Stop()

			runSubscriptionQuotaResetOnce()
			for range ticker.C {
				runSubscriptionQuotaResetOnce()
			}
		})
	})
}

func runSubscriptionQuotaResetOnce() {
	if !subscriptionResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer subscriptionResetRunning.Store(false)

	ctx := context.Background()
	totalPendingExpired := 0
	totalReset := 0
	totalExpired := 0
	pendingOrderCutoff := time.Now().Add(-subscriptionPendingOrderExpireAfter).Unix()
	for {
		n, err := model.ExpirePendingSubscriptionOrdersCreatedBefore(pendingOrderCutoff, subscriptionPendingOrderBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription pending order expire task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalPendingExpired += n
		if n < subscriptionPendingOrderBatchSize {
			break
		}
	}
	for {
		n, err := model.ExpireDueSubscriptions(subscriptionResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription expire task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalExpired += n
		if n < subscriptionResetBatchSize {
			break
		}
	}
	for {
		n, err := model.ResetDueSubscriptions(subscriptionResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription quota reset task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalReset += n
		if n < subscriptionResetBatchSize {
			break
		}
	}
	lastCleanup := time.Unix(subscriptionCleanupLast.Load(), 0)
	if time.Since(lastCleanup) >= subscriptionCleanupInterval {
		if _, err := model.CleanupSubscriptionPreConsumeRecords(7 * 24 * 3600); err == nil {
			subscriptionCleanupLast.Store(time.Now().Unix())
		}
	}
	if common.DebugEnabled && (totalPendingExpired > 0 || totalReset > 0 || totalExpired > 0) {
		logger.LogDebug(
			ctx,
			"subscription maintenance: pending_expired_count=%d, reset_count=%d, expired_count=%d",
			totalPendingExpired,
			totalReset,
			totalExpired,
		)
	}
}
