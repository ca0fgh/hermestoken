import { z } from 'zod'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import { DEFAULT_GROUP } from '../constants'
import {
  DEFAULT_MARKETPLACE_ROUTE_ENABLED,
  DEFAULT_MARKETPLACE_ROUTE_ORDER,
  MARKETPLACE_ROUTE_ORDER_VALUES,
  normalizeMarketplaceRouteEnabled,
  normalizeMarketplaceRouteOrder,
  type ApiKeyFormData,
  type ApiKey,
} from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export const apiKeyFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  remain_quota_dollars: z.number().min(0).optional(),
  expired_time: z.date().optional(),
  unlimited_quota: z.boolean(),
  model_limits: z.array(z.string()),
  allow_ips: z.string().optional(),
  group: z.string().optional(),
  cross_group_retry: z.boolean().optional(),
  marketplace_fixed_order_id: z.number().int().min(0).optional(),
  marketplace_fixed_order_ids: z.array(z.number().int().min(1)).optional(),
  marketplace_route_order: z
    .array(z.enum(MARKETPLACE_ROUTE_ORDER_VALUES))
    .optional(),
  marketplace_route_enabled: z
    .array(z.enum(MARKETPLACE_ROUTE_ORDER_VALUES))
    .optional(),
  tokenCount: z.number().min(1).optional(),
})

export type ApiKeyFormValues = z.infer<typeof apiKeyFormSchema>

// ============================================================================
// Form Defaults
// ============================================================================

export const API_KEY_FORM_DEFAULT_VALUES: ApiKeyFormValues = {
  name: '',
  remain_quota_dollars: 10,
  expired_time: undefined,
  unlimited_quota: true,
  model_limits: [],
  allow_ips: '',
  group: DEFAULT_GROUP,
  cross_group_retry: true,
  marketplace_fixed_order_id: 0,
  marketplace_fixed_order_ids: [],
  marketplace_route_order: [...DEFAULT_MARKETPLACE_ROUTE_ORDER],
  marketplace_route_enabled: [...DEFAULT_MARKETPLACE_ROUTE_ENABLED],
  tokenCount: 1,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: ApiKeyFormValues
): ApiKeyFormData {
  const primaryFixedOrderId = data.marketplace_fixed_order_id || 0
  const existingFixedOrderIds = data.marketplace_fixed_order_ids ?? []
  const fixedOrderIds =
    existingFixedOrderIds[0] === primaryFixedOrderId
      ? existingFixedOrderIds
      : primaryFixedOrderId
        ? [primaryFixedOrderId]
        : []

  return {
    name: data.name,
    remain_quota: data.unlimited_quota
      ? 0
      : parseQuotaFromDollars(data.remain_quota_dollars || 0),
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : -1,
    unlimited_quota: data.unlimited_quota,
    model_limits_enabled: data.model_limits.length > 0,
    model_limits: data.model_limits.join(','),
    allow_ips: data.allow_ips || '',
    group: data.group || DEFAULT_GROUP,
    cross_group_retry: data.group === 'auto' ? !!data.cross_group_retry : false,
    marketplace_fixed_order_id: primaryFixedOrderId,
    marketplace_fixed_order_ids: fixedOrderIds,
    marketplace_route_order: normalizeMarketplaceRouteOrder(
      data.marketplace_route_order
    ),
    marketplace_route_enabled: normalizeMarketplaceRouteEnabled(
      data.marketplace_route_enabled
    ),
  }
}

/**
 * Transform API key data to form defaults
 */
export function transformApiKeyToFormDefaults(
  apiKey: ApiKey
): ApiKeyFormValues {
  const fixedOrderIds =
    apiKey.marketplace_fixed_order_ids?.length > 0
      ? apiKey.marketplace_fixed_order_ids
      : apiKey.marketplace_fixed_order_id
        ? [apiKey.marketplace_fixed_order_id]
        : []

  return {
    name: apiKey.name,
    remain_quota_dollars: quotaUnitsToDollars(apiKey.remain_quota),
    expired_time:
      apiKey.expired_time > 0
        ? new Date(apiKey.expired_time * 1000)
        : undefined,
    unlimited_quota: apiKey.unlimited_quota,
    model_limits: apiKey.model_limits
      ? apiKey.model_limits.split(',').filter(Boolean)
      : [],
    allow_ips: apiKey.allow_ips || '',
    group: apiKey.group || DEFAULT_GROUP,
    cross_group_retry: !!apiKey.cross_group_retry,
    marketplace_fixed_order_id: fixedOrderIds[0] ?? 0,
    marketplace_fixed_order_ids: fixedOrderIds,
    marketplace_route_order: normalizeMarketplaceRouteOrder(
      apiKey.marketplace_route_order
    ),
    marketplace_route_enabled: normalizeMarketplaceRouteEnabled(
      apiKey.marketplace_route_enabled
    ),
    tokenCount: 1,
  }
}
