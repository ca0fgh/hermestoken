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

import { resolvePublicStartupBootstrap } from '../helpers/publicStartupCache.js';

export const PUBLIC_HOME_SHELL_ID = 'hermes-public-home-shell';
export const PUBLIC_BOOTSTRAP_SCOPE_HOME = 'home';

export function isHomeRoutePathname(pathname) {
  return pathname === '/';
}

export function markPublicHomeBootstrapStatus(status) {
  if (!status) {
    return status;
  }

  return {
    ...status,
    __publicBootstrapScope: PUBLIC_BOOTSTRAP_SCOPE_HOME,
  };
}

export function resolveRoutePublicBootstrap({
  pathname,
  injectedBootstrap,
  storage = globalThis.localStorage,
} = {}) {
  if (!isHomeRoutePathname(pathname)) {
    return null;
  }

  const bootstrap = resolvePublicStartupBootstrap(injectedBootstrap, storage);
  if (!bootstrap?.status) {
    return bootstrap;
  }

  return {
    ...bootstrap,
    status: markPublicHomeBootstrapStatus(bootstrap.status),
  };
}

export function removePublicHomeShell(doc = globalThis.document) {
  doc?.getElementById?.(PUBLIC_HOME_SHELL_ID)?.remove?.();
}
