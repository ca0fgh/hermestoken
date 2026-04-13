import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW'];
const requiredKeys = [
  '库存',
  '总库存',
  '剩余库存',
  '已售',
  '锁定',
  '剩余',
  '已售罄',
  '库存不足',
  '每个用户最多购买次数，0 表示不限',
  '套餐总库存，0 表示不限',
  '库存从开启后开始统计，历史销售不计入',
];

test('subscription stock locales define every new stock copy key', () => {
  locales.forEach((locale) => {
    const translation = JSON.parse(
      readFileSync(
        new URL(`../src/i18n/locales/${locale}.json`, import.meta.url),
        'utf8',
      ),
    ).translation;
    requiredKeys.forEach((key) => {
      assert.ok(
        Object.prototype.hasOwnProperty.call(translation, key),
        `missing ${key} in ${locale}`,
      );
    });
  });
});
