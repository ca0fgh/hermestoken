export type ApiResponse<T = unknown> = {
  success?: boolean
  message?: string
  data?: T
}

export type MarketplacePage<T> = {
  items: T[]
  total: number
  page?: number
  page_size?: number
}

export type MarketplaceQuotaMode = 'unlimited' | 'limited'
export type MarketplaceTimeMode = 'unlimited' | 'limited'
export type MarketplaceListingStatus = 'listed' | 'unlisted'
export type MarketplaceServiceStatus = 'enabled' | 'disabled'
export type MarketplaceHealthStatus =
  | 'untested'
  | 'healthy'
  | 'degraded'
  | 'failed'
export type MarketplaceCapacityStatus = 'available' | 'busy' | 'exhausted'
export type MarketplaceRiskStatus = 'normal' | 'watching' | 'risk_paused'
export type MarketplaceRouteStatus =
  | 'route_available'
  | 'route_unlisted'
  | 'route_disabled'
  | 'route_failed'
  | 'route_risk_paused'
  | 'route_exhausted'
  | 'route_busy'
export type MarketplaceFixedOrderStatus =
  | 'active'
  | 'exhausted'
  | 'expired'
  | 'suspended'
  | 'refunded'
export type MarketplaceSettlementStatus =
  | 'pending'
  | 'available'
  | 'withdrawn'
  | 'blocked'
  | 'reversed'

export type MarketplacePricePoint = {
  quota_type: 'price' | 'ratio' | 'tiered_expr' | 'per_second' | string
  billing_mode?:
    | 'per_request'
    | 'metered'
    | 'tiered_expr'
    | 'per_second'
    | string
  billing_expr?: string
  model_price?: number
  model_ratio?: number
  completion_ratio?: number
  cache_ratio?: number | null
  create_cache_ratio?: number | null
  input_price_per_mtok?: number
  output_price_per_mtok?: number
  cache_read_price_per_mtok?: number | null
  cache_write_price_per_mtok?: number | null
  task_per_request_price?: number
  task_per_second_price?: number
  applied_multiplier?: number
  configured: boolean
}

export type MarketplacePricePreview = {
  model: string
  official: MarketplacePricePoint
  buyer: MarketplacePricePoint
  multiplier: number
}

export type MarketplacePricingItem = {
  id?: string
  model_name: string
  quota_type: number | string
  billing_mode?: string
  billing_expr?: string
  model_price?: number
  model_ratio?: number
  completion_ratio?: number
  cache_ratio?: number | null
  create_cache_ratio?: number | null
  input_price_per_mtok?: number
  output_price_per_mtok?: number
  cache_read_price_per_mtok?: number | null
  cache_write_price_per_mtok?: number | null
  task_per_request_price?: number
  task_per_second_price?: number
  configured?: boolean
}

export type MarketplaceCredential = {
  id: number
  seller_user_id: number
  vendor_type: number
  vendor_name_snapshot: string
  openai_organization: string
  test_model: string
  base_url: string
  other: string
  model_mapping: string
  status_code_mapping: string
  setting: string
  param_override: string
  header_override: string
  settings: string
  models: string
  quota_mode: MarketplaceQuotaMode
  quota_limit: number
  time_mode: MarketplaceTimeMode
  time_limit_seconds: number
  multiplier: number
  concurrency_limit: number
  listing_status: MarketplaceListingStatus
  service_status: MarketplaceServiceStatus
  health_status: MarketplaceHealthStatus
  capacity_status: MarketplaceCapacityStatus
  route_status: MarketplaceRouteStatus
  risk_status: MarketplaceRiskStatus
  fixed_order_sold_quota?: number
  created_at: number
  updated_at: number
}

export type MarketplaceOrderListItem = MarketplaceCredential & {
  current_concurrency: number
  total_request_count: number
  pool_request_count: number
  fixed_order_request_count: number
  quota_used: number
  fixed_order_sold_quota: number
  active_fixed_order_count: number
  success_count: number
  upstream_error_count: number
  timeout_count: number
  rate_limit_count: number
  platform_error_count: number
  avg_latency_ms: number
  last_success_at: number
  last_failed_at: number
  last_failed_reason: string
  price_preview: MarketplacePricePreview[]
}

export type MarketplaceOrderFilterRanges = {
  unlimited_quota_count: number
  limited_quota_count: number
  min_quota_limit: number
  max_quota_limit: number
  unlimited_time_count: number
  limited_time_count: number
  min_time_limit_seconds: number
  max_time_limit_seconds: number
  min_multiplier: number
  max_multiplier: number
  min_concurrency_limit: number
  max_concurrency_limit: number
}

export type MarketplacePoolModel = {
  vendor_type: number
  vendor_name_snapshot: string
  model: string
  candidate_count: number
  lowest_multiplier: number
  lowest_price_preview: MarketplacePricePreview
}

export type MarketplacePoolCandidate = {
  credential: MarketplaceOrderListItem
  route_score: number
  success_rate: number
  load_ratio: number
}

export type MarketplaceFixedOrder = {
  id: number
  buyer_user_id: number
  seller_user_id: number
  credential_id: number
  purchased_quota: number
  remaining_quota: number
  spent_quota: number
  expired_quota: number
  multiplier_snapshot: number
  official_price_snapshot: string
  buyer_price_snapshot: string
  platform_fee_rate_snapshot: number
  expires_at: number
  status: MarketplaceFixedOrderStatus
  created_at: number
  updated_at: number
}

export type MarketplaceSettlement = {
  id: number
  request_id: string
  buyer_user_id: number
  seller_user_id: number
  credential_id: number
  source_type: 'fixed_order_fill' | 'fixed_order_final' | 'pool_fill' | string
  source_id: string
  buyer_charge: number
  platform_fee: number
  platform_fee_rate_snapshot: number
  seller_income: number
  official_cost: number
  multiplier_snapshot: number
  status: MarketplaceSettlementStatus
  available_at: number
  created_at: number
  updated_at: number
}

export type MarketplaceIncomeSummary = {
  pending_income: number
  available_income: number
  blocked_income: number
  reversed_income: number
  withdrawn_income: number
  total_seller_income: number
  settlement_count: number
}

export type MarketplaceSettlementReleaseResult = {
  released_count: number
  released_income: number
}

export type MarketplaceCredentialFormValues = {
  vendor_type: number
  api_key: string
  base_url: string
  proxy: string
  other: string
  model_mapping: string
  status_code_mapping: string
  setting: string
  param_override: string
  settings: string
  models: string
  quota_mode: MarketplaceQuotaMode
  quota_limit: number
  time_mode: MarketplaceTimeMode
  time_limit_minutes: number
  multiplier: number
  concurrency_limit: number
}

export type MarketplaceOrderFilters = {
  vendor_type?: number
  model?: string
  quota_mode?: MarketplaceQuotaMode | ''
  time_mode?: MarketplaceTimeMode | ''
  min_quota_limit?: number | ''
  max_quota_limit?: number | ''
  min_time_limit_seconds?: number | ''
  max_time_limit_seconds?: number | ''
  min_multiplier?: number | ''
  max_multiplier?: number | ''
  min_concurrency_limit?: number | ''
  max_concurrency_limit?: number | ''
  p?: number
  page_size?: number
}
