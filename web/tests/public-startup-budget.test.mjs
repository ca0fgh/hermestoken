import assert from 'node:assert/strict';
import test from 'node:test';

import {
  evaluatePublicStartupBudget,
  resolvePublicStartupRequestFiles,
} from '../scripts/check-public-startup-budget.mjs';

test('evaluatePublicStartupBudget accepts a healthy startup graph', () => {
  const result = evaluatePublicStartupBudget({
    requestCount: 8,
    totalStartupGzipBytes: 105 * 1024,
    jsStartupGzipBytes: 95 * 1024,
  });

  assert.deepEqual(result.errors, []);
});

test('evaluatePublicStartupBudget flags oversized startup graphs', () => {
  const result = evaluatePublicStartupBudget({
    requestCount: 16,
    totalStartupGzipBytes: 210 * 1024,
    jsStartupGzipBytes: 180 * 1024,
  });

  assert.match(
    result.errors.join('\n'),
    /public startup request budget|public startup JavaScript budget/,
  );
});

test('resolvePublicStartupRequestFiles ignores startup css that was inlined into html', () => {
  const result = resolvePublicStartupRequestFiles({
    indexHtml: `
      <!doctype html>
      <html>
        <head>
          <script type="module" crossorigin src="/assets/index.js"></script>
        </head>
      </html>
    `,
    manifest: {
      'index.html': {
        file: 'assets/index.js',
        imports: ['_react-core.js'],
        css: ['assets/index.css'],
      },
      '_react-core.js': {
        file: 'assets/react-core.js',
      },
    },
  });

  assert.deepEqual(result, ['assets/index.js', 'assets/react-core.js']);
});

test('resolvePublicStartupRequestFiles seeds the startup graph from html modulepreloads', () => {
  const result = resolvePublicStartupRequestFiles({
    indexHtml: `
      <!doctype html>
      <html>
        <head>
          <script type="module" crossorigin src="/assets/index.js"></script>
          <link rel="modulepreload" crossorigin href="/assets/react-core.js">
        </head>
      </html>
    `,
    manifest: {
      'index.html': {
        file: 'assets/index.js',
        imports: ['_react-core.js', '_chart-runtime.js'],
        css: ['assets/index.css'],
      },
      '_react-core.js': {
        file: 'assets/react-core.js',
      },
      '_chart-runtime.js': {
        file: 'assets/chart-runtime.js',
      },
    },
  });

  assert.deepEqual(result, ['assets/index.js', 'assets/react-core.js']);
});
