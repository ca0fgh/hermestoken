import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const appPath = new URL('../src/App.jsx', import.meta.url);

test('auth routes stay in the startup shell and lazy routes use retry recovery', async () => {
  const source = await readFile(appPath, 'utf8');

  assert.match(
    source,
    /import LoginForm from '\.\/components\/auth\/LoginForm';/,
  );
  assert.match(
    source,
    /import RegisterForm from '\.\/components\/auth\/RegisterForm';/,
  );
  assert.doesNotMatch(
    source,
    /const LoginForm = lazy\(\(\) => import\('\.\/components\/auth\/LoginForm'\)\);/,
  );
  assert.doesNotMatch(
    source,
    /const RegisterForm = lazy\(\(\) => import\('\.\/components\/auth\/RegisterForm'\)\);/,
  );
  assert.match(
    source,
    /import \{ lazyWithRetry \} from '\.\/helpers\/lazyWithRetry';/,
  );
  assert.match(
    source,
    /const Dashboard = lazyWithRetry\([\s\S]*import\('\.\/pages\/Dashboard'\),[\s\S]*'dashboard-route',[\s\S]*\);/,
  );
  assert.match(
    source,
    /const OAuth2Callback = lazyWithRetry\([\s\S]*import\('\.\/components\/auth\/OAuth2Callback'\),[\s\S]*'oauth-callback-route',[\s\S]*\);/,
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
