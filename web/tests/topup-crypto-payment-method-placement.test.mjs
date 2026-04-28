import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('../src/components/topup/RechargeCard.jsx', import.meta.url),
  'utf8',
);

test('USDT top-up options are rendered inside the payment method selector', () => {
  const paymentMethodSlot = source.match(
    /<Form\.Slot label=\{t\('选择支付方式'\)\}>([\s\S]*?)<\/Form\.Slot>/,
  );

  assert.ok(paymentMethodSlot, 'payment method selector should exist');
  assert.match(
    source,
    /const standardPayMethods = \(payMethods \|\| \[\]\)\.filter\([\s\S]*method\.type !== 'waffo'/,
  );
  assert.match(source, /const paymentMethodOptions = \[/);
  assert.match(source, /value: `standard:\$\{payMethod\.type\}`/);
  assert.match(source, /value: `crypto:\$\{network\.network\}`/);
  assert.match(
    source,
    /const handlePaymentMethodSelect = \(value\) =>/,
  );
  assert.match(source, /preTopUp\(option\.payMethod\.type\)/);
  assert.match(
    source,
    /createCryptoTopUpOrder\(option\.network\.network\)/,
  );
  assert.match(
    paymentMethodSlot[1],
    /<Select[\s\S]*optionList=\{paymentMethodOptions\}/,
  );
  assert.doesNotMatch(paymentMethodSlot[1], /<Button/);
});

test('RechargeCard does not render a separate USDT top-up selector', () => {
  assert.doesNotMatch(source, /<Form\.Slot label=\{t\('USDT 充值'\)\}>/);
});
