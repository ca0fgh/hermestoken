package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	marketplaceSettlementSourceFixedOrderFill  = "fixed_order_fill"
	marketplaceSettlementSourceFixedOrderFinal = "fixed_order_final"
)

type MarketplaceFixedOrderRelayInput struct {
	BuyerUserID    int
	FixedOrderID   int
	Model          string
	EstimatedQuota int
	RequestID      string
}

type MarketplaceFixedOrderRelayPreparation struct {
	Order      *model.MarketplaceFixedOrder
	Credential *model.MarketplaceCredential
	APIKey     string
	Session    *MarketplaceFixedOrderBillingSession
}

type MarketplaceFixedOrderBindingSelectInput struct {
	BuyerUserID   int
	FixedOrderIDs []int
	Model         string
}

func ListBuyerMarketplaceFixedOrderTokenModels(input MarketplaceFixedOrderBindingSelectInput) ([]string, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if input.BuyerUserID <= 0 {
		return nil, errors.New("buyer user id is required")
	}
	fixedOrderIDs := normalizeMarketplaceFixedOrderIDs(input.FixedOrderIDs)
	if len(fixedOrderIDs) == 0 {
		return []string{}, nil
	}
	if err := expireDueBuyerMarketplaceFixedOrders(input.BuyerUserID, common.GetTimestamp()); err != nil {
		return nil, err
	}

	var rows []struct {
		OrderID int
		Models  string
	}
	if err := model.DB.Table("marketplace_fixed_orders").
		Select("marketplace_fixed_orders.id as order_id, marketplace_credentials.models as models").
		Joins("JOIN marketplace_credentials ON marketplace_credentials.id = marketplace_fixed_orders.credential_id").
		Joins("LEFT JOIN marketplace_credential_stats ON marketplace_credential_stats.credential_id = marketplace_credentials.id").
		Where("marketplace_fixed_orders.buyer_user_id = ? AND marketplace_fixed_orders.id IN ?", input.BuyerUserID, fixedOrderIDs).
		Where("marketplace_fixed_orders.seller_user_id <> ?", input.BuyerUserID).
		Where("marketplace_fixed_orders.status = ? AND marketplace_fixed_orders.remaining_quota > 0", model.MarketplaceFixedOrderStatusActive).
		Where("(marketplace_fixed_orders.expires_at = 0 OR marketplace_fixed_orders.expires_at > ?)", common.GetTimestamp()).
		Where("marketplace_credentials.service_status = ?", model.MarketplaceServiceStatusEnabled).
		Where("marketplace_credentials.health_status IN ?", []string{model.MarketplaceHealthStatusHealthy, model.MarketplaceHealthStatusDegraded}).
		Where("marketplace_credentials.probe_status IN ? AND marketplace_credentials.probe_score > 0 AND marketplace_credentials.probe_score_max > 0", []string{model.MarketplaceProbeStatusPassed, model.MarketplaceProbeStatusWarning}).
		Where("marketplace_credentials.risk_status <> ?", model.MarketplaceRiskStatusRiskPaused).
		Where("marketplace_credentials.capacity_status <> ?", model.MarketplaceCapacityStatusExhausted).
		Where("(marketplace_credentials.quota_mode <> ? OR marketplace_credentials.quota_limit <= 0 OR COALESCE(marketplace_credential_stats.quota_used, 0) < marketplace_credentials.quota_limit)", model.MarketplaceQuotaModeLimited).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	orderIndex := make(map[int]int, len(fixedOrderIDs))
	for index, fixedOrderID := range fixedOrderIDs {
		orderIndex[fixedOrderID] = index
	}
	modelsByOrderIndex := make([][]string, len(fixedOrderIDs))
	for _, row := range rows {
		index, ok := orderIndex[row.OrderID]
		if !ok {
			continue
		}
		modelsByOrderIndex[index] = appendMarketplaceModels(modelsByOrderIndex[index], row.Models)
	}

	models := make([]string, 0)
	for _, orderModels := range modelsByOrderIndex {
		models = appendMarketplaceModels(models, strings.Join(orderModels, ","))
	}
	return models, nil
}

func SelectBuyerMarketplaceFixedOrderForTokenBindings(input MarketplaceFixedOrderBindingSelectInput) (*model.MarketplaceFixedOrder, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if input.BuyerUserID <= 0 {
		return nil, errors.New("buyer user id is required")
	}
	input.Model = strings.TrimSpace(input.Model)
	if input.Model == "" {
		return nil, errors.New("model is required")
	}
	fixedOrderIDs := normalizeMarketplaceFixedOrderIDs(input.FixedOrderIDs)
	if len(fixedOrderIDs) == 0 {
		return nil, errors.New("bind a marketplace fixed order to the token")
	}
	if err := expireDueBuyerMarketplaceFixedOrders(input.BuyerUserID, common.GetTimestamp()); err != nil {
		return nil, err
	}

	var orders []model.MarketplaceFixedOrder
	if err := model.DB.
		Where("buyer_user_id = ? AND id IN ?", input.BuyerUserID, fixedOrderIDs).
		Find(&orders).Error; err != nil {
		return nil, err
	}
	ordersByID := make(map[int]model.MarketplaceFixedOrder, len(orders))
	for _, order := range orders {
		ordersByID[order.ID] = order
	}

	for _, fixedOrderID := range fixedOrderIDs {
		order, ok := ordersByID[fixedOrderID]
		if !ok || order.Status != model.MarketplaceFixedOrderStatusActive || order.RemainingQuota <= 0 {
			continue
		}
		if order.SellerUserID == input.BuyerUserID {
			continue
		}
		if isMarketplaceFixedOrderExpired(order, common.GetTimestamp()) {
			continue
		}
		var credential model.MarketplaceCredential
		if err := model.DB.Where("id = ?", order.CredentialID).First(&credential).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		var stats model.MarketplaceCredentialStats
		if err := model.DB.Where("credential_id = ?", credential.ID).First(&stats).Error; err == nil {
			if credential.ConcurrencyLimit > 0 && stats.CurrentConcurrency >= credential.ConcurrencyLimit {
				continue
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if !isMarketplaceCredentialFixedRouteAvailable(credential, stats) {
			continue
		}
		if !marketplaceCredentialSupportsModel(credential, input.Model) {
			continue
		}
		selected := order
		return &selected, nil
	}

	return nil, fmt.Errorf("no bound marketplace fixed order supports model %s", input.Model)
}

type MarketplaceFixedOrderBillingSession struct {
	orderID            int
	buyerUserID        int
	sellerUserID       int
	credentialID       int
	model              string
	requestID          string
	multiplierSnapshot float64
	// Fixed orders spend the fee rate captured when the order was purchased.
	platformFeeRateSnapshot float64
	startTime               time.Time

	preConsumedQuota int
	settled          bool
	refunded         bool
	capacityReleased bool
	mu               sync.Mutex
}

func PrepareMarketplaceFixedOrderRelay(input MarketplaceFixedOrderRelayInput) (*MarketplaceFixedOrderRelayPreparation, error) {
	if err := validateMarketplaceFixedOrderRelayInput(input); err != nil {
		return nil, err
	}

	var preparation *MarketplaceFixedOrderRelayPreparation
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.MarketplaceFixedOrder
		err := marketplaceForUpdate(tx).
			Where("id = ? AND buyer_user_id = ?", input.FixedOrderID, input.BuyerUserID).
			First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("marketplace fixed order not found")
		}
		if err != nil {
			return err
		}
		if order.SellerUserID == input.BuyerUserID {
			return errors.New("cannot use own marketplace fixed order")
		}
		if order.Status != model.MarketplaceFixedOrderStatusActive {
			return fmt.Errorf("marketplace fixed order is %s", order.Status)
		}
		if isMarketplaceFixedOrderExpired(order, common.GetTimestamp()) {
			if _, err := expireMarketplaceFixedOrderTx(tx, &order); err != nil {
				return err
			}
			return errors.New("marketplace fixed order expired")
		}

		var credential model.MarketplaceCredential
		err = marketplaceForUpdate(tx).Where("id = ?", order.CredentialID).First(&credential).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("marketplace credential not found")
		}
		if err != nil {
			return err
		}
		stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, credential.ID)
		if err != nil {
			return err
		}
		if !isMarketplaceCredentialFixedRouteAvailable(credential, *stats) {
			return errors.New("marketplace credential is not available for fixed order relay")
		}
		if !marketplaceCredentialSupportsModel(credential, input.Model) {
			return fmt.Errorf("marketplace credential does not support model %s", input.Model)
		}
		if credential.ConcurrencyLimit > 0 && stats.CurrentConcurrency >= credential.ConcurrencyLimit {
			return errors.New("marketplace credential concurrency is busy")
		}
		stats.CurrentConcurrency++
		if err := tx.Save(stats).Error; err != nil {
			return err
		}
		credential.CapacityStatus = marketplaceCredentialCapacityStatus(credential, *stats)
		if err := tx.Save(&credential).Error; err != nil {
			return err
		}

		secret, err := GetMarketplaceCredentialSecret()
		if err != nil {
			return err
		}
		apiKey, err := DecryptMarketplaceAPIKey(credential.EncryptedAPIKey, secret)
		if err != nil {
			return err
		}

		session := &MarketplaceFixedOrderBillingSession{
			orderID:                 order.ID,
			buyerUserID:             order.BuyerUserID,
			sellerUserID:            order.SellerUserID,
			credentialID:            order.CredentialID,
			model:                   input.Model,
			requestID:               input.RequestID,
			multiplierSnapshot:      order.MultiplierSnapshot,
			platformFeeRateSnapshot: order.PlatformFeeRateSnapshot,
			startTime:               time.Now(),
		}
		if input.EstimatedQuota > 0 {
			if err := session.reserveTx(tx, marketplaceBuyerChargeWithFee(int64(input.EstimatedQuota), session.platformFeeRateSnapshot)); err != nil {
				return err
			}
			if err := tx.First(&order, order.ID).Error; err != nil {
				return err
			}
		}

		preparation = &MarketplaceFixedOrderRelayPreparation{
			Order:      &order,
			Credential: &credential,
			APIKey:     apiKey,
			Session:    session,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return preparation, nil
}

func (s *MarketplaceFixedOrderBillingSession) Settle(actualQuota int) error {
	if actualQuota < 0 {
		return errors.New("actual quota must not be negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled {
		return nil
	}
	if s.refunded {
		return nil
	}

	var settlementEffect marketplaceSettlementReleaseSideEffect
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.MarketplaceFixedOrder
		if err := marketplaceForUpdate(tx).Where("id = ?", s.orderID).First(&order).Error; err != nil {
			return err
		}
		buyerCharge := marketplaceCapActualFixedOrderBuyerCharge(actualQuota, s.preConsumedQuota, order.RemainingQuota, s.platformFeeRateSnapshot)
		if delta := buyerCharge - s.preConsumedQuota; delta > 0 {
			if err := s.reserveDeltaTx(tx, delta); err != nil {
				return err
			}
		} else if delta < 0 {
			if err := s.releaseDeltaTx(tx, -delta); err != nil {
				return err
			}
		}
		s.preConsumedQuota = buyerCharge

		if err := marketplaceForUpdate(tx).Where("id = ?", s.orderID).First(&order).Error; err != nil {
			return err
		}
		stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, s.credentialID)
		if err != nil {
			return err
		}
		if stats.CurrentConcurrency > 0 {
			stats.CurrentConcurrency--
		}

		buyerChargeInt64 := int64(buyerCharge)
		platformFee := marketplacePlatformFeeFromBuyerCharge(buyerChargeInt64, s.platformFeeRateSnapshot)
		sellerIncome := buyerChargeInt64 - platformFee
		officialCost := marketplaceOfficialCostFromBuyerCharge(sellerIncome, s.multiplierSnapshot)
		latencyMS := time.Since(s.startTime).Milliseconds()
		if latencyMS < 0 {
			latencyMS = 0
		}

		fill := &model.MarketplaceFixedOrderFill{
			RequestID:          s.requestID,
			FixedOrderID:       s.orderID,
			BuyerUserID:        s.buyerUserID,
			SellerUserID:       s.sellerUserID,
			CredentialID:       s.credentialID,
			Model:              s.model,
			OfficialCost:       officialCost,
			MultiplierSnapshot: s.multiplierSnapshot,
			BuyerCharge:        buyerChargeInt64,
			Status:             model.MarketplaceFillStatusSucceeded,
			LatencyMS:          latencyMS,
		}
		if err := tx.Create(fill).Error; err != nil {
			return err
		}

		settlement := &model.MarketplaceSettlement{
			RequestID:               s.requestID,
			BuyerUserID:             s.buyerUserID,
			SellerUserID:            s.sellerUserID,
			CredentialID:            s.credentialID,
			SourceType:              marketplaceSettlementSourceFixedOrderFill,
			SourceID:                strconv.Itoa(fill.ID),
			BuyerCharge:             buyerChargeInt64,
			PlatformFee:             platformFee,
			PlatformFeeRateSnapshot: s.platformFeeRateSnapshot,
			SellerIncome:            sellerIncome,
			OfficialCost:            officialCost,
			MultiplierSnapshot:      s.multiplierSnapshot,
			AvailableAt:             common.GetTimestamp(),
		}
		created, err := createReleasedMarketplaceSettlementTx(tx, settlement)
		if err != nil {
			return err
		}
		settlementEffect = newMarketplaceSettlementReleaseSideEffect(settlement, created)

		stats.FixedOrderRequestCount++
		stats.TotalRequestCount++
		stats.TotalOfficialCost += officialCost
		stats.QuotaUsed += officialCost
		stats.SuccessCount++
		stats.AvgLatencyMS = marketplaceNextAverageLatency(stats.AvgLatencyMS, stats.SuccessCount, latencyMS)
		stats.LastSuccessAt = common.GetTimestamp()
		exhausted := order.RemainingQuota == 0 && order.Status == model.MarketplaceFixedOrderStatusActive
		if exhausted {
			order.Status = model.MarketplaceFixedOrderStatusExhausted
			if stats.ActiveFixedOrderCount > 0 {
				stats.ActiveFixedOrderCount--
			}
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
		}
		if err := tx.Save(stats).Error; err != nil {
			return err
		}

		var credential model.MarketplaceCredential
		if err := marketplaceForUpdate(tx).Where("id = ?", s.credentialID).First(&credential).Error; err != nil {
			return err
		}
		credential.CapacityStatus = marketplaceCredentialCapacityStatus(credential, *stats)
		if err := tx.Save(&credential).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	applyMarketplaceSettlementReleaseSideEffect(settlementEffect)
	s.settled = true
	s.capacityReleased = true
	return nil
}

func (s *MarketplaceFixedOrderBillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled || s.refunded || (s.preConsumedQuota <= 0 && s.capacityReleased) {
		return
	}
	quotaToRefund := s.preConsumedQuota
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if quotaToRefund > 0 {
			if err := s.releaseDeltaTx(tx, quotaToRefund); err != nil {
				return err
			}
		}
		if !s.capacityReleased {
			return s.releaseCapacityTx(tx)
		}
		return nil
	}); err != nil {
		logger.LogError(c, "error refunding marketplace fixed order quota: "+err.Error())
		return
	}
	s.preConsumedQuota = 0
	s.refunded = true
	s.capacityReleased = true
}

func (s *MarketplaceFixedOrderBillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.settled && !s.refunded && (s.preConsumedQuota > 0 || !s.capacityReleased)
}

func (s *MarketplaceFixedOrderBillingSession) GetPreConsumedQuota() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preConsumedQuota
}

func (s *MarketplaceFixedOrderBillingSession) Reserve(targetQuota int) error {
	if targetQuota < 0 {
		return errors.New("target quota must not be negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	targetCharge := s.BuyerChargeForQuota(targetQuota)
	if s.settled || s.refunded || targetCharge <= s.preConsumedQuota {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		return s.reserveTx(tx, targetCharge)
	})
}

func (s *MarketplaceFixedOrderBillingSession) BuyerChargeForQuota(quota int) int {
	if s == nil {
		return quota
	}
	return marketplaceBuyerChargeWithFee(int64(quota), s.platformFeeRateSnapshot)
}

func (s *MarketplaceFixedOrderBillingSession) reserveTx(tx *gorm.DB, targetQuota int) error {
	if targetQuota <= s.preConsumedQuota {
		return nil
	}
	return s.reserveDeltaTx(tx, targetQuota-s.preConsumedQuota)
}

func (s *MarketplaceFixedOrderBillingSession) reserveDeltaTx(tx *gorm.DB, delta int) error {
	if delta <= 0 {
		return nil
	}
	var order model.MarketplaceFixedOrder
	err := marketplaceForUpdate(tx).
		Where("id = ? AND buyer_user_id = ?", s.orderID, s.buyerUserID).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("marketplace fixed order not found")
	}
	if err != nil {
		return err
	}
	if order.Status != model.MarketplaceFixedOrderStatusActive {
		return fmt.Errorf("marketplace fixed order is %s", order.Status)
	}
	if int64(delta) > order.RemainingQuota {
		return errors.New("marketplace fixed order remaining quota is insufficient")
	}
	order.RemainingQuota -= int64(delta)
	order.SpentQuota += int64(delta)
	if err := tx.Save(&order).Error; err != nil {
		return err
	}
	s.preConsumedQuota += delta
	return nil
}

func (s *MarketplaceFixedOrderBillingSession) releaseDeltaTx(tx *gorm.DB, delta int) error {
	if delta <= 0 {
		return nil
	}
	var order model.MarketplaceFixedOrder
	err := marketplaceForUpdate(tx).
		Where("id = ? AND buyer_user_id = ?", s.orderID, s.buyerUserID).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("marketplace fixed order not found")
	}
	if err != nil {
		return err
	}
	order.RemainingQuota += int64(delta)
	if order.SpentQuota >= int64(delta) {
		order.SpentQuota -= int64(delta)
	} else {
		order.SpentQuota = 0
	}
	if err := tx.Save(&order).Error; err != nil {
		return err
	}
	s.preConsumedQuota -= delta
	if s.preConsumedQuota < 0 {
		s.preConsumedQuota = 0
	}
	return nil
}

func (s *MarketplaceFixedOrderBillingSession) releaseCapacityTx(tx *gorm.DB) error {
	stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, s.credentialID)
	if err != nil {
		return err
	}
	if stats.CurrentConcurrency > 0 {
		stats.CurrentConcurrency--
	}
	if err := tx.Save(stats).Error; err != nil {
		return err
	}
	var credential model.MarketplaceCredential
	if err := marketplaceForUpdate(tx).Where("id = ?", s.credentialID).First(&credential).Error; err != nil {
		return err
	}
	credential.CapacityStatus = marketplaceCredentialCapacityStatus(credential, *stats)
	return tx.Save(&credential).Error
}

func validateMarketplaceFixedOrderRelayInput(input MarketplaceFixedOrderRelayInput) error {
	if err := validateMarketplaceEnabled(); err != nil {
		return err
	}
	if input.BuyerUserID <= 0 {
		return errors.New("buyer user id is required")
	}
	if input.FixedOrderID <= 0 {
		return errors.New("marketplace fixed order id is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		return errors.New("model is required")
	}
	if input.EstimatedQuota < 0 {
		return errors.New("estimated quota must not be negative")
	}
	if strings.TrimSpace(input.RequestID) == "" {
		return errors.New("request id is required")
	}
	return nil
}

func isMarketplaceCredentialFixedRouteAvailable(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) bool {
	if credential.ServiceStatus != model.MarketplaceServiceStatusEnabled {
		return false
	}
	if credential.HealthStatus != model.MarketplaceHealthStatusHealthy && credential.HealthStatus != model.MarketplaceHealthStatusDegraded {
		return false
	}
	if !marketplaceCredentialHasRoutableProbeScore(credential) {
		return false
	}
	if marketplaceCredentialCapacityStatus(credential, stats) == model.MarketplaceCapacityStatusExhausted {
		return false
	}
	if credential.RiskStatus == model.MarketplaceRiskStatusRiskPaused {
		return false
	}
	return true
}

func marketplaceCredentialSupportsModel(credential model.MarketplaceCredential, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	for _, supported := range strings.Split(credential.Models, ",") {
		if strings.TrimSpace(supported) == modelName {
			return true
		}
	}
	return false
}

func isMarketplaceFixedOrderExpired(order model.MarketplaceFixedOrder, now int64) bool {
	return order.ExpiresAt > 0 && order.ExpiresAt <= now
}

func marketplaceFixedOrderFinalSettlementRequestID(orderID int) string {
	return fmt.Sprintf("marketplace_fixed_order_final:%d", orderID)
}

func expireMarketplaceFixedOrderTx(tx *gorm.DB, order *model.MarketplaceFixedOrder) (marketplaceSettlementReleaseSideEffect, error) {
	if order.Status != model.MarketplaceFixedOrderStatusActive {
		return marketplaceSettlementReleaseSideEffect{}, nil
	}
	order.ExpiredQuota += order.RemainingQuota
	order.RemainingQuota = 0
	order.Status = model.MarketplaceFixedOrderStatusExpired
	settlementEffect, err := settleMarketplaceFixedOrderFinalTx(tx, order, common.GetTimestamp())
	if err != nil {
		return marketplaceSettlementReleaseSideEffect{}, err
	}
	if err := tx.Save(order).Error; err != nil {
		return marketplaceSettlementReleaseSideEffect{}, err
	}
	stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, order.CredentialID)
	if err != nil {
		return marketplaceSettlementReleaseSideEffect{}, err
	}
	if stats.ActiveFixedOrderCount > 0 {
		stats.ActiveFixedOrderCount--
	}
	if err := tx.Save(stats).Error; err != nil {
		return marketplaceSettlementReleaseSideEffect{}, err
	}
	return settlementEffect, nil
}

func settleMarketplaceFixedOrderFinalTx(tx *gorm.DB, order *model.MarketplaceFixedOrder, now int64) (marketplaceSettlementReleaseSideEffect, error) {
	if order == nil || order.ID <= 0 || order.ExpiredQuota <= 0 {
		return marketplaceSettlementReleaseSideEffect{}, nil
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}

	buyerCharge := order.ExpiredQuota
	platformFeeRateSnapshot := order.PlatformFeeRateSnapshot
	platformFee := marketplacePlatformFeeFromBuyerCharge(buyerCharge, platformFeeRateSnapshot)
	settlement := &model.MarketplaceSettlement{
		RequestID:               marketplaceFixedOrderFinalSettlementRequestID(order.ID),
		BuyerUserID:             order.BuyerUserID,
		SellerUserID:            order.SellerUserID,
		CredentialID:            order.CredentialID,
		SourceType:              marketplaceSettlementSourceFixedOrderFinal,
		SourceID:                strconv.Itoa(order.ID),
		BuyerCharge:             buyerCharge,
		PlatformFee:             platformFee,
		PlatformFeeRateSnapshot: platformFeeRateSnapshot,
		SellerIncome:            buyerCharge - platformFee,
		OfficialCost:            0,
		MultiplierSnapshot:      order.MultiplierSnapshot,
		AvailableAt:             now,
	}
	created, err := createReleasedMarketplaceSettlementTx(tx, settlement)
	if err != nil {
		return marketplaceSettlementReleaseSideEffect{}, err
	}
	return newMarketplaceSettlementReleaseSideEffect(settlement, created), nil
}

func marketplaceOfficialCostFromBuyerCharge(buyerCharge int64, multiplier float64) int64 {
	if buyerCharge <= 0 {
		return 0
	}
	if multiplier <= 0 {
		return buyerCharge
	}
	officialCost := int64(math.Round(float64(buyerCharge) / multiplier))
	if officialCost <= 0 {
		return 1
	}
	return officialCost
}

func marketplaceFeeRateSnapshot() float64 {
	if math.IsNaN(setting.MarketplaceFeeRate) || math.IsInf(setting.MarketplaceFeeRate, 0) || setting.MarketplaceFeeRate < 0 {
		return 0
	}
	return setting.MarketplaceFeeRate
}

func marketplaceBuyerChargeWithFee(baseCharge int64, feeRate float64) int {
	total, err := marketplaceBuyerChargeQuotaWithFee(baseCharge, feeRate)
	if err != nil {
		return int(^uint(0) >> 1)
	}
	return int(total)
}

func marketplaceBuyerChargeQuotaWithFee(baseCharge int64, feeRate float64) (int64, error) {
	if baseCharge <= 0 {
		return 0, nil
	}
	fee := marketplacePlatformFeeFromBaseCharge(baseCharge, feeRate)
	total := baseCharge + fee
	maxInt := int64(^uint(0) >> 1)
	if total > maxInt {
		return 0, errors.New("marketplace buyer charge exceeds supported user quota range")
	}
	return total, nil
}

func marketplacePlatformFeeFromBaseCharge(baseCharge int64, feeRate float64) int64 {
	if baseCharge <= 0 || feeRate <= 0 || math.IsNaN(feeRate) || math.IsInf(feeRate, 0) {
		return 0
	}
	fee := int64(math.Round(float64(baseCharge) * feeRate))
	if fee < 0 {
		return 0
	}
	return fee
}

func marketplacePlatformFeeFromBuyerCharge(buyerCharge int64, feeRate float64) int64 {
	if buyerCharge <= 0 || feeRate <= 0 || math.IsNaN(feeRate) || math.IsInf(feeRate, 0) {
		return 0
	}
	fee := int64(math.Round(float64(buyerCharge) * feeRate / (1 + feeRate)))
	if fee < 0 {
		return 0
	}
	if fee > buyerCharge {
		return buyerCharge
	}
	return fee
}

func marketplaceCapActualFixedOrderBuyerCharge(actualQuota int, preConsumedQuota int, remainingQuota int64, feeRate float64) int {
	targetCharge := marketplaceBuyerChargeWithFee(int64(actualQuota), feeRate)
	if targetCharge <= preConsumedQuota {
		return targetCharge
	}
	maxCharge := int64(preConsumedQuota) + remainingQuota
	if maxCharge < 0 {
		maxCharge = 0
	}
	maxInt := int64(^uint(0) >> 1)
	if maxCharge > maxInt {
		maxCharge = maxInt
	}
	if int64(targetCharge) > maxCharge {
		return int(maxCharge)
	}
	return targetCharge
}

func marketplaceNextAverageLatency(currentAvg int64, successCountAfterIncrement int64, latestLatency int64) int64 {
	if successCountAfterIncrement <= 1 {
		return latestLatency
	}
	previousCount := successCountAfterIncrement - 1
	return (currentAvg*previousCount + latestLatency) / successCountAfterIncrement
}
