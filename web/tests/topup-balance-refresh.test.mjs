import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const topupSource = readFileSync(
  new URL('../classic/src/components/topup/index.jsx', import.meta.url),
  'utf8',
);

test('topup page refreshes wallet balance when it loads', () => {
  assert.match(
    topupSource,
    /useEffect\(\(\) => \{[\s\S]*getUserQuota\(\)\.then\(\);[\s\S]*\}, \[\]\);/,
  );
});

test('topup page refreshes wallet balance when opening billing history', () => {
  assert.match(
    topupSource,
    /const handleOpenHistory = \(\) => \{\s*getUserQuota\(\)\.then\(\);\s*setOpenHistory\(true\);/,
  );
});

test('topup page refreshes wallet balance when the crypto modal closes', () => {
  assert.match(
    topupSource,
    /const handleCryptoPaymentCancel = \(\) => \{\s*setCryptoModalOpen\(false\);\s*getUserQuota\(\)\.then\(\);/,
  );
  assert.match(
    topupSource,
    /<CryptoPaymentModal[\s\S]*onCancel=\{handleCryptoPaymentCancel\}/,
  );
});
