import { formatQuotaWithCurrency, getCurrencyDisplay } from '@/lib/currency'
import type { ApiKey } from '@/features/keys/types'
import type {
  MarketplaceCapacityStatus,
  MarketplaceCredentialFormValues,
  MarketplaceFixedOrderStatus,
  MarketplaceFixedOrder,
  MarketplaceHealthStatus,
  MarketplaceListingStatus,
  MarketplaceOrderFilters,
  MarketplaceOrderListItem,
  MarketplacePricePoint,
  MarketplaceRiskStatus,
  MarketplaceRouteStatus,
  MarketplaceServiceStatus,
  MarketplaceSettlementStatus,
} from './types'

export const MARKETPLACE_FIXED_ORDER_HEADER = 'X-Marketplace-Fixed-Order-Id'
export const MARKETPLACE_UNIFIED_RELAY_ENDPOINT = '/v1/chat/completions'
export const MARKETPLACE_FIXED_RELAY_ENDPOINT =
  MARKETPLACE_UNIFIED_RELAY_ENDPOINT
export const MARKETPLACE_POOL_RELAY_ENDPOINT =
  MARKETPLACE_UNIFIED_RELAY_ENDPOINT

export function marketplaceRelayBaseURL(serverAddress: string) {
  const trimmed = serverAddress.trim().replace(/\/+$/, '')
  if (!trimmed) return '/v1'
  return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
}

export function splitMarketplaceModels(models: string): string[] {
  return models
    .split(',')
    .map((model) => model.trim())
    .filter(Boolean)
}

function parseMarketplaceCredentialSetting(setting?: string | null) {
  const trimmed = String(setting || '').trim()
  if (!trimmed) {
    return { valid: true, value: {} as Record<string, unknown>, raw: '' }
  }
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return {
        valid: true,
        value: parsed as Record<string, unknown>,
        raw: trimmed,
      }
    }
  } catch {
    return { valid: false, value: {} as Record<string, unknown>, raw: trimmed }
  }
  return { valid: false, value: {} as Record<string, unknown>, raw: trimmed }
}

export function buildMarketplaceCredentialSetting(
  values: Pick<MarketplaceCredentialFormValues, 'setting' | 'proxy'>
) {
  const parsed = parseMarketplaceCredentialSetting(values.setting)
  if (!parsed.valid) return parsed.raw
  return JSON.stringify({
    ...parsed.value,
    proxy: values.proxy.trim(),
  })
}

export function marketplaceCredentialProxy(setting?: string | null) {
  const parsed = parseMarketplaceCredentialSetting(setting)
  return typeof parsed.value.proxy === 'string' ? parsed.value.proxy : ''
}

export function marketplaceCredentialSettingWithoutProxy(
  setting?: string | null
) {
  const parsed = parseMarketplaceCredentialSetting(setting)
  if (!parsed.valid) return parsed.raw
  const { proxy: _proxy, ...settingWithoutProxy } = parsed.value
  return Object.keys(settingWithoutProxy).length > 0
    ? JSON.stringify(settingWithoutProxy)
    : ''
}

export function formatMarketplacePricePoint(point?: MarketplacePricePoint) {
  if (!point) return '-'
  if (!point.configured) return '未配置'
  if (point.quota_type === 'tiered_expr') {
    return point.billing_expr ? '表达式/阶梯计费：已配置' : '表达式/阶梯计费'
  }
  if (point.quota_type === 'per_second') {
    const parts = ['按秒计费']
    if (Number(point.task_per_request_price) > 0) {
      parts.push(
        `基础 ${formatMarketplaceUSD(point.task_per_request_price)}/次`
      )
    }
    if (Number(point.task_per_second_price) > 0) {
      parts.push(`${formatMarketplaceUSD(point.task_per_second_price)}/秒`)
    }
    return parts.join(' · ')
  }
  if (point.quota_type === 'price') {
    return `按次计费：${formatMarketplaceUSD(point.model_price)}/次`
  }

  const parts = [
    '按量计费',
    `输入 ${formatMarketplaceUSD(point.input_price_per_mtok)}/1M tokens`,
  ]
  if (Number.isFinite(Number(point.output_price_per_mtok))) {
    parts.push(
      `输出 ${formatMarketplaceUSD(point.output_price_per_mtok)}/1M tokens`
    )
  }
  if (point.cache_read_price_per_mtok != null) {
    parts.push(
      `缓存读 ${formatMarketplaceUSD(point.cache_read_price_per_mtok)}/1M tokens`
    )
  }
  if (point.cache_write_price_per_mtok != null) {
    parts.push(
      `缓存写 ${formatMarketplaceUSD(point.cache_write_price_per_mtok)}/1M tokens`
    )
  }
  return parts.join(' · ')
}

export function formatMarketplaceUSD(value?: number | null) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '$0'
  const fixed = numeric >= 1 ? numeric.toFixed(4) : numeric.toFixed(6)
  return `$${fixed.replace(/\.?0+$/, '')}`
}

export function marketplaceQuotaToDisplayAmount(quota?: number | null) {
  const numeric = Number(quota)
  if (!Number.isFinite(numeric) || numeric <= 0) return 0
  const { config, meta } = getCurrencyDisplay()
  if (meta.kind === 'tokens') return numeric
  return (numeric / config.quotaPerUnit) * meta.exchangeRate
}

export function marketplaceDisplayAmountToQuota(amount?: number | null) {
  const numeric = Number(amount)
  if (!Number.isFinite(numeric) || numeric <= 0) return 0
  const { config, meta } = getCurrencyDisplay()
  if (meta.kind === 'tokens') return Math.round(numeric)
  return Math.round((numeric / meta.exchangeRate) * config.quotaPerUnit)
}

export function marketplaceQuotaDisplayLabel() {
  const { config, meta } = getCurrencyDisplay()
  if (meta.kind === 'tokens') return 'Tokens'
  if (meta.kind === 'custom') return meta.symbol
  return `${config.quotaDisplayType} (${meta.symbol})`
}

export function marketplaceQuotaInputStep() {
  const { meta } = getCurrencyDisplay()
  return meta.kind === 'tokens' ? '1' : '0.01'
}

export function formatMarketplaceQuotaUSD(quota?: number | null) {
  return formatQuotaWithCurrency(quota)
}

export function marketplaceSuccessRate(item: MarketplaceOrderListItem) {
  const failed =
    item.upstream_error_count +
    item.timeout_count +
    item.rate_limit_count +
    item.platform_error_count
  const total = item.success_count + failed
  if (total <= 0) return 100
  return Math.round((item.success_count / total) * 10000) / 100
}

export function marketplaceQuotaText(item: MarketplaceOrderListItem) {
  if (item.quota_mode === 'unlimited') return 'Unlimited'
  return `${formatMarketplaceQuotaUSD(item.quota_used)} / ${formatMarketplaceQuotaUSD(item.quota_limit)}`
}

export function marketplaceFixedOrderUsage(order: MarketplaceFixedOrder) {
  if (order.purchased_quota <= 0) return 0
  return Math.min(
    100,
    Math.round((order.spent_quota / order.purchased_quota) * 10000) / 100
  )
}

export function fixedOrderRelayHeaderSnippet(orderId: number) {
  return `${MARKETPLACE_FIXED_ORDER_HEADER}: ${orderId}`
}

export function marketplaceTokenFixedOrderIds(
  token?: Pick<
    ApiKey,
    'marketplace_fixed_order_id' | 'marketplace_fixed_order_ids'
  > | null
) {
  const ids = token?.marketplace_fixed_order_ids ?? []
  const legacyId = Number(token?.marketplace_fixed_order_id) || 0
  return Array.from(
    new Set(
      [legacyId, ...ids]
        .map((id) => Number(id))
        .filter((id) => Number.isFinite(id) && id > 0)
    )
  )
}

type MarketplaceStatus =
  | MarketplaceListingStatus
  | MarketplaceServiceStatus
  | MarketplaceHealthStatus
  | MarketplaceCapacityStatus
  | MarketplaceRouteStatus
  | MarketplaceRiskStatus
  | MarketplaceFixedOrderStatus
  | MarketplaceSettlementStatus

export function marketplaceStatusLabel(status?: MarketplaceStatus | string) {
  return status ? `Marketplace status ${status}` : 'Unknown'
}

export function marketplaceStatusVariant(
  status:
    | MarketplaceHealthStatus
    | MarketplaceCapacityStatus
    | MarketplaceRouteStatus
    | MarketplaceRiskStatus
    | MarketplaceServiceStatus
    | MarketplaceSettlementStatus
) {
  switch (status) {
    case 'healthy':
    case 'available':
    case 'route_available':
    case 'normal':
    case 'enabled':
      return 'success'
    case 'degraded':
    case 'busy':
    case 'route_busy':
    case 'watching':
    case 'pending':
      return 'warning'
    case 'failed':
    case 'exhausted':
    case 'route_failed':
    case 'route_risk_paused':
    case 'route_exhausted':
    case 'risk_paused':
    case 'disabled':
    case 'blocked':
    case 'reversed':
      return 'danger'
    default:
      return 'neutral'
  }
}

function buildMarketplaceRelayQuery(filters?: MarketplaceOrderFilters) {
  if (!filters) return ''
  const params = new URLSearchParams()
  const keys: (keyof MarketplaceOrderFilters)[] = [
    'vendor_type',
    'quota_mode',
    'time_mode',
    'min_quota_limit',
    'max_quota_limit',
    'min_time_limit_seconds',
    'max_time_limit_seconds',
    'min_multiplier',
    'max_multiplier',
    'min_concurrency_limit',
    'max_concurrency_limit',
  ]
  keys.forEach((key) => {
    const value = filters[key]
    if (value === undefined || value === null || value === '') return
    if (typeof value === 'number' && value <= 0) return
    params.set(key, String(value))
  })
  const query = params.toString()
  return query ? `?${query}` : ''
}

export function buildMarketplaceCurl(
  orderId?: number,
  filters?: MarketplaceOrderFilters,
  token?: string,
  model = 'gpt-4o-mini',
  options: { fixedOrderBound?: boolean } = {}
) {
  const endpoint = orderId
    ? MARKETPLACE_FIXED_RELAY_ENDPOINT
    : `${MARKETPLACE_POOL_RELAY_ENDPOINT}${buildMarketplaceRelayQuery(filters)}`
  const fixedHeader =
    orderId && !options.fixedOrderBound
      ? ` \\\n  -H '${fixedOrderRelayHeaderSnippet(orderId)}'`
      : ''
  const resolvedToken = token || 'YOUR_PLATFORM_TOKEN'
  const resolvedModel = model.trim() || 'gpt-4o-mini'

  return `curl ${endpoint} \\
  -H 'Authorization: Bearer ${resolvedToken}'${fixedHeader} \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"${resolvedModel}","messages":[{"role":"user","content":"hello"}]}'`
}
