import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW'];

const requiredKeys = [
  '邀请返佣',
  '被邀请人数',
  '累计返佣收益',
  '我的返佣方案',
  '我的邀请用户',
  '设置返佣比例',
  '邀请用户返佣详情',
  '贡献流水',
  '单独返佣设置',
  '未选择邀请用户',
  '暂无邀请用户',
  '暂无邀请用户数据',
  '未找到匹配邀请用户',
  '搜索用户名 / 用户ID / 分组',
  '搜索',
  '按返佣收益排序，支持按用户名、用户ID或分组查找。',
  '共 {{count}} 位邀请用户',
  '当前返佣方案',
  '返佣模式',
  '所在分组',
  '返佣明细',
  '你的到账返佣',
  '返给对方',
  '本笔身份',
  '返佣类型',
  '订单号',
  '结算时间',
  '贡献概览',
  '按订单看',
  '按分录看',
  '贡献订单数',
  '你累计到账',
  '累计返给对方',
  '直推分录',
  '团队分录',
  '贡献分录明细',
  '本单贡献拆分',
  '每笔订单先看总贡献，再看下面拆分。',
  '按到账分录逐条看清楚每一笔返佣来源。',
  '暂无返佣明细',
  '直推返佣',
  '团队直返',
  '团队级差',
  '已到账',
  '已冲正',
  '待结算',
  '已单独设置',
  '购买订单',
  '默认返给对方比例',
  '被邀请人返佣比例',
  '查看邀请用户贡献给你的返佣流水，并为个别用户单独设置返佣比例。',
  '从左侧选择一位邀请用户后，可查看返佣流水并单独设置返佣比例。',
  '未单独设置时，自动使用当前返佣方案默认值。',
  '暂无可覆盖的模板作用域',
  '实际返给对方比例',
  '你本单保留比例',
  '使用默认',
  '已单独设置返佣',
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
