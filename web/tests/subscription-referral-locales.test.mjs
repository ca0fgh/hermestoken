/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW'];

const requiredKeys = [
  '新增覆盖',
  '返佣类型',
  '订阅返佣',
  '暂无覆盖时使用默认返佣规则',
  '暂无覆盖项，未设置时使用默认返佣规则',
  '当前默认总返佣率',
  '请选择返佣类型',
  '该返佣类型和分组组合已存在',
  '覆盖总返佣率必须为数字',
  '覆盖总返佣率必须在 0 到 100 之间',
];

function loadTranslation(locale) {
  const content = readFileSync(
    new URL(`../src/i18n/locales/${locale}.json`, import.meta.url),
    'utf8',
  );

  return JSON.parse(content).translation;
}

test('subscription referral locales define the override copy keys', () => {
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
