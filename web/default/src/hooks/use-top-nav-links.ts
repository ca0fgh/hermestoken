import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useStatus } from '@/hooks/use-status'

export type TopNavLink = {
  title: string
  href: string
  disabled?: boolean
  external?: boolean
}

// Default navigation configuration
const DEFAULT_HEADER_NAV_MODULES = {
  home: true,
  marketplace: true,
  console: true,
  pricing: { enabled: true, requireAuth: false },
  docs: true,
  about: true,
}

function normalizeHeaderNavModules(raw: unknown) {
  if (!raw || (typeof raw === 'string' && raw.trim() === '')) {
    return DEFAULT_HEADER_NAV_MODULES
  }

  try {
    const parsed =
      typeof raw === 'string'
        ? (JSON.parse(raw) as Record<string, unknown>)
        : raw
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return DEFAULT_HEADER_NAV_MODULES
    }

    const parsedModules = parsed as Record<string, unknown>
    const pricing =
      typeof parsedModules.pricing === 'boolean'
        ? {
            enabled: parsedModules.pricing,
            requireAuth: DEFAULT_HEADER_NAV_MODULES.pricing.requireAuth,
          }
        : parsedModules.pricing && typeof parsedModules.pricing === 'object'
          ? {
              enabled:
                (parsedModules.pricing as { enabled?: unknown }).enabled !==
                false,
              requireAuth:
                (parsedModules.pricing as { requireAuth?: unknown })
                  .requireAuth === true,
            }
          : DEFAULT_HEADER_NAV_MODULES.pricing

    return {
      ...DEFAULT_HEADER_NAV_MODULES,
      ...parsedModules,
      pricing,
      marketplace: parsedModules.marketplace !== false,
    }
  } catch {
    return DEFAULT_HEADER_NAV_MODULES
  }
}

/**
 * Generate top navigation links based on HeaderNavModules configuration from backend /api/status
 * Backend format example (stringified JSON):
 * {
 *   home: true,
 *   marketplace: true,
 *   console: true,
 *   pricing: { enabled: true, requireAuth: false },
 *   docs: true,
 *   about: true
 * }
 */
export function useTopNavLinks(): TopNavLink[] {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { auth } = useAuthStore()

  // Parse HeaderNavModules
  const modules = useMemo(() => {
    return normalizeHeaderNavModules(status?.HeaderNavModules)
  }, [status?.HeaderNavModules])

  // Documentation link (may be external)
  const docsLink: string | undefined = status?.docs_link as string | undefined

  const isAuthed = !!auth?.user

  const links: TopNavLink[] = []

  // Home
  if (modules?.home !== false) {
    links.push({ title: t('Home'), href: '/' })
  }

  // Marketplace
  if (modules?.marketplace !== false) {
    links.push({ title: t('Marketplace'), href: '/marketplace' })
  }

  // Console -> /dashboard (new console path)
  if (modules?.console !== false) {
    links.push({ title: t('Console'), href: '/dashboard' })
  }

  // Pricing
  const pricing = modules?.pricing
  if (pricing && typeof pricing === 'object' && pricing.enabled) {
    const disabled = pricing.requireAuth && !isAuthed
    links.push({ title: t('Pricing'), href: '/pricing', disabled })
  }

  // Docs (supports external links)
  if (modules?.docs !== false) {
    if (docsLink) {
      links.push({ title: t('Docs'), href: docsLink, external: true })
    } else {
      links.push({ title: t('Docs'), href: '/docs' })
    }
  }

  // About
  if (modules?.about !== false) {
    links.push({ title: t('About'), href: '/about' })
  }

  return links
}
