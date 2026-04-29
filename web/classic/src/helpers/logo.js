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

const LOCAL_LOGO_ENDPOINT = '/api/logo';
const RELATIVE_LOGO_BASE = 'https://logo.local';

function resolveLogoUrl(logo) {
  try {
    return new URL(logo, RELATIVE_LOGO_BASE);
  } catch {
    return null;
  }
}

export function isUploadedLogoUrl(logo) {
  if (!logo) {
    return false;
  }

  const resolvedUrl = resolveLogoUrl(logo);
  return resolvedUrl?.pathname === LOCAL_LOGO_ENDPOINT;
}

export function getOptimizedLogoUrl(logo, { size } = {}) {
  if (!logo) {
    return '';
  }

  if (!isUploadedLogoUrl(logo)) {
    return logo;
  }

  const normalizedSize = Number(size);
  if (!Number.isFinite(normalizedSize) || normalizedSize <= 0) {
    return logo;
  }

  const resolvedUrl = resolveLogoUrl(logo);
  if (!resolvedUrl) {
    return logo;
  }

  resolvedUrl.searchParams.set('size', String(Math.round(normalizedSize)));

  if (/^https?:\/\//.test(logo)) {
    return resolvedUrl.toString();
  }

  return `${resolvedUrl.pathname}${resolvedUrl.search}${resolvedUrl.hash}`;
}
