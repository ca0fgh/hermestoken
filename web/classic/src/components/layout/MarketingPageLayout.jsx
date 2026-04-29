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

import React, { Suspense } from 'react';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';

const MarketingHeaderBar = lazyWithRetry(
  () => import('./MarketingHeaderBar'),
  'marketing-header-bar',
);

const MARKETING_HEADER_FALLBACK = (
  <div className='h-16 w-full border-b border-slate-200 bg-white/95 dark:border-slate-800 dark:bg-slate-950/95' />
);

const MarketingPageLayout = ({ appContent, footerContent, isMobile }) => {
  return (
    <>
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
        <Suspense fallback={MARKETING_HEADER_FALLBACK}>
          <MarketingHeaderBar />
        </Suspense>
      </header>
      <div
        style={{
          overflow: isMobile ? 'visible' : 'auto',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div
          style={{
            marginLeft: '0',
            flex: '1 1 auto',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {appContent}
          {footerContent}
        </div>
      </div>
    </>
  );
};

export default MarketingPageLayout;
