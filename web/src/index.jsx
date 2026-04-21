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
import { readClientStartupSettings, readInjectedBootstrap } from './helpers/bootstrapData';
import { cachePublicBootstrap } from './helpers/publicStartupCache';
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

if (injectedBootstrap) {
  cachePublicBootstrap(injectedBootstrap);
}

if (window.location.pathname.startsWith('/console')) {
  renderConsoleApp(rootElement);
} else {
  renderPublicApp(rootElement, injectedBootstrap);
}

initializeI18n(startupSettings.language).catch(console.error);
