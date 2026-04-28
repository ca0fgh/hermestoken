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
const HOME_DEFERRED_PRELOAD_PATTERN = /(?:semi-core|semi-icons|visactor|data-viz)-/;
const PUBLIC_STARTUP_DEFERRED_PRELOAD_PATTERN =
  /(?:semi-runtime|chart-runtime|diagram-runtime|math-runtime|markdown-runtime|seasonal-effects)-/;
const apiProxyTarget = process.env.VITE_PROXY_TARGET || 'http://localhost:3000';
const DEFAULT_ASSET_BASE_URL = '/';
const STARTUP_STYLE_FILE_PREFIX = 'assets/index-';

function normalizeAssetBaseUrl(value) {
  const trimmed = (value || '').trim();
  if (!trimmed) {
    return DEFAULT_ASSET_BASE_URL;
  }

  if (/^https?:\/\//i.test(trimmed)) {
    return `${trimmed.replace(/\/+$/, '')}/`;
  }

  const normalizedPath = trimmed.replace(/^\/+/, '').replace(/\/+$/, '');
  if (!normalizedPath) {
    return DEFAULT_ASSET_BASE_URL;
  }

  return `/${normalizedPath}/`;
}

const assetBaseUrl = normalizeAssetBaseUrl(process.env.VITE_ASSET_BASE_URL);

function escapeHtmlTagSource(source) {
  return source.replace(/<\/style/gi, '<\\/style');
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function getStartupStyleAsset(bundle) {
  return Object.values(bundle).find((asset) => {
    return (
      asset?.type === 'asset' &&
      typeof asset.fileName === 'string' &&
      asset.fileName.startsWith(STARTUP_STYLE_FILE_PREFIX) &&
      asset.fileName.endsWith('.css')
    );
  });
}

function inlineStartupStyles() {
  return {
    name: 'inline-startup-styles',
    apply: 'build',
    transformIndexHtml: {
      order: 'post',
      handler(html, context) {
        if (!context.bundle) {
          return html;
        }

        const startupStyle = getStartupStyleAsset(context.bundle);
        if (!startupStyle) {
          return html;
        }

        const styleSource = String(startupStyle.source || '');
        if (!styleSource) {
          return html;
        }

        const startupStyleHref = `${assetBaseUrl}${startupStyle.fileName}`;
        const stylesheetTagPattern = new RegExp(
          `<link rel="stylesheet" crossorigin href="${escapeRegExp(startupStyleHref)}">`,
        );

        if (!stylesheetTagPattern.test(html)) {
          return html;
        }

        return html.replace(
          stylesheetTagPattern,
          `<style data-inline-startup-css>${escapeHtmlTagSource(styleSource)}</style>`,
        );
      },
    },
  };
}

function detachChartInteropFromReactCore() {
  return {
    name: 'detach-chart-interop-from-react-core',
    apply: 'build',
    generateBundle(_options, bundle) {
      const reactCoreChunk = Object.values(bundle).find((chunk) => {
        return chunk.type === 'chunk' && chunk.name === 'react-core';
      });
      const chartRuntimeChunk = Object.values(bundle).find((chunk) => {
        return chunk.type === 'chunk' && chunk.name === 'chart-runtime';
      });

      if (!reactCoreChunk || !chartRuntimeChunk) {
        return;
      }

      const chartInteropImportPattern =
        /import\{g as (\w+)\}from["']\.\/[^"']*chart-runtime[^"']*["'];?/;
      const match = reactCoreChunk.code.match(chartInteropImportPattern);

      if (!match) {
        return;
      }

      const helperName = match[1];

      reactCoreChunk.code = [
        `function ${helperName}(module) {`,
        `  return module && module.__esModule && Object.prototype.hasOwnProperty.call(module, 'default')`,
        `    ? module.default`,
        `    : module;`,
        `}`,
        reactCoreChunk.code.replace(chartInteropImportPattern, ''),
      ].join('\n');
      reactCoreChunk.imports = reactCoreChunk.imports.filter((fileName) => {
        return fileName !== chartRuntimeChunk.fileName;
      });
    },
  };
}

function buildManualChunkName(id) {
  if (
    id.includes('vite/preload-helper') ||
    id.includes('vite/modulepreload-polyfill')
  ) {
    return 'startup-runtime';
  }

  if (
    id.endsWith('/src/helpers/bootstrapData.js') ||
    id.endsWith('/src/helpers/lazyWithRetry.js') ||
    id.endsWith('/src/helpers/publicStartupCache.js') ||
    id.endsWith('/src/pages/Home/startupBootstrap.js')
  ) {
    return 'startup-runtime';
  }

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
    id.includes('i18next')
  ) {
    return 'react-core';
  }

  // Keep Semi icons together so authenticated routes do not fan out into
  // dozens of tiny cross-origin module requests for each icon.
  if (id.includes('@douyinfe/semi-icons')) {
    return 'semi-icons';
  }

  if (
    id.includes('@douyinfe/semi-ui') ||
    id.includes('@douyinfe/semi-foundation') ||
    id.includes('@douyinfe/semi-theme-default') ||
    id.includes('@douyinfe/semi-illustrations')
  ) {
    return 'semi-runtime';
  }

  if (
    id.includes('@visactor/vchart') ||
    id.includes('@visactor/vchart-semi-theme')
  ) {
    return 'chart-runtime';
  }

  if (id.includes('/mermaid/')) {
    return 'diagram-runtime';
  }

  if (
    id.includes('/katex/') ||
    id.includes('remark-math') ||
    id.includes('rehype-katex')
  ) {
    return 'math-runtime';
  }

  if (id.includes('axios')) {
    return 'api-client';
  }

  if (id.includes('react-toastify')) {
    return 'startup-runtime';
  }

  if (
    id.includes('/marked/') ||
    id.includes('react-markdown') ||
    id.includes('remark-breaks') ||
    id.includes('remark-gfm') ||
    id.includes('rehype-highlight') ||
    id.includes('unist-util-visit')
  ) {
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
  base: assetBaseUrl,
  json: {
    stringify: true,
  },
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
    inlineStartupStyles(),
    detachChartInteropFromReactCore(),
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
    manifest: true,
    modulePreload: {
      resolveDependencies(_filename, deps) {
        return deps.filter((dependency) => {
          return (
            !HOME_DEFERRED_PRELOAD_PATTERN.test(dependency) &&
            !PUBLIC_STARTUP_DEFERRED_PRELOAD_PATTERN.test(dependency)
          );
        });
      },
    },
    rollupOptions: {
      onwarn: handleBuildWarning,
      output: {
        hoistTransitiveImports: false,
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
