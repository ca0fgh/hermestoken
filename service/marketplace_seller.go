package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"gorm.io/gorm"
)

type MarketplaceCredentialCreateInput struct {
	SellerUserID       int
	VendorType         int
	APIKey             string
	OpenAIOrganization string
	TestModel          string
	BaseURL            string
	Other              string
	ModelMapping       string
	StatusCodeMapping  string
	Setting            string
	ParamOverride      string
	HeaderOverride     string
	OtherSettings      string
	Models             []string
	QuotaMode          string
	QuotaLimit         int64
	TimeMode           string
	TimeLimitSeconds   int64
	Multiplier         float64
	ConcurrencyLimit   int
}

type MarketplaceCredentialUpdateInput struct {
	SellerUserID       int
	CredentialID       int
	APIKey             string
	OpenAIOrganization *string
	TestModel          *string
	BaseURL            *string
	Other              *string
	ModelMapping       *string
	StatusCodeMapping  *string
	Setting            *string
	ParamOverride      *string
	HeaderOverride     *string
	OtherSettings      *string
	Models             *[]string
	QuotaMode          *string
	QuotaLimit         *int64
	TimeMode           *string
	TimeLimitSeconds   *int64
	Multiplier         *float64
	ConcurrencyLimit   *int
}

type MarketplaceSellerCredentialItem struct {
	model.MarketplaceCredential
	CurrentConcurrency     int    `json:"current_concurrency"`
	TotalRequestCount      int64  `json:"total_request_count"`
	PoolRequestCount       int64  `json:"pool_request_count"`
	FixedOrderRequestCount int64  `json:"fixed_order_request_count"`
	TotalOfficialCost      int64  `json:"total_official_cost"`
	QuotaUsed              int64  `json:"quota_used"`
	FixedOrderSoldQuota    int64  `json:"fixed_order_sold_quota"`
	ActiveFixedOrderCount  int64  `json:"active_fixed_order_count"`
	SuccessCount           int64  `json:"success_count"`
	UpstreamErrorCount     int64  `json:"upstream_error_count"`
	TimeoutCount           int64  `json:"timeout_count"`
	RateLimitCount         int64  `json:"rate_limit_count"`
	PlatformErrorCount     int64  `json:"platform_error_count"`
	AvgLatencyMS           int64  `json:"avg_latency_ms"`
	LastSuccessAt          int64  `json:"last_success_at"`
	LastFailedAt           int64  `json:"last_failed_at"`
	LastFailedReason       string `json:"last_failed_reason"`
	RouteStatus            string `json:"route_status"`
}

func CreateSellerMarketplaceCredential(input MarketplaceCredentialCreateInput) (*model.MarketplaceCredential, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if input.SellerUserID <= 0 {
		return nil, errors.New("seller user id is required")
	}
	if !setting.IsMarketplaceVendorTypeEnabled(input.VendorType) {
		return nil, fmt.Errorf("marketplace vendor type %d is not enabled", input.VendorType)
	}
	vendorName := constant.GetChannelTypeName(input.VendorType)
	if vendorName == "Unknown" {
		return nil, fmt.Errorf("marketplace vendor type %d is unknown", input.VendorType)
	}
	models, err := normalizeMarketplaceModels(input.Models)
	if err != nil {
		return nil, err
	}
	quotaMode, quotaLimit, err := normalizeMarketplaceQuota(input.QuotaMode, input.QuotaLimit)
	if err != nil {
		return nil, err
	}
	timeMode, timeLimitSeconds, err := normalizeMarketplaceTime(input.TimeMode, input.TimeLimitSeconds)
	if err != nil {
		return nil, err
	}
	multiplier, err := normalizeMarketplaceMultiplier(input.Multiplier)
	if err != nil {
		return nil, err
	}
	concurrencyLimit, err := normalizeMarketplaceConcurrency(input.ConcurrencyLimit)
	if err != nil {
		return nil, err
	}
	if err := validateMarketplaceCredentialAPIKeyHosting(input.APIKey); err != nil {
		return nil, err
	}
	channelConfig, err := normalizeMarketplaceCredentialChannelConfig(marketplaceCredentialChannelConfigInput{
		VendorType:         input.VendorType,
		OpenAIOrganization: input.OpenAIOrganization,
		TestModel:          input.TestModel,
		BaseURL:            input.BaseURL,
		Other:              input.Other,
		ModelMapping:       input.ModelMapping,
		StatusCodeMapping:  input.StatusCodeMapping,
		Setting:            input.Setting,
		ParamOverride:      input.ParamOverride,
		HeaderOverride:     input.HeaderOverride,
		OtherSettings:      input.OtherSettings,
	})
	if err != nil {
		return nil, err
	}
	encryptedKey, fingerprint, err := encryptAndFingerprintMarketplaceKey(input.APIKey)
	if err != nil {
		return nil, err
	}

	credential := &model.MarketplaceCredential{
		SellerUserID:       input.SellerUserID,
		VendorType:         input.VendorType,
		VendorNameSnapshot: vendorName,
		EncryptedAPIKey:    encryptedKey,
		KeyFingerprint:     fingerprint,
		OpenAIOrganization: channelConfig.OpenAIOrganization,
		TestModel:          channelConfig.TestModel,
		BaseURL:            channelConfig.BaseURL,
		Other:              channelConfig.Other,
		ModelMapping:       channelConfig.ModelMapping,
		StatusCodeMapping:  channelConfig.StatusCodeMapping,
		Setting:            channelConfig.Setting,
		ParamOverride:      channelConfig.ParamOverride,
		HeaderOverride:     channelConfig.HeaderOverride,
		OtherSettings:      channelConfig.OtherSettings,
		Models:             models,
		QuotaMode:          quotaMode,
		QuotaLimit:         quotaLimit,
		TimeMode:           timeMode,
		TimeLimitSeconds:   timeLimitSeconds,
		Multiplier:         multiplier,
		ConcurrencyLimit:   concurrencyLimit,
		ListingStatus:      model.MarketplaceListingStatusListed,
		ServiceStatus:      model.MarketplaceServiceStatusEnabled,
		HealthStatus:       model.MarketplaceHealthStatusUntested,
		CapacityStatus:     model.MarketplaceCapacityStatusAvailable,
		RiskStatus:         model.MarketplaceRiskStatusNormal,
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(credential).Error; err != nil {
			return err
		}
		stats := &model.MarketplaceCredentialStats{CredentialID: credential.ID}
		if err := tx.Create(stats).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return credential, nil
}

func ListSellerMarketplaceCredentials(sellerUserID int, startIdx int, pageSize int) ([]*model.MarketplaceCredential, int64, error) {
	var credentials []*model.MarketplaceCredential
	var total int64
	query := model.DB.Model(&model.MarketplaceCredential{}).Where("seller_user_id = ?", sellerUserID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Limit(pageSize).Offset(startIdx).Find(&credentials).Error
	return credentials, total, err
}

func ListSellerMarketplaceCredentialItems(sellerUserID int, startIdx int, pageSize int) ([]MarketplaceSellerCredentialItem, int64, error) {
	credentials, total, err := ListSellerMarketplaceCredentials(sellerUserID, startIdx, pageSize)
	if err != nil {
		return nil, 0, err
	}
	values := make([]model.MarketplaceCredential, 0, len(credentials))
	for _, credential := range credentials {
		if credential == nil {
			continue
		}
		values = append(values, *credential)
	}
	statsByCredentialID, err := marketplaceStatsByCredentialID(values)
	if err != nil {
		return nil, 0, err
	}
	items := make([]MarketplaceSellerCredentialItem, 0, len(values))
	for _, credential := range values {
		items = append(items, newMarketplaceSellerCredentialItem(credential, statsByCredentialID[credential.ID]))
	}
	return items, total, nil
}

func newMarketplaceSellerCredentialItem(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) MarketplaceSellerCredentialItem {
	credential.CapacityStatus = marketplaceCredentialCapacityStatus(credential, stats)
	return MarketplaceSellerCredentialItem{
		MarketplaceCredential:  credential,
		CurrentConcurrency:     stats.CurrentConcurrency,
		TotalRequestCount:      stats.TotalRequestCount,
		PoolRequestCount:       stats.PoolRequestCount,
		FixedOrderRequestCount: stats.FixedOrderRequestCount,
		TotalOfficialCost:      stats.TotalOfficialCost,
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
		RouteStatus:            marketplaceCredentialRouteStatus(credential, stats),
	}
}

func GetSellerMarketplaceCredential(sellerUserID int, credentialID int) (*model.MarketplaceCredential, error) {
	var credential model.MarketplaceCredential
	err := model.DB.Where("id = ? AND seller_user_id = ?", credentialID, sellerUserID).First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("marketplace credential not found")
	}
	return &credential, err
}

func UpdateSellerMarketplaceCredential(input MarketplaceCredentialUpdateInput) (*model.MarketplaceCredential, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	credential, err := GetSellerMarketplaceCredential(input.SellerUserID, input.CredentialID)
	if err != nil {
		return nil, err
	}
	changedFields := make([]string, 0)
	keyReplaced := strings.TrimSpace(input.APIKey) != ""

	if input.Models != nil {
		models, err := normalizeMarketplaceModels(*input.Models)
		if err != nil {
			return nil, err
		}
		if credential.Models != models {
			credential.Models = models
			changedFields = append(changedFields, "models")
		}
	}
	if input.QuotaMode != nil || input.QuotaLimit != nil {
		quotaMode := credential.QuotaMode
		quotaLimit := credential.QuotaLimit
		if input.QuotaMode != nil {
			quotaMode = *input.QuotaMode
		}
		if input.QuotaLimit != nil {
			quotaLimit = *input.QuotaLimit
		}
		normalizedMode, normalizedLimit, err := normalizeMarketplaceQuota(quotaMode, quotaLimit)
		if err != nil {
			return nil, err
		}
		if credential.QuotaMode != normalizedMode {
			credential.QuotaMode = normalizedMode
			changedFields = append(changedFields, "quota_mode")
		}
		if credential.QuotaLimit != normalizedLimit {
			credential.QuotaLimit = normalizedLimit
			changedFields = append(changedFields, "quota_limit")
		}
	}
	if input.TimeMode != nil || input.TimeLimitSeconds != nil {
		timeMode := credential.TimeMode
		timeLimitSeconds := credential.TimeLimitSeconds
		if input.TimeMode != nil {
			timeMode = *input.TimeMode
		}
		if input.TimeLimitSeconds != nil {
			timeLimitSeconds = *input.TimeLimitSeconds
		}
		normalizedMode, normalizedLimit, err := normalizeMarketplaceTime(timeMode, timeLimitSeconds)
		if err != nil {
			return nil, err
		}
		if credential.TimeMode != normalizedMode {
			credential.TimeMode = normalizedMode
			changedFields = append(changedFields, "time_mode")
		}
		if credential.TimeLimitSeconds != normalizedLimit {
			credential.TimeLimitSeconds = normalizedLimit
			changedFields = append(changedFields, "time_limit_seconds")
		}
	}
	if input.Multiplier != nil {
		multiplier, err := normalizeMarketplaceMultiplier(*input.Multiplier)
		if err != nil {
			return nil, err
		}
		if credential.Multiplier != multiplier {
			credential.Multiplier = multiplier
			changedFields = append(changedFields, "multiplier")
		}
	}
	if input.ConcurrencyLimit != nil {
		concurrencyLimit, err := normalizeMarketplaceConcurrency(*input.ConcurrencyLimit)
		if err != nil {
			return nil, err
		}
		if credential.ConcurrencyLimit != concurrencyLimit {
			credential.ConcurrencyLimit = concurrencyLimit
			changedFields = append(changedFields, "concurrency_limit")
		}
	}
	if err := applyMarketplaceCredentialChannelConfigUpdates(credential, input, &changedFields); err != nil {
		return nil, err
	}
	if keyReplaced {
		if err := validateMarketplaceCredentialAPIKeyHosting(input.APIKey); err != nil {
			return nil, err
		}
		encryptedKey, fingerprint, err := encryptAndFingerprintMarketplaceKey(input.APIKey)
		if err != nil {
			return nil, err
		}
		credential.EncryptedAPIKey = encryptedKey
		credential.KeyFingerprint = fingerprint
		changedFields = append(changedFields, "api_key")
	}
	if len(changedFields) == 0 {
		return credential, nil
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, credential.ID)
		if err != nil {
			return err
		}
		credential.CapacityStatus = marketplaceCredentialCapacityStatus(*credential, *stats)
		if err := tx.Save(credential).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return credential, nil
}

func SetSellerMarketplaceCredentialListed(sellerUserID int, credentialID int, listed bool) (*model.MarketplaceCredential, error) {
	status := model.MarketplaceListingStatusUnlisted
	if listed {
		status = model.MarketplaceListingStatusListed
	}
	return updateSellerMarketplaceCredentialStatus(sellerUserID, credentialID, "listing_status", status)
}

func SetSellerMarketplaceCredentialEnabled(sellerUserID int, credentialID int, enabled bool) (*model.MarketplaceCredential, error) {
	status := model.MarketplaceServiceStatusDisabled
	if enabled {
		status = model.MarketplaceServiceStatusEnabled
	}
	return updateSellerMarketplaceCredentialStatus(sellerUserID, credentialID, "service_status", status)
}

func updateSellerMarketplaceCredentialStatus(sellerUserID int, credentialID int, field string, value string) (*model.MarketplaceCredential, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	credential, err := GetSellerMarketplaceCredential(sellerUserID, credentialID)
	if err != nil {
		return nil, err
	}
	switch field {
	case "listing_status":
		credential.ListingStatus = value
	case "service_status":
		credential.ServiceStatus = value
	default:
		return nil, fmt.Errorf("unsupported marketplace credential status field %s", field)
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(credential).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return credential, nil
}

func validateMarketplaceEnabled() error {
	if !setting.MarketplaceEnabled {
		return errors.New("marketplace is not enabled")
	}
	return nil
}

func normalizeMarketplaceModels(models []string) (string, error) {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		normalized = append(normalized, modelName)
	}
	if len(normalized) == 0 {
		return "", errors.New("at least one marketplace model is required")
	}
	return strings.Join(normalized, ","), nil
}

func normalizeMarketplaceQuota(quotaMode string, quotaLimit int64) (string, int64, error) {
	quotaMode = strings.TrimSpace(quotaMode)
	if quotaMode == "" {
		quotaMode = model.MarketplaceQuotaModeUnlimited
	}
	switch quotaMode {
	case model.MarketplaceQuotaModeUnlimited:
		return quotaMode, 0, nil
	case model.MarketplaceQuotaModeLimited:
		if quotaLimit <= 0 {
			return "", 0, errors.New("limited marketplace quota requires positive quota_limit")
		}
		return quotaMode, quotaLimit, nil
	default:
		return "", 0, fmt.Errorf("unsupported marketplace quota mode %s", quotaMode)
	}
}

func normalizeMarketplaceTime(timeMode string, timeLimitSeconds int64) (string, int64, error) {
	timeMode = strings.TrimSpace(timeMode)
	if timeMode == "" {
		timeMode = model.MarketplaceTimeModeUnlimited
	}
	switch timeMode {
	case model.MarketplaceTimeModeUnlimited:
		return timeMode, 0, nil
	case model.MarketplaceTimeModeLimited:
		if timeLimitSeconds <= 0 {
			return "", 0, errors.New("limited marketplace time requires positive time_limit_seconds")
		}
		return timeMode, timeLimitSeconds, nil
	default:
		return "", 0, fmt.Errorf("unsupported marketplace time mode %s", timeMode)
	}
}

func normalizeMarketplaceMultiplier(multiplier float64) (float64, error) {
	if multiplier == 0 {
		multiplier = 1
	}
	if multiplier <= 0 {
		return 0, errors.New("marketplace multiplier must be positive")
	}
	if setting.MarketplaceMaxSellerMultiplier > 0 && multiplier > setting.MarketplaceMaxSellerMultiplier {
		return 0, fmt.Errorf("marketplace multiplier exceeds max %.2f", setting.MarketplaceMaxSellerMultiplier)
	}
	return multiplier, nil
}

func normalizeMarketplaceConcurrency(concurrencyLimit int) (int, error) {
	if concurrencyLimit == 0 {
		concurrencyLimit = 1
	}
	if concurrencyLimit <= 0 {
		return 0, errors.New("marketplace concurrency_limit must be positive")
	}
	if setting.MarketplaceMaxCredentialConcurrency > 0 && concurrencyLimit > setting.MarketplaceMaxCredentialConcurrency {
		return 0, fmt.Errorf("marketplace concurrency_limit exceeds max %d", setting.MarketplaceMaxCredentialConcurrency)
	}
	return concurrencyLimit, nil
}

func encryptAndFingerprintMarketplaceKey(apiKey string) (string, string, error) {
	secret, err := GetMarketplaceCredentialSecret()
	if err != nil {
		return "", "", err
	}
	encryptedKey, err := EncryptMarketplaceAPIKey(apiKey, secret)
	if err != nil {
		return "", "", err
	}
	fingerprint, err := FingerprintMarketplaceAPIKey(apiKey, secret)
	if err != nil {
		return "", "", err
	}
	return encryptedKey, fingerprint, nil
}
