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

export const AUTH_EXPIRED_REDIRECT_PATH = '/login?expired=true';

export function getHttpStatusFromError(error) {
  const directStatus = Number(error?.response?.status ?? error?.status);
  if (Number.isInteger(directStatus)) {
    return directStatus;
  }

  const message = typeof error === 'string' ? error : error?.message;
  const statusMatch = String(message || '').match(
    /status code\s+(?<status>\d{3})\b/i,
  );
  if (statusMatch?.groups?.status) {
    return Number(statusMatch.groups.status);
  }

  return undefined;
}

export function isUnauthorizedError(error) {
  return getHttpStatusFromError(error) === 401;
}

export function redirectToLoginWhenExpired() {
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem('user');
  }

  if (typeof window === 'undefined' || !window.location) {
    return;
  }

  const alreadyOnExpiredLoginPage =
    window.location.pathname === '/login' &&
    window.location.search.includes('expired=true');
  if (alreadyOnExpiredLoginPage) {
    return;
  }

  window.location.href = AUTH_EXPIRED_REDIRECT_PATH;
}
