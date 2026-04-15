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

import { lazy } from 'react';

const RETRY_STORAGE_PREFIX = 'lazy-retry:';
const RECOVERABLE_LAZY_ERROR_PATTERNS = [
  /Failed to fetch dynamically imported module/i,
  /Importing a module script failed/i,
  /ChunkLoadError/i,
  /Loading CSS chunk/i,
];

function getSessionStorage() {
  if (typeof window === 'undefined') {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function getRetryStorageKey(key) {
  return `${RETRY_STORAGE_PREFIX}${key}`;
}

export function isRecoverableLazyError(error) {
  const message = [error?.name, error?.message, typeof error === 'string' ? error : '']
    .filter(Boolean)
    .join(' ');

  return RECOVERABLE_LAZY_ERROR_PATTERNS.some((pattern) =>
    pattern.test(message),
  );
}

export function createLazyImportRecovery({
  key,
  storage = getSessionStorage(),
  reload = () => window.location.reload(),
} = {}) {
  const storageKey = getRetryStorageKey(key || 'route');

  const clearRetryState = () => {
    storage?.removeItem(storageKey);
  };

  const recover = (error) => {
    if (!isRecoverableLazyError(error)) {
      throw error;
    }

    if (!storage || typeof reload !== 'function') {
      throw error;
    }

    if (storage.getItem(storageKey) === '1') {
      clearRetryState();
      throw error;
    }

    storage.setItem(storageKey, '1');
    reload();

    // Keep Suspense pending while the browser reloads into the fresh asset graph.
    return new Promise(() => {});
  };

  recover.clearRetryState = clearRetryState;
  return recover;
}

export function lazyWithRetry(load, key) {
  const recoverImport = createLazyImportRecovery({ key });

  return lazy(() =>
    load()
      .then((module) => {
        recoverImport.clearRetryState();
        return module;
      })
      .catch((error) => recoverImport(error)),
  );
}
