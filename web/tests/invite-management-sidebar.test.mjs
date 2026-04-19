import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';

const appSource = readFileSync(new URL('../src/App.jsx', import.meta.url), 'utf8');
const consoleRoutesSource = readFileSync(
  new URL('../src/routes/ConsoleRoutes.jsx', import.meta.url),
  'utf8',
);
const appRoutesPath = new URL('../src/AppRoutes.jsx', import.meta.url);
const siderBarSource = readFileSync(
  new URL('../src/components/layout/SiderBar.jsx', import.meta.url),
  'utf8',
);
const sidebarHookSource = readFileSync(
  new URL('../src/hooks/common/useSidebar.js', import.meta.url),
  'utf8',
);
const notificationSettingsSource = readFileSync(
  new URL(
    '../src/components/settings/personal/cards/NotificationSettings.jsx',
    import.meta.url,
  ),
  'utf8',
);
const adminSidebarSettingsSource = readFileSync(
  new URL(
    '../src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('app entry lazy loads the invite rebate console page on the live route tree', () => {
  assert.match(
    appSource,
    /const ConsoleRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/ConsoleRoutes'\),/,
  );
  assert.match(
    consoleRoutesSource,
    /const InviteRebate = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/InviteRebate'\),[\s\S]*'invite-rebate-route',[\s\S]*\);/,
  );
  assert.match(
    consoleRoutesSource,
    /path='\/console\/invite\/rebate'[\s\S]*<PrivateRoute>\{renderWithSuspense\(<InviteRebate \/>\)\}<\/PrivateRoute>/,
  );
  assert.doesNotMatch(
    appSource,
    /const AppRoutes = lazy\(\(\) => import\('\.\/AppRoutes'\)\);/,
  );
});

test('invite scaffold does not leave behind a dead AppRoutes layer', () => {
  assert.equal(existsSync(appRoutesPath), false);
});

test('sidebar renders a standalone invite management group with the rebate item', () => {
  assert.match(siderBarSource, /rebate:\s*'\/console\/invite\/rebate'/);
  assert.match(siderBarSource, /hasSectionVisibleModules\('invite'\)/);
  assert.match(siderBarSource, /t\('邀请管理'\)/);
  assert.match(siderBarSource, /text:\s*t\('邀请返佣'\),\s*itemKey:\s*'rebate'/);
  assert.match(siderBarSource, /const inviteItems = useMemo\(/);
  assert.match(siderBarSource, /from '\.\.\/\.\.\/helpers\/utils';/);
  assert.doesNotMatch(
    siderBarSource,
    /from '\.\.\/\.\.\/helpers\/notifications';/,
  );
  assert.doesNotMatch(siderBarSource, /from '\.\.\/\.\.\/helpers\/session';/);
  assert.doesNotMatch(
    siderBarSource,
    /document\.body\.classList\.(add|remove)\('sidebar-collapsed'\)/,
  );
  assert.match(siderBarSource, /const sidebarWidth = collapsed\s*\?/);
  assert.match(siderBarSource, /width:\s*sidebarWidth/);
});

test('sidebar defaults include the invite section and rebate module', () => {
  assert.match(
    sidebarHookSource,
    /invite:\s*\{\s*enabled:\s*true,\s*rebate:\s*true,\s*\}/,
  );
});

test('personal sidebar settings expose the invite section and rebate toggle', () => {
  assert.match(notificationSettingsSource, /const mergeSidebarModulesWithDefaults =/);
  assert.match(
    notificationSettingsSource,
    /mergedModules\[sectionKey\]\s*=\s*\{\s*\.\.\.\(defaults\[sectionKey\] \|\| \{\}\),\s*\.\.\.sectionValue,\s*\}/s,
  );
  assert.match(notificationSettingsSource, /key:\s*'invite'/);
  assert.match(notificationSettingsSource, /title:\s*t\('邀请管理'\)/);
  assert.match(notificationSettingsSource, /description:\s*t\('邀请返佣入口显示控制'\)/);
  assert.match(notificationSettingsSource, /key:\s*'rebate'/);
  assert.match(notificationSettingsSource, /title:\s*t\('邀请返佣'\)/);
});

test('admin sidebar settings expose invite section defaults and rebate control', () => {
  assert.match(adminSidebarSettingsSource, /mergeAdminConfig/);
  assert.match(adminSidebarSettingsSource, /key:\s*'invite'/);
  assert.match(adminSidebarSettingsSource, /title:\s*t\('邀请管理'\)/);
  assert.match(adminSidebarSettingsSource, /key:\s*'rebate'/);
  assert.match(adminSidebarSettingsSource, /title:\s*t\('邀请返佣'\)/);
});
