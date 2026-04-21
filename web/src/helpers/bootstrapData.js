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

import { normalizeLanguage, supportedLanguages } from '../i18n/language.js';

export const PUBLIC_BOOTSTRAP_SCRIPT_ID = 'hermes-public-bootstrap';

const DEFAULT_THEME_MODE = 'auto';
const DEFAULT_LANGUAGE = 'zh-CN';
const SUPPORTED_THEME_MODES = new Set(['auto', 'light', 'dark']);

function normalizeThemeMode(themeMode) {
  if (!themeMode) {
    return DEFAULT_THEME_MODE;
  }

  return SUPPORTED_THEME_MODES.has(themeMode) ? themeMode : DEFAULT_THEME_MODE;
}

function normalizeStartupLanguage(language) {
  const normalizedLanguage = normalizeLanguage(language);

  if (!normalizedLanguage) {
    return DEFAULT_LANGUAGE;
  }

  if (supportedLanguages.includes(normalizedLanguage)) {
    return normalizedLanguage;
  }

  const [baseLanguage] = normalizedLanguage.split('-');
  const normalizedBaseLanguage = normalizeLanguage(baseLanguage);

  if (normalizedBaseLanguage && supportedLanguages.includes(normalizedBaseLanguage)) {
    return normalizedBaseLanguage;
  }

  return DEFAULT_LANGUAGE;
}

export function parsePublicBootstrapJson(rawValue) {
  if (!rawValue || typeof rawValue !== 'string') {
    return null;
  }

  try {
    return JSON.parse(rawValue);
  } catch {
    return null;
  }
}

export function readInjectedBootstrap(doc = globalThis.document) {
  const bootstrapElement = doc?.getElementById?.(PUBLIC_BOOTSTRAP_SCRIPT_ID);
  return parsePublicBootstrapJson(bootstrapElement?.textContent?.trim() || '');
}

export function readClientStartupSettings(storage = globalThis.localStorage) {
  try {
    const themeMode = normalizeThemeMode(storage?.getItem?.('theme-mode'));
    const language = normalizeStartupLanguage(storage?.getItem?.('i18nextLng'));

    return {
      themeMode,
      language,
    };
  } catch {
    return {
      themeMode: DEFAULT_THEME_MODE,
      language: DEFAULT_LANGUAGE,
    };
  }
}
