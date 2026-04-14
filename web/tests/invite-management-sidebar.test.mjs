import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const appRoutesSource = readFileSync(
  new URL('../src/AppRoutes.jsx', import.meta.url),
  'utf8',
);
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

test('app routes lazy load the invite rebate console page', () => {
  assert.match(
    appRoutesSource,
    /const InviteRebate = lazy\(\(\) => import\('\.\/pages\/InviteRebate'\)\);/,
  );
  assert.match(
    appRoutesSource,
    /path='\/console\/invite\/rebate'[\s\S]*<PrivateRoute>\{renderWithSuspense\(<InviteRebate \/>\)\}<\/PrivateRoute>/,
  );
});

test('sidebar renders a standalone invite management group with the rebate item', () => {
  assert.match(siderBarSource, /rebate:\s*'\/console\/invite\/rebate'/);
  assert.match(siderBarSource, /hasSectionVisibleModules\('invite'\)/);
  assert.match(siderBarSource, /t\('邀请管理'\)/);
  assert.match(siderBarSource, /text:\s*t\('邀请返佣'\),\s*itemKey:\s*'rebate'/);
  assert.match(siderBarSource, /const inviteItems = useMemo\(/);
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
