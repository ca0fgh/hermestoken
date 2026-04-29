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

export function getMediaQueryList(query) {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return null;
  }

  try {
    return window.matchMedia(query);
  } catch {
    return null;
  }
}

export function matchesMediaQuery(query, fallback = false) {
  const mediaQueryList = getMediaQueryList(query);
  if (!mediaQueryList) {
    return fallback;
  }

  return Boolean(mediaQueryList.matches);
}

export function subscribeToMediaQueryList(mediaQueryList, listener) {
  if (!mediaQueryList || typeof listener !== 'function') {
    return () => {};
  }

  if (
    typeof mediaQueryList.addEventListener === 'function' &&
    typeof mediaQueryList.removeEventListener === 'function'
  ) {
    mediaQueryList.addEventListener('change', listener);
    return () => mediaQueryList.removeEventListener('change', listener);
  }

  if (
    typeof mediaQueryList.addListener === 'function' &&
    typeof mediaQueryList.removeListener === 'function'
  ) {
    mediaQueryList.addListener(listener);
    return () => mediaQueryList.removeListener(listener);
  }

  return () => {};
}
