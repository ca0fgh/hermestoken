import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const siderBarPath = new URL(
  '../classic/src/components/layout/SiderBar.jsx',
  import.meta.url,
);
const indexCssPath = new URL('../classic/src/index.css', import.meta.url);

test('sidebar shell uses one width source and does not mutate body classes', async () => {
  const source = await readFile(siderBarPath, 'utf8');

  assert.doesNotMatch(
    source,
    /document\.body\.classList\.(add|remove)\('sidebar-collapsed'\)/,
  );
  assert.match(
    source,
    /const sidebarWidth = collapsed[\s\S]*var\(--sidebar-width-collapsed\)[\s\S]*var\(--sidebar-width\)/,
  );
  assert.match(source, /style=\{\{[\s\S]*width: sidebarWidth/);
});

test('sidebar css constrains the semi nav root and scrolls its internal list wrapper', async () => {
  const source = await readFile(indexCssPath, 'utf8');

  assert.match(source, /\.sidebar-container\s*\{[\s\S]*overflow:\s*hidden;/);
  assert.match(
    source,
    /\.sidebar-nav\.semi-navigation\s*\{[\s\S]*width:\s*100%\s*!important;[\s\S]*max-width:\s*100%;/,
  );
  assert.match(
    source,
    /\.sidebar-nav \.semi-navigation-list-wrapper\s*\{[\s\S]*overflow-y:\s*auto;[\s\S]*overflow-x:\s*hidden;/,
  );
});
