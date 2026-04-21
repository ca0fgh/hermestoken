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

import { parsePublicBootstrapJson } from './bootstrapData.js';

export const PUBLIC_BOOTSTRAP_CACHE_KEY = 'hermes-public-bootstrap-v1';
export const PUBLIC_BOOTSTRAP_CACHE_TTL_MS = 15 * 60 * 1000;

export function cachePublicBootstrap(
  payload,
  storage = globalThis.localStorage,
) {
  if (!payload) {
    return;
  }

  try {
    storage?.setItem?.(
      PUBLIC_BOOTSTRAP_CACHE_KEY,
      JSON.stringify({
        savedAt: Date.now(),
        payload,
      }),
    );
  } catch {
    // Ignore storage write failures during startup warmup.
  }
}

export function readCachedPublicBootstrap(storage = globalThis.localStorage) {
  try {
    const cachedValue = storage?.getItem?.(PUBLIC_BOOTSTRAP_CACHE_KEY);
    const parsedCache = parsePublicBootstrapJson(cachedValue);

    if (!parsedCache?.savedAt || parsedCache.payload === undefined) {
      return null;
    }

    if (Date.now() - parsedCache.savedAt > PUBLIC_BOOTSTRAP_CACHE_TTL_MS) {
      storage?.removeItem?.(PUBLIC_BOOTSTRAP_CACHE_KEY);
      return null;
    }

    return parsedCache.payload;
  } catch {
    return null;
  }
}

export function resolvePublicStartupBootstrap(
  injectedPayload,
  storage = globalThis.localStorage,
) {
  if (injectedPayload) {
    cachePublicBootstrap(injectedPayload, storage);
    return injectedPayload;
  }

  return readCachedPublicBootstrap(storage);
}
