import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const tokensColumnDefsPath = new URL(
  '../src/components/table/tokens/TokensColumnDefs.jsx',
  import.meta.url,
);

test('tokens table treats blank legacy selection data as inherit-user-default', async () => {
  const source = await readFile(tokensColumnDefsPath, 'utf8');

  assert.match(
    source,
    /selectionMode === 'inherit_user_default'\s*\|\|\s*\(!selectionMode && !fixedGroup\)/,
  );
});
