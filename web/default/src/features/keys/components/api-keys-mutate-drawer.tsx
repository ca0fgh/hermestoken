import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowDown,
  ArrowRightLeft,
  ArrowUp,
  ChevronDown,
  KeyRound,
  Settings2,
  WalletCards,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserModels, getUserTokenGroups } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { DateTimePicker } from '@/components/datetime-picker'
import { MultiSelect } from '@/components/multi-select'
import { marketplaceStatusLabel } from '@/features/marketplace/lib'
import {
  createApiKey,
  updateApiKey,
  getApiKey,
  getApiKeyMarketplaceFixedOrders,
} from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  getApiKeyFormSchema,
  type ApiKeyFormValues,
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
  transformApiKeyToFormDefaults,
} from '../lib'
import {
  MARKETPLACE_ROUTE_ORDER_VALUES,
  normalizeMarketplaceRouteEnabled,
  normalizeMarketplaceRouteOrder,
  type ApiKey,
  type MarketplaceRouteOrderItem,
} from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'

type ApiKeyMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ApiKey
  side?: 'left' | 'right'
}

type ApiKeyFormSectionProps = {
  title: string
  description: string
  icon: LucideIcon
  children: ReactNode
}

function ApiKeyFormSection(props: ApiKeyFormSectionProps) {
  const Icon = props.icon

  return (
    <section className='bg-card rounded-lg border'>
      <div className='flex items-center gap-2.5 border-b px-3 py-2.5 sm:gap-3 sm:px-4 sm:py-3'>
        <div className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg border sm:size-10'>
          <Icon className='size-4 sm:size-5' />
        </div>
        <div className='min-w-0'>
          <h3 className='text-sm leading-none font-medium'>{props.title}</h3>
          <p className='text-muted-foreground mt-0.5 text-xs sm:mt-1'>
            {props.description}
          </p>
        </div>
      </div>
      <div className='space-y-3 p-3 sm:space-y-4 sm:p-4'>{props.children}</div>
    </section>
  )
}

const MARKETPLACE_ROUTE_ORDER_LABELS: Record<
  MarketplaceRouteOrderItem,
  string
> = {
  fixed_order: 'Marketplace fixed order',
  group: 'Normal group',
  pool: 'Order pool',
}

function moveMarketplaceRouteOrderItem(
  current: MarketplaceRouteOrderItem[] | undefined,
  index: number,
  direction: -1 | 1
): MarketplaceRouteOrderItem[] {
  const next = normalizeMarketplaceRouteOrder(current)
  const targetIndex = index + direction
  if (targetIndex < 0 || targetIndex >= next.length) return next

  const moved = [...next]
  ;[moved[index], moved[targetIndex]] = [moved[targetIndex], moved[index]]

  return normalizeMarketplaceRouteOrder(moved)
}

function toggleMarketplaceRouteEnabled(
  current: MarketplaceRouteOrderItem[] | undefined,
  route: MarketplaceRouteOrderItem,
  enabled: boolean
): MarketplaceRouteOrderItem[] {
  const currentSet = new Set(normalizeMarketplaceRouteEnabled(current))
  if (enabled) {
    currentSet.add(route)
  } else {
    currentSet.delete(route)
  }
  return MARKETPLACE_ROUTE_ORDER_VALUES.filter((value) => currentSet.has(value))
}

export function ApiKeysMutateDrawer({
  open,
  onOpenChange,
  currentRow,
  side = 'right',
}: ApiKeyMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { setCurrentRow, triggerRefresh } = useApiKeys()
  const queryClient = useQueryClient()
  const { status } = useStatus()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const defaultUseAutoGroup = status?.default_use_auto_group === true

  // Fetch models
  const { data: modelsData } = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  })

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ['user-token-groups'],
    queryFn: getUserTokenGroups,
    staleTime: 5 * 60 * 1000,
  })

  const { data: fixedOrdersData } = useQuery({
    queryKey: ['api-key-marketplace-fixed-orders'],
    queryFn: () => getApiKeyMarketplaceFixedOrders({ p: 1, page_size: 100 }),
    enabled: open,
    retry: false,
    staleTime: 30 * 1000,
  })

  const models = modelsData?.data || []
  const marketplaceFixedOrders = useMemo(
    () => (fixedOrdersData?.success ? (fixedOrdersData.data?.items ?? []) : []),
    [fixedOrdersData?.data?.items, fixedOrdersData?.success]
  )
  const marketplaceFixedOrderOptions = useMemo(
    () =>
      marketplaceFixedOrders
        .filter((order) => order.status === 'active')
        .map((order) => ({
          label: `#${order.id} · ${t('Credential')} #${order.credential_id} · ${t(marketplaceStatusLabel(order.status))} · ${formatQuota(order.remaining_quota)}`,
          value: String(order.id),
        })),
    [marketplaceFixedOrders, t]
  )
  const marketplaceFixedOrderOptionValues = useMemo(
    () => new Set(marketplaceFixedOrderOptions.map((option) => option.value)),
    [marketplaceFixedOrderOptions]
  )
  const groupsRaw = groupsData?.data || {}
  const groups: ApiKeyGroupOption[] = Object.entries(groupsRaw).map(
    ([key, info]) => ({
      value: key,
      label: key,
      desc: info.desc || key,
      ratio: info.ratio,
    })
  )
  const backendHasAuto = groups.some((group) => group.value === 'auto')
  const schema = getApiKeyFormSchema(t)

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getApiKeyFormDefaultValues(defaultUseAutoGroup),
  })
  const marketplaceRouteEnabled = form.watch('marketplace_route_enabled')
  const marketplacePoolRouteEnabled = normalizeMarketplaceRouteEnabled(
    marketplaceRouteEnabled
  ).includes('pool')

  useEffect(() => {
    if (!open || !fixedOrdersData?.success) return

    const currentIds = form.getValues('marketplace_fixed_order_ids') ?? []
    const visibleIds = currentIds.filter((id) =>
      marketplaceFixedOrderOptionValues.has(String(id))
    )
    const changed =
      currentIds.length !== visibleIds.length ||
      currentIds.some((id, index) => id !== visibleIds[index])

    if (!changed) return
    form.setValue('marketplace_fixed_order_ids', visibleIds, {
      shouldDirty: false,
    })
    form.setValue('marketplace_fixed_order_id', visibleIds[0] ?? 0, {
      shouldDirty: false,
    })
  }, [fixedOrdersData?.success, form, marketplaceFixedOrderOptionValues, open])

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      getApiKey(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformApiKeyToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      form.reset(
        getApiKeyFormDefaultValues(defaultUseAutoGroup && backendHasAuto)
      )
    }
  }, [open, isUpdate, currentRow, form, defaultUseAutoGroup, backendHasAuto])

  // Correct group after groups load: if the form value is not in available groups, fall back
  useEffect(() => {
    if (groups.length === 0) return
    const currentGroup = form.getValues('group')
    if (currentGroup && !groups.some((g) => g.value === currentGroup)) {
      const fallback =
        groups.find((g) => g.value === 'default')?.value ??
        groups[0]?.value ??
        ''
      form.setValue('group', fallback)
      if (currentGroup === 'auto') {
        form.setValue('cross_group_retry', false)
      }
    }
  }, [groups, form])

  const onSubmit = async (data: ApiKeyFormValues) => {
    setIsSubmitting(true)
    try {
      const visibleFixedOrderIds = fixedOrdersData?.success
        ? (data.marketplace_fixed_order_ids ?? []).filter((id) =>
            marketplaceFixedOrderOptionValues.has(String(id))
          )
        : (data.marketplace_fixed_order_ids ?? [])
      const basePayload = transformFormDataToPayload({
        ...data,
        marketplace_fixed_order_id: visibleFixedOrderIds[0] ?? 0,
        marketplace_fixed_order_ids: visibleFixedOrderIds,
      })

      if (isUpdate && currentRow) {
        const result = await updateApiKey({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          if (result.data) {
            const updatedApiKey = result.data
            setCurrentRow((previous) =>
              previous?.id === updatedApiKey.id
                ? { ...previous, ...updatedApiKey }
                : previous
            )
          }
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          onOpenChange(false)
          triggerRefresh()
          void queryClient.invalidateQueries({ queryKey: ['keys'] })
          void queryClient.invalidateQueries({
            queryKey: ['marketplace', 'buyer-console-tokens'],
          })
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
      } else {
        // Create mode - handle batch creation
        const count = data.tokenCount || 1
        let successCount = 0

        for (let i = 0; i < count; i++) {
          const result = await createApiKey({
            ...basePayload,
            name:
              i === 0 && data.name
                ? data.name
                : `${data.name || 'default'}-${Math.random().toString(36).slice(2, 8)}`,
          })
          if (result.success) {
            successCount++
          } else {
            toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
            break
          }
        }

        if (successCount > 0) {
          toast.success(
            t('Successfully created {{count}} API Key(s)', {
              count: successCount,
            })
          )
          onOpenChange(false)
          triggerRefresh()
          void queryClient.invalidateQueries({ queryKey: ['keys'] })
          void queryClient.invalidateQueries({
            queryKey: ['marketplace', 'buyer-console-tokens'],
          })
        }
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const onInvalid: SubmitErrorHandler<ApiKeyFormValues> = () => {
    toast.error(t('Please fix the highlighted fields before saving'))
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    if (months === 0 && days === 0 && hours === 0) {
      form.setValue('expired_time', undefined)
      return
    }

    const now = new Date()
    now.setMonth(now.getMonth() + months)
    now.setDate(now.getDate() + days)
    now.setHours(now.getHours() + hours)

    form.setValue('expired_time', now)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })
  const selectedGroup = form.watch('group')
  const unlimitedQuota = form.watch('unlimited_quota')

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent
        side={side}
        className='bg-background flex !h-dvh !w-screen max-w-none gap-0 overflow-hidden p-0 sm:!w-full sm:!max-w-[620px]'
      >
        <SheetHeader className='bg-background border-b px-4 py-3 text-start sm:px-5 sm:py-4'>
          <SheetTitle className='text-base sm:text-lg'>
            {isUpdate ? t('Update API Key') : t('Create API Key')}
          </SheetTitle>
          <SheetDescription className='pr-6 text-xs sm:text-sm'>
            {isUpdate
              ? t('Update the API key by providing necessary info.')
              : t('Add a new API key by providing necessary info.')}{' '}
            {t("Click save when you're done.")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='api-key-form'
            onSubmit={form.handleSubmit(onSubmit, onInvalid)}
            className='min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain px-3 py-3 sm:space-y-4 sm:px-4 sm:py-4'
          >
            <ApiKeyFormSection
              title={t('Basic Information')}
              description={t('Set API key basic information')}
              icon={KeyRound}
            >
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <ApiKeyGroupCombobox
                        options={groups}
                        value={field.value}
                        onValueChange={field.onChange}
                        placeholder={t('Select a group')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='cross_group_retry'
                  render={({ field }) => (
                    <FormItem className='flex min-h-16 flex-row items-center justify-between gap-3 rounded-lg border px-3 py-2.5 sm:min-h-20 sm:gap-4 sm:px-4 sm:py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Cross-group retry')}
                        </FormLabel>
                        <FormDescription className='line-clamp-2 text-xs sm:line-clamp-none'>
                          {t(
                            'When enabled, if channels in the current group fail, it will try channels in the next group in order.'
                          )}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={!!field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='expired_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'>
                      <FormControl>
                        <DateTimePicker
                          value={field.value}
                          onChange={field.onChange}
                          placeholder={t('Never expires')}
                          className='min-w-0 [&_input[type=time]]:w-24 sm:[&_input[type=time]]:w-32'
                        />
                      </FormControl>
                      <div className='grid grid-cols-4 gap-2 sm:flex'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 0)}
                        >
                          {t('Never')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(1, 0, 0)}
                        >
                          {t('1 Month')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 1, 0)}
                        >
                          {t('1 Day')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 1)}
                        >
                          {t('1 Hour')}
                        </Button>
                      </div>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && (
                <FormField
                  control={form.control}
                  name='tokenCount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quantity')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          placeholder={t('Number of keys to create')}
                          onChange={(e) =>
                            field.onChange(parseInt(e.target.value, 10) || 1)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Create multiple API keys at once (random suffix will be added to names)'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </ApiKeyFormSection>

            <ApiKeyFormSection
              title={t('Quota Settings')}
              description={t('Set quota amount and limits')}
              icon={WalletCards}
            >
              {!unlimitedQuota && (
                <FormField
                  control={form.control}
                  name='remain_quota_dollars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{quotaLabel}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          step={tokensOnly ? 1 : 0.01}
                          placeholder={quotaPlaceholder}
                          onChange={(e) =>
                            field.onChange(parseFloat(e.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {tokensOnly
                          ? t('Enter the quota amount in tokens')
                          : t('Enter the quota amount in {{currency}}', {
                              currency: currencyLabel,
                            })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='unlimited_quota'
                render={({ field }) => (
                  <FormItem className='flex min-h-16 flex-row items-center justify-between gap-3 rounded-lg border px-3 py-2.5 sm:min-h-20 sm:gap-4 sm:px-4 sm:py-3'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-sm'>
                        {t('Unlimited Quota')}
                      </FormLabel>
                      <FormDescription className='text-xs'>
                        {t('Enable unlimited quota for this API key')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </ApiKeyFormSection>

            <ApiKeyFormSection
              title={t('Marketplace Routing')}
              description={t('Bind fixed orders and set token route priority')}
              icon={ArrowRightLeft}
            >
              <FormField
                control={form.control}
                name='marketplace_fixed_order_ids'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Marketplace fixed order binding')}
                    </FormLabel>
                    <FormControl>
                      <MultiSelect
                        options={marketplaceFixedOrderOptions}
                        selected={(field.value ?? [])
                          .map(String)
                          .filter((value) =>
                            marketplaceFixedOrderOptionValues.has(value)
                          )}
                        onChange={(values) => {
                          const fixedOrderIds = values
                            .map((value) => Number(value))
                            .filter(
                              (value) => Number.isFinite(value) && value > 0
                            )
                          field.onChange(fixedOrderIds)
                          form.setValue(
                            'marketplace_fixed_order_id',
                            fixedOrderIds[0] ?? 0
                          )
                        }}
                        placeholder={t('Do not bind')}
                      />
                    </FormControl>
                    <div className='space-y-1'>
                      <FormDescription>
                        {(field.value ?? []).length > 0
                          ? t(
                              'Bound marketplace fixed order calls can use this token directly without manually entering the fixed order header.'
                            )
                          : t(
                              'Optional. Bind a purchased marketplace fixed order so fixed-order calls are tied to this token.'
                            )}
                      </FormDescription>
                      <FormDescription
                        className={
                          marketplacePoolRouteEnabled
                            ? 'text-green-600 dark:text-green-500'
                            : undefined
                        }
                      >
                        {marketplacePoolRouteEnabled
                          ? t('Order pool is active for this token')
                          : t('Order pool is not active for this token')}
                      </FormDescription>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='marketplace_route_order'
                render={({ field }) => {
                  const routeOrder = normalizeMarketplaceRouteOrder(field.value)
                  const enabledRoutes = normalizeMarketplaceRouteEnabled(
                    marketplaceRouteEnabled
                  )
                  const enabledSet = new Set(enabledRoutes)

                  return (
                    <FormItem>
                      <FormLabel>{t('Token route priority')}</FormLabel>
                      <FormControl>
                        <div className='space-y-2'>
                          {routeOrder.map((route, index) => (
                            <div
                              key={route}
                              className={`flex items-center gap-3 rounded-md border p-3 ${
                                enabledSet.has(route)
                                  ? 'bg-background'
                                  : 'bg-muted/40 text-muted-foreground'
                              }`}
                            >
                              <div className='flex h-8 w-8 shrink-0 items-center justify-center rounded-md border text-sm font-medium'>
                                {index + 1}
                              </div>
                              <div className='min-w-0 flex-1'>
                                <div className='truncate text-sm font-medium'>
                                  {t(MARKETPLACE_ROUTE_ORDER_LABELS[route])}
                                </div>
                              </div>
                              <Switch
                                checked={enabledSet.has(route)}
                                onCheckedChange={(checked) =>
                                  form.setValue(
                                    'marketplace_route_enabled',
                                    toggleMarketplaceRouteEnabled(
                                      enabledRoutes,
                                      route,
                                      checked
                                    ),
                                    {
                                      shouldDirty: true,
                                      shouldTouch: true,
                                      shouldValidate: true,
                                    }
                                  )
                                }
                                aria-label={t('Enable {{route}} route', {
                                  route: t(
                                    MARKETPLACE_ROUTE_ORDER_LABELS[route]
                                  ),
                                })}
                              />
                              <div className='flex shrink-0 gap-1'>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon'
                                  disabled={index === 0}
                                  onClick={() =>
                                    form.setValue(
                                      'marketplace_route_order',
                                      moveMarketplaceRouteOrderItem(
                                        routeOrder,
                                        index,
                                        -1
                                      ),
                                      {
                                        shouldDirty: true,
                                        shouldTouch: true,
                                        shouldValidate: true,
                                      }
                                    )
                                  }
                                  aria-label={t('Move route up')}
                                >
                                  <ArrowUp className='h-4 w-4' />
                                </Button>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon'
                                  disabled={index === routeOrder.length - 1}
                                  onClick={() =>
                                    form.setValue(
                                      'marketplace_route_order',
                                      moveMarketplaceRouteOrderItem(
                                        routeOrder,
                                        index,
                                        1
                                      ),
                                      {
                                        shouldDirty: true,
                                        shouldTouch: true,
                                        shouldValidate: true,
                                      }
                                    )
                                  }
                                  aria-label={t('Move route down')}
                                >
                                  <ArrowDown className='h-4 w-4' />
                                </Button>
                              </div>
                            </div>
                          ))}
                        </div>
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Enabled routes are tried in order. Default: marketplace fixed order, normal group, order pool.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )
                }}
              />
            </ApiKeyFormSection>

            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <section className='bg-card rounded-lg border'>
                <CollapsibleTrigger
                  render={
                    <button
                      type='button'
                      className='hover:bg-muted/50 flex w-full items-center gap-2.5 px-3 py-2.5 text-left transition-colors sm:gap-3 sm:px-4 sm:py-3'
                    />
                  }
                >
                  <div className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg border sm:size-10'>
                    <Settings2 className='size-4 sm:size-5' />
                  </div>
                  <div className='min-w-0 flex-1'>
                    <h3 className='text-sm leading-none font-medium'>
                      {t('Advanced Settings')}
                    </h3>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t('Set API key access restrictions')}
                    </p>
                  </div>
                  <ChevronDown
                    className={cn(
                      'text-muted-foreground size-4 shrink-0 transition-transform',
                      advancedOpen && 'rotate-180'
                    )}
                  />
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className='space-y-3 border-t p-3 sm:space-y-4 sm:p-4'>
                    <FormField
                      control={form.control}
                      name='model_limits'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Model Limits')}</FormLabel>
                          <FormControl>
                            <MultiSelect
                              options={models.map((m) => ({
                                label: m,
                                value: m,
                              }))}
                              selected={field.value}
                              onChange={field.onChange}
                              placeholder={t(
                                'Select models (empty for allow all)'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Limit which models can be used with this key')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='allow_ips'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('IP Whitelist (supports CIDR)')}
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              {...field}
                              className='min-h-20 resize-none'
                              placeholder={t(
                                'One IP per line (empty for no restriction)'
                              )}
                              rows={3}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Do not over-trust this feature. IP may be spoofed. Please use with nginx, CDN and other gateways.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </CollapsibleContent>
              </section>
            </Collapsible>
          </form>
        </Form>
        <SheetFooter className='bg-background grid grid-cols-2 gap-2 border-t px-3 py-3 sm:flex sm:flex-row sm:justify-end sm:px-5 sm:py-4'>
          <SheetClose
            render={<Button variant='outline' className='w-full sm:w-auto' />}
          >
            {t('Close')}
          </SheetClose>
          <Button
            type='button'
            onClick={form.handleSubmit(onSubmit)}
            disabled={isSubmitting}
            className='w-full sm:w-auto'
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
