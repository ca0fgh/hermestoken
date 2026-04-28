/*
Copyright (C) 2025 QuantumNous

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

const DEFAULT_HEADER_NAV_MODULES = {
  home: true,
  console: true,
  pricing: {
    enabled: true,
    requireAuth: false,
  },
  docs: true,
  about: true,
};

function normalizePricingConfig(pricing) {
  if (typeof pricing === 'boolean') {
    return {
      enabled: pricing,
      requireAuth: false,
    };
  }

  if (pricing && typeof pricing === 'object') {
    return {
      enabled: pricing.enabled !== false,
      requireAuth: pricing.requireAuth === true,
    };
  }

  return { ...DEFAULT_HEADER_NAV_MODULES.pricing };
}

export function normalizeHeaderNavModules(rawConfig) {
  let parsed = rawConfig;

  if (typeof rawConfig === 'string') {
    if (!rawConfig.trim()) {
      parsed = {};
    } else {
      try {
        parsed = JSON.parse(rawConfig);
      } catch (error) {
        parsed = {};
      }
    }
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    parsed = {};
  }

  return {
    home: parsed.home !== false,
    console: parsed.console !== false,
    pricing: normalizePricingConfig(parsed.pricing),
    docs: parsed.docs !== false,
    about: parsed.about !== false,
  };
}

export function isHeaderNavModuleEnabled(modules, key) {
  const normalizedModules = normalizeHeaderNavModules(modules);

  if (key === 'pricing') {
    return normalizedModules.pricing.enabled;
  }

  return normalizedModules[key] === true;
}

export function getPricingModuleConfig(modules) {
  return normalizeHeaderNavModules(modules).pricing;
}

export function getPricingRequireAuth(modules) {
  return getPricingModuleConfig(modules).requireAuth === true;
}

export function getFooterSectionVisibility(modules, docsLink) {
  const normalizedModules = normalizeHeaderNavModules(modules);

  return {
    showDocsSection: Boolean(docsLink) && normalizedModules.docs,
    showAboutSection: normalizedModules.about,
  };
}

export { DEFAULT_HEADER_NAV_MODULES };
