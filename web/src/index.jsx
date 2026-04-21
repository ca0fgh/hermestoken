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

import ReactDOM from 'react-dom/client';
import 'react-toastify/dist/ReactToastify.css';
import { renderConsoleApp } from './bootstrap/consoleApp';
import { renderPublicApp } from './bootstrap/publicApp';
import { resolveRoutePublicBootstrap } from './bootstrap/publicStartup';
import { readClientStartupSettings, readInjectedBootstrap } from './helpers/bootstrapData';
import { setPublicStartupStatusData } from './helpers/data';
import { initializeI18n } from './i18n/i18n';
import './index.css';

// 欢迎信息（二次开发者未经允许不准将此移除）
// Welcome message (Do not remove this without permission from the original developer)
if (typeof window !== 'undefined') {
  console.log(
    '%cWE ❤ NEWAPI%c Github: https://github.com/QuantumNous/new-api',
    'color: #10b981; font-weight: bold; font-size: 24px;',
    'color: inherit; font-size: 14px;',
  );
}

// initialization

const rootElement = ReactDOM.createRoot(document.getElementById('root'));
const injectedBootstrap = readInjectedBootstrap();
const startupSettings = readClientStartupSettings();
const pathname = window.location.pathname;
const isConsoleRoute = pathname.startsWith('/console');
const publicBootstrap = isConsoleRoute
  ? null
  : resolveRoutePublicBootstrap({ pathname, injectedBootstrap });

if (publicBootstrap?.status) {
  try {
    setPublicStartupStatusData(publicBootstrap.status);
  } catch {
    // Ignore storage write failures during non-blocking startup.
  }
}

if (isConsoleRoute) {
  renderConsoleApp(rootElement);
} else {
  renderPublicApp(rootElement, publicBootstrap);
}

initializeI18n(startupSettings.language).catch(console.error);
