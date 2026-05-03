import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, ShoppingCart } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { StatusBadge } from '@/components/status-badge'
import { getSystemOptions } from '@/features/system-settings/api'
import { getOptionValue } from '@/features/system-settings/hooks/use-system-options'
import {
  createMarketplaceFixedOrder,
  listMarketplaceOrderFilterRanges,
  listMarketplaceOrders,
} from '../api'
import {
  formatMarketplaceFeePercent,
  formatMarketplacePricePoint,
  formatMarketplaceUSD,
  marketplaceBuyerPaymentUSD,
  marketplaceQuotaText,
  marketplaceStatusLabel,
  marketplaceSuccessRate,
  marketplaceStatusVariant,
  normalizeMarketplaceFeeRate,
} from '../lib'
import type {
  MarketplaceOrderFilters,
  MarketplaceOrderListItem,
} from '../types'
import {
  ListFooter,
  MarketplaceFilters,
  MarketplaceVendor,
  ModelBadges,
  PricePreview,
  StatPill,
} from './shared'
import { defaultFilters, unwrapPage } from './shared-data'

const MARKETPLACE_FEE_DEFAULTS = {
  MarketplaceFeeRate: 0,
}

export function OrdersTab({
  onFixedOrderCreated,
}: {
  onFixedOrderCreated?: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [filters, setFilters] =
    useState<MarketplaceOrderFilters>(defaultFilters)
  const [selectedOrder, setSelectedOrder] =
    useState<MarketplaceOrderListItem | null>(null)
  const [purchaseAmountUSD, setPurchaseAmountUSD] = useState('1')
  const purchaseAmountNumber = Number(purchaseAmountUSD) || 0

  const ordersQuery = useQuery({
    queryKey: ['marketplace', 'orders', filters],
    queryFn: () => listMarketplaceOrders(filters),
    placeholderData: (previous) => previous,
  })
  const filterRangesQuery = useQuery({
    queryKey: ['marketplace', 'order-filter-ranges', filters],
    queryFn: () => listMarketplaceOrderFilterRanges(filters),
    placeholderData: (previous) => previous,
  })
  const marketplaceSettingsQuery = useQuery({
    queryKey: ['system-options'],
    queryFn: getSystemOptions,
    staleTime: 5 * 60 * 1000,
  })
  const marketplaceFeeRate = normalizeMarketplaceFeeRate(
    getOptionValue(
      marketplaceSettingsQuery.data?.data,
      MARKETPLACE_FEE_DEFAULTS
    ).MarketplaceFeeRate
  )
  const estimatedBuyerPaymentUSD = marketplaceBuyerPaymentUSD(
    purchaseAmountUSD,
    marketplaceFeeRate
  )

  const purchaseMutation = useMutation({
    mutationFn: () =>
      createMarketplaceFixedOrder({
        credential_id: selectedOrder?.id ?? 0,
        purchased_amount_usd: purchaseAmountNumber,
      }),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Purchase failed'))
        return
      }
      toast.success(t('Fixed order created'))
      setSelectedOrder(null)
      onFixedOrderCreated?.()
      void queryClient.invalidateQueries({ queryKey: ['marketplace'] })
    },
  })

  const { items, total } = unwrapPage(ordersQuery.data)

  return (
    <div className='space-y-4'>
      <MarketplaceFilters
        filters={filters}
        filterRanges={filterRangesQuery.data?.data}
        onChange={setFilters}
      />
      <div className='grid gap-3 xl:grid-cols-2'>
        {items.map((item) => (
          <OrderCard
            key={item.id}
            item={item}
            onBuy={(order) => {
              setSelectedOrder(order)
              setPurchaseAmountUSD('1')
            }}
          />
        ))}
      </div>
      <ListFooter loading={ordersQuery.isLoading} total={total} />
      <Dialog
        open={!!selectedOrder}
        onOpenChange={(open) => !open && setSelectedOrder(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Buy fixed amount')}</DialogTitle>
            <DialogDescription>
              {t(
                'Enter the base call amount. The final fixed order balance and deduction include the global buyer transaction fee.'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-3'>
            {selectedOrder ? (
              <MarketplaceVendor
                vendorType={selectedOrder.vendor_type}
                vendorName={selectedOrder.vendor_name_snapshot}
              />
            ) : null}
            {selectedOrder ? (
              <PurchasePricePreview item={selectedOrder} />
            ) : null}
            <div className='space-y-1.5'>
              <Label>{t('Purchase amount (USD)')}</Label>
              <Input
                type='number'
                min='0.01'
                step='0.01'
                value={purchaseAmountUSD}
                onChange={(event) => setPurchaseAmountUSD(event.target.value)}
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Buyer transaction fee {{rate}}. Estimated deduction {{amount}}.',
                  {
                    rate: formatMarketplaceFeePercent(marketplaceFeeRate),
                    amount: formatMarketplaceUSD(estimatedBuyerPaymentUSD),
                  }
                )}
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setSelectedOrder(null)}
              disabled={purchaseMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              onClick={() => purchaseMutation.mutate()}
              disabled={purchaseMutation.isPending || purchaseAmountNumber <= 0}
            >
              {purchaseMutation.isPending ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <ShoppingCart className='size-4' />
              )}
              {t('Confirm purchase')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function PurchasePricePreview({ item }: { item: MarketplaceOrderListItem }) {
  const { t } = useTranslation()
  const previews = item.price_preview.slice(0, 4)
  if (previews.length === 0) {
    return null
  }
  return (
    <div className='rounded-md border'>
      <div className='text-muted-foreground grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.6fr)_minmax(0,1.6fr)] border-b px-3 py-2 text-xs font-medium'>
        <div>{t('Model')}</div>
        <div>{t('Official price')}</div>
        <div>{t('Multiplied price')}</div>
      </div>
      <div className='divide-y'>
        {previews.map((preview) => (
          <div
            key={preview.model}
            className='grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.6fr)_minmax(0,1.6fr)] gap-2 px-3 py-2 text-sm'
          >
            <div className='min-w-0 truncate font-medium'>{preview.model}</div>
            <div className='text-muted-foreground min-w-0'>
              {formatMarketplacePricePoint(preview.official)}
            </div>
            <div className='min-w-0'>
              {formatMarketplacePricePoint(preview.buyer)}
            </div>
          </div>
        ))}
      </div>
      <div className='text-muted-foreground border-t px-3 py-2 text-xs'>
        {t('Multiplier')}: {item.multiplier}x
      </div>
    </div>
  )
}

function OrderCard({
  item,
  onBuy,
}: {
  item: MarketplaceOrderListItem
  onBuy: (order: MarketplaceOrderListItem) => void
}) {
  const { t } = useTranslation()

  return (
    <div className='rounded-md border p-4'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
        <MarketplaceVendor
          vendorType={item.vendor_type}
          vendorName={item.vendor_name_snapshot}
        />
        <div className='flex flex-wrap gap-2'>
          <StatusBadge
            label={t(marketplaceStatusLabel(item.health_status))}
            variant={marketplaceStatusVariant(item.health_status)}
            copyable={false}
          />
          <StatusBadge
            label={t(marketplaceStatusLabel(item.route_status))}
            variant={marketplaceStatusVariant(item.route_status)}
            copyable={false}
          />
          <StatusBadge
            label={`${marketplaceSuccessRate(item)}%`}
            variant='success'
            copyable={false}
          />
        </div>
      </div>
      <div className='mt-4 space-y-3'>
        <ModelBadges models={item.models} />
        <div className='grid gap-2 sm:grid-cols-3'>
          <div className='rounded-md border px-3 py-2 sm:col-span-1'>
            <div className='text-muted-foreground text-xs'>{t('Price')}</div>
            <PricePreview item={item} />
          </div>
          <StatPill label={t('Quota')} value={t(marketplaceQuotaText(item))} />
          <StatPill
            label={t('Concurrency')}
            value={`${item.current_concurrency}/${item.concurrency_limit}`}
          />
        </div>
        <Button className='w-full sm:w-auto' onClick={() => onBuy(item)}>
          <ShoppingCart className='size-4' />
          {t('Buy fixed amount')}
        </Button>
      </div>
    </div>
  )
}
