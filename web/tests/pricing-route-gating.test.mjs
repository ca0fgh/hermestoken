import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const helperPath = new URL('../src/helpers/headerNavModules.js', import.meta.url);
const appPath = new URL('../src/App.jsx', import.meta.url);
const publicRoutesPath = new URL('../src/routes/PublicRoutes.jsx', import.meta.url);

test('header nav helper exposes the full pricing config object', async () => {
  const source = await readFile(helperPath, 'utf8');
  assert.match(source, /export function getPricingModuleConfig/);
  assert.match(source, /return normalizeHeaderNavModules\(modules\)\.pricing;/);
});

test('app forwards pricingEnabled and pricingRequireAuth into PublicRoutes', async () => {
  const source = await readFile(appPath, 'utf8');
  assert.match(source, /const pricingConfig = useMemo/);
  assert.match(source, /pricingEnabled=\{pricingConfig\.enabled\}/);
  assert.match(source, /pricingRequireAuth=\{pricingConfig\.requireAuth\}/);
});

test('public routes render NotFound for pricing when the marketplace is disabled', async () => {
  const source = await readFile(publicRoutesPath, 'utf8');
  assert.match(source, /function PublicRoutes\(\{ pricingEnabled = true, pricingRequireAuth = false \}\)/);
  assert.match(source, /path='\/pricing'/);
  assert.match(source, /pricingEnabled \?/);
  assert.match(source, /renderWithSuspense\(<NotFound \/>/);
});
