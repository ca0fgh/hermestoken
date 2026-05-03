import { ListFilter, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { getLobeIcon } from '@/lib/lobe-icon'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { StatusBadge } from '@/components/status-badge'
import {
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPES,
} from '@/features/channels/constants'
import {
  getChannelTypeIcon,
  getChannelTypeLabel,
} from '@/features/channels/lib'
import {
  formatMarketplacePricePoint,
  formatMarketplaceQuotaUSD,
  marketplaceDisplayAmountToQuota,
  marketplaceQuotaInputStep,
  marketplaceQuotaToDisplayAmount,
  splitMarketplaceModels,
} from '../lib'
import type {
  MarketplaceOrderFilterRanges,
  MarketplaceOrderFilters,
  MarketplaceOrderListItem,
} from '../types'
import { defaultFilters } from './shared-data'

const ALL_VALUE = '__all__'

export function MarketplaceVendor({
  vendorType,
  vendorName,
}: {
  vendorType: number
  vendorName?: string
}) {
  const { t } = useTranslation()
  const label = vendorName || getChannelTypeLabel(vendorType)
  return (
    <div className='flex min-w-0 items-center gap-2'>
      <span className='bg-background flex size-8 shrink-0 items-center justify-center rounded-md border'>
        {getLobeIcon(`${getChannelTypeIcon(vendorType)}.Color`, 18)}
      </span>
      <div className='min-w-0'>
        <div className='truncate text-sm font-medium'>{t(label)}</div>
        <div className='text-muted-foreground text-xs'>
          {t(CHANNEL_TYPES[vendorType as keyof typeof CHANNEL_TYPES] ?? label)}
        </div>
      </div>
    </div>
  )
}

export function StatPill({
  label,
  value,
}: {
  label: string
  value: string | number
}) {
  return (
    <div className='rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 text-sm font-semibold'>{value}</div>
    </div>
  )
}

export function ModelBadges({ models }: { models: string }) {
  const modelList = splitMarketplaceModels(models)
  return (
    <div className='flex min-w-0 flex-wrap gap-1.5'>
      {modelList.slice(0, 4).map((model) => (
        <StatusBadge
          key={model}
          label={model}
          variant='blue'
          copyable={false}
        />
      ))}
      {modelList.length > 4 ? (
        <StatusBadge
          label={`+${modelList.length - 4}`}
          variant='neutral'
          copyable={false}
        />
      ) : null}
    </div>
  )
}

export function PricePreview({ item }: { item: MarketplaceOrderListItem }) {
  const preview = item.price_preview[0]
  return (
    <div className='space-y-1 text-sm'>
      <div className='font-medium'>
        {formatMarketplacePricePoint(preview?.buyer)}
      </div>
      <div className='text-muted-foreground text-xs'>
        {formatMarketplacePricePoint(preview?.official)} x {item.multiplier}
      </div>
    </div>
  )
}

export function MarketplaceFilters({
  filters,
  filterRanges,
  onChange,
  showQuotaTimeFilters = true,
}: {
  filters: MarketplaceOrderFilters
  filterRanges?: MarketplaceOrderFilterRanges
  onChange: (filters: MarketplaceOrderFilters) => void
  showQuotaTimeFilters?: boolean
}) {
  const { t } = useTranslation()
  const quotaOptions = buildMarketplaceFilterOptions('quota', t)
  const timeOptions = buildMarketplaceFilterOptions('time', t)

  const updateFilter = (patch: MarketplaceOrderFilters) => {
    onChange({ ...filters, ...patch, p: 1 })
  }
  const containerClassName = showQuotaTimeFilters
    ? 'flex flex-col gap-3 rounded-md border p-3 lg:flex-row lg:items-end'
    : 'flex flex-col gap-3 rounded-md border p-3 lg:inline-flex lg:w-fit lg:max-w-full lg:flex-row lg:items-end lg:self-start'
  const gridClassName = showQuotaTimeFilters
    ? 'grid flex-1 gap-3 sm:grid-cols-2 lg:grid-cols-7'
    : 'grid gap-3 sm:grid-cols-2 lg:w-[760px] lg:max-w-full lg:grid-cols-[minmax(180px,220px)_minmax(220px,1fr)_minmax(180px,220px)_minmax(180px,220px)]'

  return (
    <div className={containerClassName}>
      <div className={gridClassName}>
        <div className='space-y-1.5'>
          <Label>{t('Vendor')}</Label>
          <Select
            value={
              filters.vendor_type ? String(filters.vendor_type) : ALL_VALUE
            }
            onValueChange={(value) =>
              updateFilter({
                vendor_type: value === ALL_VALUE ? undefined : Number(value),
              })
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_VALUE}>{t('All vendors')}</SelectItem>
              {CHANNEL_TYPE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={String(option.value)}>
                  {t(option.label)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-1.5'>
          <Label>{t('Model')}</Label>
          <Input
            value={filters.model ?? ''}
            placeholder='gpt-4o-mini'
            onChange={(event) => updateFilter({ model: event.target.value })}
          />
        </div>
        {showQuotaTimeFilters ? (
          <>
            <div className='space-y-1.5'>
              <Label>{t('Quota mode')}</Label>
              <Select
                value={filters.quota_mode || ALL_VALUE}
                onValueChange={(value) => {
                  const quotaMode =
                    value === ALL_VALUE
                      ? ''
                      : (value as MarketplaceOrderFilters['quota_mode'])
                  updateFilter({
                    quota_mode: quotaMode,
                    ...(quotaMode === 'limited'
                      ? {}
                      : clearMarketplaceQuotaRangeFilters()),
                  })
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {quotaOptions.map((option) => (
                    <SelectItem
                      key={option.value}
                      value={option.value}
                      disabled={option.disabled}
                    >
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {renderQuotaRangeInputs(filters, filterRanges, updateFilter, t)}
            <div className='space-y-1.5'>
              <Label>{t('Time mode')}</Label>
              <Select
                value={filters.time_mode || ALL_VALUE}
                onValueChange={(value) => {
                  const timeMode =
                    value === ALL_VALUE
                      ? ''
                      : (value as MarketplaceOrderFilters['time_mode'])
                  updateFilter({
                    time_mode: timeMode,
                    ...(timeMode === 'limited'
                      ? {}
                      : clearMarketplaceTimeRangeFilters()),
                  })
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {timeOptions.map((option) => (
                    <SelectItem
                      key={option.value}
                      value={option.value}
                      disabled={option.disabled}
                    >
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {renderTimeRangeInputs(filters, filterRanges, updateFilter, t)}
          </>
        ) : null}
        {renderMultiplierRangeInputs(filters, filterRanges, updateFilter, t)}
        {renderConcurrencyRangeInputs(filters, filterRanges, updateFilter, t)}
      </div>
      <Button
        variant='outline'
        onClick={() =>
          onChange({
            ...defaultFilters,
            ...clearMarketplaceMultiplierRangeFilters(),
            ...clearMarketplaceConcurrencyRangeFilters(),
          })
        }
      >
        <ListFilter className='size-4' />
        {t('Reset')}
      </Button>
    </div>
  )
}

function formatMarketplaceRange(minLabel: string, maxLabel: string) {
  return minLabel === maxLabel ? minLabel : `${minLabel} - ${maxLabel}`
}

function formatMarketplaceQuotaRange(
  filterRanges?: MarketplaceOrderFilterRanges
) {
  return formatMarketplaceRange(
    formatMarketplaceQuotaUSD(filterRanges?.min_quota_limit),
    formatMarketplaceQuotaUSD(filterRanges?.max_quota_limit)
  )
}

function formatMarketplaceTimeValue(
  seconds: number | undefined,
  t: (key: string) => string
) {
  const minutes = Math.ceil((Number(seconds) || 0) / 60)
  return `${minutes} ${t('Minutes')}`
}

function formatMarketplaceTimeRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined,
  t: (key: string) => string
) {
  return formatMarketplaceRange(
    formatMarketplaceTimeValue(filterRanges?.min_time_limit_seconds, t),
    formatMarketplaceTimeValue(filterRanges?.max_time_limit_seconds, t)
  )
}

function formatMarketplaceMultiplierValue(value: number | undefined) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric <= 0) return '0x'
  return `${Number(numeric.toFixed(2))}x`
}

function formatMarketplaceMultiplierRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined
) {
  return formatMarketplaceRange(
    formatMarketplaceMultiplierValue(filterRanges?.min_multiplier),
    formatMarketplaceMultiplierValue(filterRanges?.max_multiplier)
  )
}

function formatMarketplaceConcurrencyValue(value: number | undefined) {
  const numeric = Math.round(Number(value) || 0)
  return String(Math.max(0, numeric))
}

function formatMarketplaceConcurrencyRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined
) {
  return formatMarketplaceRange(
    formatMarketplaceConcurrencyValue(filterRanges?.min_concurrency_limit),
    formatMarketplaceConcurrencyValue(filterRanges?.max_concurrency_limit)
  )
}

function quotaRangeDisplayValue(
  value: MarketplaceOrderFilters['min_quota_limit']
) {
  const numeric = Number(value) || 0
  return numeric > 0 ? marketplaceQuotaToDisplayAmount(numeric) : ''
}

function timeRangeDisplayValue(
  value: MarketplaceOrderFilters['min_time_limit_seconds']
) {
  const numeric = Number(value) || 0
  return numeric > 0 ? Math.ceil(numeric / 60) : ''
}

function multiplierRangeDisplayValue(
  value: MarketplaceOrderFilters['min_multiplier']
) {
  const numeric = Number(value) || 0
  return numeric > 0 ? numeric : ''
}

function concurrencyRangeDisplayValue(
  value: MarketplaceOrderFilters['min_concurrency_limit']
) {
  const numeric = Math.round(Number(value) || 0)
  return numeric > 0 ? numeric : ''
}

function clearMarketplaceQuotaRangeFilters(): MarketplaceOrderFilters {
  return {
    min_quota_limit: '',
    max_quota_limit: '',
  }
}

function clearMarketplaceTimeRangeFilters(): MarketplaceOrderFilters {
  return {
    min_time_limit_seconds: '',
    max_time_limit_seconds: '',
  }
}

function clearMarketplaceMultiplierRangeFilters(): MarketplaceOrderFilters {
  return {
    min_multiplier: '',
    max_multiplier: '',
  }
}

function clearMarketplaceConcurrencyRangeFilters(): MarketplaceOrderFilters {
  return {
    min_concurrency_limit: '',
    max_concurrency_limit: '',
  }
}

function marketplaceHasLimitedQuotaRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined
) {
  return (
    Number(filterRanges?.limited_quota_count) > 0 &&
    Number(filterRanges?.min_quota_limit) > 0 &&
    Number(filterRanges?.max_quota_limit) > 0
  )
}

function marketplaceHasLimitedTimeRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined
) {
  return (
    Number(filterRanges?.limited_time_count) > 0 &&
    Number(filterRanges?.min_time_limit_seconds) > 0 &&
    Number(filterRanges?.max_time_limit_seconds) > 0
  )
}

function marketplaceHasMultiplierRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined
) {
  return (
    Number(filterRanges?.min_multiplier) > 0 &&
    Number(filterRanges?.max_multiplier) > 0 &&
    Number(filterRanges?.max_multiplier) >= Number(filterRanges?.min_multiplier)
  )
}

function marketplaceHasConcurrencyRange(
  filterRanges: MarketplaceOrderFilterRanges | undefined
) {
  return (
    Number(filterRanges?.min_concurrency_limit) > 0 &&
    Number(filterRanges?.max_concurrency_limit) > 0 &&
    Number(filterRanges?.max_concurrency_limit) >=
      Number(filterRanges?.min_concurrency_limit)
  )
}

function buildMarketplaceFilterOptions(
  type: 'quota' | 'time',
  t: (key: string) => string
): { label: string; value: string; disabled?: boolean }[] {
  if (type === 'quota') {
    return [
      { label: t('All'), value: ALL_VALUE },
      { label: t('Unlimited'), value: 'unlimited' },
      {
        label: t('Limited'),
        value: 'limited',
      },
    ]
  }

  return [
    { label: t('All'), value: ALL_VALUE },
    { label: t('Unlimited time'), value: 'unlimited' },
    {
      label: t('Limited time'),
      value: 'limited',
    },
  ]
}

function normalizeRangeSliderValue(
  value: [number, number],
  minValue: number,
  maxValue: number
): [number, number] {
  const low = Math.max(minValue, Math.min(value[0], value[1]))
  const high = Math.min(maxValue, Math.max(value[0], value[1]))
  return [low, high]
}

function rangeSliderPercent(value: number, minValue: number, maxValue: number) {
  if (maxValue <= minValue) return 0
  return ((value - minValue) / (maxValue - minValue)) * 100
}

function RangeSlider({
  label,
  value,
  min,
  max,
  step,
  title,
  emptyLabel,
  formatValue,
  onChange,
}: {
  label: string
  value: [number, number]
  min: number
  max: number
  step: number
  title?: string
  emptyLabel: string
  formatValue: (value: number) => string
  onChange: (value: [number, number]) => void
}) {
  if (
    !Number.isFinite(min) ||
    !Number.isFinite(max) ||
    min <= 0 ||
    max <= 0 ||
    max < min
  ) {
    return (
      <div className='text-muted-foreground rounded-md border border-dashed p-3 text-sm lg:col-span-2'>
        {emptyLabel}
      </div>
    )
  }

  const isSingleValueRange = max === min
  const sliderMax = isSingleValueRange ? min + step : max
  const [minValue, maxValue] = isSingleValueRange
    ? [min, min]
    : normalizeRangeSliderValue(value, min, max)
  const minPercent = rangeSliderPercent(minValue, min, sliderMax)
  const maxPercent = rangeSliderPercent(maxValue, min, sliderMax)

  return (
    <div className='space-y-2 lg:col-span-2' title={title}>
      <div className='flex min-w-0 items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground shrink-0'>{label}</span>
        <span className='min-w-0 truncate font-medium'>
          {formatMarketplaceRange(formatValue(minValue), formatValue(maxValue))}
        </span>
      </div>
      <div className='relative h-7'>
        <div className='bg-muted absolute inset-x-0 top-3 h-2 rounded-full' />
        <div
          className='bg-primary absolute top-3 h-2 rounded-full'
          style={{ left: `${minPercent}%`, right: `${100 - maxPercent}%` }}
        />
        <input
          type='range'
          min={min}
          max={sliderMax}
          step={step}
          value={minValue}
          disabled={isSingleValueRange}
          aria-label={`${label} ${formatValue(min)}`}
          onChange={(event) =>
            onChange(
              normalizeRangeSliderValue(
                [Number(event.target.value), maxValue],
                min,
                max
              )
            )
          }
          className='accent-primary pointer-events-none absolute inset-x-0 top-1 h-6 w-full appearance-none bg-transparent [&::-webkit-slider-thumb]:pointer-events-auto'
        />
        <input
          type='range'
          min={min}
          max={sliderMax}
          step={step}
          value={maxValue}
          disabled={isSingleValueRange}
          aria-label={`${label} ${formatValue(max)}`}
          onChange={(event) =>
            onChange(
              normalizeRangeSliderValue(
                [minValue, Number(event.target.value)],
                min,
                max
              )
            )
          }
          className='accent-primary pointer-events-none absolute inset-x-0 top-1 h-6 w-full appearance-none bg-transparent [&::-webkit-slider-thumb]:pointer-events-auto'
        />
      </div>
      <div className='text-muted-foreground flex items-center justify-between gap-3 text-xs'>
        <span className='truncate'>{formatValue(min)}</span>
        <span className='truncate text-right'>{formatValue(max)}</span>
      </div>
    </div>
  )
}

function renderQuotaRangeInputs(
  filters: MarketplaceOrderFilters,
  filterRanges: MarketplaceOrderFilterRanges | undefined,
  updateFilter: (patch: MarketplaceOrderFilters) => void,
  t: (key: string) => string
) {
  if (filters.quota_mode !== 'limited') return null
  const rangeLabel = marketplaceHasLimitedQuotaRange(filterRanges)
    ? formatMarketplaceQuotaRange(filterRanges)
    : ''
  const minDisplayValue = marketplaceQuotaToDisplayAmount(
    filterRanges?.min_quota_limit
  )
  const maxDisplayValue = marketplaceQuotaToDisplayAmount(
    filterRanges?.max_quota_limit
  )
  const sliderValue: [number, number] = [
    Number(quotaRangeDisplayValue(filters.min_quota_limit)) || minDisplayValue,
    Number(quotaRangeDisplayValue(filters.max_quota_limit)) || maxDisplayValue,
  ]
  return (
    <RangeSlider
      label={t('Quota range')}
      value={sliderValue}
      min={minDisplayValue}
      max={maxDisplayValue}
      step={Number(marketplaceQuotaInputStep())}
      emptyLabel={t('No quota ranges')}
      formatValue={(value) =>
        formatMarketplaceQuotaUSD(marketplaceDisplayAmountToQuota(value))
      }
      onChange={([minValue, maxValue]) =>
        updateFilter({
          min_quota_limit: marketplaceDisplayAmountToQuota(minValue),
          max_quota_limit: marketplaceDisplayAmountToQuota(maxValue),
        })
      }
      title={rangeLabel}
    />
  )
}

function renderTimeRangeInputs(
  filters: MarketplaceOrderFilters,
  filterRanges: MarketplaceOrderFilterRanges | undefined,
  updateFilter: (patch: MarketplaceOrderFilters) => void,
  t: (key: string) => string
) {
  if (filters.time_mode !== 'limited') return null
  const rangeLabel = marketplaceHasLimitedTimeRange(filterRanges)
    ? formatMarketplaceTimeRange(filterRanges, t)
    : ''
  const minValue = Math.ceil(Number(filterRanges?.min_time_limit_seconds) / 60)
  const maxValue = Math.ceil(Number(filterRanges?.max_time_limit_seconds) / 60)
  const sliderValue: [number, number] = [
    Number(timeRangeDisplayValue(filters.min_time_limit_seconds)) || minValue,
    Number(timeRangeDisplayValue(filters.max_time_limit_seconds)) || maxValue,
  ]
  return (
    <RangeSlider
      label={t('Time range')}
      value={sliderValue}
      min={minValue}
      max={maxValue}
      step={1}
      emptyLabel={t('No time ranges')}
      formatValue={(value) =>
        formatMarketplaceTimeValue(Math.round(value * 60), t)
      }
      onChange={([minValue, maxValue]) =>
        updateFilter({
          min_time_limit_seconds: Math.round(minValue * 60),
          max_time_limit_seconds: Math.round(maxValue * 60),
        })
      }
      title={rangeLabel}
    />
  )
}

function renderMultiplierRangeInputs(
  filters: MarketplaceOrderFilters,
  filterRanges: MarketplaceOrderFilterRanges | undefined,
  updateFilter: (patch: MarketplaceOrderFilters) => void,
  t: (key: string) => string
) {
  const rangeLabel = marketplaceHasMultiplierRange(filterRanges)
    ? formatMarketplaceMultiplierRange(filterRanges)
    : ''
  const minValue = Number(filterRanges?.min_multiplier) || 0
  const maxValue = Number(filterRanges?.max_multiplier) || 0
  const sliderValue: [number, number] = [
    Number(multiplierRangeDisplayValue(filters.min_multiplier)) || minValue,
    Number(multiplierRangeDisplayValue(filters.max_multiplier)) || maxValue,
  ]
  return (
    <RangeSlider
      label={t('Multiplier range')}
      value={sliderValue}
      min={minValue}
      max={maxValue}
      step={0.01}
      emptyLabel={t('No multiplier ranges')}
      formatValue={(value) => formatMarketplaceMultiplierValue(value)}
      onChange={([minValue, maxValue]) =>
        updateFilter({
          min_multiplier: Math.round(minValue * 100) / 100,
          max_multiplier: Math.round(maxValue * 100) / 100,
        })
      }
      title={rangeLabel}
    />
  )
}

function renderConcurrencyRangeInputs(
  filters: MarketplaceOrderFilters,
  filterRanges: MarketplaceOrderFilterRanges | undefined,
  updateFilter: (patch: MarketplaceOrderFilters) => void,
  t: (key: string) => string
) {
  const rangeLabel = marketplaceHasConcurrencyRange(filterRanges)
    ? formatMarketplaceConcurrencyRange(filterRanges)
    : ''
  const minValue = Math.round(Number(filterRanges?.min_concurrency_limit) || 0)
  const maxValue = Math.round(Number(filterRanges?.max_concurrency_limit) || 0)
  const sliderValue: [number, number] = [
    Number(concurrencyRangeDisplayValue(filters.min_concurrency_limit)) ||
      minValue,
    Number(concurrencyRangeDisplayValue(filters.max_concurrency_limit)) ||
      maxValue,
  ]
  return (
    <RangeSlider
      label={t('Concurrency range')}
      value={sliderValue}
      min={minValue}
      max={maxValue}
      step={1}
      emptyLabel={t('No concurrency ranges')}
      formatValue={(value) => formatMarketplaceConcurrencyValue(value)}
      onChange={([minValue, maxValue]) =>
        updateFilter({
          min_concurrency_limit: Math.round(minValue),
          max_concurrency_limit: Math.round(maxValue),
        })
      }
      title={rangeLabel}
    />
  )
}

export function MetricProgress({
  label,
  value,
}: {
  label: string
  value: number
}) {
  const boundedValue = Math.max(0, Math.min(100, value))
  return (
    <div className='space-y-1.5'>
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='text-muted-foreground'>{label}</span>
        <span className='font-medium'>{formatNumber(boundedValue)}%</span>
      </div>
      <Progress value={boundedValue} />
    </div>
  )
}

export function EmptyLine({ label }: { label: string }) {
  return (
    <div className='text-muted-foreground rounded-md border border-dashed p-6 text-center text-sm'>
      {label}
    </div>
  )
}

export function ListFooter({
  loading,
  total,
}: {
  loading: boolean
  total: number
}) {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground flex items-center justify-between text-sm'>
      <span>
        {t('Total')}: {total}
      </span>
      {loading ? (
        <span className='flex items-center gap-2'>
          <Loader2 className='size-4 animate-spin' />
          {t('Loading')}
        </span>
      ) : null}
    </div>
  )
}
