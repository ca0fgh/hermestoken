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

export function scheduleNonCriticalWork(task, timeout = 250) {
  const runtime = globalThis;

  if (typeof runtime.requestIdleCallback === 'function') {
    const idleId = runtime.requestIdleCallback(() => {
      void task();
    }, { timeout });

    return () => {
      if (typeof runtime.cancelIdleCallback === 'function') {
        runtime.cancelIdleCallback(idleId);
      }
    };
  }

  const timerId = runtime.setTimeout(() => {
    void task();
  }, timeout);

  return () => {
    runtime.clearTimeout(timerId);
  };
}
