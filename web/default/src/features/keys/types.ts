import { z } from 'zod'

// ============================================================================
// API Key Schema & Types
// ============================================================================

export const MARKETPLACE_ROUTE_ORDER_VALUES = [
  'fixed_order',
  'group',
  'pool',
] as const

export type MarketplaceRouteOrderItem =
  (typeof MARKETPLACE_ROUTE_ORDER_VALUES)[number]

export const DEFAULT_MARKETPLACE_ROUTE_ORDER: MarketplaceRouteOrderItem[] = [
  'fixed_order',
  'group',
  'pool',
]

export const DEFAULT_MARKETPLACE_ROUTE_ENABLED: MarketplaceRouteOrderItem[] = [
  'fixed_order',
  'group',
  'pool',
]

const MARKETPLACE_ROUTE_ALIASES: Record<string, MarketplaceRouteOrderItem> = {
  fixed_order: 'fixed_order',
  marketplace_fixed_order: 'fixed_order',
  fixed: 'fixed_order',
  order: 'fixed_order',
  group: 'group',
  normal_group: 'group',
  ordinary_group: 'group',
  channel: 'group',
  pool: 'pool',
  marketplace_pool: 'pool',
  order_pool: 'pool',
}

export function normalizeMarketplaceRouteOrder(
  value: unknown
): MarketplaceRouteOrderItem[] {
  const rawRoutes = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : []
  const seen = new Set<MarketplaceRouteOrderItem>()
  const normalized: MarketplaceRouteOrderItem[] = []

  rawRoutes.forEach((rawRoute) => {
    const route = MARKETPLACE_ROUTE_ALIASES[String(rawRoute).trim()]
    if (!route || seen.has(route)) return
    seen.add(route)
    normalized.push(route)
  })

  DEFAULT_MARKETPLACE_ROUTE_ORDER.forEach((route) => {
    if (seen.has(route)) return
    normalized.push(route)
  })

  return normalized
}

export function normalizeMarketplaceRouteEnabled(
  value: unknown
): MarketplaceRouteOrderItem[] {
  if (value == null) return [...DEFAULT_MARKETPLACE_ROUTE_ENABLED]

  const rawRoutes = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : []
  const seen = new Set<MarketplaceRouteOrderItem>()
  const normalized: MarketplaceRouteOrderItem[] = []

  rawRoutes.forEach((rawRoute) => {
    const route = MARKETPLACE_ROUTE_ALIASES[String(rawRoute).trim()]
    if (!route || seen.has(route)) return
    seen.add(route)
    normalized.push(route)
  })

  return normalized
}

const marketplaceRouteOrderItemSchema = z.enum(MARKETPLACE_ROUTE_ORDER_VALUES)

const marketplacePoolFiltersSchema = z.object({
  vendor_type: z.number().optional(),
  model: z.string().optional(),
  quota_mode: z.enum(['unlimited', 'limited']).or(z.literal('')).optional(),
  time_mode: z.enum(['unlimited', 'limited']).or(z.literal('')).optional(),
  min_quota_limit: z.number().optional(),
  max_quota_limit: z.number().optional(),
  min_time_limit_seconds: z.number().optional(),
  max_time_limit_seconds: z.number().optional(),
  min_multiplier: z.number().optional(),
  max_multiplier: z.number().optional(),
  min_concurrency_limit: z.number().optional(),
  max_concurrency_limit: z.number().optional(),
})

export const apiKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  key: z.string(),
  status: z.number(), // 1: enabled, 2: disabled, 3: expired, 4: exhausted
  remain_quota: z.number(),
  used_quota: z.number(),
  unlimited_quota: z.boolean(),
  expired_time: z.number(), // -1 for never expires
  created_time: z.number(),
  accessed_time: z.number(),
  group: z.string().nullish().default(''),
  cross_group_retry: z
    .preprocess((v) => {
      if (v === 1) return true
      if (v === 0) return false
      return v
    }, z.boolean())
    .optional()
    .default(false),
  model_limits_enabled: z.boolean(),
  model_limits: z.string().nullish().default(''),
  allow_ips: z.string().nullish().default(''),
  marketplace_fixed_order_id: z.number().nullish().default(0),
  marketplace_fixed_order_ids: z
    .preprocess((value) => {
      if (Array.isArray(value)) return value
      if (typeof value === 'string') {
        return value
          .split(',')
          .map((item) => Number(item.trim()))
          .filter((item) => Number.isFinite(item) && item > 0)
      }
      return []
    }, z.array(z.number()))
    .default([]),
  marketplace_route_order: z
    .preprocess(
      normalizeMarketplaceRouteOrder,
      z.array(marketplaceRouteOrderItemSchema)
    )
    .default(() => [...DEFAULT_MARKETPLACE_ROUTE_ORDER]),
  marketplace_route_enabled: z
    .preprocess(
      normalizeMarketplaceRouteEnabled,
      z.array(marketplaceRouteOrderItemSchema)
    )
    .default(() => [...DEFAULT_MARKETPLACE_ROUTE_ENABLED]),
  marketplace_pool_filters_enabled: z.boolean().optional().default(false),
  marketplace_pool_filters: z
    .preprocess((value) => {
      if (value && typeof value === 'object') return value
      if (typeof value === 'string' && value.trim()) {
        try {
          return JSON.parse(value)
        } catch {
          return {}
        }
      }
      return {}
    }, marketplacePoolFiltersSchema)
    .default({}),
})

export type ApiKey = z.infer<typeof apiKeySchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetApiKeysParams {
  p?: number
  size?: number
}

export interface GetApiKeysResponse {
  success: boolean
  message?: string
  data?: {
    items: ApiKey[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchApiKeysParams {
  keyword?: string
  token?: string
  p?: number
  size?: number
}

export interface ApiKeyFormData {
  name: string
  remain_quota: number
  expired_time: number
  unlimited_quota: boolean
  model_limits_enabled: boolean
  model_limits: string
  allow_ips: string
  group: string
  cross_group_retry: boolean
  marketplace_fixed_order_id: number
  marketplace_fixed_order_ids?: number[]
  marketplace_route_order?: MarketplaceRouteOrderItem[]
  marketplace_route_enabled?: MarketplaceRouteOrderItem[]
}

export interface ApiKeyMarketplaceFixedOrder {
  id: number
  credential_id: number
  remaining_quota: number
  spent_quota: number
  status: string
  expires_at: number
}

export interface GetApiKeyMarketplaceFixedOrdersResponse {
  success: boolean
  message?: string
  data?: {
    items: ApiKeyMarketplaceFixedOrder[]
    total: number
    page: number
    page_size: number
  }
}

// ============================================================================
// Dialog Types
// ============================================================================

export type ApiKeysDialogType =
  | 'create'
  | 'update'
  | 'delete'
  | 'batch-delete'
  | 'cc-switch'
