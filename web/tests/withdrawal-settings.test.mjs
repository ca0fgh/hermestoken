import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const readSource = (relativePath) =>
  fs.readFileSync(
    path.join('/Users/money/project/subproject/hermestoken/web', relativePath),
    'utf8',
  );

test('payment settings render withdrawal settings card', () => {
  const paymentSettingSource = readSource(
    'src/components/settings/PaymentSetting.jsx',
  );
  const settingsWithdrawalSource = readSource(
    'src/pages/Setting/Payment/SettingsWithdrawal.jsx',
  );

  assert.match(paymentSettingSource, /SettingsWithdrawal/);
  assert.match(settingsWithdrawalSource, /WithdrawalEnabled/);
  assert.match(settingsWithdrawalSource, /WithdrawalMinAmount/);
  assert.match(settingsWithdrawalSource, /WithdrawalFeeRules/);
  assert.match(settingsWithdrawalSource, /匹配第一条 enabled=true 且金额区间命中的规则/);
  assert.match(settingsWithdrawalSource, /fixed 表示固定手续费；ratio 表示按提现金额百分比计算手续费/);
  assert.match(settingsWithdrawalSource, /匹配顺序，值越小越先匹配/);
});
