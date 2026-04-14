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

import React, { lazy, Suspense, useContext, useMemo } from 'react';
import { Route, Routes, useLocation, useParams } from 'react-router-dom';
import Loading from './components/common/ui/Loading';
import SetupCheck from './components/layout/SetupCheck';
import { AuthRedirect, PrivateRoute, AdminRoute } from './helpers/auth';
import { StatusContext } from './context/Status';
import { getPricingRequireAuth } from './helpers/headerNavModules';

const Home = lazy(() => import('./pages/Home'));
const Setup = lazy(() => import('./pages/Setup'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const About = lazy(() => import('./pages/About'));
const User = lazy(() => import('./pages/User'));
const RegisterForm = lazy(() => import('./components/auth/RegisterForm'));
const LoginForm = lazy(() => import('./components/auth/LoginForm'));
const NotFound = lazy(() => import('./pages/NotFound'));
const Forbidden = lazy(() => import('./pages/Forbidden'));
const Setting = lazy(() => import('./pages/Setting'));
const PasswordResetForm = lazy(
  () => import('./components/auth/PasswordResetForm'),
);
const PasswordResetConfirm = lazy(
  () => import('./components/auth/PasswordResetConfirm'),
);
const Channel = lazy(() => import('./pages/Channel'));
const Token = lazy(() => import('./pages/Token'));
const Redemption = lazy(() => import('./pages/Redemption'));
const TopUp = lazy(() => import('./pages/TopUp'));
const Log = lazy(() => import('./pages/Log'));
const Chat = lazy(() => import('./pages/Chat'));
const Chat2Link = lazy(() => import('./pages/Chat2Link'));
const Midjourney = lazy(() => import('./pages/Midjourney'));
const Pricing = lazy(() => import('./pages/Pricing'));
const Task = lazy(() => import('./pages/Task'));
const ModelPage = lazy(() => import('./pages/Model'));
const ModelDeploymentPage = lazy(() => import('./pages/ModelDeployment'));
const Playground = lazy(() => import('./pages/Playground'));
const Subscription = lazy(() => import('./pages/Subscription'));
const InviteRebate = lazy(() => import('./pages/InviteRebate'));
const OAuth2Callback = lazy(() => import('./components/auth/OAuth2Callback'));
const PersonalSetting = lazy(
  () => import('./components/settings/PersonalSetting'),
);
const UserAgreement = lazy(() => import('./pages/UserAgreement'));
const PrivacyPolicy = lazy(() => import('./pages/PrivacyPolicy'));

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
