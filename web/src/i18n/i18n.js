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

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import zhCNTranslation from './locales/zh-CN.json';

import { normalizeLanguage, supportedLanguages } from './language';

const DEFAULT_LANGUAGE = 'zh-CN';
const defaultLanguageMessages = getTranslationMessages(zhCNTranslation);
const localeLoaders = {
  en: () => import('./locales/en.json'),
  'zh-TW': () => import('./locales/zh-TW.json'),
  fr: () => import('./locales/fr.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  vi: () => import('./locales/vi.json'),
};
const loadedLanguages = new Set(defaultLanguageMessages ? [DEFAULT_LANGUAGE] : []);
const languageLoads = new Map();

function getTranslationMessages(module) {
  return module?.default?.translation || module?.translation || module?.default;
}

export async function ensureLanguageResources(language) {
  const normalizedLanguage = normalizeLanguage(language) || DEFAULT_LANGUAGE;

  if (
    !supportedLanguages.includes(normalizedLanguage) ||
    loadedLanguages.has(normalizedLanguage)
  ) {
    return normalizedLanguage;
  }

  const existingLoad = languageLoads.get(normalizedLanguage);
  if (existingLoad) {
    await existingLoad;
    return normalizedLanguage;
  }

  const localeLoader = localeLoaders[normalizedLanguage];

  if (!localeLoader) {
    return DEFAULT_LANGUAGE;
  }

  const loadPromise = localeLoader()
    .then((module) => {
      const messages = getTranslationMessages(module);
      if (!messages) {
        return;
      }
      i18n.addResourceBundle(
        normalizedLanguage,
        'translation',
        messages,
        true,
        true,
      );
      loadedLanguages.add(normalizedLanguage);
    })
    .finally(() => {
      languageLoads.delete(normalizedLanguage);
    });

  languageLoads.set(normalizedLanguage, loadPromise);
  await loadPromise;

  return normalizedLanguage;
}

function getPreferredLanguage() {
  if (typeof window === 'undefined') {
    return DEFAULT_LANGUAGE;
  }

  try {
    const savedLanguage = window.localStorage.getItem('i18nextLng');
    return normalizeLanguage(savedLanguage) || DEFAULT_LANGUAGE;
  } catch {
    return DEFAULT_LANGUAGE;
  }
}

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    load: 'currentOnly',
    supportedLngs: supportedLanguages,
    partialBundledLanguages: true,
    resources: defaultLanguageMessages
      ? {
          [DEFAULT_LANGUAGE]: {
            translation: defaultLanguageMessages,
          },
        }
      : {},
    fallbackLng: DEFAULT_LANGUAGE,
    nsSeparator: false,
    interpolation: {
      escapeValue: false,
    },
  });

export async function initializeI18n() {
  const preferredLanguage =
    getPreferredLanguage() ||
    normalizeLanguage(i18n.resolvedLanguage || i18n.language) ||
    DEFAULT_LANGUAGE;

  await ensureLanguageResources(preferredLanguage);

  if (preferredLanguage !== i18n.language) {
    await i18n.changeLanguage(preferredLanguage);
    return i18n;
  }

  if (preferredLanguage !== DEFAULT_LANGUAGE) {
    await i18n.changeLanguage(preferredLanguage);
  }

  return i18n;
}

window.__i18n = i18n;

export default i18n;
