import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const hookPath = new URL(
  '../src/hooks/common/useSidebarCollapsed.js',
  import.meta.url,
);

test('sidebar collapsed hook broadcasts same-tab updates and listens for sync events', async () => {
  const source = await readFile(hookPath, 'utf8');

  assert.match(source, /window\.dispatchEvent\([\s\S]*new CustomEvent\(/);
  assert.match(source, /window\.addEventListener\('storage'/);
  assert.match(
    source,
    /window\.addEventListener\([\s\S]*SIDEBAR_COLLAPSED_EVENT[\s\S]*handleSidebarCollapsedChange/,
  );
  assert.match(source, /window\.removeEventListener\('storage'/);
  assert.match(source, /setCollapsed\(event\.detail\?\.collapsed === true\)/);
});
