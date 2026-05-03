package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

type MarketplacePoolFilterValues struct {
	VendorType          int     `json:"vendor_type,omitempty"`
	Model               string  `json:"model,omitempty"`
	QuotaMode           string  `json:"quota_mode,omitempty"`
	TimeMode            string  `json:"time_mode,omitempty"`
	MinQuotaLimit       int64   `json:"min_quota_limit,omitempty"`
	MaxQuotaLimit       int64   `json:"max_quota_limit,omitempty"`
	MinTimeLimitSeconds int64   `json:"min_time_limit_seconds,omitempty"`
	MaxTimeLimitSeconds int64   `json:"max_time_limit_seconds,omitempty"`
	MinMultiplier       float64 `json:"min_multiplier,omitempty"`
	MaxMultiplier       float64 `json:"max_multiplier,omitempty"`
	MinConcurrencyLimit int     `json:"min_concurrency_limit,omitempty"`
	MaxConcurrencyLimit int     `json:"max_concurrency_limit,omitempty"`
}

type MarketplacePoolFilters string

func NewMarketplacePoolFilters(values MarketplacePoolFilterValues) MarketplacePoolFilters {
	normalized := normalizeMarketplacePoolFilterValues(values)
	if normalized.Empty() {
		return ""
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return MarketplacePoolFilters(payload)
}

func (filters MarketplacePoolFilters) Values() MarketplacePoolFilterValues {
	raw := strings.TrimSpace(string(filters))
	if raw == "" {
		return MarketplacePoolFilterValues{}
	}
	var values MarketplacePoolFilterValues
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return MarketplacePoolFilterValues{}
	}
	return normalizeMarketplacePoolFilterValues(values)
}

func (values MarketplacePoolFilterValues) Empty() bool {
	return values.VendorType == 0 &&
		values.Model == "" &&
		values.QuotaMode == "" &&
		values.TimeMode == "" &&
		values.MinQuotaLimit == 0 &&
		values.MaxQuotaLimit == 0 &&
		values.MinTimeLimitSeconds == 0 &&
		values.MaxTimeLimitSeconds == 0 &&
		values.MinMultiplier == 0 &&
		values.MaxMultiplier == 0 &&
		values.MinConcurrencyLimit == 0 &&
		values.MaxConcurrencyLimit == 0
}

func (filters MarketplacePoolFilters) Value() (driver.Value, error) {
	return string(NewMarketplacePoolFilters(filters.Values())), nil
}

func (filters *MarketplacePoolFilters) Scan(value any) error {
	if filters == nil {
		return nil
	}
	switch v := value.(type) {
	case nil:
		*filters = ""
	case string:
		*filters = NewMarketplacePoolFilters(parseMarketplacePoolFilterString(v))
	case []byte:
		*filters = NewMarketplacePoolFilters(parseMarketplacePoolFilterString(string(v)))
	default:
		return fmt.Errorf("unsupported marketplace pool filters type %T", value)
	}
	return nil
}

func (filters MarketplacePoolFilters) MarshalJSON() ([]byte, error) {
	return json.Marshal(filters.Values())
}

func (filters *MarketplacePoolFilters) UnmarshalJSON(data []byte) error {
	if filters == nil {
		return nil
	}
	if string(data) == "null" {
		*filters = ""
		return nil
	}
	var values MarketplacePoolFilterValues
	if err := json.Unmarshal(data, &values); err == nil {
		*filters = NewMarketplacePoolFilters(values)
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		*filters = NewMarketplacePoolFilters(parseMarketplacePoolFilterString(raw))
		return nil
	}
	return fmt.Errorf("invalid marketplace pool filters")
}

func parseMarketplacePoolFilterString(raw string) MarketplacePoolFilterValues {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MarketplacePoolFilterValues{}
	}
	var nested string
	if err := json.Unmarshal([]byte(raw), &nested); err == nil && strings.TrimSpace(nested) != "" {
		raw = nested
	}
	var values MarketplacePoolFilterValues
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return MarketplacePoolFilterValues{}
	}
	return values
}

func normalizeMarketplacePoolFilterValues(values MarketplacePoolFilterValues) MarketplacePoolFilterValues {
	values.Model = strings.TrimSpace(values.Model)
	if values.VendorType < 0 {
		values.VendorType = 0
	}
	switch strings.TrimSpace(values.QuotaMode) {
	case MarketplaceQuotaModeUnlimited, MarketplaceQuotaModeLimited:
		values.QuotaMode = strings.TrimSpace(values.QuotaMode)
	default:
		values.QuotaMode = ""
	}
	switch strings.TrimSpace(values.TimeMode) {
	case MarketplaceTimeModeUnlimited, MarketplaceTimeModeLimited:
		values.TimeMode = strings.TrimSpace(values.TimeMode)
	default:
		values.TimeMode = ""
	}
	if values.MinQuotaLimit < 0 {
		values.MinQuotaLimit = 0
	}
	if values.MaxQuotaLimit < 0 {
		values.MaxQuotaLimit = 0
	}
	if values.MinTimeLimitSeconds < 0 {
		values.MinTimeLimitSeconds = 0
	}
	if values.MaxTimeLimitSeconds < 0 {
		values.MaxTimeLimitSeconds = 0
	}
	if values.MinMultiplier < 0 {
		values.MinMultiplier = 0
	}
	if values.MaxMultiplier < 0 {
		values.MaxMultiplier = 0
	}
	if values.MinConcurrencyLimit < 0 {
		values.MinConcurrencyLimit = 0
	}
	if values.MaxConcurrencyLimit < 0 {
		values.MaxConcurrencyLimit = 0
	}
	return values
}
