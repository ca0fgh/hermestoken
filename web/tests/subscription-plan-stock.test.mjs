import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const addEditSource = readFileSync(
  new URL(
    '../src/components/table/subscriptions/modals/AddEditSubscriptionModal.jsx',
    import.meta.url,
  ),
  'utf8',
);
const columnsSource = readFileSync(
  new URL(
    '../src/components/table/subscriptions/SubscriptionsColumnDefs.jsx',
    import.meta.url,
  ),
  'utf8',
);
const plansCardSource = readFileSync(
  new URL('../src/components/topup/SubscriptionPlansCard.jsx', import.meta.url),
  'utf8',
);
const purchaseModalSource = readFileSync(
  new URL(
    '../src/components/topup/modals/SubscriptionPurchaseModal.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('subscription admin editor exposes a separate stock_total field and stock summary copy', () => {
  assert.match(addEditSource, /field='stock_total'/);
  assert.match(addEditSource, /t\('库存'\)/);
  assert.match(addEditSource, /t\('套餐总库存，0 表示不限'\)/);
  assert.match(addEditSource, /t\('库存从开启后开始统计，历史销售不计入'\)/);
  assert.match(addEditSource, /t\('已售'\)/);
  assert.match(addEditSource, /t\('锁定'\)/);
  assert.match(addEditSource, /t\('剩余'\)/);
});

test('subscription admin table renders stock information separately from purchase limit', () => {
  assert.match(columnsSource, /t\('库存'\)/);
  assert.match(columnsSource, /t\('总库存'\)/);
  assert.match(columnsSource, /t\('剩余'\)/);
  assert.match(columnsSource, /stock_available/);
});

test('subscription purchase UI shows remaining stock and sold out state', () => {
  assert.match(plansCardSource, /t\('剩余库存'\)/);
  assert.match(plansCardSource, /t\('已售罄'\)/);
  assert.match(plansCardSource, /stock_available/);
  assert.match(purchaseModalSource, /t\('剩余库存'\)/);
  assert.match(purchaseModalSource, /t\('已售罄'\)/);
  assert.match(purchaseModalSource, /stock_available/);
});
