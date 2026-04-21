import assert from 'node:assert/strict';
import test from 'node:test';

function createStorage(seed = {}) {
  const store = new Map(Object.entries(seed));
  return {
    getItem(key) {
      return store.has(key) ? store.get(key) : null;
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
    removeItem(key) {
      store.delete(key);
    },
    clear() {
      store.clear();
    },
  };
}

test('renderQuotaWithLessThanFloor keeps tiny positive currency values below the visible floor', async () => {
  const previousStorage = globalThis.localStorage;
  globalThis.localStorage = createStorage({
    quota_per_unit: '500000',
    quota_display_type: 'CNY',
    status: JSON.stringify({
      usd_exchange_rate: 1,
    }),
  });

  const { renderQuota, renderQuotaWithLessThanFloor } = await import(
    '../src/helpers/quota.js'
  );

  assert.equal(renderQuota(15), '¥0.01');
  assert.equal(renderQuotaWithLessThanFloor(15), '<¥0.01');
  assert.equal(renderQuotaWithLessThanFloor(5000), '¥0.01');
  assert.equal(renderQuotaWithLessThanFloor(0), '¥0.00');

  if (previousStorage === undefined) {
    delete globalThis.localStorage;
  } else {
    globalThis.localStorage = previousStorage;
  }
});
