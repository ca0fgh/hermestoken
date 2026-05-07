import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Check,
  ChevronDown,
  Copy,
  Eye,
  EyeOff,
  Link as LinkIcon,
  Loader2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { useStatus } from '@/hooks/use-status'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import {
  encodeConnectionString,
  getConfiguredConnectionURL,
} from '@/features/keys/lib/connection-info'
import type { ApiKey, GetApiKeysResponse } from '@/features/keys/types'
import {
  bindMarketplaceFixedOrderTokens,
  listMarketplaceFixedOrders,
  probeMarketplaceFixedOrder,
  releaseMarketplaceFixedOrder,
} from '../api'
import {
  fixedOrderRelayHeaderSnippet,
  formatMarketplaceQuotaUSD,
  marketplaceFixedOrderUsage,
  marketplaceStatusLabel,
  marketplaceTokenFixedOrderIds,
} from '../lib'
import { CallSnippet } from './call-snippet'
import { EmptyLine, ListFooter, MetricProgress, StatPill } from './shared'
import { MARKETPLACE_PAGE_SIZE, unwrapPage } from './shared-data'

function fullTokenKey(key?: string | null) {
  if (!key) return ''
  return key.startsWith('sk-') ? key : `sk-${key}`
}

function tokenKeyLabel(token: ApiKey, key: string) {
  const name = token.name.trim()
  return name ? `${name} · ${key}` : key
}

function formatFixedOrderRemainingTime(
  seconds: number,
  t: (key: string) => string
) {
  const value = Math.max(0, seconds)
  if (value < 60) return `${Math.ceil(value)} ${t('seconds')}`
  if (value < 3600) return `${Math.ceil(value / 60)} ${t('minutes')}`
  if (value < 86400) return `${Math.ceil(value / 3600)} ${t('hours')}`
  return `${Math.ceil(value / 86400)} ${t('days')}`
}

function formatFixedOrderTimeLimit(
  order: { expires_at: number; status: string },
  t: (key: string) => string
) {
  if (!order.expires_at || order.expires_at <= 0) return t('Unlimited')
  const expiresAt = formatTimestampToDate(order.expires_at)
  const remainingSeconds = order.expires_at - Math.floor(Date.now() / 1000)
  if (order.status === 'expired' || remainingSeconds <= 0) {
    return `${t('Expired')} · ${expiresAt}`
  }
  return `${t('Remaining')} ${formatFixedOrderRemainingTime(
    remainingSeconds,
    t
  )} · ${expiresAt}`
}

type BoundTokenKeyChipProps = {
  token: ApiKey
  revealed: boolean
  resolvedKey?: string
  loading: boolean
  copied: boolean
  onToggleVisibility: (token: ApiKey) => void
  onCopyKey: (token: ApiKey) => void
  onCopyConnectionInfo: (token: ApiKey) => void
}

function BoundTokenKeyChip({
  token,
  revealed,
  resolvedKey,
  loading,
  copied,
  onToggleVisibility,
  onCopyKey,
  onCopyConnectionInfo,
}: BoundTokenKeyChipProps) {
  const { t } = useTranslation()
  const displayKey = fullTokenKey(
    revealed && resolvedKey ? resolvedKey : token.key
  )
  const displayLabel = tokenKeyLabel(token, displayKey)

  return (
    <span
      className={`bg-muted text-foreground grid w-full grid-cols-[minmax(0,1fr)_1.5rem_1.5rem] items-center gap-1 rounded-2xl py-1 pr-1.5 pl-3 font-mono text-sm ${
        revealed ? 'items-start py-2' : 'h-8 rounded-full'
      }`}
    >
      <span
        className={
          revealed
            ? 'break-all whitespace-normal'
            : 'truncate whitespace-nowrap'
        }
      >
        {displayLabel}
      </span>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground size-6 shrink-0'
            disabled={loading}
            aria-label={revealed ? t('Hide API key') : t('Full API Key')}
            onClick={() => onToggleVisibility(token)}
          >
            {loading ? (
              <Loader2 className='size-3.5 animate-spin' />
            ) : revealed ? (
              <EyeOff className='size-3.5' />
            ) : (
              <Eye className='size-3.5' />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          <p>{revealed ? t('Hide API key') : t('Full API Key')}</p>
        </TooltipContent>
      </Tooltip>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground size-6 shrink-0'
            disabled={loading}
            aria-label={copied ? t('Copied!') : t('Copy')}
          >
            {copied ? (
              <Check className='size-3.5 text-green-600' />
            ) : (
              <Copy className='size-3.5' />
            )}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-44'>
          <DropdownMenuItem onClick={() => onCopyKey(token)}>
            {t('Copy Key')}
            <DropdownMenuShortcut>
              <Copy className='size-4' />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => onCopyConnectionInfo(token)}>
            {t('Copy Connection Info')}
            <DropdownMenuShortcut>
              <LinkIcon className='size-4' />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </span>
  )
}

type BoundTokenDropdownProps = {
  tokens: ApiKey[]
  revealedTokenKeys: Record<number, boolean>
  resolvedTokenKeys: Record<number, string>
  loadingTokenKeys: Record<number, boolean>
  copiedText: string | null
  onToggleVisibility: (token: ApiKey) => void
  onCopyKey: (token: ApiKey) => void
  onCopyConnectionInfo: (token: ApiKey) => void
}

function BoundTokenDropdown({
  tokens,
  revealedTokenKeys,
  resolvedTokenKeys,
  loadingTokenKeys,
  copiedText,
  onToggleVisibility,
  onCopyKey,
  onCopyConnectionInfo,
}: BoundTokenDropdownProps) {
  const { t } = useTranslation()

  if (tokens.length === 0) {
    return (
      <StatusBadge label={t('Not bound')} variant='neutral' copyable={false} />
    )
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='h-8 rounded-full px-3'
        >
          {tokens.length} {t('Token')}
          <ChevronDown className='size-3.5' />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align='end'
        className='w-[min(42rem,calc(100vw-2rem))] overflow-x-hidden p-3'
      >
        <div className='flex max-h-60 w-full flex-col items-start gap-2 overflow-y-auto'>
          {tokens.map((token) => (
            <BoundTokenKeyChip
              key={token.id}
              token={token}
              revealed={!!revealedTokenKeys[token.id]}
              resolvedKey={resolvedTokenKeys[token.id]}
              loading={!!loadingTokenKeys[token.id]}
              copied={
                !!resolvedTokenKeys[token.id] &&
                copiedText === fullTokenKey(resolvedTokenKeys[token.id])
              }
              onToggleVisibility={onToggleVisibility}
              onCopyKey={onCopyKey}
              onCopyConnectionInfo={onCopyConnectionInfo}
            />
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}

export function FixedOrdersTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { status } = useStatus()
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const [bindingOpen, setBindingOpen] = useState(false)
  const [bindingOrderId, setBindingOrderId] = useState<number | null>(null)
  const [bindingTokenSelection, setBindingTokenSelection] = useState<number[]>(
    []
  )
  const [revealedTokenKeys, setRevealedTokenKeys] = useState<
    Record<number, boolean>
  >({})
  const [resolvedTokenKeys, setResolvedTokenKeys] = useState<
    Record<number, string>
  >({})
  const [loadingTokenKeys, setLoadingTokenKeys] = useState<
    Record<number, boolean>
  >({})
  const [releaseOrderId, setReleaseOrderId] = useState<number | null>(null)
  const ordersQuery = useQuery({
    queryKey: ['marketplace', 'fixed-orders'],
    queryFn: () =>
      listMarketplaceFixedOrders({
        p: 1,
        page_size: MARKETPLACE_PAGE_SIZE,
      }),
    placeholderData: (previous) => previous,
  })
  const tokensQuery = useQuery({
    queryKey: ['marketplace', 'buyer-console-tokens'],
    queryFn: () => getApiKeys({ p: 1, size: 100 }),
  })
  const tokens = useMemo(
    () => tokensQuery.data?.data?.items ?? [],
    [tokensQuery.data?.data?.items]
  )
  const enabledTokens = useMemo(
    () => tokens.filter((token) => token.status === API_KEY_STATUS.ENABLED),
    [tokens]
  )
  const bindMutation = useMutation({
    mutationFn: ({
      fixedOrderId,
      tokenIds,
    }: {
      fixedOrderId: number
      tokenIds: number[]
    }) =>
      bindMarketplaceFixedOrderTokens(fixedOrderId, {
        token_ids: tokenIds,
      }),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to bind tokens'))
        return
      }
      const result = response.data
      const boundTokenIds = new Set(result.token_ids)
      const applyFixedOrderBinding = (token: ApiKey): ApiKey => {
        const remainingOrderIds = marketplaceTokenFixedOrderIds(token).filter(
          (orderId) => orderId !== result.fixed_order_id
        )
        const nextOrderIds = boundTokenIds.has(token.id)
          ? [result.fixed_order_id, ...remainingOrderIds]
          : remainingOrderIds

        return {
          ...token,
          marketplace_fixed_order_id: nextOrderIds[0] ?? 0,
          marketplace_fixed_order_ids: nextOrderIds,
        }
      }

      queryClient.setQueryData<GetApiKeysResponse>(
        ['marketplace', 'buyer-console-tokens'],
        (current) => {
          if (!current?.data?.items) return current

          return {
            ...current,
            data: {
              ...current.data,
              items: current.data.items.map((token: ApiKey) =>
                applyFixedOrderBinding(token)
              ),
            },
          }
        }
      )

      toast.success(t('Token bindings updated'))
      setBindingOpen(false)
      void queryClient.invalidateQueries({
        queryKey: ['marketplace', 'buyer-console-tokens'],
      })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('Failed to bind tokens')
      )
    },
  })
  const probeMutation = useMutation({
    mutationFn: (fixedOrderId: number) => probeMarketplaceFixedOrder(fixedOrderId),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Detection failed'))
        return
      }
      const order = response.data
      const scoreDrop =
        Number(order.purchase_probe_score) - Number(order.refund_probe_score)
      const releaseAvailable = scoreDrop >= 5
      toast.success(
        releaseAvailable
          ? t('Detection complete. This order can be released.')
          : t('Detection complete. This order does not meet release conditions.')
      )
      queryClient.setQueryData<
        Awaited<ReturnType<typeof listMarketplaceFixedOrders>>
      >(['marketplace', 'fixed-orders'], (current) => {
        if (!current?.data?.items) return current
        return {
          ...current,
          data: {
            ...current.data,
            items: current.data.items.map((item) =>
              item.id === order.id ? { ...item, ...order } : item
            ),
          },
        }
      })
      queryClient.setQueryData<GetApiKeysResponse>(
        ['marketplace', 'buyer-console-tokens'],
        (current) => {
          if (!current?.data?.items) return current
          return {
            ...current,
            data: {
              ...current.data,
              items: current.data.items.map((token) => {
                const nextOrderIds = marketplaceTokenFixedOrderIds(token).filter(
                  (orderId) => orderId !== order.id
                )
                return {
                  ...token,
                  marketplace_fixed_order_id: nextOrderIds[0] ?? 0,
                  marketplace_fixed_order_ids: nextOrderIds,
                }
              }),
            },
          }
        }
      )
      void queryClient.invalidateQueries({
        queryKey: ['marketplace', 'fixed-orders'],
      })
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Detection failed'))
    },
  })
  const releaseMutation = useMutation({
    mutationFn: (fixedOrderId: number) =>
      releaseMarketplaceFixedOrder(fixedOrderId),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to release order'))
        setReleaseOrderId(null)
        void queryClient.invalidateQueries({
          queryKey: ['marketplace', 'fixed-orders'],
        })
        return
      }
      const order = response.data
      toast.success(
        `${t('Order released and remaining amount refunded')}: ${formatMarketplaceQuotaUSD(order.refunded_quota)}`
      )
      queryClient.setQueryData<
        Awaited<ReturnType<typeof listMarketplaceFixedOrders>>
      >(['marketplace', 'fixed-orders'], (current) => {
        if (!current?.data?.items) return current
        return {
          ...current,
          data: {
            ...current.data,
            items: current.data.items.map((item) =>
              item.id === order.id ? { ...item, ...order } : item
            ),
          },
        }
      })
      queryClient.setQueryData<GetApiKeysResponse>(
        ['marketplace', 'buyer-console-tokens'],
        (current) => {
          if (!current?.data?.items) return current
          return {
            ...current,
            data: {
              ...current.data,
              items: current.data.items.map((token) => {
                const nextOrderIds = marketplaceTokenFixedOrderIds(token).filter(
                  (orderId) => orderId !== order.id
                )
                return {
                  ...token,
                  marketplace_fixed_order_id: nextOrderIds[0] ?? 0,
                  marketplace_fixed_order_ids: nextOrderIds,
                }
              }),
            },
          }
        }
      )
      void queryClient.invalidateQueries({
        queryKey: ['marketplace', 'fixed-orders'],
      })
      void queryClient.invalidateQueries({
        queryKey: ['marketplace', 'buyer-console-tokens'],
      })
      setReleaseOrderId(null)
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to release order')
      )
      setReleaseOrderId(null)
    },
  })
  const { items: orders, total } = unwrapPage(ordersQuery.data)

  const tokenIdsBoundToOrder = (orderId: number) =>
    enabledTokens
      .filter((token) => marketplaceTokenFixedOrderIds(token).includes(orderId))
      .map((token) => token.id)

  const tokensBoundToOrder = (orderId: number) =>
    enabledTokens.filter((token) =>
      marketplaceTokenFixedOrderIds(token).includes(orderId)
    )

  const resolveBoundTokenKey = useCallback(
    async (token: ApiKey) => {
      const cachedKey = resolvedTokenKeys[token.id]
      if (cachedKey) return fullTokenKey(cachedKey)

      setLoadingTokenKeys((current) => ({ ...current, [token.id]: true }))
      try {
        const response = await fetchTokenKey(token.id)
        const key = response.data?.key
        if (!response.success || !key) {
          throw new Error(response.message || t('Failed to load API keys'))
        }

        setResolvedTokenKeys((current) => ({
          ...current,
          [token.id]: key,
        }))
        return fullTokenKey(key)
      } catch (error) {
        toast.error(
          error instanceof Error ? error.message : t('Failed to load API keys')
        )
        return ''
      } finally {
        setLoadingTokenKeys((current) => ({ ...current, [token.id]: false }))
      }
    },
    [resolvedTokenKeys, t]
  )

  const toggleBoundTokenVisibility = useCallback(
    async (token: ApiKey) => {
      if (revealedTokenKeys[token.id]) {
        setRevealedTokenKeys((current) => ({
          ...current,
          [token.id]: false,
        }))
        return
      }

      const key = await resolveBoundTokenKey(token)
      if (key) {
        setRevealedTokenKeys((current) => ({
          ...current,
          [token.id]: true,
        }))
      }
    },
    [resolveBoundTokenKey, revealedTokenKeys]
  )

  const copyBoundTokenKey = useCallback(
    async (token: ApiKey) => {
      const key = await resolveBoundTokenKey(token)
      if (key) {
        await copyToClipboard(key)
      }
    },
    [copyToClipboard, resolveBoundTokenKey]
  )

  const copyBoundTokenConnectionInfo = useCallback(
    async (token: ApiKey) => {
      const key = await resolveBoundTokenKey(token)
      if (key) {
        await copyToClipboard(
          encodeConnectionString(key, getConfiguredConnectionURL(status))
        )
      }
    },
    [copyToClipboard, resolveBoundTokenKey, status]
  )

  const openBindingEditor = (orderId: number) => {
    setBindingOrderId(orderId)
    setBindingTokenSelection(tokenIdsBoundToOrder(orderId))
    setBindingOpen(true)
  }

  const toggleBindingToken = (tokenId: number) => {
    setBindingTokenSelection((current) => {
      if (current.includes(tokenId)) {
        return current.filter((id) => id !== tokenId)
      }
      return [...current, tokenId]
    })
  }

  const saveBindingSelection = () => {
    if (!bindingOrderId) {
      toast.error(t('Select a fixed order first'))
      return
    }
    bindMutation.mutate({
      fixedOrderId: bindingOrderId,
      tokenIds: bindingTokenSelection,
    })
  }

  const canProbeAndReleaseOrder = (order: (typeof orders)[number]) =>
    order.status === 'active' &&
    Number(order.remaining_quota) > 0 &&
    Number(order.purchase_probe_score) > 0

  const canReleaseOrder = (order: (typeof orders)[number]) =>
    canProbeAndReleaseOrder(order) &&
    Number(order.refund_probe_checked_at) > 0 &&
    Number(order.purchase_probe_score) - Number(order.refund_probe_score) >= 5

  const confirmProbeAndReleaseOrder = () => {
    if (!releaseOrderId || releaseMutation.isPending) return
    releaseMutation.mutate(releaseOrderId)
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-3 xl:grid-cols-2'>
        {orders.map((order) => (
          <div key={order.id} className='rounded-md border p-4'>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
              <div>
                <div className='text-sm font-semibold'>
                  #{order.id} · {t(marketplaceStatusLabel(order.status))}
                </div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {fixedOrderRelayHeaderSnippet(order.id)}
                </div>
              </div>
              <StatusBadge
                label={t(marketplaceStatusLabel(order.status))}
                variant={order.status === 'active' ? 'success' : 'neutral'}
                copyable={false}
              />
              <div className='text-muted-foreground max-w-72 text-right text-xs leading-5'>
                {formatFixedOrderTimeLimit(order, t)}
              </div>
            </div>
            <div className='mt-4 grid gap-2 sm:grid-cols-2'>
              <StatPill
                label={t('Purchased amount')}
                value={formatMarketplaceQuotaUSD(order.purchased_quota)}
              />
              <StatPill
                label={t('Spent amount')}
                value={formatMarketplaceQuotaUSD(order.spent_quota)}
              />
              <StatPill
                label={t('Refundable amount')}
                value={formatMarketplaceQuotaUSD(order.remaining_quota)}
              />
            </div>
            <div className='mt-4 space-y-2'>
              <MetricProgress
                label={t('Usage')}
                value={marketplaceFixedOrderUsage(order)}
              />
              <div className='grid gap-2 sm:grid-cols-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={!canProbeAndReleaseOrder(order) || probeMutation.isPending}
                  onClick={() => probeMutation.mutate(order.id)}
                >
                  {probeMutation.isPending && probeMutation.variables === order.id
                    ? t('Detecting...')
                    : t('Detect')}
                </Button>
                <Button
                  type='button'
                  variant='destructive'
                  size='sm'
                  disabled={!canReleaseOrder(order) || releaseMutation.isPending}
                  onClick={() => setReleaseOrderId(order.id)}
                >
                  {releaseMutation.isPending &&
                  releaseMutation.variables === order.id
                    ? t('Releasing...')
                    : t('Release order')}
                </Button>
              </div>
              <CallSnippet
                orderId={order.id}
                compact
                onEditFixedOrderBindings={() => openBindingEditor(order.id)}
              />
              <div className='space-y-1.5'>
                <div className='text-muted-foreground text-xs'>
                  {t('Bound tokens')}
                </div>
                <BoundTokenDropdown
                  tokens={tokensBoundToOrder(order.id)}
                  revealedTokenKeys={revealedTokenKeys}
                  resolvedTokenKeys={resolvedTokenKeys}
                  loadingTokenKeys={loadingTokenKeys}
                  copiedText={copiedText}
                  onToggleVisibility={toggleBoundTokenVisibility}
                  onCopyKey={copyBoundTokenKey}
                  onCopyConnectionInfo={copyBoundTokenConnectionInfo}
                />
              </div>
            </div>
          </div>
        ))}
      </div>
      {orders.length === 0 ? <EmptyLine label={t('No fixed orders')} /> : null}
      <ListFooter loading={ordersQuery.isLoading} total={total} />
      <Dialog open={bindingOpen} onOpenChange={setBindingOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Edit token bindings')}</DialogTitle>
            <DialogDescription>
              {t('Select one or more console tokens for this fixed order.')}
            </DialogDescription>
          </DialogHeader>
          <div className='max-h-[50vh] space-y-2 overflow-y-auto pr-1'>
            {enabledTokens.length === 0 ? (
              <EmptyLine label={t('No enabled tokens available')} />
            ) : (
              enabledTokens.map((token) => {
                const checked = bindingTokenSelection.includes(token.id)
                return (
                  <label
                    key={token.id}
                    className='flex cursor-pointer items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm'
                  >
                    <div className='flex min-w-0 items-center gap-3'>
                      <Checkbox
                        checked={checked}
                        onCheckedChange={() => toggleBindingToken(token.id)}
                      />
                      <div className='min-w-0'>
                        <div className='font-medium'>{token.name}</div>
                        <div className='text-muted-foreground truncate text-xs'>
                          sk-{token.key}
                        </div>
                      </div>
                    </div>
                    <StatusBadge
                      label={checked ? t('Bound') : t('Not bound')}
                      variant={checked ? 'success' : 'neutral'}
                      copyable={false}
                    />
                  </label>
                )
              })
            )}
          </div>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setBindingOpen(false)}
              disabled={bindMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              onClick={saveBindingSelection}
              disabled={!bindingOrderId || bindMutation.isPending}
            >
              {bindMutation.isPending
                ? t('Saving...')
                : t('Save token bindings')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        destructive
        open={releaseOrderId !== null}
        onOpenChange={(open) => {
          if (!open && !probeMutation.isPending) {
            setReleaseOrderId(null)
          }
        }}
        title={t('Release fixed order')}
        desc={t(
          'This order has failed the latest detection. Releasing it will refund the remaining amount and remove token bindings.'
        )}
        confirmText={
          releaseMutation.isPending ? t('Releasing...') : t('Release order')
        }
        isLoading={releaseMutation.isPending}
        disabled={!releaseOrderId}
        handleConfirm={confirmProbeAndReleaseOrder}
      />
    </div>
  )
}
