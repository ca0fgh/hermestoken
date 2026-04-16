import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('settings page exposes a referral tab and settings surface', () => {
  const source = fs.readFileSync('web/src/pages/Setting/index.jsx', 'utf8');
  assert.match(source, /ReferralSetting/);
  assert.match(source, /itemKey:\s*['"]referral['"]/);
});

test('referral settings pages expose creation actions instead of read-only tables only', () => {
  const templateSource = fs.readFileSync(
    'web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx',
    'utf8',
  );
  const routeSource = fs.readFileSync(
    'web/src/pages/Setting/Referral/SettingsReferralEngineRoutes.jsx',
    'utf8',
  );

  assert.match(templateSource, /新增模板/);
  assert.match(templateSource, /保存/);
  assert.match(routeSource, /新增路由/);
  assert.match(routeSource, /保存/);
});
