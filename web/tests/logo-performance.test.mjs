import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import { getOptimizedLogoUrl, isUploadedLogoUrl } from '../src/helpers/logo.js';

const pageLayoutPath = new URL(
  '../src/components/layout/PageLayout.jsx',
  import.meta.url,
);

test('uploaded logos get a deterministic size parameter for small display surfaces', () => {
  assert.equal(
    getOptimizedLogoUrl('/api/logo?v=123', { size: 64 }),
    '/api/logo?v=123&size=64',
  );
  assert.equal(
    getOptimizedLogoUrl('/api/logo?v=123&size=512', { size: 32 }),
    '/api/logo?v=123&size=32',
  );
  assert.equal(
    getOptimizedLogoUrl('https://cdn.example.com/logo.png', { size: 64 }),
    'https://cdn.example.com/logo.png',
  );
  assert.equal(getOptimizedLogoUrl('', { size: 64 }), '');
});

test('uploaded logo detection only matches the local logo endpoint', () => {
  assert.equal(isUploadedLogoUrl('/api/logo?v=123'), true);
  assert.equal(isUploadedLogoUrl('/logo.png'), false);
  assert.equal(isUploadedLogoUrl('https://cdn.example.com/logo.png'), false);
});

test('page layout derives favicon href from the optimized logo helper', async () => {
  const source = await readFile(pageLayoutPath, 'utf8');

  assert.match(
    source,
    /const \[statusState,\s*statusDispatch\] = useContext\(StatusContext\);/,
  );
  assert.match(source, /getOptimizedLogoUrl\(logo,\s*\{\s*size:\s*32\s*\}\)/);
  assert.doesNotMatch(source, /linkElement\.href = logo \|\| 'data:,';/);
});
