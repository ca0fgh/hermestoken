import assert from 'node:assert/strict';
import test from 'node:test';

const store = new Map();

globalThis.localStorage = {
  getItem(key) {
    return store.get(key) ?? null;
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

const { formatTopUpPaymentAmount } = await import(
  '../src/components/topup/topupAmount.js'
);

test('formats ordinary gateway amount in CNY even when account display is USD', () => {
  localStorage.setItem('quota_display_type', 'USD');
  localStorage.setItem(
    'status',
    JSON.stringify({
      usd_exchange_rate: 1,
      custom_currency_symbol: '¤',
      custom_currency_exchange_rate: 1,
    }),
  );

  assert.equal(formatTopUpPaymentAmount(0.069, 'CNY'), '¥0.07');
});

test('formats Stripe amount in USD', () => {
  localStorage.setItem('quota_display_type', 'CNY');
  localStorage.setItem('status', JSON.stringify({ usd_exchange_rate: 6.9 }));

  assert.equal(formatTopUpPaymentAmount(0.01, 'USD'), '$0.01');
});

test('formats crypto amount in USDT without currency conversion', () => {
  localStorage.setItem('quota_display_type', 'USD');
  localStorage.setItem('status', JSON.stringify({ usd_exchange_rate: 6.9 }));

  assert.equal(formatTopUpPaymentAmount(0.012677, 'USDT'), '0.012677 USDT');
});
