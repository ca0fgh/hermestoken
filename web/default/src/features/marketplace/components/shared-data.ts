import type {
  ApiResponse,
  MarketplaceCredentialFormValues,
  MarketplaceOrderFilters,
  MarketplacePage,
} from '../types'

export const MARKETPLACE_PAGE_SIZE = 20

export const defaultFilters: MarketplaceOrderFilters = {
  p: 1,
  page_size: MARKETPLACE_PAGE_SIZE,
  quota_mode: '',
  time_mode: '',
  min_quota_limit: '',
  max_quota_limit: '',
  min_time_limit_seconds: '',
  max_time_limit_seconds: '',
  min_multiplier: '',
  max_multiplier: '',
  min_concurrency_limit: '',
  max_concurrency_limit: '',
}

export const defaultCredentialForm: MarketplaceCredentialFormValues = {
  vendor_type: 1,
  api_key: '',
  base_url: '',
  proxy: '',
  other: '',
  model_mapping: '',
  status_code_mapping: '',
  setting: '',
  param_override: '',
  settings: '',
  models: 'gpt-4o-mini',
  quota_mode: 'unlimited',
  quota_limit: 0,
  time_mode: 'unlimited',
  time_limit_minutes: 0,
  multiplier: 1,
  concurrency_limit: 1,
}

export function unwrapPage<T>(response?: ApiResponse<MarketplacePage<T>>) {
  return {
    items: response?.data?.items ?? [],
    total: response?.data?.total ?? 0,
  }
}
