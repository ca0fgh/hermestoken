import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

function readSource(relativePath) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

test('InviteRebate route renders the dedicated page shell', () => {
  const pageSource = readSource('src/pages/InviteRebate/index.js');

  assert.match(
    pageSource,
    /from ['"]..\/..\/components\/invite-rebate\/InviteRebatePage['"]/,
  );
  assert.match(pageSource, /<InviteRebatePage \/>/);
});

test('InviteRebatePage composes summary, default rules, invitee list, and override panel', () => {
  const pageSource = readSource(
    'src/components/invite-rebate/InviteRebatePage.jsx',
  );

  assert.match(pageSource, /InviteRebateSummary/);
  assert.match(pageSource, /InviteDefaultRuleSection/);
  assert.match(pageSource, /InviteReceivedRuleSection/);
  assert.match(pageSource, /InviteeListPanel/);
  assert.match(pageSource, /InviteeOverridePanel/);
  assert.match(pageSource, /normalizeInviteeContributionPage/);
  assert.match(pageSource, /buildInviteDefaultRuleRows/);
  assert.match(pageSource, /buildReceivedInviteeRuleRows/);
  assert.match(pageSource, /buildInviteeContributionDetailCards/);
  assert.match(pageSource, /buildInviteeOverrideRows/);
  assert.match(pageSource, /from ['"]\.\.\/\.\.\/helpers\/api['"]/);
  assert.match(pageSource, /from ['"]\.\.\/\.\.\/helpers\/notifications['"]/);
  assert.doesNotMatch(pageSource, /from ['"]\.\.\/\.\.\/helpers['"]/);
  assert.match(pageSource, /API\.get\('\/api\/user\/referral\/subscription'\)/);
  assert.match(
    pageSource,
    /API\.get\('\/api\/user\/referral\/subscription\/invitees'/,
  );
  assert.match(
    pageSource,
    /API\.get\(\s*`\/api\/user\/referral\/subscription\/invitees\/\$\{inviteeId\}`\s*,?\s*\)/,
  );
  assert.match(
    pageSource,
    /useEffect\(\(\) => \{\s*loadInvitees\(\);\s*\}, \[\]\);/s,
  );
  assert.match(
    pageSource,
    /useEffect\(\(\) => \{\s*loadDefaultRules\(\);\s*\}, \[\]\);/s,
  );
  assert.match(
    pageSource,
    /useEffect\(\(\) => \{\s*loadInviteeDetail\(selectedInvitee\?\.id\);\s*\}, \[selectedInvitee\?\.id\]\);/s,
  );
  assert.match(
    pageSource,
    /const handleSearch = \(\) => \{\s*setQueryKeyword\(keyword\);\s*setInviteePage\(\(currentPage\) => \([\s\S]*?page: 1,[\s\S]*?loadInvitees\(\{\s*page: 1,\s*page_size: inviteePage\.page_size,\s*keyword,\s*\}\);\s*\};/s,
  );
  assert.match(
    pageSource,
    /resolveInviteeSelectionAfterPageRefresh\(\{[\s\S]*?currentInvitee,[\s\S]*?nextItems: nextPage\.items,[\s\S]*?onSelectionCleared:[\s\S]*?setInviteeOverrideRows\(\[\]\);[\s\S]*?setInviteeContributionCards\(\[\]\);[\s\S]*?\}\)/s,
  );
  assert.match(
    pageSource,
    /const clearInviteeSelection = \(\) => \{[\s\S]*?setSelectedInvitee\(null\);\s*setInviteeOverrideRows\(\[\]\);\s*setInviteeContributionCards\(\[\]\);\s*\};/s,
  );
  assert.match(
    pageSource,
    /setInviteePage\(initialPageState\);\s*clearInviteeSelection\(\);\s*showError\(res\.data\?\.message \|\| t\('加载失败'\)\);/s,
  );
  assert.match(
    pageSource,
    /setInviteePage\(initialPageState\);\s*clearInviteeSelection\(\);\s*showError\(error\?\.message \|\| t\('加载失败'\)\);/s,
  );
});

test('invite rebate panels implement grouped editing, search, pagination, and delete flows', () => {
  const defaultRuleSource = readSource(
    'src/components/invite-rebate/InviteDefaultRuleSection.jsx',
  );
  const receivedRuleSource = readSource(
    'src/components/invite-rebate/InviteReceivedRuleSection.jsx',
  );
  const listSource = readSource(
    'src/components/invite-rebate/InviteeListPanel.jsx',
  );
  const overrideSource = readSource(
    'src/components/invite-rebate/InviteeOverridePanel.jsx',
  );
  const summarySource = readSource(
    'src/components/invite-rebate/InviteRebateSummary.jsx',
  );

  assert.match(summarySource, /t\('被邀请人数'\)/);
  assert.match(summarySource, /t\('累计返佣收益'\)/);

  assert.match(defaultRuleSource, /t\('我的返佣方案'\)/);
  assert.match(defaultRuleSource, /t\('当前返佣方案'\)/);
  assert.match(defaultRuleSource, /t\('返佣模式'\)/);
  assert.match(defaultRuleSource, /t\('所在分组'\)/);
  assert.match(defaultRuleSource, /t\('本组总返佣比例'\)/);
  assert.match(defaultRuleSource, /t\('默认返给对方比例'\)/);
  assert.match(defaultRuleSource, /formatRateBpsPercent/);
  assert.match(defaultRuleSource, /ListPagination/);
  assert.doesNotMatch(defaultRuleSource, /Pagination,/);
  assert.match(defaultRuleSource, /slice\(/);
  assert.doesNotMatch(defaultRuleSource, /API\.put\('\/api\/user\/referral\/subscription'/);
  assert.doesNotMatch(defaultRuleSource, /API\.delete\('\/api\/user\/referral\/subscription'/);

  assert.match(receivedRuleSource, /t\('上级给我的返佣'\)/);
  assert.match(
    receivedRuleSource,
    /t\('如果邀请人给你单独设置了返佣，这里会展示当前生效规则。'\)/,
  );
  assert.match(receivedRuleSource, /t\('邀请人'\)/);
  assert.match(receivedRuleSource, /t\('返佣模式'\)/);
  assert.match(receivedRuleSource, /t\('所在分组'\)/);
  assert.match(receivedRuleSource, /t\('本组总返佣比例'\)/);
  assert.match(receivedRuleSource, /t\('给我的返佣比例'\)/);
  assert.match(receivedRuleSource, /t\('你本单保留比例'\)/);
  assert.match(receivedRuleSource, /t\('已单独设置返佣'\)/);
  assert.match(receivedRuleSource, /ListPagination/);
  assert.match(receivedRuleSource, /slice\(/);

  assert.match(listSource, /t\('我的邀请用户'\)/);
  assert.match(listSource, /t\('搜索用户名 \/ 用户ID \/ 分组'\)/);
  assert.match(
    listSource,
    /t\('按返佣收益排序，支持按用户名、用户ID或分组查找。'\)/,
  );
  assert.match(
    listSource,
    /t\('共 {{count}} 位邀请用户'\s*,\s*\{\s*count:\s*totalInvitees\s*\}\)/,
  );
  assert.match(listSource, /keyword/);
  assert.match(listSource, /page_size/);
  assert.match(listSource, /ListPagination/);
  assert.match(listSource, /t\('暂无邀请用户数据'\)/);
  assert.match(listSource, /t\('未找到匹配邀请用户'\)/);
  assert.match(listSource, /t\('已单独设置'\)/);
  assert.match(listSource, /t\('购买订单'\)/);
  assert.match(listSource, /t\('使用默认'\)/);
  assert.match(listSource, /t\('已单独设置返佣'\)/);

  assert.match(overrideSource, /t\('邀请用户返佣详情'\)/);
  assert.match(overrideSource, /t\('贡献流水'\)/);
  assert.match(overrideSource, /t\('贡献概览'\)/);
  assert.match(overrideSource, /t\('按订单看'\)/);
  assert.match(overrideSource, /t\('按分录看'\)/);
  assert.match(overrideSource, /t\('贡献订单数'\)/);
  assert.match(overrideSource, /t\('你累计到账'\)/);
  assert.match(overrideSource, /t\('累计返给对方'\)/);
  assert.match(overrideSource, /t\('直推分录'\)/);
  assert.match(overrideSource, /t\('团队分录'\)/);
  assert.match(overrideSource, /t\('贡献分录明细'\)/);
  assert.match(overrideSource, /t\('单独返佣设置'\)/);
  assert.match(overrideSource, /t\('返佣明细'\)/);
  assert.match(overrideSource, /t\('你的到账返佣'\)/);
  assert.match(overrideSource, /t\('返给对方'\)/);
  assert.match(overrideSource, /t\('本笔身份'\)/);
  assert.match(overrideSource, /t\('返佣类型'\)/);
  assert.match(overrideSource, /t\('订单号'\)/);
  assert.match(overrideSource, /t\('结算时间'\)/);
  assert.match(overrideSource, /t\('暂无返佣明细'\)/);
  assert.match(overrideSource, /t\('未选择邀请用户'\)/);
  assert.match(
    overrideSource,
    /t\('从左侧选择一位邀请用户后，可查看返佣流水并单独设置返佣比例。'\)/,
  );
  assert.match(overrideSource, /t\('未单独设置时，自动使用当前返佣方案默认值。'\)/);
  assert.match(overrideSource, /t\('暂无可覆盖的模板作用域'\)/);
  assert.match(overrideSource, /t\('当前返佣方案'\)/);
  assert.match(overrideSource, /t\('返佣模式'\)/);
  assert.match(overrideSource, /t\('默认返给对方比例'\)/);
  assert.match(overrideSource, /t\('实际返给对方比例'\)/);
  assert.match(overrideSource, /t\('你本单保留比例'\)/);
  assert.match(overrideSource, /t\('使用默认'\)/);
  assert.match(overrideSource, /t\('已单独设置返佣'\)/);
  assert.match(overrideSource, /buildInviteeOverrideDraftPercentMap/);
  assert.match(overrideSource, /buildInviteeContributionSummary/);
  assert.match(overrideSource, /buildInviteeContributionLedgerRows/);
  assert.match(overrideSource, /<Tabs/);
  assert.match(overrideSource, /useEffect\(\(\) => \{\s*setDraftPercentByGroup\(buildInviteeOverrideDraftPercentMap\(normalizedRows\)\);\s*\}, \[invitee\?\.id, normalizedRows\]\);/s);
  assert.match(overrideSource, /t\(item\.roleLabel\)/);
  assert.match(overrideSource, /t\(item\.componentLabel\)/);
  assert.match(overrideSource, /formatContributionStatusLabel/);
  assert.match(overrideSource, /ListPagination/);
  assert.doesNotMatch(overrideSource, /Pagination,/);
  assert.match(overrideSource, /slice\(/);
  assert.match(
    overrideSource,
    /t\('贡献流水'\)[\s\S]*t\('单独返佣设置'\)/,
  );
  assert.match(
    overrideSource,
    /API\.put\(\s*`\/api\/user\/referral\/subscription\/invitees\/\$\{invitee\.id\}`,\s*\{/,
  );
  assert.match(
    overrideSource,
    /API\.delete\(\s*`\/api\/user\/referral\/subscription\/invitees\/\$\{invitee\.id\}`,\s*\{\s*params: \{ group \}/,
  );
});
