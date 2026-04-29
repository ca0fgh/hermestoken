import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

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

test('top-up ordinary gateway amount displays the CNY settlement amount', async () => {
  const previousStorage = globalThis.localStorage;
  globalThis.localStorage = createStorage({
    quota_display_type: 'USD',
    status: JSON.stringify({ usd_exchange_rate: 7 }),
  });

  const { formatTopUpPaymentAmount } = await import(
    '../classic/src/components/topup/topupAmount.js'
  );

  assert.equal(formatTopUpPaymentAmount(70, 'CNY'), '¥70.00');

  if (previousStorage === undefined) {
    delete globalThis.localStorage;
  } else {
    globalThis.localStorage = previousStorage;
  }
});

test('top-up Stripe amount displays the USD settlement amount', async () => {
  const previousStorage = globalThis.localStorage;
  globalThis.localStorage = createStorage({
    quota_display_type: 'CNY',
    status: JSON.stringify({ usd_exchange_rate: 7 }),
  });

  const { formatTopUpPaymentAmount } = await import(
    '../classic/src/components/topup/topupAmount.js'
  );

  assert.equal(formatTopUpPaymentAmount(10, 'USD'), '$10.00');

  if (previousStorage === undefined) {
    delete globalThis.localStorage;
  } else {
    globalThis.localStorage = previousStorage;
  }
});

test('top-up form accepts cent-level recharge quantities', () => {
  const source = readFileSync(
    new URL('../classic/src/components/topup/RechargeCard.jsx', import.meta.url),
    'utf8',
  );
  const topupSource = readFileSync(
    new URL('../classic/src/components/topup/index.jsx', import.meta.url),
    'utf8',
  );

  assert.match(source, /min=\{minTopUp\}/);
  assert.match(source, /step=\{0\.01\}/);
  assert.match(source, /precision=\{2\}/);
  assert.match(source, /parseFloat\(value\.replace\(\/\[\^\\d\.\]\/g, ''\)\)/);
  assert.doesNotMatch(source, /precision=\{0\}/);
  assert.doesNotMatch(source, /parseInt\(value\.replace/);
  assert.doesNotMatch(topupSource, /amount: parseInt\(topUpCount\)/);
  assert.match(topupSource, /amount: Number\(topUpCount\)/);
});
