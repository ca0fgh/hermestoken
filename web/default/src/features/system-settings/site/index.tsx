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
import { SettingsPage } from '../components/settings-page'
import {
  SITE_DEFAULT_SECTION,
  getSiteSectionContent,
  type SitePageSettings,
  getSiteSectionMeta,
} from './section-registry.tsx'

const defaultSiteSettings: SitePageSettings = {
  'theme.frontend': 'default',
  Notice: '',
  SystemName: 'New API',
  Logo: '',
  Footer: '',
  About: '',
  HomePageContent: '',
  ServerAddress: '',
  'legal.user_agreement': '',
  'legal.privacy_policy': '',
  HeaderNavModules: '',
  SidebarModulesAdmin: '',
  // Mirrors setting/marketplace.go; only used until the options request lands.
  MarketplaceEnabled: true,
  MarketplaceEnabledVendorTypes: '',
  MarketplaceFeeRate: 0,
  MarketplaceSellerIncomeHoldSeconds: 7 * 24 * 60 * 60,
  MarketplaceMinFixedOrderQuota: 0,
  MarketplaceMaxFixedOrderQuota: 0,
  MarketplaceFixedOrderDefaultExpirySeconds: 30 * 24 * 60 * 60,
  MarketplaceMaxSellerMultiplier: 10,
  MarketplaceMaxCredentialConcurrency: 5,
}

export function SiteSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/site/$section'
      defaultSettings={defaultSiteSettings}
      defaultSection={SITE_DEFAULT_SECTION}
      getSectionContent={getSiteSectionContent}
      getSectionMeta={getSiteSectionMeta}
    />
  )
}
