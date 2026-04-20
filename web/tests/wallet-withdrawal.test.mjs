import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const testDir = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(testDir, '..');

const readSource = (relativePath) =>
  fs.readFileSync(
    path.join(webRoot, relativePath),
    'utf8',
  );

test('wallet topup page loads withdrawal config and renders withdrawal entry', () => {
  const source = readSource('src/components/topup/index.jsx');
  const rechargeCardSource = readSource('src/components/topup/RechargeCard.jsx');
  const withdrawalModalSource = readSource(
    'src/components/topup/modals/WithdrawalApplyModal.jsx',
  );
  assert.match(source, /\/api\/user\/withdrawal\/config/);
  assert.match(source, /WithdrawalCard/);
  assert.match(source, /WithdrawalApplyModal/);
  assert.match(source, /WithdrawalHistoryModal/);
  assert.match(source, /submitWithdrawal/);
  assert.match(source, /支付宝姓名不能为空/);
  assert.match(source, /withdrawalPreview\?\.isValid/);
  assert.match(source, /当前提现金额未命中任何手续费规则，请调整金额或联系管理员/);
  assert.match(source, /withdrawalSection=\{/);
  assert.doesNotMatch(source, /<\/RechargeCard>\s*<WithdrawalCard/);
  assert.match(rechargeCardSource, /withdrawalSection = null/);
  assert.match(rechargeCardSource, /{withdrawalSection}/);
  assert.match(withdrawalModalSource, /preview\?\.isValid/);
  assert.doesNotMatch(withdrawalModalSource, /未命中手续费规则，按 0 手续费计算/);
});
