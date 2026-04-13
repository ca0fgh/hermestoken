import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW'];
const requiredKeys = [
  '关闭后游客按 default 分组浏览模型广场',
  'default 是游客和新注册用户的公开基础分组',
];

test('header nav module locales define the guest default-group copy keys', () => {
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
