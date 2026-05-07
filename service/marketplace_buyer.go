package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
	"github.com/ca0fgh/hermestoken/setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const marketplaceFixedOrderProbeRefundScoreDrop = 5

type MarketplaceOrderListInput struct {
	BuyerUserID         int
	VendorType          int
	Model               string
	QuotaMode           string
	TimeMode            string
	MinQuotaLimit       int64
	MaxQuotaLimit       int64
	MinTimeLimitSeconds int64
	MaxTimeLimitSeconds int64
	MinMultiplier       float64
	MaxMultiplier       float64
	MinConcurrencyLimit int
	MaxConcurrencyLimit int
}

type MarketplaceOrderListItem struct {
	ID                     int                       `json:"id"`
	SellerUserID           int                       `json:"seller_user_id"`
	VendorType             int                       `json:"vendor_type"`
	VendorNameSnapshot     string                    `json:"vendor_name_snapshot"`
	Models                 string                    `json:"models"`
	QuotaMode              string                    `json:"quota_mode"`
	QuotaLimit             int64                     `json:"quota_limit"`
	TimeMode               string                    `json:"time_mode"`
	TimeLimitSeconds       int64                     `json:"time_limit_seconds"`
	Multiplier             float64                   `json:"multiplier"`
	ConcurrencyLimit       int                       `json:"concurrency_limit"`
	ListingStatus          string                    `json:"listing_status"`
	ServiceStatus          string                    `json:"service_status"`
	HealthStatus           string                    `json:"health_status"`
	CapacityStatus         string                    `json:"capacity_status"`
	RouteStatus            string                    `json:"route_status"`
	RiskStatus             string                    `json:"risk_status"`
	ProbeStatus            string                    `json:"probe_status"`
	ProbeScore             int                       `json:"probe_score"`
	ProbeScoreMax          int                       `json:"probe_score_max"`
	ProbeGrade             string                    `json:"probe_grade"`
	ProbeCheckedAt         int64                     `json:"probe_checked_at"`
	CurrentConcurrency     int                       `json:"current_concurrency"`
	TotalRequestCount      int64                     `json:"total_request_count"`
	PoolRequestCount       int64                     `json:"pool_request_count"`
	FixedOrderRequestCount int64                     `json:"fixed_order_request_count"`
	QuotaUsed              int64                     `json:"quota_used"`
	FixedOrderSoldQuota    int64                     `json:"fixed_order_sold_quota"`
	ActiveFixedOrderCount  int64                     `json:"active_fixed_order_count"`
	SuccessCount           int64                     `json:"success_count"`
	UpstreamErrorCount     int64                     `json:"upstream_error_count"`
	TimeoutCount           int64                     `json:"timeout_count"`
	RateLimitCount         int64                     `json:"rate_limit_count"`
	PlatformErrorCount     int64                     `json:"platform_error_count"`
	AvgLatencyMS           int64                     `json:"avg_latency_ms"`
	LastSuccessAt          int64                     `json:"last_success_at"`
	LastFailedAt           int64                     `json:"last_failed_at"`
	LastFailedReason       string                    `json:"last_failed_reason"`
	PricePreview           []MarketplacePricePreview `json:"price_preview"`
}

type MarketplaceOrderFilterRanges struct {
	UnlimitedQuotaCount int64   `json:"unlimited_quota_count"`
	LimitedQuotaCount   int64   `json:"limited_quota_count"`
	MinQuotaLimit       int64   `json:"min_quota_limit"`
	MaxQuotaLimit       int64   `json:"max_quota_limit"`
	UnlimitedTimeCount  int64   `json:"unlimited_time_count"`
	LimitedTimeCount    int64   `json:"limited_time_count"`
	MinTimeLimitSeconds int64   `json:"min_time_limit_seconds"`
	MaxTimeLimitSeconds int64   `json:"max_time_limit_seconds"`
	MinMultiplier       float64 `json:"min_multiplier"`
	MaxMultiplier       float64 `json:"max_multiplier"`
	MinConcurrencyLimit int     `json:"min_concurrency_limit"`
	MaxConcurrencyLimit int     `json:"max_concurrency_limit"`
}

type MarketplaceFixedOrderCreateInput struct {
	BuyerUserID        int
	CredentialID       int
	PurchasedQuota     int64
	PurchasedAmountUSD float64
}

type MarketplaceFixedOrderTokenBindingResult struct {
	FixedOrderID int            `json:"fixed_order_id"`
	TokenIDs     []int          `json:"token_ids"`
	Tokens       []*model.Token `json:"tokens"`
}

type MarketplaceFixedOrderItem struct {
	model.MarketplaceFixedOrder
	ProbeStatus    string `json:"probe_status"`
	ProbeScore     int    `json:"probe_score"`
	ProbeScoreMax  int    `json:"probe_score_max"`
	ProbeGrade     string `json:"probe_grade"`
	ProbeCheckedAt int64  `json:"probe_checked_at"`
}

func ListMarketplaceOrders(input MarketplaceOrderListInput, startIdx int, pageSize int) ([]MarketplaceOrderListItem, int64, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, 0, err
	}
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var credentials []model.MarketplaceCredential
	if err := query.Order("id desc").Limit(pageSize).Offset(startIdx).Find(&credentials).Error; err != nil {
		return nil, 0, err
	}
	statsByCredentialID, err := marketplaceStatsByCredentialID(credentials)
	if err != nil {
		return nil, 0, err
	}

	items := make([]MarketplaceOrderListItem, 0, len(credentials))
	for _, credential := range credentials {
		stats := statsByCredentialID[credential.ID]
		items = append(items, newMarketplaceOrderListItem(credential, stats))
	}
	return items, total, nil
}

func GetMarketplaceOrderFilterRanges(input MarketplaceOrderListInput) (MarketplaceOrderFilterRanges, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}

	rangeInput := input
	rangeInput.QuotaMode = ""
	rangeInput.TimeMode = ""
	rangeInput.MinQuotaLimit = 0
	rangeInput.MaxQuotaLimit = 0
	rangeInput.MinTimeLimitSeconds = 0
	rangeInput.MaxTimeLimitSeconds = 0
	rangeInput.MinMultiplier = 0
	rangeInput.MaxMultiplier = 0
	rangeInput.MinConcurrencyLimit = 0
	rangeInput.MaxConcurrencyLimit = 0

	unlimitedQuotaCount, err := countMarketplaceOrderQuotaMode(rangeInput, model.MarketplaceQuotaModeUnlimited)
	if err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}
	limitedQuota, err := marketplaceOrderQuotaLimitRange(rangeInput)
	if err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}
	unlimitedTimeCount, err := countMarketplaceOrderTimeMode(rangeInput, model.MarketplaceTimeModeUnlimited)
	if err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}
	limitedTime, err := marketplaceOrderTimeLimitRange(rangeInput)
	if err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}
	multiplier, err := marketplaceOrderMultiplierRange(rangeInput)
	if err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}
	concurrency, err := marketplaceOrderConcurrencyLimitRange(rangeInput)
	if err != nil {
		return MarketplaceOrderFilterRanges{}, err
	}

	return MarketplaceOrderFilterRanges{
		UnlimitedQuotaCount: unlimitedQuotaCount,
		LimitedQuotaCount:   limitedQuota.Count,
		MinQuotaLimit:       nullInt64Value(limitedQuota.MinLimit),
		MaxQuotaLimit:       nullInt64Value(limitedQuota.MaxLimit),
		UnlimitedTimeCount:  unlimitedTimeCount,
		LimitedTimeCount:    limitedTime.Count,
		MinTimeLimitSeconds: nullInt64Value(limitedTime.MinLimit),
		MaxTimeLimitSeconds: nullInt64Value(limitedTime.MaxLimit),
		MinMultiplier:       nullFloat64Value(multiplier.MinLimit),
		MaxMultiplier:       nullFloat64Value(multiplier.MaxLimit),
		MinConcurrencyLimit: nullIntValue(concurrency.MinLimit),
		MaxConcurrencyLimit: nullIntValue(concurrency.MaxLimit),
	}, nil
}

func CreateMarketplaceFixedOrder(input MarketplaceFixedOrderCreateInput) (*model.MarketplaceFixedOrder, error) {
	input = normalizeMarketplaceFixedOrderCreateInput(input)
	platformFeeRateSnapshot := marketplaceFeeRateSnapshot()
	purchasedQuotaWithFee, err := marketplaceBuyerChargeQuotaWithFee(input.PurchasedQuota, platformFeeRateSnapshot)
	if err != nil {
		return nil, err
	}
	input.PurchasedQuota = purchasedQuotaWithFee
	if err := validateMarketplaceFixedOrderInput(input); err != nil {
		return nil, err
	}

	var createdOrder *model.MarketplaceFixedOrder
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var credential model.MarketplaceCredential
		if err := marketplaceForUpdate(tx).
			Where("id = ?", input.CredentialID).
			First(&credential).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("marketplace credential not found")
			}
			return err
		}
		if credential.SellerUserID == input.BuyerUserID {
			return errors.New("cannot buy own marketplace credential")
		}
		stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, credential.ID)
		if err != nil {
			return err
		}
		if !isMarketplaceCredentialPurchaseEligible(credential, *stats) {
			return errors.New("marketplace credential is not eligible for fixed order purchase")
		}

		var buyer model.User
		if err := marketplaceForUpdate(tx).Where("id = ?", input.BuyerUserID).First(&buyer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("buyer user not found")
			}
			return err
		}
		if buyer.Status != common.UserStatusEnabled {
			return errors.New("buyer user is disabled")
		}
		if int64(buyer.Quota) < input.PurchasedQuota {
			return errors.New("insufficient buyer quota")
		}
		quotaUpdate := tx.Model(&model.User{}).
			Where("id = ? AND quota >= ?", input.BuyerUserID, int(input.PurchasedQuota)).
			Update("quota", gorm.Expr("quota - ?", int(input.PurchasedQuota)))
		if quotaUpdate.Error != nil {
			return quotaUpdate.Error
		}
		if quotaUpdate.RowsAffected != 1 {
			return errors.New("insufficient buyer quota")
		}

		pricePreview := marketplacePricePreviewForCredential(credential)
		order := &model.MarketplaceFixedOrder{
			BuyerUserID:             input.BuyerUserID,
			SellerUserID:            credential.SellerUserID,
			CredentialID:            credential.ID,
			PurchasedQuota:          input.PurchasedQuota,
			RemainingQuota:          input.PurchasedQuota,
			PurchaseProbeStatus:     marketplaceProbeStatusForCredential(credential),
			PurchaseProbeScore:      credential.ProbeScore,
			PurchaseProbeScoreMax:   credential.ProbeScoreMax,
			PurchaseProbeGrade:      credential.ProbeGrade,
			PurchaseProbeCheckedAt:  credential.ProbeCheckedAt,
			MultiplierSnapshot:      credential.Multiplier,
			OfficialPriceSnapshot:   marshalMarketplaceOfficialPriceSnapshot(pricePreview),
			BuyerPriceSnapshot:      marshalMarketplaceBuyerPriceSnapshot(pricePreview),
			PlatformFeeRateSnapshot: platformFeeRateSnapshot,
			ExpiresAt:               marketplaceFixedOrderExpiresAt(credential),
			Status:                  model.MarketplaceFixedOrderStatusActive,
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}

		stats.FixedOrderSoldQuota += input.PurchasedQuota
		stats.ActiveFixedOrderCount++
		if err := tx.Save(stats).Error; err != nil {
			return err
		}

		createdOrder = order
		return nil
	})
	if err != nil {
		return nil, err
	}

	_, _ = model.GetUserQuota(input.BuyerUserID, true)
	model.RecordLog(input.BuyerUserID, model.LogTypeSystem, fmt.Sprintf("市场买断订单创建，订单ID %d，托管Key %d，扣除额度 %d", createdOrder.ID, createdOrder.CredentialID, createdOrder.PurchasedQuota))
	return createdOrder, nil
}

func ListBuyerMarketplaceFixedOrders(buyerUserID int, startIdx int, pageSize int) ([]model.MarketplaceFixedOrder, int64, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, 0, err
	}
	if err := expireDueBuyerMarketplaceFixedOrders(buyerUserID, common.GetTimestamp()); err != nil {
		return nil, 0, err
	}
	var total int64
	query := model.DB.Model(&model.MarketplaceFixedOrder{}).Where("buyer_user_id = ?", buyerUserID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orders []model.MarketplaceFixedOrder
	if err := query.Order("id desc").Limit(pageSize).Offset(startIdx).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func ListBuyerMarketplaceFixedOrderItems(buyerUserID int, startIdx int, pageSize int) ([]MarketplaceFixedOrderItem, int64, error) {
	orders, total, err := ListBuyerMarketplaceFixedOrders(buyerUserID, startIdx, pageSize)
	if err != nil {
		return nil, 0, err
	}
	credentialsByID, err := marketplaceCredentialsByIDForFixedOrders(orders)
	if err != nil {
		return nil, 0, err
	}
	items := make([]MarketplaceFixedOrderItem, 0, len(orders))
	for _, order := range orders {
		items = append(items, newMarketplaceFixedOrderItem(order, credentialsByID[order.CredentialID]))
	}
	return items, total, nil
}

func GetBuyerMarketplaceFixedOrder(buyerUserID int, fixedOrderID int) (*model.MarketplaceFixedOrder, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	var order model.MarketplaceFixedOrder
	err := model.DB.Where("id = ? AND buyer_user_id = ?", fixedOrderID, buyerUserID).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("marketplace fixed order not found")
	}
	if err == nil && isMarketplaceFixedOrderExpired(order, common.GetTimestamp()) {
		var settlementEffect marketplaceSettlementReleaseSideEffect
		if txErr := model.DB.Transaction(func(tx *gorm.DB) error {
			effect, err := expireMarketplaceFixedOrderTx(tx, &order)
			if err != nil {
				return err
			}
			settlementEffect = effect
			return nil
		}); txErr != nil {
			return nil, txErr
		}
		applyMarketplaceSettlementReleaseSideEffect(settlementEffect)
		err = model.DB.Where("id = ? AND buyer_user_id = ?", fixedOrderID, buyerUserID).First(&order).Error
	}
	return &order, err
}

func GetBuyerMarketplaceFixedOrderItem(buyerUserID int, fixedOrderID int) (*MarketplaceFixedOrderItem, error) {
	order, err := GetBuyerMarketplaceFixedOrder(buyerUserID, fixedOrderID)
	if err != nil {
		return nil, err
	}
	return marketplaceFixedOrderItemFromOrder(*order)
}

func marketplaceFixedOrderItemFromOrder(order model.MarketplaceFixedOrder) (*MarketplaceFixedOrderItem, error) {
	credentialsByID, err := marketplaceCredentialsByIDForFixedOrders([]model.MarketplaceFixedOrder{order})
	if err != nil {
		return nil, err
	}
	item := newMarketplaceFixedOrderItem(order, credentialsByID[order.CredentialID])
	return &item, nil
}

func ProbeBuyerMarketplaceFixedOrder(ctx context.Context, buyerUserID int, fixedOrderID int) (*MarketplaceFixedOrderItem, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if buyerUserID <= 0 {
		return nil, errors.New("buyer user id is required")
	}
	if fixedOrderID <= 0 {
		return nil, errors.New("marketplace fixed order id is required")
	}

	var order model.MarketplaceFixedOrder
	var credential model.MarketplaceCredential
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := marketplaceForUpdate(tx).
			Where("id = ? AND buyer_user_id = ?", fixedOrderID, buyerUserID).
			First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("marketplace fixed order not found")
			}
			return err
		}
		if order.Status != model.MarketplaceFixedOrderStatusActive {
			return fmt.Errorf("marketplace fixed order is %s", order.Status)
		}
		if order.RemainingQuota <= 0 {
			return errors.New("marketplace fixed order has no remaining quota to refund")
		}
		if order.PurchaseProbeScore <= 0 {
			return errors.New("marketplace fixed order has no purchase probe score")
		}
		if isMarketplaceFixedOrderExpired(order, common.GetTimestamp()) {
			return errors.New("marketplace fixed order expired")
		}
		if err := marketplaceForUpdate(tx).Where("id = ?", order.CredentialID).First(&credential).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("marketplace credential not found")
			}
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	completion, probeErr := runMarketplaceFixedOrderRefundProbe(ctx, credential)
	if probeErr != nil {
		_ = recordMarketplaceFixedOrderRefundProbeFailure(fixedOrderID, credential, probeErr)
		return nil, probeErr
	}
	if err := recordMarketplaceFixedOrderRefundProbeCompletion(fixedOrderID, completion); err != nil {
		return nil, err
	}
	return GetBuyerMarketplaceFixedOrderItem(buyerUserID, fixedOrderID)
}

func ReleaseBuyerMarketplaceFixedOrderAfterProbe(buyerUserID int, fixedOrderID int) (*MarketplaceFixedOrderItem, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if buyerUserID <= 0 {
		return nil, errors.New("buyer user id is required")
	}
	if fixedOrderID <= 0 {
		return nil, errors.New("marketplace fixed order id is required")
	}

	var order model.MarketplaceFixedOrder
	if err := model.DB.Where("id = ? AND buyer_user_id = ?", fixedOrderID, buyerUserID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("marketplace fixed order not found")
		}
		return nil, err
	}
	completion, err := marketplaceFixedOrderRefundProbeCompletionFromOrder(order)
	if err != nil {
		return nil, err
	}
	if err := validateMarketplaceFixedOrderReleaseProbe(order, completion); err != nil {
		return nil, err
	}
	refundedOrder, err := refundBuyerMarketplaceFixedOrderRemainingQuota(buyerUserID, fixedOrderID, completion)
	if err != nil {
		return nil, err
	}
	return marketplaceFixedOrderItemFromOrder(*refundedOrder)
}

func ValidateBuyerMarketplaceFixedOrderBindings(buyerUserID int, fixedOrderIDs []int) ([]int, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	normalized := normalizeMarketplaceFixedOrderIDs(fixedOrderIDs)
	if len(normalized) > 100 {
		return nil, errors.New("marketplace fixed order binding exceeds max 100")
	}
	for _, fixedOrderID := range normalized {
		order, err := GetBuyerMarketplaceFixedOrder(buyerUserID, fixedOrderID)
		if err != nil {
			return nil, err
		}
		if order.Status != model.MarketplaceFixedOrderStatusActive {
			return nil, fmt.Errorf("marketplace fixed order %d is %s", order.ID, order.Status)
		}
	}
	return normalized, nil
}

func SetBuyerMarketplaceFixedOrderTokenBindings(buyerUserID int, fixedOrderID int, tokenIDs []int) (*MarketplaceFixedOrderTokenBindingResult, error) {
	if fixedOrderID <= 0 {
		return nil, errors.New("marketplace fixed order id is required")
	}
	validatedFixedOrderIDs, err := ValidateBuyerMarketplaceFixedOrderBindings(buyerUserID, []int{fixedOrderID})
	if err != nil {
		return nil, err
	}
	fixedOrderID = validatedFixedOrderIDs[0]

	normalizedTokenIDs := normalizeMarketplaceFixedOrderIDs(tokenIDs)
	if len(normalizedTokenIDs) > 100 {
		return nil, errors.New("marketplace fixed order token binding exceeds max 100")
	}

	selectedTokenIDs := make(map[int]struct{}, len(normalizedTokenIDs))
	for _, tokenID := range normalizedTokenIDs {
		selectedTokenIDs[tokenID] = struct{}{}
	}

	var userTokens []model.Token
	if err := model.DB.Where("user_id = ?", buyerUserID).Find(&userTokens).Error; err != nil {
		return nil, err
	}

	tokensByID := make(map[int]*model.Token, len(userTokens))
	for i := range userTokens {
		token := &userTokens[i]
		tokensByID[token.Id] = token
	}
	for _, tokenID := range normalizedTokenIDs {
		if _, ok := tokensByID[tokenID]; !ok {
			return nil, fmt.Errorf("token %d not found", tokenID)
		}
	}

	updatedTokensByID := make(map[int]*model.Token, len(normalizedTokenIDs))
	for i := range userTokens {
		token := &userTokens[i]
		currentOrderIDs := token.MarketplaceFixedOrderIDList()
		_, shouldBind := selectedTokenIDs[token.Id]
		hasBinding := marketplaceIDInList(currentOrderIDs, fixedOrderID)

		nextOrderIDs := currentOrderIDs
		if shouldBind {
			nextOrderIDs = prependMarketplaceFixedOrderID(currentOrderIDs, fixedOrderID)
		} else if hasBinding {
			nextOrderIDs = removeMarketplaceFixedOrderID(currentOrderIDs, fixedOrderID)
		}

		if shouldBind || hasBinding {
			token.SetMarketplaceFixedOrderIDList(nextOrderIDs)
			if err := token.Update(); err != nil {
				return nil, err
			}
		}
		if shouldBind {
			updatedTokensByID[token.Id] = token
		}
	}

	boundTokens := make([]*model.Token, 0, len(normalizedTokenIDs))
	for _, tokenID := range normalizedTokenIDs {
		if token, ok := updatedTokensByID[tokenID]; ok {
			boundTokens = append(boundTokens, token)
		}
	}

	return &MarketplaceFixedOrderTokenBindingResult{
		FixedOrderID: fixedOrderID,
		TokenIDs:     normalizedTokenIDs,
		Tokens:       boundTokens,
	}, nil
}

func normalizeMarketplaceFixedOrderIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func marketplaceIDInList(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func prependMarketplaceFixedOrderID(ids []int, id int) []int {
	return normalizeMarketplaceFixedOrderIDs(append([]int{id}, ids...))
}

func removeMarketplaceFixedOrderID(ids []int, target int) []int {
	filtered := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != target {
			filtered = append(filtered, id)
		}
	}
	return normalizeMarketplaceFixedOrderIDs(filtered)
}

func removeBuyerMarketplaceFixedOrderBindingsTx(tx *gorm.DB, buyerUserID int, fixedOrderID int) error {
	if buyerUserID <= 0 || fixedOrderID <= 0 {
		return nil
	}
	var tokens []model.Token
	if err := tx.Where("user_id = ?", buyerUserID).Find(&tokens).Error; err != nil {
		return err
	}
	for i := range tokens {
		token := &tokens[i]
		currentOrderIDs := token.MarketplaceFixedOrderIDList()
		if !marketplaceIDInList(currentOrderIDs, fixedOrderID) {
			continue
		}
		nextOrderIDs := removeMarketplaceFixedOrderID(currentOrderIDs, fixedOrderID)
		token.SetMarketplaceFixedOrderIDList(nextOrderIDs)
		if err := tx.Model(token).Select("marketplace_fixed_order_id", "marketplace_fixed_order_ids").Updates(token).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyMarketplaceOrderListFilters(query *gorm.DB, input MarketplaceOrderListInput) (*gorm.DB, error) {
	query = query.
		Joins("LEFT JOIN marketplace_credential_stats ON marketplace_credential_stats.credential_id = marketplace_credentials.id").
		Where("marketplace_credentials.listing_status = ?", model.MarketplaceListingStatusListed).
		Where("marketplace_credentials.service_status = ?", model.MarketplaceServiceStatusEnabled).
		Where("marketplace_credentials.health_status IN ?", []string{model.MarketplaceHealthStatusUntested, model.MarketplaceHealthStatusHealthy, model.MarketplaceHealthStatusDegraded}).
		Where("marketplace_credentials.risk_status IN ?", []string{model.MarketplaceRiskStatusNormal, model.MarketplaceRiskStatusWatching}).
		Where(
			"(marketplace_credentials.quota_mode <> ? OR marketplace_credentials.quota_limit <= 0 OR COALESCE(marketplace_credential_stats.quota_used, 0) < marketplace_credentials.quota_limit)",
			model.MarketplaceQuotaModeLimited,
		)

	if input.VendorType > 0 {
		if !setting.IsMarketplaceVendorTypeEnabled(input.VendorType) {
			return nil, fmt.Errorf("marketplace vendor type %d is not enabled", input.VendorType)
		}
		query = query.Where("marketplace_credentials.vendor_type = ?", input.VendorType)
	}
	if strings.TrimSpace(input.Model) != "" {
		modelName := strings.TrimSpace(input.Model)
		escaped := escapeMarketplaceLikePattern(modelName)
		query = query.Where(
			"(marketplace_credentials.models = ? OR marketplace_credentials.models LIKE ? ESCAPE '\\' OR marketplace_credentials.models LIKE ? ESCAPE '\\' OR marketplace_credentials.models LIKE ? ESCAPE '\\')",
			modelName,
			escaped+",%",
			"%,"+escaped,
			"%,"+escaped+",%",
		)
	}
	if strings.TrimSpace(input.QuotaMode) != "" {
		switch input.QuotaMode {
		case model.MarketplaceQuotaModeUnlimited, model.MarketplaceQuotaModeLimited:
			query = query.Where("marketplace_credentials.quota_mode = ?", input.QuotaMode)
		default:
			return nil, fmt.Errorf("unsupported marketplace quota mode %s", input.QuotaMode)
		}
	}
	if strings.TrimSpace(input.TimeMode) != "" {
		switch input.TimeMode {
		case model.MarketplaceTimeModeUnlimited, model.MarketplaceTimeModeLimited:
			query = query.Where("marketplace_credentials.time_mode = ?", input.TimeMode)
		default:
			return nil, fmt.Errorf("unsupported marketplace time mode %s", input.TimeMode)
		}
	}
	if input.MinQuotaLimit > 0 || input.MaxQuotaLimit > 0 {
		query = query.Where("marketplace_credentials.quota_mode = ?", model.MarketplaceQuotaModeLimited)
		if input.MinQuotaLimit > 0 {
			query = query.Where("marketplace_credentials.quota_limit >= ?", input.MinQuotaLimit)
		}
		if input.MaxQuotaLimit > 0 {
			query = query.Where("marketplace_credentials.quota_limit <= ?", input.MaxQuotaLimit)
		}
	}
	if input.MinTimeLimitSeconds > 0 || input.MaxTimeLimitSeconds > 0 {
		query = query.Where("marketplace_credentials.time_mode = ?", model.MarketplaceTimeModeLimited)
		if input.MinTimeLimitSeconds > 0 {
			query = query.Where("marketplace_credentials.time_limit_seconds >= ?", input.MinTimeLimitSeconds)
		}
		if input.MaxTimeLimitSeconds > 0 {
			query = query.Where("marketplace_credentials.time_limit_seconds <= ?", input.MaxTimeLimitSeconds)
		}
	}
	if input.MinMultiplier > 0 {
		query = query.Where("marketplace_credentials.multiplier >= ?", input.MinMultiplier)
	}
	if input.MaxMultiplier > 0 {
		query = query.Where("marketplace_credentials.multiplier <= ?", input.MaxMultiplier)
	}
	if input.MinConcurrencyLimit > 0 {
		query = query.Where("marketplace_credentials.concurrency_limit >= ?", input.MinConcurrencyLimit)
	}
	if input.MaxConcurrencyLimit > 0 {
		query = query.Where("marketplace_credentials.concurrency_limit <= ?", input.MaxConcurrencyLimit)
	}
	return query, nil
}

type marketplaceOrderRangeAggregate struct {
	Count    int64
	MinLimit sql.NullInt64 `gorm:"column:min_limit"`
	MaxLimit sql.NullInt64 `gorm:"column:max_limit"`
}

type marketplaceOrderFloatRangeAggregate struct {
	Count    int64
	MinLimit sql.NullFloat64 `gorm:"column:min_limit"`
	MaxLimit sql.NullFloat64 `gorm:"column:max_limit"`
}

func countMarketplaceOrderQuotaMode(input MarketplaceOrderListInput, mode string) (int64, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := query.Where("marketplace_credentials.quota_mode = ?", mode).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func countMarketplaceOrderTimeMode(input MarketplaceOrderListInput, mode string) (int64, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := query.Where("marketplace_credentials.time_mode = ?", mode).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func marketplaceOrderQuotaLimitRange(input MarketplaceOrderListInput) (marketplaceOrderRangeAggregate, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return marketplaceOrderRangeAggregate{}, err
	}

	var aggregate marketplaceOrderRangeAggregate
	err = query.
		Where("marketplace_credentials.quota_mode = ?", model.MarketplaceQuotaModeLimited).
		Select("COUNT(*) as count, MIN(marketplace_credentials.quota_limit) as min_limit, MAX(marketplace_credentials.quota_limit) as max_limit").
		Scan(&aggregate).Error
	return aggregate, err
}

func marketplaceOrderTimeLimitRange(input MarketplaceOrderListInput) (marketplaceOrderRangeAggregate, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return marketplaceOrderRangeAggregate{}, err
	}

	var aggregate marketplaceOrderRangeAggregate
	err = query.
		Where("marketplace_credentials.time_mode = ?", model.MarketplaceTimeModeLimited).
		Select("COUNT(*) as count, MIN(marketplace_credentials.time_limit_seconds) as min_limit, MAX(marketplace_credentials.time_limit_seconds) as max_limit").
		Scan(&aggregate).Error
	return aggregate, err
}

func marketplaceOrderMultiplierRange(input MarketplaceOrderListInput) (marketplaceOrderFloatRangeAggregate, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return marketplaceOrderFloatRangeAggregate{}, err
	}

	var aggregate marketplaceOrderFloatRangeAggregate
	err = query.
		Where("marketplace_credentials.multiplier > 0").
		Select("COUNT(*) as count, MIN(marketplace_credentials.multiplier) as min_limit, MAX(marketplace_credentials.multiplier) as max_limit").
		Scan(&aggregate).Error
	return aggregate, err
}

func marketplaceOrderConcurrencyLimitRange(input MarketplaceOrderListInput) (marketplaceOrderRangeAggregate, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return marketplaceOrderRangeAggregate{}, err
	}

	var aggregate marketplaceOrderRangeAggregate
	err = query.
		Where("marketplace_credentials.concurrency_limit > 0").
		Select("COUNT(*) as count, MIN(marketplace_credentials.concurrency_limit) as min_limit, MAX(marketplace_credentials.concurrency_limit) as max_limit").
		Scan(&aggregate).Error
	return aggregate, err
}

func nullInt64Value(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func nullIntValue(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}

func nullFloat64Value(value sql.NullFloat64) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}

func marketplaceStatsByCredentialID(credentials []model.MarketplaceCredential) (map[int]model.MarketplaceCredentialStats, error) {
	statsByCredentialID := make(map[int]model.MarketplaceCredentialStats, len(credentials))
	if len(credentials) == 0 {
		return statsByCredentialID, nil
	}
	credentialIDs := make([]int, 0, len(credentials))
	for _, credential := range credentials {
		credentialIDs = append(credentialIDs, credential.ID)
	}
	var statsRows []model.MarketplaceCredentialStats
	if err := model.DB.Where("credential_id IN ?", credentialIDs).Find(&statsRows).Error; err != nil {
		return nil, err
	}
	for _, stats := range statsRows {
		statsByCredentialID[stats.CredentialID] = stats
	}
	return statsByCredentialID, nil
}

func newMarketplaceOrderListItem(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) MarketplaceOrderListItem {
	capacityStatus := marketplaceCredentialCapacityStatus(credential, stats)
	routeStatus := marketplaceCredentialRouteStatus(credential, stats)
	return MarketplaceOrderListItem{
		ID:                     credential.ID,
		SellerUserID:           credential.SellerUserID,
		VendorType:             credential.VendorType,
		VendorNameSnapshot:     credential.VendorNameSnapshot,
		Models:                 credential.Models,
		QuotaMode:              credential.QuotaMode,
		QuotaLimit:             credential.QuotaLimit,
		TimeMode:               credential.TimeMode,
		TimeLimitSeconds:       credential.TimeLimitSeconds,
		Multiplier:             credential.Multiplier,
		ConcurrencyLimit:       credential.ConcurrencyLimit,
		ListingStatus:          credential.ListingStatus,
		ServiceStatus:          credential.ServiceStatus,
		HealthStatus:           credential.HealthStatus,
		CapacityStatus:         capacityStatus,
		RouteStatus:            routeStatus,
		RiskStatus:             credential.RiskStatus,
		ProbeStatus:            marketplaceProbeStatusForCredential(credential),
		ProbeScore:             credential.ProbeScore,
		ProbeScoreMax:          credential.ProbeScoreMax,
		ProbeGrade:             credential.ProbeGrade,
		ProbeCheckedAt:         credential.ProbeCheckedAt,
		CurrentConcurrency:     stats.CurrentConcurrency,
		TotalRequestCount:      stats.TotalRequestCount,
		PoolRequestCount:       stats.PoolRequestCount,
		FixedOrderRequestCount: stats.FixedOrderRequestCount,
		QuotaUsed:              stats.QuotaUsed,
		FixedOrderSoldQuota:    stats.FixedOrderSoldQuota,
		ActiveFixedOrderCount:  stats.ActiveFixedOrderCount,
		SuccessCount:           stats.SuccessCount,
		UpstreamErrorCount:     stats.UpstreamErrorCount,
		TimeoutCount:           stats.TimeoutCount,
		RateLimitCount:         stats.RateLimitCount,
		PlatformErrorCount:     stats.PlatformErrorCount,
		AvgLatencyMS:           stats.AvgLatencyMS,
		LastSuccessAt:          stats.LastSuccessAt,
		LastFailedAt:           stats.LastFailedAt,
		LastFailedReason:       stats.LastFailedReason,
		PricePreview:           marketplacePricePreviewForCredential(credential),
	}
}

func marketplaceCredentialsByIDForFixedOrders(orders []model.MarketplaceFixedOrder) (map[int]model.MarketplaceCredential, error) {
	credentialsByID := make(map[int]model.MarketplaceCredential)
	if len(orders) == 0 {
		return credentialsByID, nil
	}
	credentialIDs := make([]int, 0, len(orders))
	seen := make(map[int]struct{}, len(orders))
	for _, order := range orders {
		if order.CredentialID <= 0 {
			continue
		}
		if _, ok := seen[order.CredentialID]; ok {
			continue
		}
		seen[order.CredentialID] = struct{}{}
		credentialIDs = append(credentialIDs, order.CredentialID)
	}
	if len(credentialIDs) == 0 {
		return credentialsByID, nil
	}
	var credentials []model.MarketplaceCredential
	if err := model.DB.Where("id IN ?", credentialIDs).Find(&credentials).Error; err != nil {
		return nil, err
	}
	for _, credential := range credentials {
		credentialsByID[credential.ID] = credential
	}
	return credentialsByID, nil
}

func newMarketplaceFixedOrderItem(order model.MarketplaceFixedOrder, credential model.MarketplaceCredential) MarketplaceFixedOrderItem {
	probeStatus := strings.TrimSpace(order.RefundProbeStatus)
	probeScore := order.RefundProbeScore
	probeScoreMax := order.RefundProbeScoreMax
	probeGrade := order.RefundProbeGrade
	probeCheckedAt := order.RefundProbeCheckedAt
	if probeStatus == "" {
		probeStatus = strings.TrimSpace(order.PurchaseProbeStatus)
		probeScore = order.PurchaseProbeScore
		probeScoreMax = order.PurchaseProbeScoreMax
		probeGrade = order.PurchaseProbeGrade
		probeCheckedAt = order.PurchaseProbeCheckedAt
	}
	if probeStatus == "" {
		probeStatus = marketplaceProbeStatusForCredential(credential)
		probeScore = credential.ProbeScore
		probeScoreMax = credential.ProbeScoreMax
		probeGrade = credential.ProbeGrade
		probeCheckedAt = credential.ProbeCheckedAt
	}
	return MarketplaceFixedOrderItem{
		MarketplaceFixedOrder: order,
		ProbeStatus:           probeStatus,
		ProbeScore:            probeScore,
		ProbeScoreMax:         probeScoreMax,
		ProbeGrade:            probeGrade,
		ProbeCheckedAt:        probeCheckedAt,
	}
}

func marketplaceProbeStatusForCredential(credential model.MarketplaceCredential) string {
	if strings.TrimSpace(credential.ProbeStatus) == "" {
		return model.MarketplaceProbeStatusUnscored
	}
	return credential.ProbeStatus
}

func runMarketplaceFixedOrderRefundProbe(ctx context.Context, credential model.MarketplaceCredential) (marketplaceProbeCompletion, error) {
	secret, err := GetMarketplaceCredentialSecret()
	if err != nil {
		return marketplaceProbeCompletion{}, err
	}
	apiKey, err := DecryptMarketplaceAPIKey(credential.EncryptedAPIKey, secret)
	if err != nil {
		return marketplaceProbeCompletion{}, err
	}
	provider := marketplaceProbeProviderForCredential(credential)
	profile := marketplaceProbeProfileForProvider(provider)
	clientProfile := marketplaceProbeClientProfileForProvider(provider)
	modelName, err := marketplaceProbeModelForCredential(credential, provider)
	if err != nil {
		return marketplaceProbeCompletion{}, marketplaceCredentialProbeSanitizedError(err, apiKey, credential.BaseURL)
	}
	baseURL, err := marketplaceProbeBaseURLForCredential(credential, apiKey)
	if err != nil {
		return marketplaceProbeCompletion{}, marketplaceCredentialProbeSanitizedError(err, apiKey, credential.BaseURL)
	}
	probeCtx, cancel := marketplaceCredentialProbeContext(ctx, profile)
	defer cancel()
	result, err := runMarketplaceCredentialDirectProbe(probeCtx, tokenverifier.DirectProbeRequest{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Provider:      provider,
		Model:         modelName,
		ProbeProfile:  profile,
		ClientProfile: clientProfile,
	})
	if err != nil {
		return marketplaceProbeCompletion{}, marketplaceCredentialProbeSanitizedError(err, apiKey, baseURL)
	}
	if result == nil {
		return marketplaceProbeCompletion{}, errors.New("marketplace probe returned empty result")
	}
	return marketplaceProbeCompletion{
		Status:         marketplaceProbeStatusForReport(result.Report),
		Score:          marketplaceProbeReportScore(result.Report),
		ScoreMax:       marketplaceProbeReportScoreMax(result.Report),
		Grade:          marketplaceProbeReportGrade(result.Report),
		Provider:       provider,
		Profile:        profile,
		Model:          modelName,
		ClientProfile:  clientProfile,
		ScoringVersion: result.Report.ScoringVersion,
	}, nil
}

func recordMarketplaceFixedOrderRefundProbeCompletion(fixedOrderID int, completion marketplaceProbeCompletion) error {
	return model.DB.Model(&model.MarketplaceFixedOrder{}).
		Where("id = ? AND status = ?", fixedOrderID, model.MarketplaceFixedOrderStatusActive).
		Updates(marketplaceFixedOrderRefundProbeCompletionFields(completion, common.GetTimestamp())).Error
}

func recordMarketplaceFixedOrderRefundProbeFailure(fixedOrderID int, credential model.MarketplaceCredential, probeErr error) error {
	message := ""
	if probeErr != nil {
		message = probeErr.Error()
	}
	return model.DB.Model(&model.MarketplaceFixedOrder{}).
		Where("id = ? AND status = ?", fixedOrderID, model.MarketplaceFixedOrderStatusActive).
		Updates(map[string]any{
			"refund_probe_status":     model.MarketplaceProbeStatusFailed,
			"refund_probe_checked_at": common.GetTimestamp(),
			"refund_probe_error":      sanitizeMarketplaceProbeMessage(message, "", credential.BaseURL),
		}).Error
}

func marketplaceFixedOrderRefundProbeCompletionFields(completion marketplaceProbeCompletion, now int64) map[string]any {
	return map[string]any{
		"refund_probe_status":     completion.Status,
		"refund_probe_score":      completion.Score,
		"refund_probe_score_max":  completion.ScoreMax,
		"refund_probe_grade":      completion.Grade,
		"refund_probe_checked_at": now,
		"refund_probe_error":      "",
	}
}

func marketplaceFixedOrderRefundProbeCompletionFromOrder(order model.MarketplaceFixedOrder) (marketplaceProbeCompletion, error) {
	if strings.TrimSpace(order.RefundProbeStatus) == "" || order.RefundProbeCheckedAt <= 0 {
		return marketplaceProbeCompletion{}, errors.New("please detect the marketplace fixed order first")
	}
	return marketplaceProbeCompletion{
		Status:   order.RefundProbeStatus,
		Score:    order.RefundProbeScore,
		ScoreMax: order.RefundProbeScoreMax,
		Grade:    order.RefundProbeGrade,
	}, nil
}

func validateMarketplaceFixedOrderReleaseProbe(order model.MarketplaceFixedOrder, completion marketplaceProbeCompletion) error {
	if order.Status != model.MarketplaceFixedOrderStatusActive {
		return fmt.Errorf("marketplace fixed order is %s", order.Status)
	}
	if order.RemainingQuota <= 0 {
		return errors.New("marketplace fixed order has no remaining quota to refund")
	}
	if order.PurchaseProbeScore <= 0 {
		return errors.New("marketplace fixed order has no purchase probe score")
	}
	if isMarketplaceFixedOrderExpired(order, common.GetTimestamp()) {
		return errors.New("marketplace fixed order expired")
	}
	if completion.Score > order.PurchaseProbeScore-marketplaceFixedOrderProbeRefundScoreDrop {
		return fmt.Errorf("marketplace fixed order probe score drop is less than %d", marketplaceFixedOrderProbeRefundScoreDrop)
	}
	return nil
}

func refundBuyerMarketplaceFixedOrderRemainingQuota(buyerUserID int, fixedOrderID int, completion marketplaceProbeCompletion) (*model.MarketplaceFixedOrder, error) {
	var refundedOrder model.MarketplaceFixedOrder
	var refundedQuota int64
	now := common.GetTimestamp()
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.MarketplaceFixedOrder
		if err := marketplaceForUpdate(tx).
			Where("id = ? AND buyer_user_id = ?", fixedOrderID, buyerUserID).
			First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("marketplace fixed order not found")
			}
			return err
		}
		if order.Status != model.MarketplaceFixedOrderStatusActive {
			return fmt.Errorf("marketplace fixed order is %s", order.Status)
		}
		if order.RemainingQuota <= 0 {
			return errors.New("marketplace fixed order has no remaining quota to refund")
		}
		if order.PurchaseProbeScore <= 0 {
			return errors.New("marketplace fixed order has no purchase probe score")
		}
		if isMarketplaceFixedOrderExpired(order, now) {
			return errors.New("marketplace fixed order expired")
		}
		storedCompletion, err := marketplaceFixedOrderRefundProbeCompletionFromOrder(order)
		if err != nil {
			return err
		}
		if storedCompletion.Score != completion.Score ||
			storedCompletion.ScoreMax != completion.ScoreMax ||
			storedCompletion.Status != completion.Status ||
			storedCompletion.Grade != completion.Grade {
			return errors.New("marketplace fixed order probe result changed, please detect again")
		}
		if err := validateMarketplaceFixedOrderReleaseProbe(order, completion); err != nil {
			return err
		}
		refundedQuota = order.RemainingQuota
		order.RefundProbeStatus = completion.Status
		order.RefundProbeScore = completion.Score
		order.RefundProbeScoreMax = completion.ScoreMax
		order.RefundProbeGrade = completion.Grade
		order.RefundProbeCheckedAt = now
		order.RefundProbeError = ""
		order.RefundedQuota += refundedQuota
		order.RemainingQuota = 0
		order.Status = model.MarketplaceFixedOrderStatusRefunded
		order.RefundedAt = now
		fields := marketplaceFixedOrderRefundProbeCompletionFields(completion, now)
		if err := tx.Model(&model.MarketplaceFixedOrder{}).
			Where("id = ?", order.ID).
			Updates(fields).Error; err != nil {
			return err
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, order.CredentialID)
		if err != nil {
			return err
		}
		if stats.ActiveFixedOrderCount > 0 {
			stats.ActiveFixedOrderCount--
		}
		if err := tx.Save(stats).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).
			Where("id = ?", buyerUserID).
			Update("quota", gorm.Expr("quota + ?", int(refundedQuota))).Error; err != nil {
			return err
		}
		if err := removeBuyerMarketplaceFixedOrderBindingsTx(tx, buyerUserID, order.ID); err != nil {
			return err
		}
		if err := tx.First(&refundedOrder, order.ID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_, _ = model.GetUserQuota(buyerUserID, true)
	model.RecordLog(buyerUserID, model.LogTypeRefund, fmt.Sprintf("市场买断订单解除退款，订单ID %d，托管Key %d，退款额度 %d，托管检测分 %d，当前检测分 %d", refundedOrder.ID, refundedOrder.CredentialID, refundedQuota, refundedOrder.PurchaseProbeScore, completion.Score))
	return &refundedOrder, nil
}

func validateMarketplaceFixedOrderInput(input MarketplaceFixedOrderCreateInput) error {
	if err := validateMarketplaceEnabled(); err != nil {
		return err
	}
	if input.BuyerUserID <= 0 {
		return errors.New("buyer user id is required")
	}
	if input.CredentialID <= 0 {
		return errors.New("marketplace credential id is required")
	}
	if input.PurchasedQuota <= 0 {
		return errors.New("purchased quota must be positive")
	}
	maxInt := int64(^uint(0) >> 1)
	if input.PurchasedQuota > maxInt {
		return errors.New("purchased quota exceeds supported user quota range")
	}
	if setting.MarketplaceMinFixedOrderQuota > 0 && input.PurchasedQuota < int64(setting.MarketplaceMinFixedOrderQuota) {
		return fmt.Errorf("purchased quota must be at least %d", setting.MarketplaceMinFixedOrderQuota)
	}
	if setting.MarketplaceMaxFixedOrderQuota > 0 && input.PurchasedQuota > int64(setting.MarketplaceMaxFixedOrderQuota) {
		return fmt.Errorf("purchased quota must be at most %d", setting.MarketplaceMaxFixedOrderQuota)
	}
	return nil
}

func normalizeMarketplaceFixedOrderCreateInput(input MarketplaceFixedOrderCreateInput) MarketplaceFixedOrderCreateInput {
	if input.PurchasedQuota <= 0 && input.PurchasedAmountUSD > 0 {
		input.PurchasedQuota = int64(math.Round(input.PurchasedAmountUSD * common.QuotaPerUnit))
	}
	return input
}

func isMarketplaceCredentialPurchaseEligible(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) bool {
	if credential.ListingStatus != model.MarketplaceListingStatusListed {
		return false
	}
	if credential.ServiceStatus != model.MarketplaceServiceStatusEnabled {
		return false
	}
	switch credential.HealthStatus {
	case model.MarketplaceHealthStatusUntested, model.MarketplaceHealthStatusHealthy, model.MarketplaceHealthStatusDegraded:
	default:
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

func getOrCreateMarketplaceCredentialStatsForUpdate(tx *gorm.DB, credentialID int) (*model.MarketplaceCredentialStats, error) {
	var stats model.MarketplaceCredentialStats
	err := marketplaceForUpdate(tx).Where("credential_id = ?", credentialID).First(&stats).Error
	if err == nil {
		return &stats, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	stats = model.MarketplaceCredentialStats{CredentialID: credentialID}
	if err := tx.Create(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

func marketplaceFixedOrderExpiresAt(credential model.MarketplaceCredential) int64 {
	if credential.TimeMode == model.MarketplaceTimeModeLimited {
		return common.GetTimestamp() + credential.TimeLimitSeconds
	}
	if credential.TimeMode == "" && setting.MarketplaceFixedOrderDefaultExpirySeconds > 0 {
		return common.GetTimestamp() + int64(setting.MarketplaceFixedOrderDefaultExpirySeconds)
	}
	return 0
}

func expireDueBuyerMarketplaceFixedOrders(buyerUserID int, now int64) error {
	if buyerUserID <= 0 {
		return nil
	}
	var orders []model.MarketplaceFixedOrder
	if err := model.DB.
		Where("buyer_user_id = ? AND status = ? AND expires_at > 0 AND expires_at <= ?", buyerUserID, model.MarketplaceFixedOrderStatusActive, now).
		Find(&orders).Error; err != nil {
		return err
	}
	for i := range orders {
		order := orders[i]
		var settlementEffect marketplaceSettlementReleaseSideEffect
		if err := model.DB.Transaction(func(tx *gorm.DB) error {
			effect, err := expireMarketplaceFixedOrderTx(tx, &order)
			if err != nil {
				return err
			}
			settlementEffect = effect
			return nil
		}); err != nil {
			return err
		}
		applyMarketplaceSettlementReleaseSideEffect(settlementEffect)
	}
	return nil
}

func marketplaceForUpdate(tx *gorm.DB) *gorm.DB {
	if common.UsingMySQL || common.UsingPostgreSQL {
		return tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return tx
}

func escapeMarketplaceLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
