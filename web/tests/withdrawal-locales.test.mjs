import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const testDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(testDir, '../src/i18n/locales');

test('withdrawal locales define wallet and admin copy', () => {
  const keys = [
    '申请提现',
    '提现记录',
    '提现管理',
    '冻结中余额',
    '确认已打款',
    '待审核',
    '规则说明',
    '命中规则',
    '未命中任何手续费规则',
    '当前提现金额未命中任何手续费规则，请调整金额或联系管理员',
    '大于 0 且不超过 {{amountWithSymbol}}',
    '高于 {{minWithSymbol}} 至 {{maxWithSymbol}}',
    '高于 {{amountWithSymbol}}',
    '固定手续费 {{amountWithSymbol}}',
    '按 {{rate}}% 收费',
    '最低手续费 {{amountWithSymbol}}',
    '最高手续费 {{amountWithSymbol}}',
  ];
  for (const locale of ['en', 'zh-CN', 'zh-TW', 'ja', 'fr', 'ru', 'vi']) {
    const raw = fs.readFileSync(path.join(root, `${locale}.json`), 'utf8');
    for (const key of keys) {
      assert.match(raw, new RegExp(`"${key}"`));
    }
  }
});
