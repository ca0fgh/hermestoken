import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('personal settings persist the wallet topup toggle', () => {
  const personalSettingSource = readFileSync(
    new URL('../classic/src/components/settings/PersonalSetting.jsx', import.meta.url),
    'utf8',
  );

  assert.match(personalSettingSource, /quotaTopupEnabled/);
  assert.match(
    personalSettingSource,
    /quotaTopupEnabled:\s*settings\.quota_topup_enabled\s*\?\?\s*true/,
  );
  assert.match(
    personalSettingSource,
    /quota_topup_enabled:\s*notificationSettings\.quotaTopupEnabled/,
  );
});

test('notification settings expose a dedicated wallet management toggle', () => {
  const notificationSettingsSource = readFileSync(
    new URL(
      '../classic/src/components/settings/personal/cards/NotificationSettings.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(notificationSettingsSource, /itemKey='wallet'/);
  assert.match(notificationSettingsSource, /field='quotaTopupEnabled'/);
  assert.match(notificationSettingsSource, /t\('钱包管理'\)/);
  assert.match(notificationSettingsSource, /t\('余额充值管理'\)/);
});

test('topup page wires quota topup visibility into RechargeCard', () => {
  const topupSource = readFileSync(
    new URL('../classic/src/components/topup/index.jsx', import.meta.url),
    'utf8',
  );
  const rechargeCardSource = readFileSync(
    new URL('../classic/src/components/topup/RechargeCard.jsx', import.meta.url),
    'utf8',
  );

  assert.match(
    topupSource,
    /const \[quotaTopupEnabled,\s*setQuotaTopupEnabled\] = useState\(true\)/,
  );
  assert.match(topupSource, /parsedSetting\.quota_topup_enabled \?\? true/);
  assert.match(topupSource, /quotaTopupEnabled=\{quotaTopupEnabled\}/);
  assert.match(rechargeCardSource, /quotaTopupEnabled = true/);
  assert.match(rechargeCardSource, /showTopupTab/);
});
