package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/joho/godotenv"
)

func main() {
	batchID := flag.Int("batch-id", 0, "repair a specific referral settlement batch id")
	tradeNo := flag.String("trade-no", "", "repair all referral settlement batches for a specific subscription trade number")
	limit := flag.Int("limit", 100, "maximum number of auto-discovered batches to preview or repair")
	apply := flag.Bool("apply", false, "apply repairs instead of running in dry-run mode")
	flag.Parse()

	if err := initRepairResources(); err != nil {
		fmt.Fprintf(os.Stderr, "init resources failed: %v\n", err)
		os.Exit(1)
	}

	batchIDs, err := resolveRepairBatchIDs(*batchID, *tradeNo, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve batch ids failed: %v\n", err)
		os.Exit(1)
	}
	if len(batchIDs) == 0 {
		fmt.Println("no matching referral settlement batches found")
		return
	}

	modeLabel := "dry-run"
	if *apply {
		modeLabel = "apply"
	}
	fmt.Printf("subscription referral invitee-share repair (%s)\n", modeLabel)

	changedCount := 0
	for _, currentBatchID := range batchIDs {
		preview, err := model.PreviewSubscriptionReferralSettlementBatchRepair(currentBatchID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "batch=%d preview error: %v\n", currentBatchID, err)
			if *apply {
				os.Exit(1)
			}
			continue
		}
		if preview == nil {
			fmt.Printf("batch=%d preview skipped\n", currentBatchID)
			continue
		}

		fmt.Printf(
			"batch=%d trade_no=%s immediate=%d->%d invitee=%d->%d changed=%t\n",
			preview.BatchID,
			preview.SourceTradeNo,
			preview.ImmediateOldQuota,
			preview.ImmediateNewQuota,
			preview.InviteeOldQuota,
			preview.InviteeNewQuota,
			preview.Changed,
		)

		if !preview.Changed {
			continue
		}
		changedCount++

		if !*apply {
			continue
		}

		result, err := model.RepairSubscriptionReferralSettlementBatchByID(currentBatchID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "batch=%d apply error: %v\n", currentBatchID, err)
			os.Exit(1)
		}

		fmt.Printf(
			"applied batch=%d trade_no=%s immediate=%d->%d invitee=%d->%d\n",
			result.BatchID,
			result.SourceTradeNo,
			result.ImmediateOldQuota,
			result.ImmediateNewQuota,
			result.InviteeOldQuota,
			result.InviteeNewQuota,
		)
	}

	fmt.Printf("processed=%d changed=%d mode=%s\n", len(batchIDs), changedCount, modeLabel)
}

func initRepairResources() error {
	_ = godotenv.Load(".env")
	_ = godotenv.Load(".env.production")

	common.InitEnv()
	if os.Getenv("SQL_DSN") == "" {
		return fmt.Errorf("SQL_DSN is required; refusing to fall back to sqlite for repair")
	}
	logger.SetupLogger()

	if err := model.InitDB(); err != nil {
		return err
	}
	model.InitOptionMap()
	return nil
}

func resolveRepairBatchIDs(batchID int, tradeNo string, limit int) ([]int, error) {
	if batchID > 0 {
		return []int{batchID}, nil
	}

	query := model.DB.Model(&model.ReferralSettlementBatch{}).
		Where("referral_type = ?", model.ReferralTypeSubscription).
		Where("status = ?", model.SubscriptionReferralStatusCredited)

	if tradeNo != "" {
		query = query.Where("source_trade_no = ?", tradeNo)
	} else {
		inviteeRewardBatchIDs := model.DB.Model(&model.ReferralSettlementRecord{}).
			Select("DISTINCT batch_id").
			Where("referral_type = ?", model.ReferralTypeSubscription).
			Where("reward_component = ?", "invitee_reward").
			Where("status = ?", model.SubscriptionReferralStatusCredited).
			Where("reversed_quota = 0 AND debt_quota = 0")
		query = query.Where("id IN (?)", inviteeRewardBatchIDs)
		if limit > 0 {
			query = query.Limit(limit)
		}
	}

	ids := make([]int, 0)
	if err := query.Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
