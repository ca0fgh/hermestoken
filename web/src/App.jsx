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
import { getPricingRequireAuth } from './helpers/headerNavModules';
import { lazyWithRetry } from './helpers/lazyWithRetry';

const ConsoleRoutes = lazyWithRetry(
  () => import('./routes/ConsoleRoutes'),
  'console-routes',
);
const PublicRoutes = lazyWithRetry(
  () => import('./routes/PublicRoutes'),
  'public-routes',
);

function App() {
  const location = useLocation();
  const [statusState] = useContext(StatusContext);
  const pricingRequireAuth = useMemo(() => {
    return getPricingRequireAuth(statusState?.status?.HeaderNavModules);
  }, [statusState?.status?.HeaderNavModules]);
  const RoutesComponent = location.pathname.startsWith('/console')
    ? ConsoleRoutes
    : PublicRoutes;

  return (
    <SetupCheck>
      <Suspense fallback={<Loading />} key={location.pathname}>
        <RoutesComponent pricingRequireAuth={pricingRequireAuth} />
      </Suspense>
    </SetupCheck>
  );
}

export default App;
