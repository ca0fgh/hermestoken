import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserModels, getUserTokenGroups } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
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
import { DEFAULT_GROUP, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  apiKeyFormSchema,
  type ApiKeyFormValues,
  API_KEY_FORM_DEFAULT_VALUES,
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
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)

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
    () =>
      fixedOrdersData?.success ? (fixedOrdersData.data?.items ?? []) : [],
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

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(apiKeyFormSchema),
    defaultValues: API_KEY_FORM_DEFAULT_VALUES,
  })
  const marketplaceRouteEnabled = form.watch('marketplace_route_enabled')
  const marketplacePoolRouteEnabled = normalizeMarketplaceRouteEnabled(
    marketplaceRouteEnabled
  ).includes('pool')

  useEffect(() => {
    if (!open || groups.length === 0) return
    const currentGroup = form.getValues('group')
    if (currentGroup && !groups.some((group) => group.value === currentGroup)) {
      form.setValue('group', DEFAULT_GROUP, { shouldDirty: false })
    }
  }, [open, form, groups])

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
      // For update, fetch fresh data
      getApiKey(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformApiKeyToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(API_KEY_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

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
        className='flex w-full flex-col sm:max-w-[600px]'
      >
        <SheetHeader className='text-start'>
          <SheetTitle>
            {isUpdate ? t('Update API Key') : t('Create API Key')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the API key by providing necessary info.')
              : t('Add a new API key by providing necessary info.')}{' '}
            {t("Click save when you're done.")}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='api-key-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-6 overflow-y-auto px-4'
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
                  <FormDescription>
                    {t('Auto group enables circuit breaker mechanism')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {form.watch('group') === 'auto' && (
              <FormField
                control={form.control}
                name='cross_group_retry'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Cross-group retry')}
                      </FormLabel>
                      <FormDescription>
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
              name='marketplace_fixed_order_ids'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Marketplace fixed order binding')}</FormLabel>
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
                                route: t(MARKETPLACE_ROUTE_ORDER_LABELS[route]),
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

            <FormField
              control={form.control}
              name='unlimited_quota'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Unlimited Quota')}
                    </FormLabel>
                    <FormDescription>
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

            {!form.watch('unlimited_quota') && (
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
              name='expired_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Expiration Time')}</FormLabel>
                  <div className='space-y-2'>
                    <FormControl>
                      <DateTimePicker
                        value={field.value}
                        onChange={field.onChange}
                        placeholder={t('Never expires')}
                      />
                    </FormControl>
                    <div className='flex gap-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 0, 0)}
                      >
                        {t('Never')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(1, 0, 0)}
                      >
                        {t('1 Month')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 1, 0)}
                      >
                        {t('1 Day')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 0, 1)}
                      >
                        {t('1 Hour')}
                      </Button>
                    </div>
                  </div>
                  <FormDescription>
                    {t('Leave empty for never expires')}
                  </FormDescription>
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

            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <CollapsibleTrigger asChild>
                <Button
                  type='button'
                  variant='outline'
                  className='flex w-full items-center justify-between'
                >
                  <span className='font-medium'>{t('Advanced Options')}</span>
                  <ChevronDown
                    className={`h-4 w-4 transition-transform duration-200 ${
                      advancedOpen ? 'rotate-180' : ''
                    }`}
                  />
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className='space-y-6 pt-6'>
                <FormField
                  control={form.control}
                  name='model_limits'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Model Limits')}</FormLabel>
                      <FormControl>
                        <MultiSelect
                          options={models.map((m) => ({ label: m, value: m }))}
                          selected={field.value}
                          onChange={field.onChange}
                          placeholder={t('Select models (empty for allow all)')}
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
                      <FormLabel>{t('IP Whitelist (supports CIDR)')}</FormLabel>
                      <FormControl>
                        <Textarea
                          {...field}
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
              </CollapsibleContent>
            </Collapsible>
          </form>
        </Form>
        <SheetFooter className='gap-2'>
          <SheetClose asChild>
            <Button variant='outline'>{t('Close')}</Button>
          </SheetClose>
          <Button form='api-key-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
