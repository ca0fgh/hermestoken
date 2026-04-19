import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('payment setting container includes explicit gateway toggle state', () => {
  const source = readFileSync(
    new URL('../src/components/settings/PaymentSetting.jsx', import.meta.url),
    'utf8',
  );

  assert.match(source, /EpayEnabled:\s*true/);
  assert.match(source, /StripeEnabled:\s*true/);
  assert.match(source, /CreemEnabled:\s*true/);
});

test('gateway setting forms render and submit dedicated enable switches', () => {
  const epaySource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGateway.jsx',
      import.meta.url,
    ),
    'utf8',
  );
  const stripeSource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGatewayStripe.jsx',
      import.meta.url,
    ),
    'utf8',
  );
  const creemSource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGatewayCreem.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(epaySource, /field='EpayEnabled'/);
  assert.match(epaySource, /key:\s*'EpayEnabled'/);
  assert.match(stripeSource, /field='StripeEnabled'/);
  assert.match(stripeSource, /key:\s*'StripeEnabled'/);
  assert.match(creemSource, /field='CreemEnabled'/);
  assert.match(creemSource, /key:\s*'CreemEnabled'/);
});
