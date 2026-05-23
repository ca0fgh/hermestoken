import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const subscriptionPlansCardSource = readFileSync(
  new URL(
    '../default/src/features/wallet/components/subscription-plans-card.tsx',
    import.meta.url,
  ),
  'utf8',
);
const walletTypesSource = readFileSync(
  new URL('../default/src/features/wallet/types.ts', import.meta.url),
  'utf8',
);
const classicSubscriptionPlansCardSource = readFileSync(
  new URL(
    '../classic/src/components/topup/SubscriptionPlansCard.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('default subscription purchase gates Waffo Pancake by subscription availability instead of wallet topup availability', () => {
  assert.match(walletTypesSource, /enable_waffo_pancake_subscription\?: boolean/);
  assert.match(
    subscriptionPlansCardSource,
    /const enableWaffoPancake =\s*!!topupInfo\?\.enable_waffo_pancake_subscription/,
  );
  assert.doesNotMatch(
    subscriptionPlansCardSource,
    /const enableWaffoPancake =\s*!!topupInfo\?\.enable_waffo_pancake_topup/,
  );
});

test('subscription epay method lists exclude dedicated Waffo gateways', () => {
  for (const source of [
    subscriptionPlansCardSource,
    classicSubscriptionPlansCardSource,
  ]) {
    assert.match(source, /m\.type !== 'waffo'/);
    assert.match(source, /m\.type !== 'waffo_pancake'/);
  }
});
