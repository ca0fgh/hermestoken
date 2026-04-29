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
*/

export const PUBLIC_BOOTSTRAP_REFRESH_REQUEST_INIT = Object.freeze({
  cache: 'no-store',
  headers: Object.freeze({
    'Cache-Control': 'no-store',
  }),
});

export async function fetchPublicBootstrap(fetchFn = globalThis.fetch) {
  const response = await fetchFn(
    '/api/public/bootstrap',
    PUBLIC_BOOTSTRAP_REFRESH_REQUEST_INIT,
  );

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return response.json();
}
