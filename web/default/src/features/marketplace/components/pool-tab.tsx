import { useEffect, useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, CheckCircle2, Loader2, Power, Store } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { updateApiKey } from '@/features/keys/api'
import {
  normalizeMarketplaceRouteEnabled,
  normalizeMarketplaceRouteOrder,
  type ApiKey,
  type ApiKeyFormData,
} from '@/features/keys/types'
import {
  listMarketplaceOrderFilterRanges,
  listMarketplacePoolCandidates,
  listMarketplacePoolModels,
} from '../api'
import { formatMarketplacePricePoint } from '../lib'
import type {
  MarketplaceOrderFilters,
  MarketplacePoolCandidate,
  MarketplacePoolModel,
} from '../types'
import { BuyerTokenPanel } from './buyer-token-panel'
import { CallSnippet } from './call-snippet'
import {
  EmptyLine,
  ListFooter,
  MarketplaceFilters,
  MarketplaceVendor,
  MetricProgress,
  ModelBadges,
} from './shared'
import { defaultFilters, unwrapPage } from './shared-data'

const MARKETPLACE_TOKEN_ROUTES = ['fixed_order', 'group', 'pool'] as const

function marketplacePoolRouteEnabled(token: ApiKey | null) {
  if (!token) return false
  return normalizeMarketplaceRouteEnabled(
    token.marketplace_route_enabled
  ).includes('pool')
}

function tokenWithMarketplacePoolRoute(token: ApiKey): ApiKey {
  const enabledRoutes = new Set(
    normalizeMarketplaceRouteEnabled(token.marketplace_route_enabled)
  )
  enabledRoutes.add('pool')
  return {
    ...token,
    marketplace_route_order: normalizeMarketplaceRouteOrder(
      token.marketplace_route_order
    ),
    marketplace_route_enabled: MARKETPLACE_TOKEN_ROUTES.filter((route) =>
      enabledRoutes.has(route)
    ),
  }
}

function marketplacePoolTokenUpdatePayload(token: ApiKey): ApiKeyFormData & {
  id: number
} {
  return {
    id: token.id,
    name: token.name,
    remain_quota: token.unlimited_quota ? 0 : token.remain_quota,
    expired_time: token.expired_time,
    unlimited_quota: token.unlimited_quota,
    model_limits_enabled: token.model_limits_enabled,
    model_limits: token.model_limits || '',
    allow_ips: token.allow_ips || '',
    group: token.group || '',
    cross_group_retry:
      token.group === 'auto' ? !!token.cross_group_retry : false,
    marketplace_fixed_order_id: token.marketplace_fixed_order_id || 0,
    marketplace_fixed_order_ids: token.marketplace_fixed_order_ids ?? [],
    marketplace_route_order: normalizeMarketplaceRouteOrder(
      token.marketplace_route_order
    ),
    marketplace_route_enabled: normalizeMarketplaceRouteEnabled(
      token.marketplace_route_enabled
    ),
  }
}

export function PoolTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [filters, setFilters] =
    useState<MarketplaceOrderFilters>(defaultFilters)
  const [selectedBuyerToken, setSelectedBuyerToken] = useState<ApiKey | null>(
    null
  )
  const [activatingPool, setActivatingPool] = useState(false)
  const modelsQuery = useQuery({
    queryKey: ['marketplace', 'pool-models', filters],
    queryFn: () => listMarketplacePoolModels(filters),
    placeholderData: (previous) => previous,
  })
  const candidatesQuery = useQuery({
    queryKey: ['marketplace', 'pool-candidates', filters],
    queryFn: () => listMarketplacePoolCandidates(filters),
    placeholderData: (previous) => previous,
  })
  const filterRangesQuery = useQuery({
    queryKey: ['marketplace', 'order-filter-ranges', filters],
    queryFn: () => listMarketplaceOrderFilterRanges(filters),
    placeholderData: (previous) => previous,
  })
  const poolModels = modelsQuery.data?.data ?? []
  const { items: candidates, total } = unwrapPage(candidatesQuery.data)
  const selectedPoolModel = useMemo(
    () => (filters.model || '').trim() || poolModels[0]?.model || 'gpt-4o-mini',
    [filters.model, poolModels]
  )
  const poolActivated = marketplacePoolRouteEnabled(selectedBuyerToken)

  useEffect(() => {
    if (!selectedBuyerToken) setActivatingPool(false)
  }, [selectedBuyerToken])

  const activateMarketplacePoolForToken = async () => {
    if (!selectedBuyerToken || poolActivated) return
    const nextToken = tokenWithMarketplacePoolRoute(selectedBuyerToken)
    setActivatingPool(true)
    try {
      const response = await updateApiKey(
        marketplacePoolTokenUpdatePayload(nextToken)
      )
      if (!response.success) {
        toast.error(response.message || t('Failed to activate order pool'))
        return
      }
      setSelectedBuyerToken({
        ...nextToken,
        ...(response.data ?? {}),
        marketplace_route_order: nextToken.marketplace_route_order,
        marketplace_route_enabled: nextToken.marketplace_route_enabled,
      })
      await queryClient.invalidateQueries({
        queryKey: ['marketplace', 'buyer-console-tokens'],
      })
      await queryClient.invalidateQueries({ queryKey: ['keys'] })
      toast.success(t('Order pool activated for this token'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to activate order pool')
      )
    } finally {
      setActivatingPool(false)
    }
  }

  return (
    <div className='space-y-4'>
      <BuyerTokenPanel
        selectedTokenId={selectedBuyerToken?.id}
        onTokenChange={setSelectedBuyerToken}
      />
      <div className='rounded-md border p-4'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex items-center gap-2 font-medium'>
            {poolActivated ? (
              <CheckCircle2 className='size-4 text-green-600' />
            ) : (
              <Power className='size-4' />
            )}
            {t('Order pool activation')}
          </div>
          <Button
            type='button'
            size='sm'
            disabled={!selectedBuyerToken || poolActivated || activatingPool}
            onClick={activateMarketplacePoolForToken}
            variant={poolActivated ? 'secondary' : 'default'}
          >
            {activatingPool ? (
              <Loader2 className='size-4 animate-spin' />
            ) : poolActivated ? (
              <CheckCircle2 className='size-4' />
            ) : (
              <Power className='size-4' />
            )}
            {activatingPool
              ? t('Activating order pool')
              : poolActivated
                ? t('Order pool activated')
                : t('Activate order pool')}
          </Button>
        </div>
        {poolActivated ? (
          <div className='mt-3'>
            <CallSnippet
              filters={filters}
              selectedToken={selectedBuyerToken}
              model={selectedPoolModel}
            />
          </div>
        ) : null}
      </div>
      <MarketplaceFilters
        filters={filters}
        filterRanges={filterRangesQuery.data?.data}
        onChange={setFilters}
      />
      <div className='grid gap-3 lg:grid-cols-2'>
        <PoolModelsPanel models={poolModels} />
        <PoolCandidatesPanel candidates={candidates} />
      </div>
      <ListFooter
        loading={modelsQuery.isLoading || candidatesQuery.isLoading}
        total={total}
      />
    </div>
  )
}

function PoolModelsPanel({ models }: { models: MarketplacePoolModel[] }) {
  const { t } = useTranslation()
  return (
    <div className='rounded-md border p-4'>
      <div className='mb-3 flex items-center gap-2 font-medium'>
        <Activity className='size-4' />
        {t('Pool models')}
      </div>
      <div className='space-y-3'>
        {models.map((model) => (
          <div
            key={`${model.vendor_type}-${model.model}`}
            className='flex items-center justify-between gap-3 border-b pb-3 last:border-b-0 last:pb-0'
          >
            <div className='min-w-0'>
              <MarketplaceVendor
                vendorType={model.vendor_type}
                vendorName={model.vendor_name_snapshot}
              />
              <div className='mt-2 truncate text-sm'>{model.model}</div>
            </div>
            <div className='text-end text-sm'>
              <div className='font-medium'>{model.candidate_count}</div>
              <div className='text-muted-foreground text-xs'>
                {formatMarketplacePricePoint(model.lowest_price_preview.buyer)}
              </div>
            </div>
          </div>
        ))}
        {models.length === 0 ? <EmptyLine label={t('No pool models')} /> : null}
      </div>
    </div>
  )
}

function PoolCandidatesPanel({
  candidates,
}: {
  candidates: MarketplacePoolCandidate[]
}) {
  const { t } = useTranslation()
  return (
    <div className='rounded-md border p-4'>
      <div className='mb-3 flex items-center gap-2 font-medium'>
        <Store className='size-4' />
        {t('Route candidates')}
      </div>
      <div className='space-y-3'>
        {candidates.map((candidate) => (
          <div
            key={candidate.credential.id}
            className='space-y-2 border-b pb-3 last:border-b-0 last:pb-0'
          >
            <div className='flex items-start justify-between gap-3'>
              <MarketplaceVendor
                vendorType={candidate.credential.vendor_type}
                vendorName={candidate.credential.vendor_name_snapshot}
              />
              <StatusBadge
                label={`${formatNumber(candidate.route_score)} pts`}
                variant='blue'
                copyable={false}
              />
            </div>
            <ModelBadges models={candidate.credential.models} />
            <div className='grid gap-2 sm:grid-cols-2'>
              <MetricProgress
                label={t('Success rate')}
                value={candidate.success_rate * 100}
              />
              <MetricProgress
                label={t('Load')}
                value={candidate.load_ratio * 100}
              />
            </div>
          </div>
        ))}
        {candidates.length === 0 ? (
          <EmptyLine label={t('No route candidates')} />
        ) : null}
      </div>
    </div>
  )
}
