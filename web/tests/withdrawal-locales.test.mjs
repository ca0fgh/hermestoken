import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const root = '/Users/money/project/subproject/hermestoken/web/src/i18n/locales';

test('withdrawal locales define wallet and admin copy', () => {
  const keys = ['申请提现', '提现记录', '提现管理', '冻结中余额', '确认已打款', '待审核'];
  for (const locale of ['en', 'zh-CN', 'zh-TW', 'ja', 'fr', 'ru', 'vi']) {
    const raw = fs.readFileSync(path.join(root, `${locale}.json`), 'utf8');
    for (const key of keys) {
      assert.match(raw, new RegExp(`"${key}"`));
    }
  }
});

