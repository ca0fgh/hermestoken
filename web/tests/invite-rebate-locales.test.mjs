import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW'];

const requiredKeys = [
  '邀请返佣',
  '被邀请人数',
  '累计返佣收益',
  '默认返佣规则',
  '邀请用户',
  '邀请用户独立返佣',
  '未选择邀请用户',
  '暂无邀请用户',
  '搜索用户名',
  '搜索',
  '返佣类型',
  '分组',
  '被邀请人返佣比例',
  '未设置独立返佣时，使用默认规则',
  '当前默认返佣率',
];

function loadTranslation(locale) {
  const content = readFileSync(
    new URL(`../src/i18n/locales/${locale}.json`, import.meta.url),
    'utf8',
  );

  return JSON.parse(content).translation;
}

test('invite rebate locales define the page copy keys', () => {
  locales.forEach((locale) => {
    const translation = loadTranslation(locale);
    assert.ok(
      translation && typeof translation === 'object',
      `unexpected structure for ${locale}`,
    );

    requiredKeys.forEach((key) => {
      assert.ok(
        Object.prototype.hasOwnProperty.call(translation, key),
        `missing ${key} in ${locale}`,
      );
    });
  });
});
