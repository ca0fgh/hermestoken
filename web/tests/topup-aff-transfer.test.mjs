import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('aff transfer modal accepts display currency amount instead of raw quota', () => {
  const source = readFileSync(
    new URL(
      '../classic/src/components/topup/modals/TransferModal.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(source, /quotaToDisplayAmount/);
  assert.match(source, /displayAmountToQuota/);
  assert.match(source, /transferDisplayAmount/);
  assert.match(source, /minTransferDisplayAmount/);
  assert.match(source, /maxTransferDisplayAmount/);
  assert.match(source, /setTransferAmount\(displayAmountToQuota\(value\)\)/);
  assert.match(source, /confirmLoading=\{transferSubmitting\}/);
});

test('topup page guards aff transfer against duplicate submissions', () => {
  const source = readFileSync(
    new URL('../classic/src/components/topup/index.jsx', import.meta.url),
    'utf8',
  );

  assert.match(source, /transferSubmitting,\s*setTransferSubmitting/);
  assert.match(source, /if \(transferSubmitting\) \{/);
  assert.match(source, /setTransferSubmitting\(true\)/);
  assert.match(source, /setTransferSubmitting\(false\)/);
  assert.match(source, /transferSubmitting=\{transferSubmitting\}/);
});
