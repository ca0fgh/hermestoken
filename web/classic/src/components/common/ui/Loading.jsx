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

const Loading = ({ size = 'small' }) => {
  const dimensionClass =
    size === 'large' ? 'h-12 w-12 border-[5px]' : 'h-8 w-8 border-4';

  return (
    <div className='fixed inset-0 w-screen h-screen flex items-center justify-center'>
      <div
        className={`animate-spin rounded-full border-slate-300 border-t-slate-900 dark:border-slate-700 dark:border-t-slate-100 ${dimensionClass}`}
      />
    </div>
  );
};

export default Loading;
