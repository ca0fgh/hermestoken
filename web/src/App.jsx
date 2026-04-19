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
import { Route, Routes, useLocation, useParams } from 'react-router-dom';
import Loading from './components/common/ui/Loading';
import SetupCheck from './components/layout/SetupCheck';
import { AuthRedirect, PrivateRoute, AdminRoute } from './helpers/auth';
import { StatusContext } from './context/Status';
import { getPricingRequireAuth } from './helpers/headerNavModules';
import { lazyWithRetry } from './helpers/lazyWithRetry';
import RegisterForm from './components/auth/RegisterForm';
import LoginForm from './components/auth/LoginForm';

const Home = lazyWithRetry(() => import('./pages/Home'), 'home-route');
const Setup = lazyWithRetry(() => import('./pages/Setup'), 'setup-route');
const Dashboard = lazyWithRetry(
  () => import('./pages/Dashboard'),
  'dashboard-route',
);
const About = lazyWithRetry(() => import('./pages/About'), 'about-route');
const User = lazyWithRetry(() => import('./pages/User'), 'user-route');
const NotFound = lazyWithRetry(
  () => import('./pages/NotFound'),
  'not-found-route',
);
const Forbidden = lazyWithRetry(
  () => import('./pages/Forbidden'),
  'forbidden-route',
);
const Setting = lazyWithRetry(() => import('./pages/Setting'), 'setting-route');
const PasswordResetForm = lazyWithRetry(
  () => import('./components/auth/PasswordResetForm'),
  'password-reset-route',
);
const PasswordResetConfirm = lazyWithRetry(
  () => import('./components/auth/PasswordResetConfirm'),
  'password-reset-confirm-route',
);
const Channel = lazyWithRetry(() => import('./pages/Channel'), 'channel-route');
const Token = lazyWithRetry(() => import('./pages/Token'), 'token-route');
const Redemption = lazyWithRetry(
  () => import('./pages/Redemption'),
  'redemption-route',
);
const TopUp = lazyWithRetry(() => import('./pages/TopUp'), 'topup-route');
const Log = lazyWithRetry(() => import('./pages/Log'), 'log-route');
const Chat = lazyWithRetry(() => import('./pages/Chat'), 'chat-route');
const Chat2Link = lazyWithRetry(
  () => import('./pages/Chat2Link'),
  'chat2link-route',
);
const Midjourney = lazyWithRetry(
  () => import('./pages/Midjourney'),
  'midjourney-route',
);
const Pricing = lazyWithRetry(() => import('./pages/Pricing'), 'pricing-route');
const Task = lazyWithRetry(() => import('./pages/Task'), 'task-route');
const ModelPage = lazyWithRetry(() => import('./pages/Model'), 'model-route');
const ModelDeploymentPage = lazyWithRetry(
  () => import('./pages/ModelDeployment'),
  'model-deployment-route',
);
const Playground = lazyWithRetry(
  () => import('./pages/Playground'),
  'playground-route',
);
const Subscription = lazyWithRetry(
  () => import('./pages/Subscription'),
  'subscription-route',
);
const InviteRebate = lazyWithRetry(
  () => import('./pages/InviteRebate'),
  'invite-rebate-route',
);
const Withdrawal = lazyWithRetry(
  () => import('./pages/Withdrawal'),
  'withdrawal-route',
);
const OAuth2Callback = lazyWithRetry(
  () => import('./components/auth/OAuth2Callback'),
  'oauth-callback-route',
);
const PersonalSetting = lazyWithRetry(
  () => import('./components/settings/PersonalSetting'),
  'personal-setting-route',
);
const UserAgreement = lazyWithRetry(
  () => import('./pages/UserAgreement'),
  'user-agreement-route',
);
const PrivacyPolicy = lazyWithRetry(
  () => import('./pages/PrivacyPolicy'),
  'privacy-policy-route',
);

function DynamicOAuth2Callback() {
  const { provider } = useParams();
  return <OAuth2Callback type={provider} />;
}

function App() {
  const location = useLocation();
  const [statusState] = useContext(StatusContext);
  const renderWithSuspense = (element, key = location.pathname) => (
    <Suspense fallback={<Loading></Loading>} key={key}>
      {element}
    </Suspense>
  );

  const pricingRequireAuth = useMemo(() => {
    return getPricingRequireAuth(statusState?.status?.HeaderNavModules);
  }, [statusState?.status?.HeaderNavModules]);

  return (
    <SetupCheck>
      <Routes>
        <Route path='/' element={renderWithSuspense(<Home />, 'home')} />
        <Route path='/setup' element={renderWithSuspense(<Setup />)} />
        <Route path='/forbidden' element={renderWithSuspense(<Forbidden />)} />
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
          element={
            <AdminRoute>{renderWithSuspense(<Redemption />)}</AdminRoute>
          }
        />
        <Route
          path='/console/user'
          element={<AdminRoute>{renderWithSuspense(<User />)}</AdminRoute>}
        />
        <Route
          path='/console/withdrawal'
          element={
            <AdminRoute>{renderWithSuspense(<Withdrawal />)}</AdminRoute>
          }
        />
        <Route
          path='/user/reset'
          element={renderWithSuspense(<PasswordResetConfirm />)}
        />
        <Route
          path='/login'
          element={renderWithSuspense(
            <AuthRedirect>
              <LoginForm />
            </AuthRedirect>,
          )}
        />
        <Route
          path='/register'
          element={renderWithSuspense(
            <AuthRedirect>
              <RegisterForm />
            </AuthRedirect>,
          )}
        />
        <Route
          path='/reset'
          element={renderWithSuspense(<PasswordResetForm />)}
        />
        <Route
          path='/oauth/github'
          element={renderWithSuspense(
            <OAuth2Callback type='github'></OAuth2Callback>,
          )}
        />
        <Route
          path='/oauth/discord'
          element={renderWithSuspense(
            <OAuth2Callback type='discord'></OAuth2Callback>,
          )}
        />
        <Route
          path='/oauth/oidc'
          element={renderWithSuspense(
            <OAuth2Callback type='oidc'></OAuth2Callback>,
          )}
        />
        <Route
          path='/oauth/linuxdo'
          element={renderWithSuspense(
            <OAuth2Callback type='linuxdo'></OAuth2Callback>,
          )}
        />
        <Route
          path='/oauth/:provider'
          element={renderWithSuspense(<DynamicOAuth2Callback />)}
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
          path='/pricing'
          element={
            pricingRequireAuth ? (
              <PrivateRoute>{renderWithSuspense(<Pricing />)}</PrivateRoute>
            ) : (
              renderWithSuspense(<Pricing />)
            )
          }
        />
        <Route path='/about' element={renderWithSuspense(<About />)} />
        <Route
          path='/user-agreement'
          element={renderWithSuspense(<UserAgreement />)}
        />
        <Route
          path='/privacy-policy'
          element={renderWithSuspense(<PrivacyPolicy />)}
        />
        <Route
          path='/console/chat/:id?'
          element={renderWithSuspense(<Chat />)}
        />
        <Route
          path='/chat2link'
          element={
            <PrivateRoute>{renderWithSuspense(<Chat2Link />)}</PrivateRoute>
          }
        />
        <Route path='*' element={renderWithSuspense(<NotFound />)} />
      </Routes>
    </SetupCheck>
  );
}

export default App;
