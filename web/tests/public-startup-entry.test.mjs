import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const indexEntryPath = new URL('../src/index.jsx', import.meta.url);
const indexHtmlPath = new URL('../index.html', import.meta.url);

const loadSource = (path) => readFile(path, 'utf8');

test('index entry renders immediately instead of waiting for initializeI18n finally', async () => {
  const source = await loadSource(indexEntryPath);

  assert.doesNotMatch(
    source,
    /initializeI18n\(\)\.catch\(console\.error\)\.finally\(renderApp\)/,
  );
  assert.match(source, /renderPublicApp\(/);
  assert.match(source, /renderConsoleApp\(/);
  assert.match(source, /resolvePublicStartupBootstrap\(/);
  assert.match(source, /setPublicStartupStatusData\(/);
});

test('index template primes startup preferences before module boot', async () => {
  const source = await loadSource(indexHtmlPath);

  assert.match(source, /window\.__HERMES_CLIENT_PREFS__/);
  assert.match(source, /theme-mode/);
  assert.match(source, /prefers-color-scheme: dark/);
});
