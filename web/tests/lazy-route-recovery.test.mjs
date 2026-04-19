import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const appPath = new URL('../src/App.jsx', import.meta.url);
const consoleRoutesPath = new URL('../src/routes/ConsoleRoutes.jsx', import.meta.url);
const publicRoutesPath = new URL('../src/routes/PublicRoutes.jsx', import.meta.url);

test('route groups keep auth screens and console pages behind lazy retry recovery boundaries', async () => {
  const appSource = await readFile(appPath, 'utf8');
  const consoleRoutesSource = await readFile(consoleRoutesPath, 'utf8');
  const publicRoutesSource = await readFile(publicRoutesPath, 'utf8');

  assert.match(
    appSource,
    /const PublicRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/PublicRoutes'\),/,
  );
  assert.match(
    appSource,
    /const ConsoleRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/ConsoleRoutes'\),/,
  );
  assert.match(
    appSource,
    /import \{ lazyWithRetry \} from '\.\/helpers\/lazyWithRetry';/,
  );
  assert.match(
    publicRoutesSource,
    /const LoginForm = lazyWithRetry\([\s\S]*import\('\.\.\/components\/auth\/LoginForm'\),[\s\S]*'login-route',[\s\S]*\);/,
  );
  assert.match(
    publicRoutesSource,
    /const RegisterForm = lazyWithRetry\([\s\S]*import\('\.\.\/components\/auth\/RegisterForm'\),[\s\S]*'register-route',[\s\S]*\);/,
  );
  assert.match(
    consoleRoutesSource,
    /const Dashboard = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/Dashboard'\),[\s\S]*'dashboard-route',[\s\S]*\);/,
  );
  assert.match(
    publicRoutesSource,
    /const OAuth2Callback = lazyWithRetry\([\s\S]*import\('\.\.\/components\/auth\/OAuth2Callback'\),[\s\S]*'oauth-callback-route',[\s\S]*\);/,
  );
});

test('lazy route recovery reloads once for chunk fetch failures and then clears the retry flag', async () => {
  const { createLazyImportRecovery, isRecoverableLazyError } = await import(
    '../src/helpers/lazyWithRetry.js'
  );

  assert.equal(
    isRecoverableLazyError(
      new TypeError('Failed to fetch dynamically imported module'),
    ),
    true,
  );
  assert.equal(isRecoverableLazyError(new Error('ChunkLoadError')), true);
  assert.equal(isRecoverableLazyError(new Error('ordinary failure')), false);

  const sessionValues = new Map();
  const reloadCalls = [];
  globalThis.window = {
    location: {
      reload: () => {
        reloadCalls.push('reload');
      },
    },
    sessionStorage: {
      getItem: (key) => sessionValues.get(key) ?? null,
      setItem: (key, value) => sessionValues.set(key, String(value)),
      removeItem: (key) => sessionValues.delete(key),
    },
  };

  const recovery = createLazyImportRecovery({
    key: 'dashboard-route',
  });

  const firstResult = recovery(new TypeError('Failed to fetch dynamically imported module'));

  assert.equal(reloadCalls.length, 1);
  assert.equal(
    sessionValues.get('lazy-retry:dashboard-route'),
    '1',
  );
  assert.equal(typeof firstResult.then, 'function');

  assert.throws(
    () => recovery(new TypeError('Failed to fetch dynamically imported module')),
    /Failed to fetch dynamically imported module/,
  );
  assert.equal(reloadCalls.length, 1);

  recovery.clearRetryState();
  assert.equal(sessionValues.has('lazy-retry:dashboard-route'), false);

  delete globalThis.window;
});
