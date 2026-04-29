import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const plansCardSource = readFileSync(
  new URL('../classic/src/components/topup/SubscriptionPlansCard.jsx', import.meta.url),
  'utf8',
);
const purchaseModalSource = readFileSync(
  new URL(
    '../classic/src/components/topup/modals/SubscriptionPurchaseModal.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('subscription purchase modal exposes quantity input and total price summary', () => {
  assert.match(purchaseModalSource, /InputNumber/);
  assert.match(purchaseModalSource, /t\('购买数量'\)/);
  assert.match(purchaseModalSource, /quantity \* rate/);
  assert.match(purchaseModalSource, /quantity > availablePurchaseQuantity/);
});

test('subscription payment requests include quantity for every gateway', () => {
  assert.match(plansCardSource, /quantity: purchaseCount/);
  assert.match(plansCardSource, /setPurchaseQuantity\(1\)/);
  assert.match(plansCardSource, /purchaseQuantity=\{purchaseQuantity\}/);
  assert.match(plansCardSource, /setPurchaseQuantity=\{setPurchaseQuantity\}/);
});
