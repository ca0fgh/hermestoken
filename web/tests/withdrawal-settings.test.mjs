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

test('settings withdrawal tracks persisted fee rule parse errors instead of silently normalizing them away', () => {
  const settingsWithdrawalSource = readSource(
    'src/pages/Setting/Payment/SettingsWithdrawal.jsx',
  );
  const helperSource = readSource('src/helpers/withdrawal.js');

  assert.match(helperSource, /export const parsePersistedWithdrawalFeeRules\s*=/);
  assert.match(settingsWithdrawalSource, /withdrawalFeeRulesInvalidState/i);
  assert.match(settingsWithdrawalSource, /showError\(t\('提现手续费规则配置已损坏，请先修复或替换后再保存。'\)\)/);
  assert.match(
    settingsWithdrawalSource,
    /invalid persisted WithdrawalFeeRules before normalizing into editor state/i,
  );
  assert.match(
    settingsWithdrawalSource,
    /description=\{t\('检测到已保存的提现手续费规则配置无效。当前不会自动覆盖原始配置；请修复规则后重新保存，或恢复默认示例并重新配置。'\)\}/,
  );
});

test('inline rule form rejects invalid optional numeric input instead of treating it as unlimited', () => {
  const inlineFormSource = readSource(
    'src/components/settings/withdrawal/WithdrawalFeeRuleInlineForm.jsx',
  );

  assert.match(inlineFormSource, /optionalFieldErrors/);
  assert.match(inlineFormSource, /结束金额必须是有效数字/);
  assert.match(inlineFormSource, /最高手续费必须是有效数字/);
  assert.match(inlineFormSource, /disabled=\{hasOptionalFieldErrors\}/);
});

test('withdrawal fee rule editor guards unsaved draft changes before replacing the active draft', () => {
  const editorSource = readSource(
    'src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx',
  );

  assert.match(editorSource, /hasDraftChanges/);
  assert.match(editorSource, /window\.confirm/);
  assert.match(editorSource, /handleDraftReplacement/);
  assert.match(editorSource, /当前有未保存的规则修改/);
  assert.match(editorSource, /describeWithdrawalFeeRule\(rule,\s*t\)/);
});
