import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const testDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(testDir, '../src/i18n/locales');
const locales = ['en', 'zh-CN', 'zh-TW', 'ja', 'fr', 'ru', 'vi'];
const localizedTask4Keys = [
  '规则说明',
  '命中规则',
  '未命中任何手续费规则',
  '当前提现金额未命中任何手续费规则，请调整金额或联系管理员',
];
const localizedAdminKeys = [
  '保存提现设置',
  '使用可视化编辑器维护区间、收费方式和预览结果，保存时会自动转换为系统配置格式。',
  '检测到已保存的提现手续费规则配置无效。当前不会自动覆盖原始配置；请修复规则后重新保存，或恢复默认示例并重新配置。',
  '提现手续费规则配置已损坏，请先修复或替换后再保存。',
  '按列表顺序匹配金额区间，系统会使用第一条启用且命中的规则。',
  '暂未配置提现手续费规则，可点击“新增规则”或“恢复默认示例”。',
  '当前有未保存的规则修改，确定要放弃并继续吗？',
  '费率按百分比填写，例如 2 表示按提现金额的 2% 收费。',
  '固定金额表示每笔提现直接收取的手续费。',
  '结束金额必须是有效数字',
  '最高手续费必须是有效数字',
  '固定',
  '费率',
  '固定手续费 {{amount}}',
  '按 {{rate}}% 收费',
  '最低 {{amount}}',
  '最高 {{amount}}',
  '金额 > {{amount}}',
  '0 < 金额 <= {{amount}}',
  '{{min}} < 金额 <= {{max}}',
  '未命中手续费规则',
];
const getLocaleValue = (payload, key) => payload?.translation?.[key] ?? payload?.[key];

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
    ...localizedAdminKeys,
  ];
  for (const locale of locales) {
    const raw = fs.readFileSync(path.join(root, `${locale}.json`), 'utf8');
    for (const key of keys) {
      assert.match(raw, new RegExp(`"${key}"`));
    }
  }
});

test('task 4 user-facing labels are localized outside english locale', () => {
  const englishPayload = JSON.parse(
    fs.readFileSync(path.join(root, 'en.json'), 'utf8'),
  );

  for (const locale of ['ja', 'fr', 'ru', 'vi']) {
    const payload = JSON.parse(
      fs.readFileSync(path.join(root, `${locale}.json`), 'utf8'),
    );

    for (const key of localizedTask4Keys) {
      const localizedValue = getLocaleValue(payload, key);
      const englishValue = getLocaleValue(englishPayload, key);
      assert.ok(localizedValue, `${locale} missing translation for ${key}`);
      assert.notEqual(
        localizedValue,
        englishValue,
        `${locale} still uses English copy for ${key}`,
      );
    }
  }
});

test('new admin withdrawal editor strings are localized outside english locale', () => {
  const englishPayload = JSON.parse(
    fs.readFileSync(path.join(root, 'en.json'), 'utf8'),
  );

  for (const locale of ['ja', 'fr', 'ru', 'vi']) {
    const payload = JSON.parse(
      fs.readFileSync(path.join(root, `${locale}.json`), 'utf8'),
    );

    for (const key of localizedAdminKeys) {
      const localizedValue = getLocaleValue(payload, key);
      const englishValue = getLocaleValue(englishPayload, key);
      assert.ok(localizedValue, `${locale} missing translation for ${key}`);
      assert.notEqual(
        localizedValue,
        englishValue,
        `${locale} still uses English copy for ${key}`,
      );
    }
  }
});
