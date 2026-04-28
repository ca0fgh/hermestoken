import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL(
    '../src/components/topup/modals/CryptoPaymentModal.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('crypto payment modal renders an obvious success state after completion', () => {
  assert.match(source, /const isSuccess = order\?\.status === 'success'/);
  assert.doesNotMatch(source, /type=\{isSuccess \? 'success' : 'warning'\}/);
  assert.doesNotMatch(source, /t\('充值成功，额度已自动入账'\)/);
  assert.match(source, /\{isSuccess \? t\('订单已完成'\) : order\.status\}/);
  assert.match(
    source,
    /color=\{isSuccess \? 'green' : terminal \? 'grey' : 'orange'\}/,
  );
});

test('crypto payment modal keeps order countdown ticking while pending', () => {
  assert.match(
    source,
    /import React, \{ useEffect, useMemo, useState \} from 'react';/,
  );
  assert.match(source, /const \[now, setNow\] = useState/);
  assert.match(source, /setInterval\(\(\) => setNow\(Math\.floor\(Date\.now\(\) \/ 1000\)\), 1000\)/);
  assert.match(source, /clearInterval\(timer\)/);
  assert.match(source, /order\.expires_at - now/);
});

test('crypto payment modal hides countdown and caps confirmation text after success', () => {
  assert.match(source, /!isSuccess && \(/);
  assert.match(
    source,
    /Math\.min\(order\.confirmations \|\| 0, order\.required_confirmations\)/,
  );
});

test('crypto payment modal renders a scannable receive address QR code', () => {
  assert.match(source, /import \{ QRCodeSVG \} from 'qrcode\.react';/);
  assert.match(source, /const paymentQrValue = order\?\.receive_address \|\| '';/);
  assert.match(source, /<QRCodeSVG[\s\S]*value=\{paymentQrValue\}/);
  assert.match(source, /aria-label=\{t\('收款地址二维码'\)\}/);
});
