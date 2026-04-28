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

import React, { Suspense } from 'react';
import { Route, Routes, useLocation } from 'react-router-dom';
import Loading from '../components/common/ui/Loading';
import { AdminRoute, PrivateRoute } from '../helpers/auth';
import { lazyWithRetry } from '../helpers/lazyWithRetry';

const Dashboard = lazyWithRetry(
  () => import('../pages/Dashboard'),
  'dashboard-route',
);
const NotFound = lazyWithRetry(
  () => import('../pages/NotFound'),
  'not-found-route',
);
const Setting = lazyWithRetry(() => import('../pages/Setting'), 'setting-route');
const Channel = lazyWithRetry(() => import('../pages/Channel'), 'channel-route');
const Token = lazyWithRetry(() => import('../pages/Token'), 'token-route');
const TopUp = lazyWithRetry(() => import('../pages/TopUp'), 'topup-route');
const Redemption = lazyWithRetry(
  () => import('../pages/Redemption'),
  'redemption-route',
);
const Log = lazyWithRetry(() => import('../pages/Log'), 'log-route');
const Chat = lazyWithRetry(() => import('../pages/Chat'), 'chat-route');
const Midjourney = lazyWithRetry(
  () => import('../pages/Midjourney'),
  'midjourney-route',
);
const Task = lazyWithRetry(() => import('../pages/Task'), 'task-route');
const ModelPage = lazyWithRetry(() => import('../pages/Model'), 'model-route');
const ModelDeploymentPage = lazyWithRetry(
  () => import('../pages/ModelDeployment'),
  'model-deployment-route',
);
const Playground = lazyWithRetry(
  () => import('../pages/Playground'),
  'playground-route',
);
const Subscription = lazyWithRetry(
  () => import('../pages/Subscription'),
  'subscription-route',
);
const InviteRebate = lazyWithRetry(
  () => import('../pages/InviteRebate'),
  'invite-rebate-route',
);
const Withdrawal = lazyWithRetry(
  () => import('../pages/Withdrawal'),
  'withdrawal-route',
);
const User = lazyWithRetry(() => import('../pages/User'), 'user-route');
const PersonalSetting = lazyWithRetry(
  () => import('../components/settings/PersonalSetting'),
  'personal-setting-route',
);
const CONSOLE_ROUTES_BUILD_TAG = '2026-04-21-dashboard-restore-v2';

function ConsoleRoutes() {
  const location = useLocation();
  const renderWithSuspense = (
    element,
    key = `${CONSOLE_ROUTES_BUILD_TAG}:${location.pathname}`,
  ) => (
    <Suspense fallback={<Loading />} key={key}>
      {element}
    </Suspense>
  );

  return (
    <Routes>
      <Route
        path='/console/models'
        element={<AdminRoute>{renderWithSuspense(<ModelPage />)}</AdminRoute>}
      />
      <Route
        path='/console/deployment'
        element={
          <AdminRoute>
            {renderWithSuspense(<ModelDeploymentPage />)}
          </AdminRoute>
        }
      />
      <Route
        path='/console/subscription'
        element={
          <AdminRoute>{renderWithSuspense(<Subscription />)}</AdminRoute>
        }
      />
      <Route
        path='/console/channel'
        element={<AdminRoute>{renderWithSuspense(<Channel />)}</AdminRoute>}
      />
      <Route
        path='/console/token'
        element={<PrivateRoute>{renderWithSuspense(<Token />)}</PrivateRoute>}
      />
      <Route
        path='/console/playground'
        element={
          <PrivateRoute>{renderWithSuspense(<Playground />)}</PrivateRoute>
        }
      />
      <Route
        path='/console/redemption'
        element={<AdminRoute>{renderWithSuspense(<Redemption />)}</AdminRoute>}
      />
      <Route
        path='/console/user'
        element={<AdminRoute>{renderWithSuspense(<User />)}</AdminRoute>}
      />
      <Route
        path='/console/withdrawal'
        element={<AdminRoute>{renderWithSuspense(<Withdrawal />)}</AdminRoute>}
      />
      <Route
        path='/console/setting'
        element={<AdminRoute>{renderWithSuspense(<Setting />)}</AdminRoute>}
      />
      <Route
        path='/console/personal'
        element={
          <PrivateRoute>
            {renderWithSuspense(<PersonalSetting />)}
          </PrivateRoute>
        }
      />
      <Route
        path='/console/topup'
        element={<PrivateRoute>{renderWithSuspense(<TopUp />)}</PrivateRoute>}
      />
      <Route
        path='/console/invite/rebate'
        element={
          <PrivateRoute>{renderWithSuspense(<InviteRebate />)}</PrivateRoute>
        }
      />
      <Route
        path='/console/log'
        element={<PrivateRoute>{renderWithSuspense(<Log />)}</PrivateRoute>}
      />
      <Route
        path='/console'
        element={
          <PrivateRoute>{renderWithSuspense(<Dashboard />)}</PrivateRoute>
        }
      />
      <Route
        path='/console/midjourney'
        element={
          <PrivateRoute>{renderWithSuspense(<Midjourney />)}</PrivateRoute>
        }
      />
      <Route
        path='/console/task'
        element={<PrivateRoute>{renderWithSuspense(<Task />)}</PrivateRoute>}
      />
      <Route
        path='/console/chat/:id?'
        element={renderWithSuspense(<Chat />)}
      />
      <Route path='*' element={renderWithSuspense(<NotFound />)} />
    </Routes>
  );
}

export default ConsoleRoutes;
