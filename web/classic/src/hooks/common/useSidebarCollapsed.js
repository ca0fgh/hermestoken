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

import { useState, useCallback, useEffect } from 'react';

const KEY = 'default_collapse_sidebar';
const SIDEBAR_COLLAPSED_EVENT = 'sidebar-collapsed-change';

const readCollapsedState = () => {
  if (typeof window === 'undefined') {
    return false;
  }

  return window.localStorage.getItem(KEY) === 'true';
};

const broadcastCollapsedState = (collapsed) => {
  if (typeof window === 'undefined') {
    return;
  }

  window.localStorage.setItem(KEY, collapsed.toString());
  window.dispatchEvent(
    new CustomEvent(SIDEBAR_COLLAPSED_EVENT, {
      detail: { collapsed },
    }),
  );
};

export const useSidebarCollapsed = () => {
  const [collapsed, setCollapsed] = useState(readCollapsedState);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return undefined;
    }

    const handleStorageChange = (event) => {
      if (event.key !== KEY) {
        return;
      }
      setCollapsed(event.newValue === 'true');
    };

    const handleSidebarCollapsedChange = (event) => {
      setCollapsed(event.detail?.collapsed === true);
    };

    window.addEventListener('storage', handleStorageChange);
    window.addEventListener(
      SIDEBAR_COLLAPSED_EVENT,
      handleSidebarCollapsedChange,
    );

    return () => {
      window.removeEventListener('storage', handleStorageChange);
      window.removeEventListener(
        SIDEBAR_COLLAPSED_EVENT,
        handleSidebarCollapsedChange,
      );
    };
  }, []);

  const toggle = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      broadcastCollapsedState(next);
      return next;
    });
  }, []);

  const set = useCallback((value) => {
    const next = value === true;
    setCollapsed(next);
    broadcastCollapsedState(next);
  }, []);

  return [collapsed, toggle, set];
};
