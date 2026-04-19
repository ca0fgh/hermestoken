import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const readSource = (relativePath) =>
  fs.readFileSync(
    path.join('/Users/money/project/subproject/hermestoken/web', relativePath),
    'utf8',
  );

test('admin route tree exposes wallet withdrawal management', () => {
  const appSource = readSource('src/routes/ConsoleRoutes.jsx');
  const sidebarSource = readSource('src/components/layout/SiderBar.jsx');
  const useSidebarSource = readSource('src/hooks/common/useSidebar.js');
  const adminSettingsSource = readSource(
    'src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx',
  );

  assert.match(appSource, /const Withdrawal = lazyWithRetry\(/);
  assert.match(appSource, /path='\/console\/withdrawal'/);
  assert.match(sidebarSource, /withdrawal:\s*'\/console\/withdrawal'/);
  assert.match(sidebarSource, /text:\s*t\('提现管理'\)/);
  assert.match(useSidebarSource, /withdrawal:\s*true/);
  assert.match(adminSettingsSource, /key:\s*'withdrawal'/);
});

test('admin withdrawal table derives amount symbol from withdrawal currency', () => {
  const columnDefsSource = readSource(
    'src/components/table/withdrawals/WithdrawalsColumnDefs.jsx',
  );
  assert.match(columnDefsSource, /getWithdrawalCurrencySymbol/);
  assert.match(columnDefsSource, /record\?\.currency/);
  assert.match(columnDefsSource, /支付宝姓名/);
  assert.match(columnDefsSource, /copyable/);
  assert.doesNotMatch(columnDefsSource, /maskAlipayAccount/);
});

test('admin withdrawal filters use a single unified keyword search', () => {
  const filtersSource = readSource(
    'src/components/table/withdrawals/WithdrawalsFilters.jsx',
  );
  const hookSource = readSource('src/hooks/withdrawals/useWithdrawalsData.jsx');

  assert.match(filtersSource, /搜索提现单号 \/ 用户名 \/ 支付宝账号/);
  assert.match(filtersSource, /用户 ID/);
  assert.doesNotMatch(filtersSource, /placeholder=\{t\('用户 ID'\)\}/);
  assert.doesNotMatch(filtersSource, /filters\.userId/);
  assert.doesNotMatch(hookSource, /userId:/);
  assert.doesNotMatch(hookSource, /username:/);
  assert.doesNotMatch(hookSource, /alipayAccount:/);
  assert.doesNotMatch(hookSource, /params\.set\('user_id'/);
});
