package service

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/model"
)

type MarketplacePoolModel struct {
	VendorType         int                     `json:"vendor_type"`
	VendorNameSnapshot string                  `json:"vendor_name_snapshot"`
	Model              string                  `json:"model"`
	CandidateCount     int                     `json:"candidate_count"`
	LowestMultiplier   float64                 `json:"lowest_multiplier"`
	LowestPricePreview MarketplacePricePreview `json:"lowest_price_preview"`
}

type MarketplacePoolCandidate struct {
	Credential  MarketplaceOrderListItem `json:"credential"`
	RouteScore  float64                  `json:"route_score"`
	SuccessRate float64                  `json:"success_rate"`
	LoadRatio   float64                  `json:"load_ratio"`
}

func ListMarketplacePoolModels(input MarketplaceOrderListInput) ([]MarketplacePoolModel, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	credentials, statsByCredentialID, err := listMarketplacePoolEligibleCredentials(input)
	if err != nil {
		return nil, err
	}
	modelsByKey := make(map[string]MarketplacePoolModel)
	modelFilter := strings.TrimSpace(input.Model)
	for _, credential := range credentials {
		stats := statsByCredentialID[credential.ID]
		if !isMarketplacePoolCredentialEligible(credential, stats) {
			continue
		}
		for _, modelName := range strings.Split(credential.Models, ",") {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			if modelFilter != "" && modelName != modelFilter {
				continue
			}
			key := marketplacePoolModelKey(credential.VendorType, modelName)
			current, ok := modelsByKey[key]
			pricePreview := marketplacePricePreviewForModel(credential, modelName)
			if !ok {
				modelsByKey[key] = MarketplacePoolModel{
					VendorType:         credential.VendorType,
					VendorNameSnapshot: credential.VendorNameSnapshot,
					Model:              modelName,
					CandidateCount:     1,
					LowestMultiplier:   credential.Multiplier,
					LowestPricePreview: pricePreview,
				}
				continue
			}
			current.CandidateCount++
			if credential.Multiplier < current.LowestMultiplier {
				current.LowestMultiplier = credential.Multiplier
				current.LowestPricePreview = pricePreview
			}
			modelsByKey[key] = current
		}
	}
	models := make([]MarketplacePoolModel, 0, len(modelsByKey))
	for _, poolModel := range modelsByKey {
		models = append(models, poolModel)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].VendorType != models[j].VendorType {
			return models[i].VendorType < models[j].VendorType
		}
		return models[i].Model < models[j].Model
	})
	return models, nil
}

func ListMarketplacePoolRelayModels(input MarketplaceOrderListInput) ([]string, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if input.BuyerUserID <= 0 {
		return nil, errors.New("buyer user id is required")
	}
	credentials, statsByCredentialID, err := listMarketplacePoolEligibleCredentials(input)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0)
	for _, credential := range credentials {
		stats := statsByCredentialID[credential.ID]
		if !isMarketplacePoolRelayEligible(credential, stats, input.BuyerUserID) {
			continue
		}
		models = appendMarketplaceModels(models, credential.Models)
	}
	sort.Strings(models)
	return models, nil
}

func ListMarketplacePoolCandidates(input MarketplaceOrderListInput, startIdx int, pageSize int) ([]MarketplacePoolCandidate, int64, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, 0, err
	}
	credentials, statsByCredentialID, err := listMarketplacePoolEligibleCredentials(input)
	if err != nil {
		return nil, 0, err
	}
	candidates := make([]MarketplacePoolCandidate, 0, len(credentials))
	for _, credential := range credentials {
		stats := statsByCredentialID[credential.ID]
		if !isMarketplacePoolCredentialEligible(credential, stats) {
			continue
		}
		candidates = append(candidates, newMarketplacePoolCandidate(credential, stats))
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].RouteScore != candidates[j].RouteScore {
			return candidates[i].RouteScore > candidates[j].RouteScore
		}
		if candidates[i].Credential.Multiplier != candidates[j].Credential.Multiplier {
			return candidates[i].Credential.Multiplier < candidates[j].Credential.Multiplier
		}
		return candidates[i].Credential.ID < candidates[j].Credential.ID
	})
	total := int64(len(candidates))
	if startIdx >= len(candidates) {
		return []MarketplacePoolCandidate{}, total, nil
	}
	endIdx := startIdx + pageSize
	if endIdx > len(candidates) {
		endIdx = len(candidates)
	}
	return candidates[startIdx:endIdx], total, nil
}

func listMarketplacePoolEligibleCredentials(input MarketplaceOrderListInput) ([]model.MarketplaceCredential, map[int]model.MarketplaceCredentialStats, error) {
	query, err := applyMarketplaceOrderListFilters(model.DB.Model(&model.MarketplaceCredential{}), input)
	if err != nil {
		return nil, nil, err
	}
	var credentials []model.MarketplaceCredential
	if err := query.Find(&credentials).Error; err != nil {
		return nil, nil, err
	}
	statsByCredentialID, err := marketplaceStatsByCredentialID(credentials)
	if err != nil {
		return nil, nil, err
	}
	return credentials, statsByCredentialID, nil
}

func isMarketplacePoolCredentialEligible(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) bool {
	if !isMarketplacePoolVisibleCredential(credential) {
		return false
	}
	if marketplaceCredentialCapacityStatus(credential, stats) != model.MarketplaceCapacityStatusAvailable {
		return false
	}
	if credential.ConcurrencyLimit > 0 && stats.CurrentConcurrency >= credential.ConcurrencyLimit {
		return false
	}
	if credential.QuotaMode == model.MarketplaceQuotaModeLimited && stats.QuotaUsed >= credential.QuotaLimit {
		return false
	}
	return true
}

func isMarketplacePoolVisibleCredential(credential model.MarketplaceCredential) bool {
	if credential.ListingStatus != model.MarketplaceListingStatusListed {
		return false
	}
	if credential.ServiceStatus != model.MarketplaceServiceStatusEnabled {
		return false
	}
	if credential.HealthStatus != model.MarketplaceHealthStatusUntested &&
		credential.HealthStatus != model.MarketplaceHealthStatusHealthy &&
		credential.HealthStatus != model.MarketplaceHealthStatusDegraded {
		return false
	}
	if credential.RiskStatus == model.MarketplaceRiskStatusRiskPaused {
		return false
	}
	return true
}

func isMarketplacePoolRelayEligible(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats, buyerUserID int) bool {
	if buyerUserID > 0 && credential.SellerUserID == buyerUserID {
		return false
	}
	if !isMarketplaceCredentialPurchaseEligible(credential, stats) {
		return false
	}
	return isMarketplacePoolCredentialEligible(credential, stats)
}

func newMarketplacePoolCandidate(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) MarketplacePoolCandidate {
	successRate := marketplacePoolSuccessRate(stats)
	loadRatio := marketplacePoolLoadRatio(credential, stats)
	return MarketplacePoolCandidate{
		Credential:  newMarketplaceOrderListItem(credential, stats),
		RouteScore:  marketplacePoolRouteScore(credential, stats, successRate, loadRatio),
		SuccessRate: successRate,
		LoadRatio:   loadRatio,
	}
}

func marketplacePoolSuccessRate(stats model.MarketplaceCredentialStats) float64 {
	total := stats.SuccessCount + stats.UpstreamErrorCount + stats.TimeoutCount + stats.RateLimitCount + stats.PlatformErrorCount
	if total <= 0 {
		return 1
	}
	return float64(stats.SuccessCount) / float64(total)
}

func marketplacePoolLoadRatio(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) float64 {
	if credential.ConcurrencyLimit <= 0 {
		return 0
	}
	return math.Min(1, float64(stats.CurrentConcurrency)/float64(credential.ConcurrencyLimit))
}

func marketplacePoolRouteScore(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats, successRate float64, loadRatio float64) float64 {
	latencyPenalty := math.Min(1, float64(stats.AvgLatencyMS)/10000)
	score := 100 + successRate*100 - loadRatio*50 - credential.Multiplier*10 - latencyPenalty*10
	if score < 0 {
		return 0
	}
	return score
}

func marketplacePoolModelKey(vendorType int, modelName string) string {
	return strings.Join([]string{strconv.Itoa(vendorType), modelName}, "\x00")
}
