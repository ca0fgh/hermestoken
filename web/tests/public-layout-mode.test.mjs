import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const pageLayoutPath = new URL(
  '../src/components/layout/PageLayout.jsx',
  import.meta.url,
);
const publicAppPath = new URL('../src/bootstrap/publicApp.jsx', import.meta.url);
const consoleAppPath = new URL(
  '../src/bootstrap/consoleApp.jsx',
  import.meta.url,
);

test('page layout only fetches full status for console startup or missing bootstrap', async () => {
  const source = await readFile(pageLayoutPath, 'utf8');

  assert.match(source, /startupMode === 'console'/);
  assert.match(source, /if \(!shouldFetchFullStatus\) \{\s+return;\s+\}/);
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
