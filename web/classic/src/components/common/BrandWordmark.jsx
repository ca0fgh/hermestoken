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
import clsx from 'clsx';

const variantClasses = {
  header:
    'text-sm sm:text-base md:text-lg tracking-[0.24em] text-slate-900 dark:text-slate-100',
  auth: 'text-2xl tracking-[0.2em] text-slate-900',
};

const BrandWordmark = ({ variant = 'header', className = '' }) => {
  return (
    <span
      className={clsx(
        'font-semibold uppercase leading-none whitespace-nowrap',
        variantClasses[variant] || variantClasses.header,
        className,
      )}
    >
      HERMESTOKEN
    </span>
  );
};

export default BrandWordmark;
