import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const readSource = (relativePath) =>
  fs.readFileSync(
    path.join('/Users/money/project/subproject/hermestoken/web', relativePath),
    'utf8',
  );

test('wallet topup page loads withdrawal config and renders withdrawal entry', () => {
  const source = readSource('src/components/topup/index.jsx');
  const rechargeCardSource = readSource('src/components/topup/RechargeCard.jsx');
  assert.match(source, /\/api\/user\/withdrawal\/config/);
  assert.match(source, /WithdrawalCard/);
  assert.match(source, /WithdrawalApplyModal/);
  assert.match(source, /WithdrawalHistoryModal/);
  assert.match(source, /submitWithdrawal/);
  assert.match(source, /支付宝姓名不能为空/);
  assert.match(source, /withdrawalSection=\{/);
  assert.doesNotMatch(source, /<\/RechargeCard>\s*<WithdrawalCard/);
  assert.match(rechargeCardSource, /withdrawalSection = null/);
  assert.match(rechargeCardSource, /{withdrawalSection}/);
});
