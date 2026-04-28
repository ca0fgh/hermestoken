import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('USDT payment tab hides the redundant inner section title', () => {
  const paymentSource = readFileSync(
    new URL('../src/components/settings/PaymentSetting.jsx', import.meta.url),
    'utf8',
  );
  const cryptoSource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(
    paymentSource,
    /<SettingsPaymentGatewayCrypto[\s\S]*hideSectionTitle[\s\S]*\/>/,
  );
  assert.match(
    cryptoSource,
    /const sectionTitle = props\.hideSectionTitle \? undefined : t\('USDT 设置'\);/,
  );
  assert.doesNotMatch(
    cryptoSource,
    /<Form\.Section text=\{t\('USDT 设置'\)\}>/,
  );
});

test('USDT unique suffix setting supports six digits', () => {
  const cryptoSource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(cryptoSource, /CryptoUniqueSuffixMax: 9999/);
  assert.match(
    cryptoSource,
    /field='CryptoUniqueSuffixMax'[\s\S]*max=\{999999\}/,
  );
});

test('USDT payment settings expose Polygon PoS and Solana chains', () => {
  const paymentSource = readFileSync(
    new URL('../src/components/settings/PaymentSetting.jsx', import.meta.url),
    'utf8',
  );
  const cryptoSource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(paymentSource, /CryptoPolygonEnabled: false/);
  assert.match(paymentSource, /CryptoSolanaEnabled: false/);
  assert.match(cryptoSource, /CryptoPolygonUSDTContract/);
  assert.match(cryptoSource, /0xc2132D05D31c914a87C6611C10748AEb04B58e8F/);
  assert.match(cryptoSource, /CryptoSolanaUSDTMint/);
  assert.match(cryptoSource, /Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB/);
  assert.match(cryptoSource, /<Form\.Section text='Polygon PoS'>/);
  assert.match(cryptoSource, /<Form\.Section text='Solana'>/);
});

test('USDT payment settings submit cleared Solana address as empty string', async () => {
  const { buildCryptoPaymentOptionUpdates } = await import(
    '../src/helpers/paymentCrypto.js'
  );
  const updates = buildCryptoPaymentOptionUpdates(
    {
      CryptoSolanaReceiveAddress: 'old-address',
      CryptoSolanaRPCURL: 'https://rpc.ankr.com/solana',
      CryptoTronAPIKey: '',
    },
    {
      CryptoSolanaReceiveAddress: undefined,
    },
  );

  assert.deepEqual(
    updates.find((item) => item.key === 'CryptoSolanaReceiveAddress'),
    { key: 'CryptoSolanaReceiveAddress', value: '' },
  );
  assert.equal(
    updates.find((item) => item.key === 'CryptoSolanaRPCURL')?.value,
    'https://rpc.ankr.com/solana',
  );
  assert.equal(
    updates.some((item) => item.key === 'CryptoTronAPIKey'),
    false,
  );

  const cryptoSource = readFileSync(
    new URL(
      '../src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx',
      import.meta.url,
    ),
    'utf8',
  );
  assert.match(cryptoSource, /formApiRef\.current\?\.getValues\?\.\(\)/);
  assert.match(
    cryptoSource,
    /buildCryptoPaymentOptionUpdates\(inputs, formValues\)/,
  );
});
