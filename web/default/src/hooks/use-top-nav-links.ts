/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import type { HeaderNavModules } from '@/lib/nav-modules'
import { useStatus } from '@/hooks/use-status'

export type TopNavLink = {
  title: string
  href: string
  disabled?: boolean
  requiresAuth?: boolean
  external?: boolean
}

// Default navigation configuration
const DEFAULT_HEADER_NAV_MODULES: HeaderNavModules & {
  marketplace: boolean
} = {
  home: true,
  marketplace: true,
  console: true,
  pricing: { enabled: true, requireAuth: false },
  rankings: { enabled: true, requireAuth: false },
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
 *   rankings: { enabled: true, requireAuth: false },
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
    const requiresAuth = pricing.requireAuth && !isAuthed
    links.push({ title: t('Model Square'), href: '/pricing', requiresAuth })
  }

  // Rankings
  const rankings = modules?.rankings
  if (rankings && typeof rankings === 'object' && rankings.enabled) {
    const requiresAuth = rankings.requireAuth && !isAuthed
    links.push({ title: t('Rankings'), href: '/rankings', requiresAuth })
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
