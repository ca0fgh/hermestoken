import { useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Check,
  Copy,
  Eye,
  EyeOff,
  KeyRound,
  Link as LinkIcon,
  Loader2,
  Plus,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import {
  encodeConnectionString,
  getConfiguredConnectionURL,
} from '@/features/keys/lib/connection-info'
import type { ApiKey } from '@/features/keys/types'

type BuyerTokenPanelProps = {
  selectedTokenId?: number
  onTokenChange: (token: ApiKey | null) => void
}

function fullTokenKey(key?: string | null) {
  if (!key) return ''
  return key.startsWith('sk-') ? key : `sk-${key}`
}

function tokenKeyLabel(token: ApiKey, key: string) {
  const name = token.name.trim()
  return name ? `${name} · ${key}` : key
}

export function BuyerTokenPanel({
  selectedTokenId,
  onTokenChange,
}: BuyerTokenPanelProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const [revealedTokenKeys, setRevealedTokenKeys] = useState<
    Record<number, boolean>
  >({})
  const [resolvedTokenKeys, setResolvedTokenKeys] = useState<
    Record<number, string>
  >({})
  const [loadingTokenKeys, setLoadingTokenKeys] = useState<
    Record<number, boolean>
  >({})
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
  const selectedToken = useMemo(
    () => tokens.find((token) => token.id === selectedTokenId) ?? null,
    [tokens, selectedTokenId]
  )
  const selectedTokenDisplayKey = selectedToken
    ? fullTokenKey(
        revealedTokenKeys[selectedToken.id] &&
          resolvedTokenKeys[selectedToken.id]
          ? resolvedTokenKeys[selectedToken.id]
          : selectedToken.key
      )
    : ''
  const selectedTokenLoading = selectedToken
    ? !!loadingTokenKeys[selectedToken.id]
    : false
  const selectedTokenCopied =
    selectedToken &&
    !!resolvedTokenKeys[selectedToken.id] &&
    copiedText === fullTokenKey(resolvedTokenKeys[selectedToken.id])

  useEffect(() => {
    if (selectedTokenId || enabledTokens.length === 0) return
    onTokenChange(enabledTokens[0])
  }, [enabledTokens, onTokenChange, selectedTokenId])

  useEffect(() => {
    if (!selectedTokenId || !selectedToken) return
    onTokenChange(selectedToken)
  }, [onTokenChange, selectedToken, selectedTokenId])

  useEffect(() => {
    if (!selectedTokenId) return
    setRevealedTokenKeys((current) => ({
      ...current,
      [selectedTokenId]: false,
    }))
  }, [selectedTokenId])

  const resolveSelectedTokenKey = useCallback(async () => {
    if (!selectedToken) return ''
    const cachedKey = resolvedTokenKeys[selectedToken.id]
    if (cachedKey) return fullTokenKey(cachedKey)

    setLoadingTokenKeys((current) => ({ ...current, [selectedToken.id]: true }))
    try {
      const response = await fetchTokenKey(selectedToken.id)
      const key = response.data?.key
      if (!response.success || !key) {
        throw new Error(response.message || t('Failed to load API keys'))
      }

      setResolvedTokenKeys((current) => ({
        ...current,
        [selectedToken.id]: key,
      }))
      return fullTokenKey(key)
    } catch {
      return ''
    } finally {
      setLoadingTokenKeys((current) => ({
        ...current,
        [selectedToken.id]: false,
      }))
    }
  }, [resolvedTokenKeys, selectedToken, t])

  const toggleSelectedTokenVisibility = useCallback(async () => {
    if (!selectedToken) return
    if (revealedTokenKeys[selectedToken.id]) {
      setRevealedTokenKeys((current) => ({
        ...current,
        [selectedToken.id]: false,
      }))
      return
    }

    const key = await resolveSelectedTokenKey()
    if (key) {
      setRevealedTokenKeys((current) => ({
        ...current,
        [selectedToken.id]: true,
      }))
    }
  }, [resolveSelectedTokenKey, revealedTokenKeys, selectedToken])

  const copySelectedTokenKey = useCallback(async () => {
    const key = await resolveSelectedTokenKey()
    if (key) {
      await copyToClipboard(key)
    }
  }, [copyToClipboard, resolveSelectedTokenKey])

  const copySelectedTokenConnectionInfo = useCallback(async () => {
    const key = await resolveSelectedTokenKey()
    if (key) {
      await copyToClipboard(
        encodeConnectionString(key, getConfiguredConnectionURL(status))
      )
    }
  }, [copyToClipboard, resolveSelectedTokenKey, status])

  return (
    <div className='rounded-md border p-4'>
      <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
        <div className='space-y-1'>
          <div className='flex items-center gap-2 text-sm font-semibold'>
            <KeyRound className='size-4' />
            {t('Buyer token')}
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Marketplace calls reuse console API keys for buyer identity, quota, model limits, and IP restrictions.'
            )}
          </p>
        </div>
        <Button asChild variant='outline' size='sm'>
          <Link to='/keys'>
            <Plus className='size-4' />
            {t('Manage console tokens')}
          </Link>
        </Button>
      </div>
      <div className='mt-4 max-w-md'>
        <div className='grid grid-cols-[minmax(0,1fr)_2rem_2rem] items-center gap-1'>
          <Select
            value={selectedTokenId ? String(selectedTokenId) : ''}
            onValueChange={(value) => {
              const token = tokens.find((item) => item.id === Number(value))
              onTokenChange(token ?? null)
            }}
            disabled={tokensQuery.isLoading || enabledTokens.length === 0}
          >
            <SelectTrigger className='w-full'>
              <SelectValue
                placeholder={
                  enabledTokens.length === 0
                    ? t('No enabled tokens available')
                    : t('Select a console token')
                }
              />
            </SelectTrigger>
            <SelectContent>
              {enabledTokens.map((token) => (
                <SelectItem key={token.id} value={String(token.id)}>
                  {tokenKeyLabel(
                    token,
                    token.id === selectedToken?.id && selectedTokenDisplayKey
                      ? selectedTokenDisplayKey
                      : fullTokenKey(token.key)
                  )}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground size-8 shrink-0'
            disabled={!selectedToken || selectedTokenLoading}
            aria-label={
              selectedToken && revealedTokenKeys[selectedToken.id]
                ? t('Hide API key')
                : t('Full API Key')
            }
            onClick={toggleSelectedTokenVisibility}
          >
            {selectedTokenLoading ? (
              <Loader2 className='size-4 animate-spin' />
            ) : selectedToken && revealedTokenKeys[selectedToken.id] ? (
              <EyeOff className='size-4' />
            ) : (
              <Eye className='size-4' />
            )}
          </Button>
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='text-muted-foreground hover:text-foreground size-8 shrink-0'
                disabled={!selectedToken || selectedTokenLoading}
                aria-label={selectedTokenCopied ? t('Copied!') : t('Copy')}
              >
                {selectedTokenCopied ? (
                  <Check className='size-4 text-green-600' />
                ) : (
                  <Copy className='size-4' />
                )}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end' className='w-44'>
              <DropdownMenuItem onClick={copySelectedTokenKey}>
                {t('Copy Key')}
                <DropdownMenuShortcut>
                  <Copy className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={copySelectedTokenConnectionInfo}>
                {t('Copy Connection Info')}
                <DropdownMenuShortcut>
                  <LinkIcon className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </div>
  )
}
