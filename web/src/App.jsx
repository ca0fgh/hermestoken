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

import React, { Suspense, useContext, useMemo } from 'react';
import { useLocation } from 'react-router-dom';
import Loading from './components/common/ui/Loading';
import SetupCheck from './components/layout/SetupCheck';
import { StatusContext } from './context/Status';
import { getPricingModuleConfig } from './helpers/headerNavModules';
import { lazyWithRetry } from './helpers/lazyWithRetry';
import HomeRoutes from './routes/HomeRoutes';

const ConsoleRoutes = lazyWithRetry(
  () => import('./routes/ConsoleRoutes'),
  'console-routes',
);
const PublicRoutes = lazyWithRetry(
  () => import('./routes/PublicRoutes'),
  'public-routes',
);
const APP_BUILD_TAG = '2026-04-21-dashboard-restore-v2';

function App() {
  const location = useLocation();
  const [statusState] = useContext(StatusContext);
  const pricingConfig = useMemo(() => {
    if (!statusState?.status) {
      return {
        enabled: false,
        requireAuth: false,
      };
    }

    return getPricingModuleConfig(statusState?.status?.HeaderNavModules);
  }, [statusState?.status, statusState?.status?.HeaderNavModules]);
  const isConsoleRoute = location.pathname.startsWith('/console');
  const isHomeRoute = location.pathname === '/';
  const RoutesComponent = isConsoleRoute
    ? ConsoleRoutes
    : isHomeRoute
      ? HomeRoutes
      : PublicRoutes;

  return (
    <SetupCheck>
      <Suspense
        fallback={<Loading />}
        key={`${APP_BUILD_TAG}:${location.pathname}`}
      >
        <RoutesComponent
          pricingEnabled={pricingConfig.enabled}
          pricingRequireAuth={pricingConfig.requireAuth}
        />
      </Suspense>
    </SetupCheck>
  );
}

export default App;
