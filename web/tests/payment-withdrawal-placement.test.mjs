import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('withdrawal settings are only rendered inside the payment general tab', () => {
  const source = readFileSync(
    new URL('../classic/src/components/settings/PaymentSetting.jsx', import.meta.url),
    'utf8',
  );

  const generalPane = source.match(
    /<Tabs\.TabPane tab=\{t\('通用设置'\)\} itemKey='general'>([\s\S]*?)<\/Tabs\.TabPane>/,
  );

  assert.ok(generalPane, 'payment general tab pane should exist');
  assert.match(generalPane[1], /<SettingsWithdrawal\b/);

  const afterTabs = source.slice(source.indexOf('</Tabs>'));
  assert.doesNotMatch(afterTabs, /<SettingsWithdrawal\b/);
});
