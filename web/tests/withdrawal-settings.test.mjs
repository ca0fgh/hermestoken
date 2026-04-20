import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const testDir = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(testDir, '..');

const readSource = (relativePath) =>
  fs.readFileSync(path.join(webRoot, relativePath), 'utf8');

test('payment settings render withdrawal settings card with inline rule editor', () => {
  const paymentSettingSource = readSource(
    'src/components/settings/PaymentSetting.jsx',
  );
  const settingsWithdrawalSource = readSource(
    'src/pages/Setting/Payment/SettingsWithdrawal.jsx',
  );
  const editorSource = readSource(
    'src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx',
  );

  assert.match(paymentSettingSource, /SettingsWithdrawal/);
  assert.match(settingsWithdrawalSource, /WithdrawalEnabled/);
  assert.match(settingsWithdrawalSource, /WithdrawalMinAmount/);
  assert.match(settingsWithdrawalSource, /WithdrawalFeeRulesEditor/);
  assert.doesNotMatch(
    settingsWithdrawalSource,
    /匹配第一条 enabled=true 且金额区间命中的规则/,
  );
  assert.match(editorSource, /新增规则/);
  assert.match(editorSource, /上移/);
  assert.match(editorSource, /下移/);
  assert.match(editorSource, /恢复默认示例/);
});
