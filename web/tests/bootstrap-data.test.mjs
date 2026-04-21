import assert from 'node:assert/strict';
import test from 'node:test';

const bootstrapDataModulePath = new URL(
  '../src/helpers/bootstrapData.js',
  import.meta.url,
);
const publicStartupCacheModulePath = new URL(
  '../src/helpers/publicStartupCache.js',
  import.meta.url,
);

test('parsePublicBootstrapJson returns null for empty or malformed payloads', async () => {
  const { parsePublicBootstrapJson } = await import(bootstrapDataModulePath);

  assert.equal(parsePublicBootstrapJson(''), null);
  assert.equal(parsePublicBootstrapJson('{not-json'), null);
});

test('readClientStartupSettings normalizes saved theme and language', async () => {
  const { readClientStartupSettings } = await import(bootstrapDataModulePath);

  const fakeStorage = {
    getItem(key) {
      if (key === 'theme-mode') {
        return 'dark';
      }

      if (key === 'i18nextLng') {
        return 'en_US';
      }

      return null;
    },
  };

  assert.deepEqual(readClientStartupSettings(fakeStorage), {
    themeMode: 'dark',
    language: 'en',
  });
});

test('readClientStartupSettings prefers primed client preferences over storage', async () => {
  const { readClientStartupSettings } = await import(bootstrapDataModulePath);

  const fakeStorage = {
    getItem(key) {
      if (key === 'theme-mode') {
        return 'light';
      }

      if (key === 'i18nextLng') {
        return 'zh_CN';
      }

      return null;
    },
  };

  const fakeWindow = {
    __HERMES_CLIENT_PREFS__: {
      themeMode: 'auto',
      language: 'en_US',
    },
  };

  assert.deepEqual(readClientStartupSettings(fakeStorage, fakeWindow), {
    themeMode: 'auto',
    language: 'en',
  });
});

test('resolvePublicStartupBootstrap falls back to cached bootstrap payload', async () => {
  const {
    cachePublicBootstrap,
    resolvePublicStartupBootstrap,
  } = await import(publicStartupCacheModulePath);

  const storage = createMemoryStorage();
  const payload = {
    status: {
      system_name: 'HermesToken Cache',
      logo: '/cached-logo.png',
    },
  };

  cachePublicBootstrap(payload, storage);

  assert.deepEqual(resolvePublicStartupBootstrap(null, storage), payload);
});

test('resolvePublicStartupBootstrap prefers injected payload and refreshes cache', async () => {
  const {
    readCachedPublicBootstrap,
    resolvePublicStartupBootstrap,
  } = await import(publicStartupCacheModulePath);

  const storage = createMemoryStorage();
  const injectedPayload = {
    status: {
      system_name: 'HermesToken Fresh',
      logo: '/fresh-logo.png',
    },
  };

  assert.deepEqual(
    resolvePublicStartupBootstrap(injectedPayload, storage),
    injectedPayload,
  );
  assert.deepEqual(readCachedPublicBootstrap(storage), injectedPayload);
});

test('readCachedPublicBootstrap clears stale cache entries', async (t) => {
  const {
    PUBLIC_BOOTSTRAP_CACHE_KEY,
    PUBLIC_BOOTSTRAP_CACHE_TTL_MS,
    readCachedPublicBootstrap,
  } = await import(publicStartupCacheModulePath);

  const originalDateNow = Date.now;
  t.after(() => {
    Date.now = originalDateNow;
  });

  Date.now = () => 10_000;

  const storage = createMemoryStorage({
    [PUBLIC_BOOTSTRAP_CACHE_KEY]: JSON.stringify({
      savedAt: 10_000 - PUBLIC_BOOTSTRAP_CACHE_TTL_MS - 1,
      payload: {
        status: {
          system_name: 'Expired',
        },
      },
    }),
  });

  assert.equal(readCachedPublicBootstrap(storage), null);
  assert.equal(storage.removedKey, PUBLIC_BOOTSTRAP_CACHE_KEY);
});

function createMemoryStorage(initialState = {}) {
  const state = new Map(Object.entries(initialState));

  return {
    removedKey: null,
    getItem(key) {
      return state.has(key) ? state.get(key) : null;
    },
    setItem(key, value) {
      state.set(key, value);
    },
    removeItem(key) {
      this.removedKey = key;
      state.delete(key);
    },
  };
}
