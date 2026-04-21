import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const editTokenModalPath = new URL(
  '../src/components/table/tokens/modals/EditTokenModal.jsx',
  import.meta.url,
);
const tokenHookPath = new URL('../src/hooks/tokens/useTokensData.jsx', import.meta.url);

const load = async (path) => readFile(path, 'utf8');

test('token management uses token-specific group endpoint', async () => {
  const modalSource = await load(editTokenModalPath);
  const tokenHookSource = await load(tokenHookPath);

  assert.match(modalSource, /API\.get\(`\/api\/token\/groups`\)/);
  assert.doesNotMatch(modalSource, /API\.get\(`\/api\/user\/self\/groups`\)/);

  assert.match(tokenHookSource, /API\.get\('\/api\/token\/groups'\)/);
  assert.doesNotMatch(tokenHookSource, /API\.get\('\/api\/user\/self\/groups'\)/);
});
