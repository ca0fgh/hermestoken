package service

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/model"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const marketplaceSettlementSourcePoolFill = "pool_fill"

type MarketplacePoolRelayInput struct {
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
	RequestID           string
}

type MarketplacePoolRelayPreparation struct {
	Credential *model.MarketplaceCredential
	APIKey     string
	Session    *MarketplacePoolRelaySession
}

type MarketplacePoolRelaySession struct {
	buyerUserID        int
	sellerUserID       int
	credentialID       int
	model              string
	requestID          string
	multiplierSnapshot float64
	// Buyer calls use the global fee rate captured when the call is prepared.
	platformFeeRateSnapshot float64
	startTime               time.Time

	settled          bool
	capacityReleased bool
	mu               sync.Mutex
}

type MarketplacePoolBillingSession struct {
	funding relaycommon.BillingSettler
	pool    *MarketplacePoolRelaySession
}

func PrepareMarketplacePoolRelay(input MarketplacePoolRelayInput) (*MarketplacePoolRelayPreparation, error) {
	if err := validateMarketplacePoolRelayInput(input); err != nil {
		return nil, err
	}

	var preparation *MarketplacePoolRelayPreparation
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		query, err := applyMarketplaceOrderListFilters(tx.Model(&model.MarketplaceCredential{}), MarketplaceOrderListInput{
			BuyerUserID:         input.BuyerUserID,
			VendorType:          input.VendorType,
			Model:               input.Model,
			QuotaMode:           input.QuotaMode,
			TimeMode:            input.TimeMode,
			MinQuotaLimit:       input.MinQuotaLimit,
			MaxQuotaLimit:       input.MaxQuotaLimit,
			MinTimeLimitSeconds: input.MinTimeLimitSeconds,
			MaxTimeLimitSeconds: input.MaxTimeLimitSeconds,
			MinMultiplier:       input.MinMultiplier,
			MaxMultiplier:       input.MaxMultiplier,
			MinConcurrencyLimit: input.MinConcurrencyLimit,
			MaxConcurrencyLimit: input.MaxConcurrencyLimit,
		})
		if err != nil {
			return err
		}

		var credentials []model.MarketplaceCredential
		if err := query.Find(&credentials).Error; err != nil {
			return err
		}

		var selected *model.MarketplaceCredential
		var selectedStats *model.MarketplaceCredentialStats
		var selectedCandidate MarketplacePoolCandidate
		for i := range credentials {
			credential := credentials[i]
			if !setting.IsMarketplaceVendorTypeEnabled(credential.VendorType) {
				continue
			}
			stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, credential.ID)
			if err != nil {
				return err
			}
			if !isMarketplacePoolRelayEligible(credential, *stats, input.BuyerUserID) {
				continue
			}
			candidate := newMarketplacePoolCandidate(credential, *stats)
			if selected == nil || marketplacePoolCandidateBetter(candidate, selectedCandidate) {
				selected = &credentials[i]
				selectedStats = stats
				selectedCandidate = candidate
			}
		}
		if selected == nil || selectedStats == nil {
			return errors.New("no eligible marketplace pool credential")
		}

		selectedStats.CurrentConcurrency++
		if err := tx.Save(selectedStats).Error; err != nil {
			return err
		}
		selected.CapacityStatus = marketplaceCredentialCapacityStatus(*selected, *selectedStats)
		if err := tx.Save(selected).Error; err != nil {
			return err
		}

		secret, err := GetMarketplaceCredentialSecret()
		if err != nil {
			return err
		}
		apiKey, err := DecryptMarketplaceAPIKey(selected.EncryptedAPIKey, secret)
		if err != nil {
			return err
		}

		preparation = &MarketplacePoolRelayPreparation{
			Credential: selected,
			APIKey:     apiKey,
			Session: &MarketplacePoolRelaySession{
				buyerUserID:             input.BuyerUserID,
				sellerUserID:            selected.SellerUserID,
				credentialID:            selected.ID,
				model:                   input.Model,
				requestID:               input.RequestID,
				multiplierSnapshot:      selected.Multiplier,
				platformFeeRateSnapshot: marketplaceFeeRateSnapshot(),
				startTime:               time.Now(),
			},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return preparation, nil
}

func NewMarketplacePoolBillingSession(funding relaycommon.BillingSettler, pool *MarketplacePoolRelaySession) *MarketplacePoolBillingSession {
	return &MarketplacePoolBillingSession{
		funding: funding,
		pool:    pool,
	}
}

func (s *MarketplacePoolBillingSession) Settle(actualQuota int) error {
	if s == nil {
		return nil
	}
	buyerCharge := actualQuota
	if s.pool != nil {
		buyerCharge = marketplaceBuyerChargeWithFee(int64(actualQuota), s.pool.platformFeeRateSnapshot)
	}
	if s.funding != nil {
		if err := s.funding.Settle(buyerCharge); err != nil {
			if s.pool != nil {
				s.pool.Release(nil)
			}
			return err
		}
	}
	if s.pool == nil {
		return nil
	}
	return s.pool.Settle(buyerCharge)
}

func (s *MarketplacePoolBillingSession) Refund(c *gin.Context) {
	if s == nil {
		return
	}
	if s.funding != nil {
		s.funding.Refund(c)
	}
	if s.pool != nil {
		s.pool.Release(c)
	}
}

func (s *MarketplacePoolBillingSession) NeedsRefund() bool {
	if s == nil || s.funding == nil {
		return false
	}
	return s.funding.NeedsRefund()
}

func (s *MarketplacePoolBillingSession) GetPreConsumedQuota() int {
	if s == nil || s.funding == nil {
		return 0
	}
	return s.funding.GetPreConsumedQuota()
}

func (s *MarketplacePoolBillingSession) Reserve(targetQuota int) error {
	if s == nil || s.funding == nil {
		return nil
	}
	return s.funding.Reserve(s.BuyerChargeForQuota(targetQuota))
}

func (s *MarketplacePoolBillingSession) BuyerChargeForQuota(quota int) int {
	if s == nil || s.pool == nil {
		return quota
	}
	return s.pool.BuyerChargeForQuota(quota)
}

func (s *MarketplacePoolRelaySession) BuyerChargeForQuota(quota int) int {
	if s == nil {
		return quota
	}
	return marketplaceBuyerChargeWithFee(int64(quota), s.platformFeeRateSnapshot)
}

func (s *MarketplacePoolRelaySession) Settle(buyerCharge int) error {
	if buyerCharge < 0 {
		return errors.New("buyer charge must not be negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled {
		return nil
	}

	var settlementEffect marketplaceSettlementReleaseSideEffect
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var credential model.MarketplaceCredential
		if err := marketplaceForUpdate(tx).Where("id = ?", s.credentialID).First(&credential).Error; err != nil {
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
		fill := &model.MarketplacePoolFill{
			RequestID:          s.requestID,
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
			SourceType:              marketplaceSettlementSourcePoolFill,
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

		stats.PoolRequestCount++
		stats.TotalRequestCount++
		stats.TotalOfficialCost += officialCost
		stats.QuotaUsed += officialCost
		stats.SuccessCount++
		stats.AvgLatencyMS = marketplaceNextAverageLatency(stats.AvgLatencyMS, stats.SuccessCount, latencyMS)
		stats.LastSuccessAt = common.GetTimestamp()
		if err := tx.Save(stats).Error; err != nil {
			return err
		}

		credential.CapacityStatus = marketplaceCredentialCapacityStatus(credential, *stats)
		if err := tx.Save(&credential).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		common.SysLog("marketplace pool settle failed, releasing capacity: " + err.Error())
		s.releaseCapacityLocked(nil)
		return err
	}
	applyMarketplaceSettlementReleaseSideEffect(settlementEffect)
	s.settled = true
	s.capacityReleased = true
	return nil
}

func (s *MarketplacePoolRelaySession) Release(c *gin.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseCapacityLocked(c)
}

func (s *MarketplacePoolRelaySession) releaseCapacityLocked(c *gin.Context) {
	if s.capacityReleased {
		return
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
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
	})
	if err != nil {
		if c != nil {
			logger.LogError(c, "error releasing marketplace pool capacity: "+err.Error())
		} else {
			common.SysLog("error releasing marketplace pool capacity: " + err.Error())
		}
		return
	}
	s.capacityReleased = true
}

func validateMarketplacePoolRelayInput(input MarketplacePoolRelayInput) error {
	if err := validateMarketplaceEnabled(); err != nil {
		return err
	}
	if input.BuyerUserID <= 0 {
		return errors.New("buyer user id is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		return errors.New("model is required")
	}
	if strings.TrimSpace(input.RequestID) == "" {
		return errors.New("request id is required")
	}
	return nil
}

func marketplacePoolCandidateBetter(candidate MarketplacePoolCandidate, selected MarketplacePoolCandidate) bool {
	if candidate.RouteScore != selected.RouteScore {
		return candidate.RouteScore > selected.RouteScore
	}
	if candidate.Credential.Multiplier != selected.Credential.Multiplier {
		return candidate.Credential.Multiplier < selected.Credential.Multiplier
	}
	return candidate.Credential.ID < selected.Credential.ID
}

func marketplacePoolRemainingSellerQuota(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) int64 {
	if credential.QuotaMode != model.MarketplaceQuotaModeLimited {
		return -1
	}
	remaining := credential.QuotaLimit - stats.QuotaUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}
