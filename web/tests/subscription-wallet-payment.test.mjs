import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const topupSource = readFileSync(
  new URL('../classic/src/components/topup/index.jsx', import.meta.url),
  'utf8',
);
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

test('subscription purchase supports wallet balance payment', () => {
  assert.match(plansCardSource, /\/api\/subscription\/wallet\/pay/);
  assert.match(plansCardSource, /selectedEpayMethod === 'wallet'/);
  assert.match(plansCardSource, /userState=\{userState\}/);
  assert.match(topupSource, /userState=\{userState\}/);
  assert.match(purchaseModalSource, /value: 'wallet'/);
  assert.match(purchaseModalSource, /paymentMethodOptions/);
  assert.match(purchaseModalSource, /optionList=\{paymentMethodOptions\}/);
  assert.match(purchaseModalSource, /renderQuota\(walletBalanceQuota\)/);
  assert.doesNotMatch(purchaseModalSource, /t\('余额支付'\)/);
});
