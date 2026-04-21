import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const pageLayoutPath = new URL('../src/components/layout/PageLayout.jsx', import.meta.url);
const statusFetchModePath = new URL(
  '../src/components/layout/pageLayoutStatusFetch.js',
  import.meta.url,
);
const publicAppPath = new URL('../src/bootstrap/publicApp.jsx', import.meta.url);
const consoleAppPath = new URL(
  '../src/bootstrap/consoleApp.jsx',
  import.meta.url,
);

test('status fetch helper only fetches full status when bootstrap requires it', async () => {
  const { shouldFetchFullStatus } = await import(statusFetchModePath);

  assert.equal(
    shouldFetchFullStatus({
      startupMode: 'public',
      isConsoleRoute: false,
      status: { system_name: 'HermesToken' },
    }),
    false,
  );
  assert.equal(
    shouldFetchFullStatus({
      startupMode: 'public',
      isConsoleRoute: false,
      status: {
        system_name: 'HermesToken',
        __publicBootstrapScope: 'home',
      },
    }),
    true,
  );
  assert.equal(
    shouldFetchFullStatus({
      startupMode: 'console',
      isConsoleRoute: false,
      status: { system_name: 'HermesToken' },
    }),
    true,
  );
  assert.equal(
    shouldFetchFullStatus({
      startupMode: 'public',
      isConsoleRoute: true,
      status: { system_name: 'HermesToken' },
    }),
    true,
  );
  assert.equal(
    shouldFetchFullStatus({
      startupMode: 'public',
      isConsoleRoute: false,
      status: null,
    }),
    true,
  );
});

test('page layout uses the shared status fetch helper and early return gate', async () => {
  const source = await readFile(pageLayoutPath, 'utf8');

  assert.match(source, /import \{ shouldFetchFullStatus \} from '\.\/pageLayoutStatusFetch';/);
  assert.match(source, /shouldFetchFullStatus\(\{/);
  assert.match(source, /if \(!shouldLoadFullStatus\) \{\s+return;\s+\}/);
});

test('public and console bootstraps pass explicit startup modes', async () => {
  const publicSource = await readFile(publicAppPath, 'utf8');
  const consoleSource = await readFile(consoleAppPath, 'utf8');

  assert.match(publicSource, /startupMode='public'/);
  assert.match(consoleSource, /startupMode='console'/);
});

test('console bootstrap no longer aliases the public bootstrap renderer', async () => {
  const source = await readFile(consoleAppPath, 'utf8');

  assert.doesNotMatch(
    source,
    /export function renderConsoleApp\(rootElement\)\s*\{\s*return renderPublicApp\(rootElement\);\s*\}/,
  );
});
