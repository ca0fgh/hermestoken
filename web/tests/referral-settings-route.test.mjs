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
  const referralSettingSource = fs.readFileSync(
    'web/src/components/settings/ReferralSetting.jsx',
    'utf8',
  );

  assert.match(templateSource, /新增模板/);
  assert.match(templateSource, /保存/);
  assert.match(templateSource, /同一返佣类型和分组下可以创建多个模板/);
  assert.doesNotMatch(referralSettingSource, /SettingsReferralEngineRoutes/);
});

test('referral settings page explains the meaning of template fields without engine route surface', () => {
  const templateSource = fs.readFileSync(
    'web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx',
    'utf8',
  );
  const referralSettingSource = fs.readFileSync(
    'web/src/components/settings/ReferralSetting.jsx',
    'utf8',
  );

  assert.match(templateSource, /比例字段按百分比输入，保存时会自动换算成 bps/);
  assert.match(templateSource, /API\.get\('\/api\/group'\)/);
  assert.match(templateSource, /API\.get\('\/api\/referral\/settings\/subscription'\)/);
  assert.match(templateSource, /API\.put\('\/api\/referral\/settings\/subscription'/);
  assert.match(templateSource, /buildReferralTypeOptions/);
  assert.match(templateSource, /buildReferralLevelTypeOptions/);
  assert.match(templateSource, /模板身份/);
  assert.match(templateSource, /isDirectTemplate/);
  assert.doesNotMatch(templateSource, /团队级差总上限比例/);
  assert.match(templateSource, /团队返佣比例/);
  assert.match(templateSource, /直推返佣比例/);
  assert.match(templateSource, /团队池按“首个命中 team 的比例 - direct 直推比例”成立/);
  assert.match(templateSource, /订阅返佣全局设置/);
  assert.match(templateSource, /这些参数对 subscription_referral 的整条团队返佣链统一生效/);
  assert.match(templateSource, /模板名全局唯一/);
  assert.match(templateSource, /被邀请人默认返佣比例/);
  assert.match(templateSource, /rateBpsToPercentNumber/);
  assert.match(templateSource, /percentNumberToRateBps/);
  assert.match(templateSource, /max=\{100\}/);
  assert.match(templateSource, /团队衰减系数/);
  assert.match(templateSource, /团队最大深度/);
  assert.match(templateSource, /teamMaxDepth:\s*0/);
  assert.match(templateSource, /默认值为 0，表示不限深度/);
  assert.match(templateSource, /当前不限深度/);
  assert.match(templateSource, /关键规则/);
  assert.match(templateSource, /第一层直接邀请人没有活动模板：本单不返佣/);
  assert.match(templateSource, /上层没有模板或模板未启用：跳过，但不断链/);
  assert.match(templateSource, /继续向上找到第一个有效 team 后，才成立团队池/);
  assert.match(templateSource, /没命中任何有效 team 时，本单不成立团队级差返佣/);
  assert.match(templateSource, /实际生效优先级：单个 invitee 覆盖 > 用户绑定默认值 > 模板默认值/);
  assert.doesNotMatch(templateSource, /团队池说明/);
  assert.doesNotMatch(templateSource, /团队模板说明/);
  assert.match(templateSource, /必须选择一个已存在的系统分组/);
  assert.match(templateSource, /optionList=\{groupOptions\}/);
  assert.doesNotMatch(templateSource, /留空表示通用模板/);
  assert.doesNotMatch(referralSettingSource, /返佣引擎路由/);
});
