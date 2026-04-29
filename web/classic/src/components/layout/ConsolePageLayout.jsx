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

import React, { Suspense, lazy, useEffect, useState } from 'react';
import { useSidebarCollapsed } from '../../hooks/common/useSidebarCollapsed';

const ConsoleHeaderBar = lazy(() => import('./headerbar'));
const ConsoleSiderBar = lazy(() => import('./SiderBar'));
const SemiRuntime = lazy(() => import('../common/SemiRuntime'));

const ConsolePageLayout = ({ appContent, footerContent, isMobile }) => {
  const [collapsed, , setCollapsed] = useSidebarCollapsed();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const showSider = !isMobile || drawerOpen;
  const sidebarWidth = collapsed
    ? 'var(--sidebar-width-collapsed)'
    : 'var(--sidebar-width)';

  useEffect(() => {
    if (isMobile && drawerOpen && collapsed) {
      setCollapsed(false);
    }
  }, [isMobile, drawerOpen, collapsed, setCollapsed]);

  return (
    <SemiRuntime>
      <header
        style={{
          padding: 0,
          height: 'auto',
          lineHeight: 'normal',
          position: 'fixed',
          width: '100%',
          top: 0,
          zIndex: 100,
        }}
      >
        <ConsoleHeaderBar
          onMobileMenuToggle={() => setDrawerOpen((prev) => !prev)}
          drawerOpen={drawerOpen}
        />
      </header>
      <div
        style={{
          overflow: isMobile ? 'visible' : 'auto',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {showSider ? (
          <aside
            className='app-sider'
            style={{
              position: 'fixed',
              left: 0,
              top: '64px',
              zIndex: 99,
              border: 'none',
              paddingRight: '0',
              width: sidebarWidth,
            }}
          >
            <ConsoleSiderBar
              onNavigate={() => {
                if (isMobile) {
                  setDrawerOpen(false);
                }
              }}
            />
          </aside>
        ) : null}
        <div
          style={{
            marginLeft: isMobile ? '0' : showSider ? sidebarWidth : '0',
            flex: '1 1 auto',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {appContent}
          {footerContent}
        </div>
      </div>
    </SemiRuntime>
  );
};

const ConsolePageLayoutWithSuspense = (props) => {
  return (
    <Suspense fallback={null}>
      <ConsolePageLayout {...props} />
    </Suspense>
  );
};

export default ConsolePageLayoutWithSuspense;
