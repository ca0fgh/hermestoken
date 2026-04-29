import { useState } from 'react'
import { Copy, KeyRound, Loader2, Settings2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { fetchTokenKey } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { buildMarketplaceCurl } from '../lib'
import type { MarketplaceOrderFilters } from '../types'

type CallSnippetProps = {
  orderId?: number
  filters?: MarketplaceOrderFilters
  selectedToken?: ApiKey | null
  model?: string
  compact?: boolean
  boundOrderIds?: number[]
  onEditFixedOrderBindings?: () => void
}

export function CallSnippet({
  orderId,
  filters,
  selectedToken,
  model,
  compact = false,
  boundOrderIds = [],
  onEditFixedOrderBindings,
}: CallSnippetProps) {
  const { t } = useTranslation()
  const [copying, setCopying] = useState(false)
  const previewToken = selectedToken ? `sk-${selectedToken.key}` : undefined
  const fixedOrderBound = !!orderId && boundOrderIds.includes(orderId)
  const tokenStatusLabel = orderId
    ? selectedToken
      ? fixedOrderBound
        ? t('Using bound console token: {{name}}', {
            name: selectedToken.name,
          })
        : t('Using console token: {{name}}', { name: selectedToken.name })
      : t('Use a console token bound to this fixed order')
    : selectedToken
      ? t('Using console token: {{name}}', { name: selectedToken.name })
      : t('Select a console token to generate a callable request')
  const previewCurl = buildMarketplaceCurl(
    orderId,
    filters,
    previewToken,
    model,
    {
      fixedOrderBound,
    }
  )

  const copyCurl = async () => {
    if (!selectedToken) {
      toast.error(t('Select a console token first'))
      return
    }
    setCopying(true)
    try {
      const response = await fetchTokenKey(selectedToken.id)
      if (!response.success || !response.data?.key) {
        toast.error(response.message || t('Failed to load API key'))
        return
      }
      const fullToken = response.data.key.startsWith('sk-')
        ? response.data.key
        : `sk-${response.data.key}`
      const ok = await copyToClipboard(
        buildMarketplaceCurl(orderId, filters, fullToken, model, {
          fixedOrderBound,
        })
      )
      if (ok) toast.success(t('Copied'))
    } catch {
      toast.error(t('Failed to load API key'))
    } finally {
      setCopying(false)
    }
  }

  return (
    <div className='space-y-2'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground flex items-center gap-2 text-xs'>
          <KeyRound className='size-3.5' />
          {tokenStatusLabel}
        </div>
        <div className='flex flex-wrap gap-2'>
          {orderId ? (
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={onEditFixedOrderBindings}
            >
              <Settings2 className='size-4' />
              {t('Edit token bindings')}
            </Button>
          ) : null}
          {!orderId ? (
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={copyCurl}
              disabled={!selectedToken || copying}
            >
              {copying ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <Copy className='size-4' />
              )}
              {t('Copy call config')}
            </Button>
          ) : null}
        </div>
      </div>
      {!orderId ? (
        <pre
          className={`bg-muted overflow-x-auto rounded-md p-3 font-mono text-xs ${compact ? 'max-h-48' : ''}`}
        >
          {previewCurl}
        </pre>
      ) : null}
    </div>
  )
}
