package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"gorm.io/gorm"
)

type MarketplaceCredentialTestResultInput struct {
	SellerUserID   int
	CredentialID   int
	Success        bool
	Skipped        bool
	ResponseTimeMS int64
	Reason         string
}

type marketplaceCredentialChannelConfigInput struct {
	VendorType         int
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
}

func TestSellerMarketplaceCredential(sellerUserID int, credentialID int) error {
	if err := validateMarketplaceEnabled(); err != nil {
		return err
	}
	credential, err := GetSellerMarketplaceCredential(sellerUserID, credentialID)
	if err != nil {
		return err
	}
	secret, err := GetMarketplaceCredentialSecret()
	if err != nil {
		return err
	}
	if _, err := DecryptMarketplaceAPIKey(credential.EncryptedAPIKey, secret); err != nil {
		return err
	}
	return nil
}

func BuildSellerMarketplaceChannel(sellerUserID int, credentialID int) (*model.Channel, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	credential, err := GetSellerMarketplaceCredential(sellerUserID, credentialID)
	if err != nil {
		return nil, err
	}
	secret, err := GetMarketplaceCredentialSecret()
	if err != nil {
		return nil, err
	}
	apiKey, err := DecryptMarketplaceAPIKey(credential.EncryptedAPIKey, secret)
	if err != nil {
		return nil, err
	}
	return MarketplaceChannelFromCredential(credential, apiKey), nil
}

func MarketplaceChannelFromCredential(credential *model.MarketplaceCredential, apiKey string) *model.Channel {
	if credential == nil {
		return nil
	}
	channel := &model.Channel{
		Id:                credential.ID,
		Type:              credential.VendorType,
		Key:               apiKey,
		Status:            common.ChannelStatusEnabled,
		Name:              fmt.Sprintf("marketplace:%d", credential.ID),
		CreatedTime:       credential.CreatedAt,
		BaseURL:           marketplaceStringPointer(normalizeMarketplaceCredentialBaseURL(credential.VendorType, credential.BaseURL)),
		Other:             credential.Other,
		Models:            credential.Models,
		Group:             "default",
		ModelMapping:      marketplaceStringPointer(credential.ModelMapping),
		StatusCodeMapping: marketplaceStringPointer(credential.StatusCodeMapping),
		Priority:          common.GetPointer(int64(0)),
		Weight:            common.GetPointer(uint(0)),
		Setting:           marketplaceStringPointer(credential.Setting),
		ParamOverride:     marketplaceStringPointer(credential.ParamOverride),
		HeaderOverride:    marketplaceStringPointer(credential.HeaderOverride),
		OtherSettings:     credential.OtherSettings,
	}
	if strings.TrimSpace(credential.OpenAIOrganization) != "" {
		channel.OpenAIOrganization = common.GetPointer(strings.TrimSpace(credential.OpenAIOrganization))
	}
	if strings.TrimSpace(credential.TestModel) != "" {
		channel.TestModel = common.GetPointer(strings.TrimSpace(credential.TestModel))
	}
	return channel
}

func ApplySellerMarketplaceCredentialTestResult(input MarketplaceCredentialTestResultInput) (*model.MarketplaceCredential, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if input.SellerUserID <= 0 {
		return nil, errors.New("seller user id is required")
	}
	credential, err := GetSellerMarketplaceCredential(input.SellerUserID, input.CredentialID)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		stats, err := getOrCreateMarketplaceCredentialStatsForUpdate(tx, credential.ID)
		if err != nil {
			return err
		}
		switch {
		case input.Success:
			credential.HealthStatus = model.MarketplaceHealthStatusHealthy
		case input.Skipped:
			credential.HealthStatus = model.MarketplaceHealthStatusDegraded
		default:
			credential.HealthStatus = model.MarketplaceHealthStatusFailed
		}
		if input.ResponseTimeMS > 0 {
			credential.ResponseTime = int(input.ResponseTimeMS)
		} else {
			credential.ResponseTime = 0
		}
		credential.TestTime = time.Now().Unix()
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

func normalizeMarketplaceCredentialChannelConfig(input marketplaceCredentialChannelConfigInput) (marketplaceCredentialChannelConfigInput, error) {
	var err error
	input.OpenAIOrganization = strings.TrimSpace(input.OpenAIOrganization)
	input.TestModel = strings.TrimSpace(input.TestModel)
	input.BaseURL = normalizeMarketplaceCredentialBaseURL(input.VendorType, input.BaseURL)
	input.Other = strings.TrimSpace(input.Other)
	if input.ModelMapping, err = normalizeMarketplaceJSONConfig("model_mapping", input.ModelMapping); err != nil {
		return input, err
	}
	if input.StatusCodeMapping, err = normalizeMarketplaceJSONConfig("status_code_mapping", input.StatusCodeMapping); err != nil {
		return input, err
	}
	if input.Setting, err = normalizeMarketplaceJSONConfig("setting", input.Setting); err != nil {
		return input, err
	}
	if input.ParamOverride, err = normalizeMarketplaceJSONConfig("param_override", input.ParamOverride); err != nil {
		return input, err
	}
	if input.HeaderOverride, err = normalizeMarketplaceJSONConfig("header_override", input.HeaderOverride); err != nil {
		return input, err
	}
	if input.OtherSettings, err = normalizeMarketplaceJSONConfig("settings", input.OtherSettings); err != nil {
		return input, err
	}
	return input, nil
}

func applyMarketplaceCredentialChannelConfigUpdates(credential *model.MarketplaceCredential, input MarketplaceCredentialUpdateInput, changedFields *[]string) error {
	updates := []struct {
		field     string
		value     *string
		apply     func(string)
		current   func() string
		jsonField bool
	}{
		{"openai_organization", input.OpenAIOrganization, func(v string) { credential.OpenAIOrganization = v }, func() string { return credential.OpenAIOrganization }, false},
		{"test_model", input.TestModel, func(v string) { credential.TestModel = v }, func() string { return credential.TestModel }, false},
		{"base_url", input.BaseURL, func(v string) { credential.BaseURL = v }, func() string { return credential.BaseURL }, false},
		{"other", input.Other, func(v string) { credential.Other = v }, func() string { return credential.Other }, false},
		{"model_mapping", input.ModelMapping, func(v string) { credential.ModelMapping = v }, func() string { return credential.ModelMapping }, true},
		{"status_code_mapping", input.StatusCodeMapping, func(v string) { credential.StatusCodeMapping = v }, func() string { return credential.StatusCodeMapping }, true},
		{"setting", input.Setting, func(v string) { credential.Setting = v }, func() string { return credential.Setting }, true},
		{"param_override", input.ParamOverride, func(v string) { credential.ParamOverride = v }, func() string { return credential.ParamOverride }, true},
		{"header_override", input.HeaderOverride, func(v string) { credential.HeaderOverride = v }, func() string { return credential.HeaderOverride }, true},
		{"settings", input.OtherSettings, func(v string) { credential.OtherSettings = v }, func() string { return credential.OtherSettings }, true},
	}
	for _, update := range updates {
		if update.value == nil {
			continue
		}
		value := strings.TrimSpace(*update.value)
		var err error
		if update.field == "base_url" {
			value = normalizeMarketplaceCredentialBaseURL(credential.VendorType, value)
		} else if update.jsonField {
			value, err = normalizeMarketplaceJSONConfig(update.field, value)
			if err != nil {
				return err
			}
		}
		if update.current() != value {
			update.apply(value)
			*changedFields = append(*changedFields, update.field)
		}
	}
	return nil
}

func normalizeMarketplaceJSONConfig(field string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !json.Valid([]byte(value)) {
		return "", fmt.Errorf("marketplace %s must be valid JSON", field)
	}
	return value, nil
}

func normalizeMarketplaceCredentialBaseURL(vendorType int, value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" || vendorType == constant.ChannelTypeCustom {
		return value
	}
	lowerValue := strings.ToLower(value)
	endpointSuffixes := []string{
		"/v1/chat/completions",
		"/v1/responses/compact",
		"/v1/responses",
		"/v1/embeddings",
		"/v1/images/generations",
		"/v1/models",
		"/chat/completions",
		"/responses/compact",
		"/responses",
		"/embeddings",
		"/images/generations",
		"/models",
	}
	for _, suffix := range endpointSuffixes {
		if strings.HasSuffix(lowerValue, suffix) {
			return strings.TrimRight(value[:len(value)-len(suffix)], "/")
		}
	}
	if vendorType == constant.ChannelTypeAli && strings.HasSuffix(lowerValue, "/compatible-mode/v1") {
		return strings.TrimRight(value[:len(value)-len("/compatible-mode/v1")], "/")
	}
	if strings.HasSuffix(lowerValue, "/v1") {
		return strings.TrimRight(value[:len(value)-len("/v1")], "/")
	}
	return value
}

func marketplaceStringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return common.GetPointer(value)
}

func marketplaceCredentialCapacityStatus(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) string {
	if credential.QuotaMode == model.MarketplaceQuotaModeLimited && credential.QuotaLimit > 0 && stats.QuotaUsed >= credential.QuotaLimit {
		return model.MarketplaceCapacityStatusExhausted
	}
	if credential.ConcurrencyLimit > 0 && stats.CurrentConcurrency >= credential.ConcurrencyLimit {
		return model.MarketplaceCapacityStatusBusy
	}
	return model.MarketplaceCapacityStatusAvailable
}

func marketplaceCredentialRouteStatus(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) string {
	status, _ := marketplaceCredentialRouteStatusAndReason(credential, stats)
	return status
}

func marketplaceCredentialRouteReason(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) string {
	_, reason := marketplaceCredentialRouteStatusAndReason(credential, stats)
	return reason
}

func marketplaceCredentialRouteStatusAndReason(credential model.MarketplaceCredential, stats model.MarketplaceCredentialStats) (string, string) {
	if credential.ListingStatus != model.MarketplaceListingStatusListed {
		return model.MarketplaceRouteStatusUnlisted, model.MarketplaceRouteReasonUnlisted
	}
	if credential.ServiceStatus != model.MarketplaceServiceStatusEnabled {
		return model.MarketplaceRouteStatusDisabled, model.MarketplaceRouteReasonDisabled
	}
	switch credential.HealthStatus {
	case model.MarketplaceHealthStatusUntested, model.MarketplaceHealthStatusHealthy, model.MarketplaceHealthStatusDegraded:
	default:
		return model.MarketplaceRouteStatusFailed, marketplaceCredentialHealthRouteReason(credential.HealthStatus)
	}
	if reason := marketplaceCredentialProbeRouteReason(credential); reason != "" {
		return model.MarketplaceRouteStatusFailed, reason
	}
	if credential.RiskStatus == model.MarketplaceRiskStatusRiskPaused {
		return model.MarketplaceRouteStatusRiskPaused, model.MarketplaceRouteReasonRiskPaused
	}
	switch marketplaceCredentialCapacityStatus(credential, stats) {
	case model.MarketplaceCapacityStatusExhausted:
		return model.MarketplaceRouteStatusExhausted, model.MarketplaceRouteReasonQuotaExhausted
	case model.MarketplaceCapacityStatusBusy:
		return model.MarketplaceRouteStatusBusy, model.MarketplaceRouteReasonConcurrencyBusy
	default:
		return model.MarketplaceRouteStatusAvailable, ""
	}
}

func marketplaceCredentialHasRoutableProbeScore(credential model.MarketplaceCredential) bool {
	switch credential.ProbeStatus {
	case model.MarketplaceProbeStatusPassed, model.MarketplaceProbeStatusWarning:
	default:
		return false
	}
	return credential.ProbeScore > 0 && credential.ProbeScoreMax > 0
}

func marketplaceCredentialHealthRouteReason(healthStatus string) string {
	switch healthStatus {
	case model.MarketplaceHealthStatusFailed:
		return model.MarketplaceRouteReasonHealthFailed
	default:
		return model.MarketplaceRouteReasonHealthUnavailable
	}
}

func marketplaceCredentialProbeRouteReason(credential model.MarketplaceCredential) string {
	switch credential.ProbeStatus {
	case model.MarketplaceProbeStatusPassed, model.MarketplaceProbeStatusWarning:
		switch {
		case credential.ProbeScoreMax <= 0:
			return model.MarketplaceRouteReasonProbeScoreMissing
		case credential.ProbeScore <= 0:
			return model.MarketplaceRouteReasonProbeScoreZero
		default:
			return ""
		}
	case model.MarketplaceProbeStatusPending, model.MarketplaceProbeStatusRunning:
		return model.MarketplaceRouteReasonProbeInProgress
	case model.MarketplaceProbeStatusFailed:
		return model.MarketplaceRouteReasonProbeFailed
	case model.MarketplaceProbeStatusUnscored, "":
		return model.MarketplaceRouteReasonProbeUnscored
	default:
		return model.MarketplaceRouteReasonUnavailableGeneric
	}
}
