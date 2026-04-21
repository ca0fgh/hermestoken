import assert from 'node:assert/strict';
import test from 'node:test';

const startupBootstrapModulePath = new URL(
  '../src/pages/Home/startupBootstrap.js',
  import.meta.url,
);
const publicStartupCacheModulePath = new URL(
  '../src/helpers/publicStartupCache.js',
  import.meta.url,
);

test('cached bootstrap replay does not extend TTL', async () => {
  const { resolveHomeStartupBootstrap } = await import(startupBootstrapModulePath);
  const {
    PUBLIC_BOOTSTRAP_CACHE_KEY,
    cachePublicBootstrap,
    readCachedPublicBootstrap,
  } = await import(publicStartupCacheModulePath);

  const storage = createMemoryStorage();
  const payload = {
    status: {
      system_name: 'HermesToken Cache',
    },
  };

  cachePublicBootstrap(payload, storage);

  const cachedBeforeReplay = JSON.parse(storage.getItem(PUBLIC_BOOTSTRAP_CACHE_KEY));

  const resolvedPayload = resolveHomeStartupBootstrap({
    readInjectedBootstrap: () => null,
    readCachedPublicBootstrap: () => readCachedPublicBootstrap(storage),
    cachePublicBootstrap: (nextPayload) => cachePublicBootstrap(nextPayload, storage),
  });

  const cachedAfterReplay = JSON.parse(storage.getItem(PUBLIC_BOOTSTRAP_CACHE_KEY));

  assert.deepEqual(resolvedPayload, payload);
  assert.equal(cachedAfterReplay.savedAt, cachedBeforeReplay.savedAt);
});

test('injected bootstrap refreshes the cache for startup warmup', async (t) => {
  const { resolveHomeStartupBootstrap } = await import(startupBootstrapModulePath);
  const {
    PUBLIC_BOOTSTRAP_CACHE_KEY,
    cachePublicBootstrap,
  } = await import(publicStartupCacheModulePath);

  const originalDateNow = Date.now;
  t.after(() => {
    Date.now = originalDateNow;
  });

  Date.now = () => 20_000;

  const storage = createMemoryStorage();
  const payload = {
    status: {
      system_name: 'HermesToken Fresh',
    },
  };

  const resolvedPayload = resolveHomeStartupBootstrap({
    readInjectedBootstrap: () => payload,
    readCachedPublicBootstrap: () => null,
    cachePublicBootstrap: (nextPayload) => cachePublicBootstrap(nextPayload, storage),
  });

  const cachedEntry = JSON.parse(storage.getItem(PUBLIC_BOOTSTRAP_CACHE_KEY));

  assert.deepEqual(resolvedPayload, payload);
  assert.equal(cachedEntry.savedAt, 20_000);
});

function createMemoryStorage(initialState = {}) {
  const state = new Map(Object.entries(initialState));

  return {
    getItem(key) {
      return state.has(key) ? state.get(key) : null;
    },
    setItem(key, value) {
      state.set(key, value);
    },
    removeItem(key) {
      state.delete(key);
    },
  };
}
