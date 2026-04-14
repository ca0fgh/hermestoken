import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';

const appSource = readFileSync(new URL('../src/App.jsx', import.meta.url), 'utf8');
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
    /const InviteRebate = lazy\(\(\) => import\('\.\/pages\/InviteRebate'\)\);/,
  );
  assert.match(
    appSource,
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
  assert.match(siderBarSource, /document\.body\.classList\.add\('sidebar-collapsed'\)/);
  assert.match(
    siderBarSource,
    /document\.body\.classList\.remove\('sidebar-collapsed'\)/,
  );
  assert.match(siderBarSource, /width:\s*'var\(--sidebar-current-width\)'/);
  assert.doesNotMatch(siderBarSource, /const sidebarWidth = collapsed/);
});

test('sidebar defaults include the invite section and rebate module', () => {
  assert.match(
    sidebarHookSource,
    /invite:\s*\{\s*enabled:\s*true,\s*rebate:\s*true,\s*\}/,
  );
});

test('personal sidebar settings expose the invite section and rebate toggle', () => {
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
