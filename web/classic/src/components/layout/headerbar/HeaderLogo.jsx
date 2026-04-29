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

import React from 'react';
import { Link } from 'react-router-dom';
import BrandWordmark from '../../common/BrandWordmark';
import { getOptimizedLogoUrl } from '../../../helpers/logo';

const HeaderLogo = ({
  isMobile,
  isConsoleRoute,
  logo,
  isSelfUseMode,
  isDemoSiteMode,
  t,
}) => {
  if (isMobile && isConsoleRoute) {
    return null;
  }

  const displayLogo = getOptimizedLogoUrl(logo, { size: 64 });

  return (
    <Link to='/' className='group flex items-center gap-2'>
      {displayLogo ? (
        <img
          src={displayLogo}
          alt='Logo'
          width='32'
          height='32'
          decoding='async'
          className='h-8 w-8 rounded-full object-cover transition-transform duration-200 group-hover:scale-105'
        />
      ) : null}
      <BrandWordmark
        variant='header'
        className='transition-opacity duration-200 group-hover:opacity-80'
      />
      {(isSelfUseMode || isDemoSiteMode) && (
        <span
          className={`hidden rounded-full px-2 py-0.5 text-[11px] font-medium shadow-sm md:inline-flex ${
            isSelfUseMode
              ? 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-500/15 dark:text-fuchsia-200'
              : 'bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-200'
          }`}
        >
          {isSelfUseMode ? t('自用模式') : t('演示站点')}
        </span>
      )}
    </Link>
  );
};

export default HeaderLogo;
