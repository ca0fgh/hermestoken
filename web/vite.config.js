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

import react from '@vitejs/plugin-react';
import { defineConfig, transformWithEsbuild } from 'vite';
import pkg from '@douyinfe/vite-plugin-semi';
import path from 'path';
import { codeInspectorPlugin } from 'code-inspector-plugin';
const { vitePluginSemi } = pkg;
const LOTTIE_EVAL_WARNING_PATH = 'lottie-web/build/player/lottie.js';
const HOME_DEFERRED_PRELOAD_PATTERN = /(?:semi-core|visactor|data-viz)-/;
const apiProxyTarget = process.env.VITE_PROXY_TARGET || 'http://localhost:3000';

function buildManualChunkName(id) {
  if (!id.includes('node_modules')) {
    return undefined;
  }

  if (id.includes('react-fireworks')) {
    return 'seasonal-effects';
  }

  // Keep startup-critical runtime and shared UI primitives together.
  // Route-specific heavyweight libraries move to dedicated async chunks.
  if (
    id.includes('react-router-dom') ||
    id.includes('/react/') ||
    id.includes('scheduler') ||
    id.includes('i18next') ||
    id.includes('lucide-react')
  ) {
    return 'react-core';
  }

  if (
    id.includes('@douyinfe/semi') ||
    id.includes('@douyinfe/semi-ui') ||
    id.includes('@douyinfe/semi-icons')
  ) {
    return 'semi-vendor';
  }

  if (id.includes('/history/')) {
    return 'history';
  }

  if (id.includes('axios')) {
    return 'api-client';
  }

  if (id.includes('/marked/')) {
    return 'markdown-runtime';
  }

  return undefined;
}

function handleBuildWarning(warning, warn) {
  const warningId = warning.id || warning.loc?.file || '';
  const warningMessage = warning.message || '';

  if (
    warningId.includes(LOTTIE_EVAL_WARNING_PATH) &&
    (warning.code === 'EVAL' || warningMessage.includes('Use of eval'))
  ) {
    return;
  }

  warn(warning);
}

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  plugins: [
    codeInspectorPlugin({
      bundler: 'vite',
    }),
    {
      name: 'treat-js-files-as-jsx',
      async transform(code, id) {
        if (!/src\/.*\.js$/.test(id)) {
          return null;
        }

        // Use the exposed transform from vite, instead of directly
        // transforming with esbuild
        return transformWithEsbuild(code, id, {
          loader: 'jsx',
          jsx: 'automatic',
        });
      },
    },
    react(),
    vitePluginSemi({
      cssLayer: true,
    }),
  ],
  optimizeDeps: {
    force: true,
    esbuildOptions: {
      loader: {
        '.js': 'jsx',
        '.json': 'json',
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 3500,
    modulePreload: {
      resolveDependencies(_filename, deps) {
        return deps.filter((dependency) => {
          return !HOME_DEFERRED_PRELOAD_PATTERN.test(dependency);
        });
      },
    },
    rollupOptions: {
      onwarn: handleBuildWarning,
      output: {
        manualChunks: buildManualChunkName,
      },
    },
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true,
      },
      '/mj': {
        target: apiProxyTarget,
        changeOrigin: true,
      },
      '/pg': {
        target: apiProxyTarget,
        changeOrigin: true,
      },
    },
  },
});
