import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BadgeDollarSign,
  CircleDollarSign,
  Loader2,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { StatusBadge } from '@/components/status-badge'
import { CHANNEL_TYPE_OPTIONS } from '@/features/channels/constants'
import { getSystemOptions } from '@/features/system-settings/api'
import { getOptionValue } from '@/features/system-settings/hooks/use-system-options'
import {
  createSellerMarketplaceCredential,
  getSellerMarketplaceIncome,
  listSellerMarketplaceCredentials,
  listSellerMarketplacePricedModels,
  listSellerMarketplaceSettlements,
  probeSellerMarketplaceCredential,
  releaseSellerMarketplaceSettlements,
  setSellerMarketplaceCredentialEnabled,
  setSellerMarketplaceCredentialListed,
  testSellerMarketplaceCredential,
  updateSellerMarketplaceCredential,
} from '../api'
import {
  formatMarketplacePricePoint,
  marketplaceCredentialProxy,
  marketplaceCredentialSettingWithoutProxy,
  marketplaceProbeInProgress,
  marketplaceProbeStatusLabel,
  marketplaceQuotaDisplayLabel,
  marketplaceQuotaInputStep,
  marketplaceQuotaToDisplayAmount,
  marketplaceStatusLabel,
  marketplaceStatusVariant,
  splitMarketplaceModels,
} from '../lib'
import type {
  ApiResponse,
  MarketplaceCredential,
  MarketplaceCredentialFormValues,
  MarketplacePricePoint,
  MarketplacePricingItem,
  MarketplaceSettlement,
} from '../types'
import { EmptyLine, MarketplaceVendor, ModelBadges, StatPill } from './shared'
import {
  MARKETPLACE_PAGE_SIZE,
  defaultCredentialForm,
  unwrapPage,
} from './shared-data'

const MARKETPLACE_LIMIT_DEFAULTS = {
  MarketplaceMaxCredentialConcurrency: 5,
}

function normalizeMarketplaceMaxConcurrency(value: number) {
  const parsed = Math.floor(Number(value))
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1
}

function clampMarketplaceConcurrency(value: number, maxConcurrency: number) {
  const parsed = Math.floor(Number(value))
  if (!Number.isFinite(parsed) || parsed < 1) {
    return 1
  }
  return Math.min(parsed, normalizeMarketplaceMaxConcurrency(maxConcurrency))
}

export function SellerTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<MarketplaceCredential | null>(null)
  const [formValues, setFormValues] = useState<MarketplaceCredentialFormValues>(
    defaultCredentialForm
  )

  const credentialsQuery = useQuery({
    queryKey: ['marketplace', 'seller-credentials'],
    queryFn: () =>
      listSellerMarketplaceCredentials({
        p: 1,
        page_size: MARKETPLACE_PAGE_SIZE,
      }),
    placeholderData: (previous) => previous,
  })
  const incomeQuery = useQuery({
    queryKey: ['marketplace', 'seller-income'],
    queryFn: getSellerMarketplaceIncome,
  })
  const settlementsQuery = useQuery({
    queryKey: ['marketplace', 'seller-settlements'],
    queryFn: () =>
      listSellerMarketplaceSettlements({
        p: 1,
        page_size: 8,
      }),
    placeholderData: (previous) => previous,
  })
  const pricedModelsQuery = useQuery({
    queryKey: ['marketplace', 'seller-priced-models'],
    queryFn: listSellerMarketplacePricedModels,
    enabled: dialogOpen,
  })
  const marketplaceSettingsQuery = useQuery({
    queryKey: ['system-options'],
    queryFn: getSystemOptions,
    staleTime: 5 * 60 * 1000,
  })

  const maxCredentialConcurrency = useMemo(() => {
    const limits = getOptionValue(
      marketplaceSettingsQuery.data?.data,
      MARKETPLACE_LIMIT_DEFAULTS
    )
    return normalizeMarketplaceMaxConcurrency(
      limits.MarketplaceMaxCredentialConcurrency
    )
  }, [marketplaceSettingsQuery.data?.data])

  const saveMutation = useMutation({
    mutationFn: () => {
      const payloadValues = {
        ...formValues,
        concurrency_limit: clampMarketplaceConcurrency(
          formValues.concurrency_limit,
          maxCredentialConcurrency
        ),
      }
      return editing
        ? updateSellerMarketplaceCredential(editing.id, payloadValues)
        : createSellerMarketplaceCredential(payloadValues)
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Save failed'))
        return
      }
      toast.success(t('Saved'))
      setDialogOpen(false)
      setEditing(null)
      setFormValues(defaultCredentialForm)
      void queryClient.invalidateQueries({ queryKey: ['marketplace'] })
    },
  })

  const actionMutation = useMutation<
    ApiResponse<unknown>,
    Error,
    {
      credentialId: number
      action: 'list' | 'unlist' | 'enable' | 'disable' | 'test'
    }
  >({
    mutationFn: async (request) => {
      if (request.action === 'list') {
        return (await setSellerMarketplaceCredentialListed(
          request.credentialId,
          true
        )) as ApiResponse<unknown>
      }
      if (request.action === 'unlist') {
        return (await setSellerMarketplaceCredentialListed(
          request.credentialId,
          false
        )) as ApiResponse<unknown>
      }
      if (request.action === 'enable') {
        return (await setSellerMarketplaceCredentialEnabled(
          request.credentialId,
          true
        )) as ApiResponse<unknown>
      }
      if (request.action === 'disable') {
        return (await setSellerMarketplaceCredentialEnabled(
          request.credentialId,
          false
        )) as ApiResponse<unknown>
      }
      return (await testSellerMarketplaceCredential(
        request.credentialId
      )) as ApiResponse<unknown>
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Action failed'))
        return
      }
      toast.success(t('Done'))
      void queryClient.invalidateQueries({ queryKey: ['marketplace'] })
    },
  })

  const probeMutation = useMutation({
    mutationFn: probeSellerMarketplaceCredential,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Detection failed'))
        return
      }
      toast.success(t('Detection queued'))
      void queryClient.invalidateQueries({ queryKey: ['marketplace'] })
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Detection failed'))
    },
  })

  const releaseMutation = useMutation({
    mutationFn: releaseSellerMarketplaceSettlements,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Release failed'))
        return
      }
      toast.success(t('Income released'))
      void queryClient.invalidateQueries({ queryKey: ['marketplace'] })
    },
  })

  const credentials = unwrapPage(credentialsQuery.data).items
  const settlements = unwrapPage(settlementsQuery.data).items
  const income = incomeQuery.data?.data

  const openCreate = () => {
    setEditing(null)
    setFormValues(defaultCredentialForm)
    setDialogOpen(true)
  }

  const openEdit = (credential: MarketplaceCredential) => {
    setEditing(credential)
    setFormValues({
      vendor_type: credential.vendor_type,
      api_key: '',
      base_url: credential.base_url ?? '',
      proxy: marketplaceCredentialProxy(credential.setting),
      other: credential.other ?? '',
      model_mapping: credential.model_mapping ?? '',
      status_code_mapping: credential.status_code_mapping ?? '',
      setting: marketplaceCredentialSettingWithoutProxy(credential.setting),
      param_override: credential.param_override ?? '',
      settings: credential.settings ?? '',
      models: credential.models,
      quota_mode: credential.quota_mode,
      quota_limit: marketplaceQuotaToDisplayAmount(credential.quota_limit),
      time_mode: credential.time_mode,
      time_limit_minutes: Math.ceil((credential.time_limit_seconds ?? 0) / 60),
      multiplier: credential.multiplier,
      concurrency_limit: clampMarketplaceConcurrency(
        credential.concurrency_limit,
        maxCredentialConcurrency
      ),
    })
    setDialogOpen(true)
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-col gap-3 rounded-md border p-4 md:flex-row md:items-center md:justify-between'>
        <div className='grid flex-1 gap-2 sm:grid-cols-4'>
          <StatPill
            label={t('Pending income')}
            value={formatQuota(income?.pending_income ?? 0)}
          />
          <StatPill
            label={t('Available income')}
            value={formatQuota(income?.available_income ?? 0)}
          />
          <StatPill
            label={t('Blocked income')}
            value={formatQuota(income?.blocked_income ?? 0)}
          />
          <StatPill
            label={t('Settlements')}
            value={income?.settlement_count ?? 0}
          />
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            onClick={() => releaseMutation.mutate()}
            disabled={releaseMutation.isPending}
          >
            <CircleDollarSign className='size-4' />
            {t('Release income')}
          </Button>
          <Button onClick={openCreate}>
            <Plus className='size-4' />
            {t('Escrow API key')}
          </Button>
        </div>
      </div>

      <div className='grid gap-3 xl:grid-cols-2'>
        {credentials.map((credential) => (
          <CredentialPanel
            key={credential.id}
            credential={credential}
            onEdit={openEdit}
            onAction={(action) =>
              actionMutation.mutate({ credentialId: credential.id, action })
            }
            onProbe={() => probeMutation.mutate(credential.id)}
            probePending={
              probeMutation.isPending && probeMutation.variables === credential.id
            }
          />
        ))}
      </div>
      {credentials.length === 0 ? (
        <EmptyLine label={t('No escrowed credentials')} />
      ) : null}
      <SettlementsList settlements={settlements} />
      <CredentialDialog
        open={dialogOpen}
        editing={editing}
        values={formValues}
        pricingItems={pricedModelsQuery.data?.data ?? []}
        pricingLoading={pricedModelsQuery.isFetching}
        maxConcurrency={maxCredentialConcurrency}
        pending={saveMutation.isPending}
        onOpenChange={setDialogOpen}
        onChange={setFormValues}
        onSubmit={() => saveMutation.mutate()}
      />
    </div>
  )
}

function CredentialPanel({
  credential,
  onEdit,
  onAction,
  onProbe,
  probePending,
}: {
  credential: MarketplaceCredential
  onEdit: (credential: MarketplaceCredential) => void
  onAction: (action: 'list' | 'unlist' | 'enable' | 'disable' | 'test') => void
  onProbe: () => void
  probePending: boolean
}) {
  const { t } = useTranslation()
  const probeInProgress = marketplaceProbeInProgress(credential.probe_status)
  const probeScore =
    Number(credential.probe_score_max) > 0
      ? `${credential.probe_score}/${credential.probe_score_max}`
      : t(marketplaceProbeStatusLabel(credential.probe_status || 'unscored'))
  return (
    <div className='rounded-md border p-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <MarketplaceVendor
          vendorType={credential.vendor_type}
          vendorName={credential.vendor_name_snapshot}
        />
        <div className='flex flex-wrap gap-2'>
          <StatusBadge
            label={t(marketplaceStatusLabel(credential.listing_status))}
            variant={
              credential.listing_status === 'listed' ? 'success' : 'neutral'
            }
            copyable={false}
          />
          <StatusBadge
            label={t(marketplaceStatusLabel(credential.service_status))}
            variant={marketplaceStatusVariant(credential.service_status)}
            copyable={false}
          />
          <StatusBadge
            label={t(marketplaceStatusLabel(credential.health_status))}
            variant={marketplaceStatusVariant(credential.health_status)}
            copyable={false}
          />
          <StatusBadge
            label={t(marketplaceStatusLabel(credential.route_status))}
            variant={marketplaceStatusVariant(credential.route_status)}
            copyable={false}
          />
        </div>
      </div>
      <div className='mt-4 space-y-3'>
        <ModelBadges models={credential.models} />
        <div className='grid gap-2 sm:grid-cols-3'>
          <StatPill
            label={t('Multiplier')}
            value={`${credential.multiplier}x`}
          />
          <StatPill label={t('Quota mode')} value={t(credential.quota_mode)} />
          <StatPill
            label={t('Time mode')}
            value={
              credential.time_mode === 'limited'
                ? `${Math.ceil(credential.time_limit_seconds / 60)} ${t('minutes')}`
                : t('Unlimited')
            }
          />
          <StatPill
            label={t('Concurrency')}
            value={credential.concurrency_limit}
          />
          <StatPill label={t('Probe score')} value={probeScore} />
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => onEdit(credential)}
          >
            {t('Edit')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() =>
              onAction(
                credential.listing_status === 'listed' ? 'unlist' : 'list'
              )
            }
          >
            {credential.listing_status === 'listed' ? (
              <PowerOff className='size-4' />
            ) : (
              <Power className='size-4' />
            )}
            {credential.listing_status === 'listed' ? t('Unlist') : t('List')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() =>
              onAction(
                credential.service_status === 'enabled' ? 'disable' : 'enable'
              )
            }
          >
            <Power className='size-4' />
            {credential.service_status === 'enabled'
              ? t('Disable')
              : t('Enable')}
          </Button>
          <Button variant='outline' size='sm' onClick={() => onAction('test')}>
            <RefreshCw className='size-4' />
            {t('Test')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            disabled={probeInProgress || probePending}
            onClick={onProbe}
          >
            <RefreshCw
              className={`size-4 ${probeInProgress || probePending ? 'animate-spin' : ''}`}
            />
            {probeInProgress || probePending ? t('Detecting...') : t('Detect')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function pricingPointFromPricingItem(
  item?: MarketplacePricingItem
): MarketplacePricePoint {
  if (!item) {
    return { quota_type: 'ratio', model_ratio: 0, configured: false }
  }
  const quotaType =
    item.quota_type === 1 || item.quota_type === 'price'
      ? 'price'
      : item.quota_type === 0
        ? 'ratio'
        : String(item.quota_type || 'ratio')
  const configured =
    item.configured ??
    (quotaType === 'price'
      ? Number(item.model_price) > 0
      : quotaType === 'tiered_expr' || quotaType === 'per_second'
        ? true
        : Number(item.model_ratio) > 0)
  return {
    quota_type: quotaType,
    billing_mode: item.billing_mode,
    billing_expr: item.billing_expr,
    model_price: Number(item.model_price) || 0,
    model_ratio: Number(item.model_ratio) || 0,
    completion_ratio: Number(item.completion_ratio) || 0,
    cache_ratio: item.cache_ratio,
    create_cache_ratio: item.create_cache_ratio,
    input_price_per_mtok: Number(item.input_price_per_mtok) || 0,
    output_price_per_mtok: Number(item.output_price_per_mtok) || 0,
    cache_read_price_per_mtok: item.cache_read_price_per_mtok,
    cache_write_price_per_mtok: item.cache_write_price_per_mtok,
    task_per_request_price: Number(item.task_per_request_price) || 0,
    task_per_second_price: Number(item.task_per_second_price) || 0,
    configured,
  }
}

function multiplyPricePoint(
  point: MarketplacePricePoint,
  multiplier: number
): MarketplacePricePoint {
  if (!point.configured) return point
  return {
    ...point,
    applied_multiplier: multiplier,
    model_price: (Number(point.model_price) || 0) * multiplier,
    model_ratio: (Number(point.model_ratio) || 0) * multiplier,
    input_price_per_mtok:
      (Number(point.input_price_per_mtok) || 0) * multiplier,
    output_price_per_mtok:
      (Number(point.output_price_per_mtok) || 0) * multiplier,
    cache_read_price_per_mtok:
      point.cache_read_price_per_mtok == null
        ? point.cache_read_price_per_mtok
        : Number(point.cache_read_price_per_mtok) * multiplier,
    cache_write_price_per_mtok:
      point.cache_write_price_per_mtok == null
        ? point.cache_write_price_per_mtok
        : Number(point.cache_write_price_per_mtok) * multiplier,
    task_per_request_price:
      (Number(point.task_per_request_price) || 0) * multiplier,
    task_per_second_price:
      (Number(point.task_per_second_price) || 0) * multiplier,
  }
}

function buildSellerPricePreview(
  models: string,
  pricingItems: MarketplacePricingItem[],
  multiplier: number
) {
  const pricingByModel = new Map(
    pricingItems.map((item) => [
      item.model_name || item.id || '',
      pricingPointFromPricingItem(item),
    ])
  )
  return splitMarketplaceModels(models).map((model) => {
    const official =
      pricingByModel.get(model) ?? pricingPointFromPricingItem(undefined)
    return {
      model,
      official,
      buyer: multiplyPricePoint(official, multiplier),
    }
  })
}

function CredentialDialog({
  open,
  editing,
  values,
  pricingItems,
  pricingLoading,
  maxConcurrency,
  pending,
  onOpenChange,
  onChange,
  onSubmit,
}: {
  open: boolean
  editing: MarketplaceCredential | null
  values: MarketplaceCredentialFormValues
  pricingItems: MarketplacePricingItem[]
  pricingLoading: boolean
  maxConcurrency: number
  pending: boolean
  onOpenChange: (open: boolean) => void
  onChange: (values: MarketplaceCredentialFormValues) => void
  onSubmit: () => void
}) {
  const { t } = useTranslation()
  const pricePreviewRows = useMemo(
    () =>
      buildSellerPricePreview(
        values.models,
        pricingItems,
        values.multiplier || 1
      ),
    [pricingItems, values.models, values.multiplier]
  )
  const setField = <K extends keyof MarketplaceCredentialFormValues>(
    key: K,
    value: MarketplaceCredentialFormValues[K]
  ) => onChange({ ...values, [key]: value })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>
            {editing ? t('Edit escrowed key') : t('Escrow API key')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Configure once. The same terms power order list filters and pool routing.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-4 sm:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label>{t('Vendor')}</Label>
            <Select
              value={String(values.vendor_type)}
              onValueChange={(value) => setField('vendor_type', Number(value))}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CHANNEL_TYPE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={String(option.value)}>
                    {t(option.label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-1.5'>
            <Label>{t('API key')}</Label>
            <Input
              type='password'
              value={values.api_key}
              placeholder={
                editing ? t('Leave blank to keep current key') : 'sk-...'
              }
              onChange={(event) => setField('api_key', event.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('API URL / Base URL')}</Label>
            <Input
              value={values.base_url}
              placeholder='https://api.openai.com'
              onChange={(event) => setField('base_url', event.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Proxy Address')}</Label>
            <Input
              value={values.proxy}
              placeholder={t('socks5://user:pass@host:port')}
              onChange={(event) => setField('proxy', event.target.value)}
            />
            <p className='text-muted-foreground text-xs'>
              {t('Network proxy for this channel (supports socks5 protocol)')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Other')}</Label>
            <Input
              value={values.other}
              onChange={(event) => setField('other', event.target.value)}
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label>{t('Models')}</Label>
            <ModelPicker
              value={values.models}
              pricingItems={pricingItems}
              pricingLoading={pricingLoading}
              onChange={(models) => setField('models', models)}
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label>{t('Quota condition')}</Label>
            <QuotaCompoundControl
              value={values.quota_mode}
              limit={values.quota_limit}
              onModeChange={(value) =>
                onChange({
                  ...values,
                  quota_mode: value,
                  quota_limit: value === 'unlimited' ? 0 : values.quota_limit,
                })
              }
              onLimitChange={(value) => setField('quota_limit', value)}
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label>{t('Time condition')}</Label>
            <TimeCompoundControl
              value={values.time_mode}
              limit={values.time_limit_minutes}
              onModeChange={(value) =>
                onChange({
                  ...values,
                  time_mode: value,
                  time_limit_minutes:
                    value === 'unlimited' ? 0 : values.time_limit_minutes,
                })
              }
              onLimitChange={(value) => setField('time_limit_minutes', value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Multiplier')}</Label>
            <Input
              type='number'
              min='0.01'
              step='0.01'
              value={values.multiplier}
              onChange={(event) =>
                setField('multiplier', Number(event.target.value) || 1)
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Concurrency limit')}</Label>
            <Input
              type='number'
              min='1'
              max={maxConcurrency}
              value={values.concurrency_limit}
              onChange={(event) =>
                setField(
                  'concurrency_limit',
                  clampMarketplaceConcurrency(
                    Number(event.target.value),
                    maxConcurrency
                  )
                )
              }
            />
          </div>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label>{t('Pricing preview')}</Label>
            <div className='overflow-hidden rounded-md border'>
              <table className='w-full text-sm'>
                <thead className='bg-muted/50 text-muted-foreground'>
                  <tr>
                    <th className='px-3 py-2 text-left font-medium'>
                      {t('Model')}
                    </th>
                    <th className='px-3 py-2 text-left font-medium'>
                      {t('Official billing')}
                    </th>
                    <th className='px-3 py-2 text-left font-medium'>
                      {t('Buyer billing')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {pricePreviewRows.map((row) => (
                    <tr key={row.model} className='border-t'>
                      <td className='px-3 py-2 font-medium'>{row.model}</td>
                      <td className='px-3 py-2'>
                        {formatMarketplacePricePoint(row.official)}
                      </td>
                      <td className='px-3 py-2'>
                        {formatMarketplacePricePoint(row.buyer)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('Buyer billing = official billing x multiplier')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Model mapping JSON')}</Label>
            <Textarea
              value={values.model_mapping}
              rows={3}
              onChange={(event) =>
                setField('model_mapping', event.target.value)
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Status code mapping JSON')}</Label>
            <Textarea
              value={values.status_code_mapping}
              rows={3}
              onChange={(event) =>
                setField('status_code_mapping', event.target.value)
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Channel setting JSON')}</Label>
            <Textarea
              value={values.setting}
              rows={3}
              onChange={(event) => setField('setting', event.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Other settings JSON')}</Label>
            <Textarea
              value={values.settings}
              rows={3}
              onChange={(event) => setField('settings', event.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Param override JSON')}</Label>
            <Textarea
              value={values.param_override}
              rows={3}
              onChange={(event) =>
                setField('param_override', event.target.value)
              }
            />
          </div>
        </div>
        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={pending}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={onSubmit} disabled={pending}>
            {pending ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function QuotaCompoundControl({
  value,
  limit,
  onModeChange,
  onLimitChange,
}: {
  value: MarketplaceCredentialFormValues['quota_mode']
  limit: number
  onModeChange: (value: MarketplaceCredentialFormValues['quota_mode']) => void
  onLimitChange: (value: number) => void
}) {
  const { t } = useTranslation()
  const isUnlimited = value === 'unlimited'
  const inputLabel = marketplaceQuotaDisplayLabel()

  return (
    <div
      className={
        isUnlimited
          ? 'grid overflow-hidden rounded-md border'
          : 'grid overflow-hidden rounded-md border sm:grid-cols-[180px_1fr]'
      }
    >
      <Select
        value={value}
        onValueChange={(nextValue) =>
          onModeChange(
            nextValue as MarketplaceCredentialFormValues['quota_mode']
          )
        }
      >
        <SelectTrigger
          className={
            isUnlimited
              ? 'w-full rounded-none border-0 shadow-none focus-visible:ring-0'
              : 'w-full rounded-none border-0 border-b shadow-none focus-visible:ring-0 sm:border-r sm:border-b-0'
          }
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='unlimited'>{t('Unlimited')}</SelectItem>
          <SelectItem value='limited'>{t('Limited')}</SelectItem>
        </SelectContent>
      </Select>
      {value === 'unlimited' ? null : (
        <Input
          type='number'
          min='0'
          step={marketplaceQuotaInputStep()}
          value={limit}
          placeholder={inputLabel}
          onChange={(event) => onLimitChange(Number(event.target.value) || 0)}
          className='rounded-none border-0 shadow-none focus-visible:ring-0'
        />
      )}
    </div>
  )
}

function TimeCompoundControl({
  value,
  limit,
  onModeChange,
  onLimitChange,
}: {
  value: MarketplaceCredentialFormValues['time_mode']
  limit: number
  onModeChange: (value: MarketplaceCredentialFormValues['time_mode']) => void
  onLimitChange: (value: number) => void
}) {
  const { t } = useTranslation()
  const isUnlimited = value === 'unlimited'

  return (
    <div
      className={
        isUnlimited
          ? 'grid overflow-hidden rounded-md border'
          : 'grid overflow-hidden rounded-md border sm:grid-cols-[180px_1fr]'
      }
    >
      <Select
        value={value}
        onValueChange={(nextValue) =>
          onModeChange(
            nextValue as MarketplaceCredentialFormValues['time_mode']
          )
        }
      >
        <SelectTrigger
          className={
            isUnlimited
              ? 'w-full rounded-none border-0 shadow-none focus-visible:ring-0'
              : 'w-full rounded-none border-0 border-b shadow-none focus-visible:ring-0 sm:border-r sm:border-b-0'
          }
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='unlimited'>{t('Unlimited time')}</SelectItem>
          <SelectItem value='limited'>{t('Limited time')}</SelectItem>
        </SelectContent>
      </Select>
      {value === 'unlimited' ? null : (
        <Input
          type='number'
          min='1'
          value={limit}
          placeholder={t('Minutes')}
          onChange={(event) => onLimitChange(Number(event.target.value) || 0)}
          className='rounded-none border-0 shadow-none focus-visible:ring-0'
        />
      )}
    </div>
  )
}

function ModelPicker({
  value,
  pricingItems,
  pricingLoading,
  onChange,
}: {
  value: string
  pricingItems: MarketplacePricingItem[]
  pricingLoading: boolean
  onChange: (models: string) => void
}) {
  const { t } = useTranslation()
  const [customModel, setCustomModel] = useState('')
  const selectedModels = splitMarketplaceModels(value)
  const modelOptions = Array.from(
    new Set([
      ...pricingItems
        .map((item) => item.model_name || item.id || '')
        .filter(Boolean),
      ...selectedModels,
    ])
  )

  const setModels = (models: string[]) => {
    onChange(
      Array.from(
        new Set(models.map((model) => model.trim()).filter(Boolean))
      ).join(',')
    )
  }

  const toggleModel = (model: string) => {
    setModels(
      selectedModels.includes(model)
        ? selectedModels.filter((item) => item !== model)
        : [...selectedModels, model]
    )
  }

  const addCustomModel = () => {
    const nextModel = customModel.trim()
    if (!nextModel) return
    setModels([...selectedModels, nextModel])
    setCustomModel('')
  }

  return (
    <div className='space-y-2'>
      {pricingLoading ? (
        <div className='text-muted-foreground flex items-center gap-2 rounded-md border p-3 text-sm'>
          <Loader2 className='size-4 animate-spin' />
          {t('Loading models')}
        </div>
      ) : null}
      {modelOptions.length > 0 ? (
        <div className='flex flex-wrap gap-2'>
          {modelOptions.map((model) => (
            <Button
              key={model}
              type='button'
              size='sm'
              variant={selectedModels.includes(model) ? 'default' : 'outline'}
              onClick={() => toggleModel(model)}
            >
              {model}
            </Button>
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground rounded-md border p-3 text-sm'>
          {t('No models configured in model pricing')}
        </div>
      )}
      <div className='flex gap-2'>
        <Input
          value={customModel}
          placeholder={t('Custom model name')}
          onChange={(event) => setCustomModel(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              addCustomModel()
            }
          }}
        />
        <Button type='button' variant='outline' onClick={addCustomModel}>
          {t('Add')}
        </Button>
      </div>
    </div>
  )
}

function SettlementsList({
  settlements,
}: {
  settlements: MarketplaceSettlement[]
}) {
  const { t } = useTranslation()
  return (
    <div className='rounded-md border p-4'>
      <div className='mb-3 flex items-center gap-2 font-medium'>
        <BadgeDollarSign className='size-4' />
        {t('Recent settlements')}
      </div>
      <div className='space-y-2'>
        {settlements.map((settlement) => (
          <div
            key={settlement.id}
            className='grid gap-2 border-b py-2 text-sm last:border-b-0 sm:grid-cols-5'
          >
            <div>#{settlement.id}</div>
            <div>{t(settlement.source_type)}</div>
            <div>{formatQuota(settlement.seller_income)}</div>
            <StatusBadge
              label={t(marketplaceStatusLabel(settlement.status))}
              variant={marketplaceStatusVariant(settlement.status)}
              copyable={false}
            />
            <div className='text-muted-foreground'>
              {formatTimestampToDate(settlement.created_at)}
            </div>
          </div>
        ))}
        {settlements.length === 0 ? (
          <EmptyLine label={t('No settlements')} />
        ) : null}
      </div>
    </div>
  )
}
