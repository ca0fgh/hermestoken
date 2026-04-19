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
import { Route, Routes, useLocation, useParams } from 'react-router-dom';
import Loading from '../components/common/ui/Loading';
import { AuthRedirect, PrivateRoute } from '../helpers/auth';
import { lazyWithRetry } from '../helpers/lazyWithRetry';

const Home = lazyWithRetry(() => import('../pages/Home'), 'home-route');
const Setup = lazyWithRetry(() => import('../pages/Setup'), 'setup-route');
const About = lazyWithRetry(() => import('../pages/About'), 'about-route');
const NotFound = lazyWithRetry(
  () => import('../pages/NotFound'),
  'not-found-route',
);
const Forbidden = lazyWithRetry(
  () => import('../pages/Forbidden'),
  'forbidden-route',
);
const PasswordResetForm = lazyWithRetry(
  () => import('../components/auth/PasswordResetForm'),
  'password-reset-route',
);
const PasswordResetConfirm = lazyWithRetry(
  () => import('../components/auth/PasswordResetConfirm'),
  'password-reset-confirm-route',
);
const LoginForm = lazyWithRetry(
  () => import('../components/auth/LoginForm'),
  'login-route',
);
const RegisterForm = lazyWithRetry(
  () => import('../components/auth/RegisterForm'),
  'register-route',
);
const Chat2Link = lazyWithRetry(
  () => import('../pages/Chat2Link'),
  'chat2link-route',
);
const Pricing = lazyWithRetry(() => import('../pages/Pricing'), 'pricing-route');
const OAuth2Callback = lazyWithRetry(
  () => import('../components/auth/OAuth2Callback'),
  'oauth-callback-route',
);
const UserAgreement = lazyWithRetry(
  () => import('../pages/UserAgreement'),
  'user-agreement-route',
);
const PrivacyPolicy = lazyWithRetry(
  () => import('../pages/PrivacyPolicy'),
  'privacy-policy-route',
);

function DynamicOAuth2Callback() {
  const { provider } = useParams();
  return <OAuth2Callback type={provider} />;
}

function PublicRoutes({ pricingRequireAuth = false }) {
  const location = useLocation();
  const renderWithSuspense = (element, key = location.pathname) => (
    <Suspense fallback={<Loading />} key={key}>
      {element}
    </Suspense>
  );

  return (
    <Routes>
      <Route path='/' element={renderWithSuspense(<Home />, 'home')} />
      <Route path='/setup' element={renderWithSuspense(<Setup />)} />
      <Route path='/forbidden' element={renderWithSuspense(<Forbidden />)} />
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
      <Route path='/reset' element={renderWithSuspense(<PasswordResetForm />)} />
      <Route
        path='/oauth/github'
        element={renderWithSuspense(<OAuth2Callback type='github' />)}
      />
      <Route
        path='/oauth/discord'
        element={renderWithSuspense(<OAuth2Callback type='discord' />)}
      />
      <Route
        path='/oauth/oidc'
        element={renderWithSuspense(<OAuth2Callback type='oidc' />)}
      />
      <Route
        path='/oauth/linuxdo'
        element={renderWithSuspense(<OAuth2Callback type='linuxdo' />)}
      />
      <Route
        path='/oauth/:provider'
        element={renderWithSuspense(<DynamicOAuth2Callback />)}
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
        path='/chat2link'
        element={
          <PrivateRoute>{renderWithSuspense(<Chat2Link />)}</PrivateRoute>
        }
      />
      <Route path='*' element={renderWithSuspense(<NotFound />)} />
    </Routes>
  );
}

export default PublicRoutes;
