import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const footerPath = new URL(
  '../classic/src/components/layout/Footer.jsx',
  import.meta.url,
);
const dataPath = new URL('../classic/src/helpers/data.js', import.meta.url);

const load = async (path) => readFile(path, 'utf8');

test('footer component clears stale cached footer html when storage is empty', async () => {
  const source = await load(footerPath);

  assert.match(source, /const loadFooter = \(\) => {/);
  assert.match(source, /setFooter\(footer_html \|\| ''\)/);
});

test('status persistence removes footer_html when backend returns empty', async () => {
  const source = await load(dataPath);

  assert.match(source, /if \(data\.footer_html\) {/);
  assert.match(
    source,
    /localStorage\.setItem\('footer_html', data\.footer_html\)/,
  );
  assert.match(source, /localStorage\.removeItem\('footer_html'\)/);
});
