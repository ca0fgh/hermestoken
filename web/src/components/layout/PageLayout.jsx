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

import App from '../../App';
import { ToastContainer } from 'react-toastify';
import ErrorBoundary from '../common/ErrorBoundary';
import React, { Suspense, lazy, useContext, useEffect } from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useTranslation } from 'react-i18next';
import { setStatusData } from '../../helpers/data';
import { getLogo, getSystemName } from '../../helpers/branding';
import { getOptimizedLogoUrl } from '../../helpers/logo';
import { showError } from '../../helpers/notifications';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { ensureLanguageResources } from '../../i18n/i18n';
import { useLocation } from 'react-router-dom';
import { normalizeLanguage } from '../../i18n/language';
import MarketingPageLayout from './MarketingPageLayout';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';
import { shouldFetchFullStatus } from './pageLayoutStatusFetch';

const FooterBar = lazy(() => import('./Footer'));
const ConsolePageLayout = lazyWithRetry(
  () => import('./ConsolePageLayout'),
  'console-page-layout',
);

const MINIMAL_SHELL_FALLBACK = (
  <div className='min-h-screen bg-white dark:bg-slate-950' />
);


async function fetchStatusPayload() {
  const response = await fetch('/api/status', {
    headers: {
      'Cache-Control': 'no-store',
    },
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return response.json();
}

const PageLayout = ({ startupMode = 'console' }) => {
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const isMobile = useIsMobile();
  const { i18n } = useTranslation();
  const location = useLocation();

  const cardProPages = [
    '/console/channel',
    '/console/log',
    '/console/redemption',
    '/console/user',
    '/console/token',
    '/console/midjourney',
    '/console/task',
    '/console/models',
    '/pricing',
  ];

  const shouldHideFooter = cardProPages.includes(location.pathname);
  const shouldInnerPadding =
    location.pathname.includes('/console') &&
    !location.pathname.startsWith('/console/chat') &&
    location.pathname !== '/console/playground';

  const isConsoleRoute = location.pathname.startsWith('/console');
  const shouldLoadFullStatus = shouldFetchFullStatus({
    startupMode,
    isConsoleRoute,
    status: statusState?.status,
  });

  useEffect(() => {
    const rawUser = localStorage.getItem('user');
    if (!rawUser) {
      return;
    }

    try {
      userDispatch({ type: 'login', payload: JSON.parse(rawUser) });
    } catch {
      localStorage.removeItem('user');
    }
  }, [userDispatch]);

  useEffect(() => {
    if (!shouldLoadFullStatus) {
      return;
    }

    const loadStatus = async () => {
      try {
        const { success, data } = await fetchStatusPayload();
        if (success) {
          statusDispatch({ type: 'set', payload: data });
          setStatusData(data);
          return;
        }
        showError('Unable to connect to server');
      } catch {
        showError('Failed to load status');
      }
    };

    loadStatus().catch(console.error);
  }, [shouldLoadFullStatus, statusDispatch]);

  useEffect(() => {
    const systemName = getSystemName();
    if (systemName) {
      document.title = systemName;
    }

    const logo = getLogo();
    const faviconHref = getOptimizedLogoUrl(logo, { size: 32 }) || 'data:,';
    const linkElement = document.querySelector("link[rel~='icon']");
    if (linkElement) {
      linkElement.href = faviconHref;
    }
  }, [statusState?.status?.logo, statusState?.status?.system_name]);

  useEffect(() => {
    let preferredLang;

    if (userState?.user?.setting) {
      try {
        const settings = JSON.parse(userState.user.setting);
        preferredLang = normalizeLanguage(settings.language);
      } catch {
        preferredLang = undefined;
      }
    }

    if (!preferredLang) {
      const savedLang = localStorage.getItem('i18nextLng');
      if (savedLang) {
        preferredLang = normalizeLanguage(savedLang);
      }
    }

    if (!preferredLang) {
      return;
    }

    let cancelled = false;

    const applyPreferredLanguage = async () => {
      localStorage.setItem('i18nextLng', preferredLang);
      await ensureLanguageResources(preferredLang);
      if (!cancelled && preferredLang !== i18n.language) {
        i18n.changeLanguage(preferredLang);
      }
    };

    void applyPreferredLanguage();

    return () => {
      cancelled = true;
    };
  }, [i18n, userState?.user?.setting]);

  const appContent = (
    <main
      style={{
        flex: '1 0 auto',
        overflowY: isMobile ? 'visible' : 'hidden',
        WebkitOverflowScrolling: 'touch',
        padding: shouldInnerPadding ? (isMobile ? '5px' : '24px') : '0',
        position: 'relative',
      }}
    >
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </main>
  );

  const footerContent = !shouldHideFooter ? (
    <div
      style={{
        flex: '0 0 auto',
        width: '100%',
      }}
    >
      <Suspense fallback={null}>
        <FooterBar />
      </Suspense>
    </div>
  ) : null;

  const marketingShell = (
    <MarketingPageLayout
      appContent={appContent}
      footerContent={footerContent}
      isMobile={isMobile}
    />
  );

  const consoleShell = (
    <ConsolePageLayout
      appContent={appContent}
      footerContent={footerContent}
      isMobile={isMobile}
    />
  );

  return (
    <div
      className='app-layout'
      style={{
        display: 'flex',
        flexDirection: 'column',
        overflow: isMobile ? 'visible' : 'hidden',
      }}
    >
      {isConsoleRoute ? (
        <Suspense fallback={MINIMAL_SHELL_FALLBACK}>{consoleShell}</Suspense>
      ) : (
        marketingShell
      )}
      <ToastContainer />
    </div>
  );
};

export default PageLayout;
