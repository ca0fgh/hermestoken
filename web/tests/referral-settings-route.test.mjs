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

test('referral settings pages explain the meaning of template and route fields', () => {
  const templateSource = fs.readFileSync(
    'web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx',
    'utf8',
  );
  const routeSource = fs.readFileSync(
    'web/src/pages/Setting/Referral/SettingsReferralEngineRoutes.jsx',
    'utf8',
  );

  assert.match(templateSource, /所有 bps 字段都按万分比填写/);
  assert.match(templateSource, /模板身份/);
  assert.match(templateSource, /直推上限比例/);
  assert.match(templateSource, /团队总上限比例/);
  assert.match(templateSource, /被邀请人默认返佣比例/);
  assert.match(templateSource, /团队衰减系数/);
  assert.match(templateSource, /团队最大深度/);
  assert.match(templateSource, /关键规则/);
  assert.match(templateSource, /第一层直接邀请人没有活动模板：本单不返佣/);
  assert.match(templateSource, /上层没有模板或模板未启用：跳过，但不断链/);
  assert.match(templateSource, /命中有效 team 后，才触发团队级差分配/);
  assert.match(templateSource, /没命中任何有效 team 时，本单不成立团队级差返佣/);
  assert.match(templateSource, /实际生效优先级：单个 invitee 覆盖 > 用户绑定默认值 > 模板默认值/);

  assert.match(routeSource, /引擎模式/);
  assert.match(routeSource, /legacy/);
  assert.match(routeSource, /template/);
  assert.match(routeSource, /未配置路由时默认走 legacy/);
});
