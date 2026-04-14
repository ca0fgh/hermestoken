import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const appPath = new URL('../src/App.jsx', import.meta.url);
const siderBarPath = new URL(
  '../src/components/layout/SiderBar.jsx',
  import.meta.url,
);

test('invite management route is wired through the live app entry', async () => {
  const source = await readFile(appPath, 'utf8');

  assert.match(source, /from '\.\/helpers\/auth';/);
  assert.match(
    source,
    /const InviteRebate = lazy\(\(\) => import\('\.\/pages\/InviteRebate'\)\);/,
  );
  assert.match(
    source,
    /path='\/console\/invite\/rebate'[\s\S]*<PrivateRoute>\{renderWithSuspense\(<InviteRebate \/>\)\}<\/PrivateRoute>/,
  );
  assert.doesNotMatch(source, /const AppRoutes = lazy\(/);
});

test('invite sidebar slice uses the live helper import path', async () => {
  const source = await readFile(siderBarPath, 'utf8');

  assert.match(source, /from '\.\.\/\.\.\/helpers\/utils';/);
  assert.doesNotMatch(source, /helpers\/notifications/);
  assert.doesNotMatch(source, /helpers\/session/);
});
