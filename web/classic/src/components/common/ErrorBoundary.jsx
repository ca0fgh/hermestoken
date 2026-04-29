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
import { AlertTriangle } from 'lucide-react';
import { withTranslation } from 'react-i18next';

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error, errorInfo) {
    console.error('[ErrorBoundary]', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      const { t } = this.props;
      return (
        <div className='flex flex-col justify-center items-center h-screen p-8'>
          <div className='flex max-w-md flex-col items-center rounded-3xl border border-slate-200 bg-white/92 px-8 py-10 text-center shadow-lg dark:border-slate-700 dark:bg-slate-900/92'>
            <div className='mb-5 rounded-full bg-rose-100 p-4 text-rose-600 dark:bg-rose-500/15 dark:text-rose-300'>
              <AlertTriangle size={36} />
            </div>
            <p className='text-base font-medium text-slate-800 dark:text-slate-100'>
              {t('页面渲染出错，请刷新页面重试')}
            </p>
            <button
              type='button'
              className='mt-6 rounded-full bg-slate-900 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'
              onClick={() => window.location.reload()}
            >
              {t('刷新页面')}
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

export default withTranslation()(ErrorBoundary);
