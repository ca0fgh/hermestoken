package model

const (
	MarketplaceQuotaModeUnlimited = "unlimited"
	MarketplaceQuotaModeLimited   = "limited"

	MarketplaceTimeModeUnlimited = "unlimited"
	MarketplaceTimeModeLimited   = "limited"

	MarketplaceListingStatusListed   = "listed"
	MarketplaceListingStatusUnlisted = "unlisted"

	MarketplaceServiceStatusEnabled  = "enabled"
	MarketplaceServiceStatusDisabled = "disabled"

	MarketplaceHealthStatusUntested = "untested"
	MarketplaceHealthStatusHealthy  = "healthy"
	MarketplaceHealthStatusDegraded = "degraded"
	MarketplaceHealthStatusFailed   = "failed"

	MarketplaceCapacityStatusAvailable = "available"
	MarketplaceCapacityStatusBusy      = "busy"
	MarketplaceCapacityStatusExhausted = "exhausted"

	MarketplaceRiskStatusNormal     = "normal"
	MarketplaceRiskStatusWatching   = "watching"
	MarketplaceRiskStatusRiskPaused = "risk_paused"

	MarketplaceRouteStatusAvailable  = "route_available"
	MarketplaceRouteStatusUnlisted   = "route_unlisted"
	MarketplaceRouteStatusDisabled   = "route_disabled"
	MarketplaceRouteStatusFailed     = "route_failed"
	MarketplaceRouteStatusRiskPaused = "route_risk_paused"
	MarketplaceRouteStatusExhausted  = "route_exhausted"
	MarketplaceRouteStatusBusy       = "route_busy"
)

const (
	MarketplaceFixedOrderStatusActive    = "active"
	MarketplaceFixedOrderStatusExhausted = "exhausted"
	MarketplaceFixedOrderStatusExpired   = "expired"
	MarketplaceFixedOrderStatusSuspended = "suspended"
	MarketplaceFixedOrderStatusRefunded  = "refunded"

	MarketplaceSettlementStatusPending   = "pending"
	MarketplaceSettlementStatusAvailable = "available"
	MarketplaceSettlementStatusWithdrawn = "withdrawn"
	MarketplaceSettlementStatusBlocked   = "blocked"
	MarketplaceSettlementStatusReversed  = "reversed"
)

const (
	MarketplaceFillStatusSucceeded = "succeeded"
	MarketplaceFillStatusFailed    = "failed"
)

type MarketplaceCredential struct {
	ID                 int     `json:"id"`
	SellerUserID       int     `json:"seller_user_id" gorm:"not null;index"`
	VendorType         int     `json:"vendor_type" gorm:"not null;index"`
	VendorNameSnapshot string  `json:"vendor_name_snapshot" gorm:"type:varchar(64);not null;default:''"`
	EncryptedAPIKey    string  `json:"-" gorm:"column:encrypted_api_key;type:text;not null"`
	KeyFingerprint     string  `json:"key_fingerprint" gorm:"type:varchar(128);not null;index"`
	OpenAIOrganization string  `json:"openai_organization" gorm:"type:varchar(255);not null;default:''"`
	TestModel          string  `json:"test_model" gorm:"type:varchar(255);not null;default:''"`
	BaseURL            string  `json:"base_url" gorm:"column:base_url;type:varchar(1024);not null;default:''"`
	Other              string  `json:"other" gorm:"type:text"`
	ModelMapping       string  `json:"model_mapping" gorm:"type:text"`
	StatusCodeMapping  string  `json:"status_code_mapping" gorm:"type:varchar(1024);not null;default:''"`
	Setting            string  `json:"setting" gorm:"type:text"`
	ParamOverride      string  `json:"param_override" gorm:"type:text"`
	HeaderOverride     string  `json:"header_override" gorm:"type:text"`
	OtherSettings      string  `json:"settings" gorm:"column:settings;type:text"`
	Models             string  `json:"models" gorm:"type:text;not null"`
	QuotaMode          string  `json:"quota_mode" gorm:"type:varchar(32);not null;default:'unlimited';index"`
	QuotaLimit         int64   `json:"quota_limit" gorm:"bigint;not null;default:0"`
	TimeMode           string  `json:"time_mode" gorm:"type:varchar(32);not null;default:'unlimited';index"`
	TimeLimitSeconds   int64   `json:"time_limit_seconds" gorm:"bigint;not null;default:0"`
	Multiplier         float64 `json:"multiplier" gorm:"type:decimal(10,4);not null;default:1"`
	ConcurrencyLimit   int     `json:"concurrency_limit" gorm:"not null;default:1"`
	ListingStatus      string  `json:"listing_status" gorm:"type:varchar(32);not null;default:'listed';index"`
	ServiceStatus      string  `json:"service_status" gorm:"type:varchar(32);not null;default:'enabled';index"`
	HealthStatus       string  `json:"health_status" gorm:"type:varchar(32);not null;default:'untested';index"`
	CapacityStatus     string  `json:"capacity_status" gorm:"type:varchar(32);not null;default:'available';index"`
	RiskStatus         string  `json:"risk_status" gorm:"type:varchar(32);not null;default:'normal';index"`
	ResponseTime       int     `json:"response_time" gorm:"not null;default:0"`
	TestTime           int64   `json:"test_time" gorm:"bigint;not null;default:0"`
	CreatedAt          int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

type MarketplaceCredentialStats struct {
	CredentialID           int    `json:"credential_id" gorm:"primaryKey"`
	CurrentConcurrency     int    `json:"current_concurrency" gorm:"not null;default:0"`
	TotalRequestCount      int64  `json:"total_request_count" gorm:"bigint;not null;default:0"`
	PoolRequestCount       int64  `json:"pool_request_count" gorm:"bigint;not null;default:0"`
	FixedOrderRequestCount int64  `json:"fixed_order_request_count" gorm:"bigint;not null;default:0"`
	TotalOfficialCost      int64  `json:"total_official_cost" gorm:"bigint;not null;default:0"`
	QuotaUsed              int64  `json:"quota_used" gorm:"bigint;not null;default:0"`
	FixedOrderSoldQuota    int64  `json:"fixed_order_sold_quota" gorm:"bigint;not null;default:0"`
	ActiveFixedOrderCount  int64  `json:"active_fixed_order_count" gorm:"bigint;not null;default:0"`
	SuccessCount           int64  `json:"success_count" gorm:"bigint;not null;default:0"`
	UpstreamErrorCount     int64  `json:"upstream_error_count" gorm:"bigint;not null;default:0"`
	TimeoutCount           int64  `json:"timeout_count" gorm:"bigint;not null;default:0"`
	RateLimitCount         int64  `json:"rate_limit_count" gorm:"bigint;not null;default:0"`
	PlatformErrorCount     int64  `json:"platform_error_count" gorm:"bigint;not null;default:0"`
	AvgLatencyMS           int64  `json:"avg_latency_ms" gorm:"bigint;not null;default:0"`
	LastSuccessAt          int64  `json:"last_success_at" gorm:"bigint;not null;default:0"`
	LastFailedAt           int64  `json:"last_failed_at" gorm:"bigint;not null;default:0"`
	LastFailedReason       string `json:"last_failed_reason" gorm:"type:varchar(255);not null;default:''"`
	UpdatedAt              int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type MarketplaceFixedOrder struct {
	ID                      int     `json:"id"`
	BuyerUserID             int     `json:"buyer_user_id" gorm:"not null;index"`
	SellerUserID            int     `json:"seller_user_id" gorm:"not null;index"`
	CredentialID            int     `json:"credential_id" gorm:"not null;index"`
	PurchasedQuota          int64   `json:"purchased_quota" gorm:"bigint;not null;default:0"`
	RemainingQuota          int64   `json:"remaining_quota" gorm:"bigint;not null;default:0"`
	SpentQuota              int64   `json:"spent_quota" gorm:"bigint;not null;default:0"`
	ExpiredQuota            int64   `json:"expired_quota" gorm:"bigint;not null;default:0"`
	MultiplierSnapshot      float64 `json:"multiplier_snapshot" gorm:"type:decimal(10,4);not null;default:1"`
	OfficialPriceSnapshot   string  `json:"official_price_snapshot" gorm:"type:text"`
	BuyerPriceSnapshot      string  `json:"buyer_price_snapshot" gorm:"type:text"`
	PlatformFeeRateSnapshot float64 `json:"platform_fee_rate_snapshot" gorm:"type:decimal(10,6);not null;default:0"`
	ExpiresAt               int64   `json:"expires_at" gorm:"bigint;not null;default:0;index"`
	Status                  string  `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	CreatedAt               int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

type MarketplaceFixedOrderFill struct {
	ID                 int     `json:"id"`
	RequestID          string  `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	FixedOrderID       int     `json:"fixed_order_id" gorm:"not null;index"`
	BuyerUserID        int     `json:"buyer_user_id" gorm:"not null;index"`
	SellerUserID       int     `json:"seller_user_id" gorm:"not null;index"`
	CredentialID       int     `json:"credential_id" gorm:"not null;index"`
	Model              string  `json:"model" gorm:"type:varchar(128);not null;index"`
	OfficialCost       int64   `json:"official_cost" gorm:"bigint;not null;default:0"`
	MultiplierSnapshot float64 `json:"multiplier_snapshot" gorm:"type:decimal(10,4);not null;default:1"`
	BuyerCharge        int64   `json:"buyer_charge" gorm:"bigint;not null;default:0"`
	Status             string  `json:"status" gorm:"type:varchar(32);not null;index"`
	LatencyMS          int64   `json:"latency_ms" gorm:"bigint;not null;default:0"`
	ErrorCode          string  `json:"error_code" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt          int64   `json:"created_at" gorm:"autoCreateTime;index"`
}

type MarketplacePoolFill struct {
	ID                 int     `json:"id"`
	RequestID          string  `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	BuyerUserID        int     `json:"buyer_user_id" gorm:"not null;index"`
	SellerUserID       int     `json:"seller_user_id" gorm:"not null;index"`
	CredentialID       int     `json:"credential_id" gorm:"not null;index"`
	Model              string  `json:"model" gorm:"type:varchar(128);not null;index"`
	OfficialCost       int64   `json:"official_cost" gorm:"bigint;not null;default:0"`
	MultiplierSnapshot float64 `json:"multiplier_snapshot" gorm:"type:decimal(10,4);not null;default:1"`
	BuyerCharge        int64   `json:"buyer_charge" gorm:"bigint;not null;default:0"`
	Status             string  `json:"status" gorm:"type:varchar(32);not null;index"`
	LatencyMS          int64   `json:"latency_ms" gorm:"bigint;not null;default:0"`
	ErrorCode          string  `json:"error_code" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt          int64   `json:"created_at" gorm:"autoCreateTime;index"`
}

type MarketplaceSettlement struct {
	ID                      int     `json:"id"`
	RequestID               string  `json:"request_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	BuyerUserID             int     `json:"buyer_user_id" gorm:"not null;index"`
	SellerUserID            int     `json:"seller_user_id" gorm:"not null;index"`
	CredentialID            int     `json:"credential_id" gorm:"not null;index"`
	SourceType              string  `json:"source_type" gorm:"type:varchar(64);not null;index"`
	SourceID                string  `json:"source_id" gorm:"type:varchar(64);not null;index"`
	BuyerCharge             int64   `json:"buyer_charge" gorm:"bigint;not null;default:0"`
	PlatformFee             int64   `json:"platform_fee" gorm:"bigint;not null;default:0"`
	PlatformFeeRateSnapshot float64 `json:"platform_fee_rate_snapshot" gorm:"type:decimal(10,6);not null;default:0"`
	SellerIncome            int64   `json:"seller_income" gorm:"bigint;not null;default:0"`
	OfficialCost            int64   `json:"official_cost" gorm:"bigint;not null;default:0"`
	MultiplierSnapshot      float64 `json:"multiplier_snapshot" gorm:"type:decimal(10,4);not null;default:1"`
	Status                  string  `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	AvailableAt             int64   `json:"available_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt               int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

func marketplaceMigrationModels() []interface{} {
	return []interface{}{
		&MarketplaceCredential{},
		&MarketplaceCredentialStats{},
		&MarketplaceFixedOrder{},
		&MarketplaceFixedOrderFill{},
		&MarketplacePoolFill{},
		&MarketplaceSettlement{},
	}
}
