import { api } from '@/lib/api'
import type { ApiKey } from '@/features/keys/types'
import {
  buildMarketplaceCredentialSetting,
  marketplaceDisplayAmountToQuota,
} from './lib'
import type {
  ApiResponse,
  MarketplaceCredential,
  MarketplaceCredentialFormValues,
  MarketplaceFixedOrder,
  MarketplaceIncomeSummary,
  MarketplaceOrderFilters,
  MarketplaceOrderFilterRanges,
  MarketplaceOrderListItem,
  MarketplacePage,
  MarketplacePoolCandidate,
  MarketplacePoolModel,
  MarketplacePricingItem,
  MarketplaceSettlement,
  MarketplaceSettlementReleaseResult,
} from './types'

function compactParams(params: MarketplaceOrderFilters = {}) {
  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => {
      if (value === undefined || value === null || value === '') return false
      if (typeof value === 'number' && value <= 0) return false
      return true
    })
  )
}

function compactRangeParams(params: MarketplaceOrderFilters = {}) {
  const rangeParams = { ...params }
  delete rangeParams.quota_mode
  delete rangeParams.time_mode
  delete rangeParams.min_quota_limit
  delete rangeParams.max_quota_limit
  delete rangeParams.min_time_limit_seconds
  delete rangeParams.max_time_limit_seconds
  delete rangeParams.min_multiplier
  delete rangeParams.max_multiplier
  delete rangeParams.min_concurrency_limit
  delete rangeParams.max_concurrency_limit
  delete rangeParams.p
  delete rangeParams.page_size
  return compactParams(rangeParams)
}

function compactRangeFallbackParams(params: MarketplaceOrderFilters = {}) {
  const rangeParams = { ...params, p: 1, page_size: 1000 }
  delete rangeParams.quota_mode
  delete rangeParams.time_mode
  delete rangeParams.min_quota_limit
  delete rangeParams.max_quota_limit
  delete rangeParams.min_time_limit_seconds
  delete rangeParams.max_time_limit_seconds
  delete rangeParams.min_multiplier
  delete rangeParams.max_multiplier
  delete rangeParams.min_concurrency_limit
  delete rangeParams.max_concurrency_limit
  return compactParams(rangeParams)
}

function compactPoolTokenFilterParams(params: MarketplaceOrderFilters = {}) {
  const filterParams = { ...params }
  delete filterParams.p
  delete filterParams.page_size
  return compactParams(filterParams)
}

export async function listMarketplaceOrders(params: MarketplaceOrderFilters) {
  const res = await api.get<
    ApiResponse<MarketplacePage<MarketplaceOrderListItem>>
  >('/api/marketplace/orders', { params: compactParams(params) })
  return res.data
}

export async function listMarketplaceOrderFilterRanges(
  params: MarketplaceOrderFilters
) {
  try {
    const res = await api.get<ApiResponse<MarketplaceOrderFilterRanges>>(
      '/api/marketplace/order-filter-ranges',
      {
        params: compactRangeParams(params),
        skipBusinessError: true,
        skipErrorHandler: true,
      } as Record<string, unknown>
    )
    return res.data
  } catch {
    try {
      const fallbackRes = await api.get<
        ApiResponse<MarketplacePage<MarketplaceOrderListItem>>
      >('/api/marketplace/orders', {
        params: compactRangeFallbackParams(params),
        skipBusinessError: true,
        skipErrorHandler: true,
      } as Record<string, unknown>)
      return {
        success: true,
        data: marketplaceFilterRangesFromOrders(
          fallbackRes.data.data?.items ?? []
        ),
      }
    } catch {
      return {
        success: true,
        data: emptyMarketplaceOrderFilterRanges(),
      }
    }
  }
}

function marketplaceFilterRangesFromOrders(
  orders: MarketplaceOrderListItem[]
): MarketplaceOrderFilterRanges {
  return orders.reduce((ranges, order) => {
    const quotaModePatch =
      order.quota_mode === 'unlimited'
        ? { unlimited_quota_count: ranges.unlimited_quota_count + 1 }
        : order.quota_mode === 'limited'
          ? limitedQuotaRangePatch(ranges, Number(order.quota_limit) || 0)
          : {}
    const timeModePatch =
      order.time_mode === 'unlimited'
        ? { unlimited_time_count: ranges.unlimited_time_count + 1 }
        : order.time_mode === 'limited'
          ? limitedTimeRangePatch(ranges, Number(order.time_limit_seconds) || 0)
          : {}
    const multiplierPatch = multiplierRangePatch(
      ranges,
      Number(order.multiplier) || 0
    )
    const concurrencyPatch = concurrencyRangePatch(
      ranges,
      Number(order.concurrency_limit) || 0
    )
    return {
      ...ranges,
      ...quotaModePatch,
      ...timeModePatch,
      ...multiplierPatch,
      ...concurrencyPatch,
    }
  }, emptyMarketplaceOrderFilterRanges())
}

function limitedQuotaRangePatch(
  ranges: MarketplaceOrderFilterRanges,
  quotaLimit: number
) {
  if (quotaLimit <= 0) return {}
  return {
    limited_quota_count: ranges.limited_quota_count + 1,
    min_quota_limit:
      ranges.min_quota_limit > 0
        ? Math.min(ranges.min_quota_limit, quotaLimit)
        : quotaLimit,
    max_quota_limit: Math.max(ranges.max_quota_limit, quotaLimit),
  }
}

function limitedTimeRangePatch(
  ranges: MarketplaceOrderFilterRanges,
  timeLimitSeconds: number
) {
  if (timeLimitSeconds <= 0) return {}
  return {
    limited_time_count: ranges.limited_time_count + 1,
    min_time_limit_seconds:
      ranges.min_time_limit_seconds > 0
        ? Math.min(ranges.min_time_limit_seconds, timeLimitSeconds)
        : timeLimitSeconds,
    max_time_limit_seconds: Math.max(
      ranges.max_time_limit_seconds,
      timeLimitSeconds
    ),
  }
}

function multiplierRangePatch(
  ranges: MarketplaceOrderFilterRanges,
  multiplier: number
) {
  if (multiplier <= 0) return {}
  return {
    min_multiplier:
      ranges.min_multiplier > 0
        ? Math.min(ranges.min_multiplier, multiplier)
        : multiplier,
    max_multiplier: Math.max(ranges.max_multiplier, multiplier),
  }
}

function concurrencyRangePatch(
  ranges: MarketplaceOrderFilterRanges,
  concurrencyLimit: number
) {
  if (concurrencyLimit <= 0) return {}
  return {
    min_concurrency_limit:
      ranges.min_concurrency_limit > 0
        ? Math.min(ranges.min_concurrency_limit, concurrencyLimit)
        : concurrencyLimit,
    max_concurrency_limit: Math.max(
      ranges.max_concurrency_limit,
      concurrencyLimit
    ),
  }
}

function emptyMarketplaceOrderFilterRanges(): MarketplaceOrderFilterRanges {
  return {
    unlimited_quota_count: 0,
    limited_quota_count: 0,
    min_quota_limit: 0,
    max_quota_limit: 0,
    unlimited_time_count: 0,
    limited_time_count: 0,
    min_time_limit_seconds: 0,
    max_time_limit_seconds: 0,
    min_multiplier: 0,
    max_multiplier: 0,
    min_concurrency_limit: 0,
    max_concurrency_limit: 0,
  }
}

export async function createMarketplaceFixedOrder(request: {
  credential_id: number
  purchased_quota?: number
  purchased_amount_usd?: number
}) {
  const res = await api.post<ApiResponse<MarketplaceFixedOrder>>(
    '/api/marketplace/fixed-orders',
    request
  )
  return res.data
}

export async function listMarketplaceFixedOrders(params: {
  p?: number
  page_size?: number
}) {
  const res = await api.get<
    ApiResponse<MarketplacePage<MarketplaceFixedOrder>>
  >('/api/marketplace/fixed-orders', { params: compactParams(params) })
  return res.data
}

export async function probeMarketplaceFixedOrder(fixedOrderId: number) {
  const res = await api.post<ApiResponse<MarketplaceFixedOrder>>(
    `/api/marketplace/fixed-orders/${fixedOrderId}/probe`,
    undefined,
    {
      skipBusinessError: true,
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  return res.data
}

export async function releaseMarketplaceFixedOrder(fixedOrderId: number) {
  const res = await api.post<ApiResponse<MarketplaceFixedOrder>>(
    `/api/marketplace/fixed-orders/${fixedOrderId}/release`,
    undefined,
    {
      skipBusinessError: true,
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  return res.data
}

export async function bindMarketplaceFixedOrderToken(request: {
  token_id: number
  fixed_order_ids: number[]
}) {
  const res = await api.post<ApiResponse<ApiKey>>(
    '/api/marketplace/fixed-orders/bind-token',
    request
  )
  return res.data
}

export async function bindMarketplaceFixedOrderTokens(
  fixedOrderId: number,
  request: {
    token_ids: number[]
  }
) {
  const res = await api.post<
    ApiResponse<{
      fixed_order_id: number
      token_ids: number[]
      tokens: ApiKey[]
    }>
  >(`/api/marketplace/fixed-orders/${fixedOrderId}/bind-tokens`, request)
  return res.data
}

export async function listMarketplacePoolModels(
  params: MarketplaceOrderFilters
) {
  const res = await api.get<ApiResponse<MarketplacePoolModel[]>>(
    '/api/marketplace/pool/models',
    { params: compactParams(params) }
  )
  return res.data
}

export async function listMarketplacePoolCandidates(
  params: MarketplaceOrderFilters
) {
  const res = await api.get<
    ApiResponse<MarketplacePage<MarketplacePoolCandidate>>
  >('/api/marketplace/pool/candidates', { params: compactParams(params) })
  return res.data
}

export async function saveMarketplacePoolTokenFilters(request: {
  token_id: number
  filters: MarketplaceOrderFilters
}) {
  const res = await api.post<ApiResponse<ApiKey>>(
    '/api/marketplace/pool/token-filters',
    {
      token_id: request.token_id,
      filters: compactPoolTokenFilterParams(request.filters),
    }
  )
  return res.data
}

export async function listSellerMarketplaceCredentials(params: {
  p?: number
  page_size?: number
}) {
  const res = await api.get<
    ApiResponse<MarketplacePage<MarketplaceCredential>>
  >('/api/marketplace/seller/credentials', { params: compactParams(params) })
  return res.data
}

export async function listMarketplacePricing() {
  const res = await api.get<ApiResponse<MarketplacePricingItem[]>>(
    '/api/marketplace/pricing'
  )
  return res.data
}

export async function listSellerMarketplacePricedModels() {
  const res = await api.get<ApiResponse<MarketplacePricingItem[]>>(
    '/api/marketplace/seller/priced-models'
  )
  return res.data
}

function normalizeCredentialPayload(values: MarketplaceCredentialFormValues) {
  return {
    vendor_type: values.vendor_type,
    api_key: values.api_key.trim(),
    base_url: values.base_url.trim(),
    other: values.other.trim(),
    model_mapping: values.model_mapping.trim(),
    status_code_mapping: values.status_code_mapping.trim(),
    setting: buildMarketplaceCredentialSetting(values),
    param_override: values.param_override.trim(),
    settings: values.settings.trim(),
    models: values.models
      .split(',')
      .map((model) => model.trim())
      .filter(Boolean),
    quota_mode: values.quota_mode,
    quota_limit:
      values.quota_mode === 'limited'
        ? marketplaceDisplayAmountToQuota(values.quota_limit)
        : 0,
    time_mode: values.time_mode,
    time_limit_seconds:
      values.time_mode === 'limited'
        ? Math.max(0, values.time_limit_minutes) * 60
        : 0,
    multiplier: values.multiplier,
    concurrency_limit: values.concurrency_limit,
  }
}

export async function createSellerMarketplaceCredential(
  values: MarketplaceCredentialFormValues
) {
  const res = await api.post<ApiResponse<MarketplaceCredential>>(
    '/api/marketplace/seller/credentials',
    normalizeCredentialPayload(values)
  )
  return res.data
}

export async function updateSellerMarketplaceCredential(
  credentialId: number,
  values: MarketplaceCredentialFormValues
) {
  const payload = normalizeCredentialPayload(values)
  const res = await api.put<ApiResponse<MarketplaceCredential>>(
    `/api/marketplace/seller/credentials/${credentialId}`,
    payload
  )
  return res.data
}

export async function setSellerMarketplaceCredentialListed(
  credentialId: number,
  listed: boolean
) {
  const action = listed ? 'list' : 'unlist'
  const res = await api.post<ApiResponse<MarketplaceCredential>>(
    `/api/marketplace/seller/credentials/${credentialId}/${action}`
  )
  return res.data
}

export async function setSellerMarketplaceCredentialEnabled(
  credentialId: number,
  enabled: boolean
) {
  const action = enabled ? 'enable' : 'disable'
  const res = await api.post<ApiResponse<MarketplaceCredential>>(
    `/api/marketplace/seller/credentials/${credentialId}/${action}`
  )
  return res.data
}

export async function testSellerMarketplaceCredential(credentialId: number) {
  const res = await api.post<ApiResponse<{ status: string }>>(
    `/api/marketplace/seller/credentials/${credentialId}/test`
  )
  return res.data
}

export async function probeSellerMarketplaceCredential(credentialId: number) {
  const res = await api.post<ApiResponse<MarketplaceCredential>>(
    `/api/marketplace/seller/credentials/${credentialId}/probe`,
    undefined,
    {
      skipBusinessError: true,
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  return res.data
}

export async function getSellerMarketplaceIncome() {
  const res = await api.get<ApiResponse<MarketplaceIncomeSummary>>(
    '/api/marketplace/seller/income'
  )
  return res.data
}

export async function listSellerMarketplaceSettlements(params: {
  status?: string
  source_type?: string
  credential_id?: number
  p?: number
  page_size?: number
}) {
  const res = await api.get<
    ApiResponse<MarketplacePage<MarketplaceSettlement>>
  >('/api/marketplace/seller/settlements', { params: compactParams(params) })
  return res.data
}

export async function releaseSellerMarketplaceSettlements() {
  const res = await api.post<ApiResponse<MarketplaceSettlementReleaseResult>>(
    '/api/marketplace/seller/settlements/release'
  )
  return res.data
}
