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
  assert.match(pageSource, /InviteeListPanel/);
  assert.match(pageSource, /InviteeOverridePanel/);
  assert.match(pageSource, /normalizeInviteeContributionPage/);
  assert.match(pageSource, /buildInviteDefaultRuleRows/);
  assert.match(pageSource, /buildInviteeOverrideRows/);
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
    /const clearInviteeSelection = \(\) => \{[\s\S]*?setSelectedInvitee\(null\);\s*setInviteeOverrideRows\(\[\]\);\s*\};/s,
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

  assert.match(defaultRuleSource, /t\('邀请人分账规则'\)/);
  assert.match(defaultRuleSource, /t\('返佣类型'\)/);
  assert.match(defaultRuleSource, /getTypeLabel/);
  assert.match(defaultRuleSource, /t\('分组'\)/);
  assert.match(defaultRuleSource, /t\('被邀请人返佣比例'\)/);
  assert.match(
    defaultRuleSource,
    /API\.delete\('\/api\/user\/referral\/subscription', \{\s*params: \{ group \}/,
  );

  assert.match(listSource, /t\('邀请用户'\)/);
  assert.match(listSource, /t\('搜索用户名'\)/);
  assert.match(listSource, /keyword/);
  assert.match(listSource, /page_size/);
  assert.match(listSource, /Pagination/);
  assert.match(listSource, /t\('暂无邀请用户'\)/);

  assert.match(overrideSource, /t\('邀请用户返佣指定'\)/);
  assert.match(overrideSource, /t\('未选择邀请用户'\)/);
  assert.match(overrideSource, /t\('未指定时使用邀请人分账规则'\)/);
  assert.match(overrideSource, /t\('暂无指定项，未指定时使用邀请人分账规则'\)/);
  assert.match(overrideSource, /t\('返佣类型'\)/);
  assert.match(overrideSource, /t\('当前邀请人分账比例'\)/);
  assert.match(overrideSource, /getTypeLabel/);
  assert.match(
    overrideSource,
    /API\.put\(\s*`\/api\/user\/referral\/subscription\/invitees\/\$\{invitee\.id\}`,\s*\{/,
  );
  assert.match(
    overrideSource,
    /API\.delete\(\s*`\/api\/user\/referral\/subscription\/invitees\/\$\{invitee\.id\}`,\s*\{\s*params: \{ group \}/,
  );
});
