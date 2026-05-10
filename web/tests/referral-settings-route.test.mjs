import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import { normalizeReferralTemplateItems } from "../classic/src/helpers/referralTemplate.js";

test("settings page exposes a referral tab and settings surface", () => {
  const source = fs.readFileSync(
    "web/classic/src/pages/Setting/index.jsx",
    "utf8",
  );
  assert.match(source, /ReferralSetting/);
  assert.match(source, /itemKey:\s*['"]referral['"]/);
});

test("referral settings pages expose creation actions instead of read-only tables only", () => {
  const templateSource = fs.readFileSync(
    "web/classic/src/pages/Setting/Referral/SettingsReferralTemplates.jsx",
    "utf8",
  );
  const referralSettingSource = fs.readFileSync(
    "web/classic/src/components/settings/ReferralSetting.jsx",
    "utf8",
  );

  assert.match(templateSource, /新增模板组/);
  assert.match(templateSource, /保存/);
  assert.match(templateSource, /一个模板组可以覆盖多个系统分组/);
  assert.doesNotMatch(referralSettingSource, /SettingsReferralEngineRoutes/);
});

test("referral settings page explains the meaning of template fields without engine route surface", () => {
  const templateSource = fs.readFileSync(
    "web/classic/src/pages/Setting/Referral/SettingsReferralTemplates.jsx",
    "utf8",
  );
  const referralSettingSource = fs.readFileSync(
    "web/classic/src/components/settings/ReferralSetting.jsx",
    "utf8",
  );

  assert.match(templateSource, /比例字段按百分比输入，保存时会自动换算成 bps/);
  assert.match(templateSource, /API\.get\('\/api\/group'\)/);
  assert.match(
    templateSource,
    /API\.get\('\/api\/referral\/settings\/subscription'\)/,
  );
  assert.match(
    templateSource,
    /API\.put\('\/api\/referral\/settings\/subscription'/,
  );
  assert.match(templateSource, /buildReferralTypeOptions/);
  assert.match(templateSource, /buildReferralLevelTypeOptions/);
  assert.match(templateSource, /模板身份/);
  assert.match(templateSource, /isDirectTemplate/);
  assert.doesNotMatch(templateSource, /团队级差总上限比例/);
  assert.match(templateSource, /团队返佣比例/);
  assert.match(templateSource, /直推返佣比例/);
  assert.match(
    templateSource,
    /团队池按“首个命中 team 的比例 - direct 直推比例”成立/,
  );
  assert.match(templateSource, /订阅返佣全局设置/);
  assert.match(
    templateSource,
    /这些参数对 subscription_referral 的整条团队返佣链统一生效/,
  );
  assert.match(templateSource, /模板名只需要在同一返佣类型 \+ 分组内保持唯一/);
  assert.match(templateSource, /被邀请人默认返佣比例/);
  assert.match(templateSource, /rateBpsToPercentNumber/);
  assert.match(templateSource, /percentNumberToRateBps/);
  assert.match(templateSource, /max=\{100\}/);
  assert.match(templateSource, /团队衰减系数/);
  assert.match(templateSource, /团队最大深度/);
  assert.match(templateSource, /teamMaxDepth:\s*0/);
  assert.match(templateSource, /autoAssignInviteeTemplate:\s*true/);
  assert.match(templateSource, /auto_assign_invitee_template/);
  assert.match(templateSource, /邀请注册自动开通返佣资格/);
  assert.match(templateSource, /自动绑定当前最低档订阅返佣模板/);
  assert.match(templateSource, /默认值为 0，表示不限深度/);
  assert.match(templateSource, /当前不限深度/);
  assert.match(templateSource, /关键规则/);
  assert.match(templateSource, /第一层直接邀请人没有活动模板：本单不返佣/);
  assert.match(templateSource, /上层没有模板或模板未启用：跳过，但不断链/);
  assert.match(templateSource, /继续向上找到第一个有效 team 后，才成立团队池/);
  assert.match(
    templateSource,
    /没命中任何有效 team 时，本单不成立团队级差返佣/,
  );
  assert.match(
    templateSource,
    /实际生效优先级：单个 invitee 覆盖 > 用户绑定默认值 > 模板默认值/,
  );
  assert.doesNotMatch(templateSource, /团队池说明/);
  assert.doesNotMatch(templateSource, /团队模板说明/);
  assert.match(templateSource, /必须选择至少一个已存在的系统分组/);
  assert.match(templateSource, /optionList=\{groupOptions\}/);
  assert.doesNotMatch(templateSource, /留空表示通用模板/);
  assert.doesNotMatch(referralSettingSource, /返佣引擎路由/);
});

test("referral settings page requests bundle view and edits bundle groups", () => {
  const templateSource = fs.readFileSync(
    "web/classic/src/pages/Setting/Referral/SettingsReferralTemplates.jsx",
    "utf8",
  );

  assert.match(
    templateSource,
    /API\.get\('\/api\/referral\/templates',\s*\{\s*params:\s*\{\s*view:\s*'bundle'\s*\}\s*\}\)/,
  );
  assert.match(templateSource, /multiple=\{true\}/);
  assert.match(templateSource, /value=\{row\.groups\}/);
  assert.match(templateSource, /updateRow\(row\.id,\s*\{\s*groups:/);
  assert.doesNotMatch(templateSource, /group:\s*row\.group/);
});

test("referral settings page saves bundle group arrays instead of a single group", () => {
  const templateSource = fs.readFileSync(
    "web/classic/src/pages/Setting/Referral/SettingsReferralTemplates.jsx",
    "utf8",
  );

  assert.match(templateSource, /groups:\s*row\.groups/);
  assert.match(
    templateSource,
    /group_rates:\s*buildReferralTemplateGroupRatesPayload\(row\)/,
  );
  assert.doesNotMatch(templateSource, /group:\s*row\.group/);
});

test("referral settings page merges invitee and identity rate editors by group", () => {
  const templateSource = fs.readFileSync(
    "web/classic/src/pages/Setting/Referral/SettingsReferralTemplates.jsx",
    "utf8",
  );

  assert.match(templateSource, /分组返佣比例/);
  assert.match(templateSource, /全部分组批量设置/);
  assert.match(
    templateSource,
    /按分组同时设置付款用户本人默认比例和模板身份返佣比例/,
  );
  assert.match(templateSource, /被邀请人默认返佣比例/);
  assert.match(templateSource, /直推返佣比例/);
  assert.match(templateSource, /团队返佣比例/);
  assert.match(templateSource, /ReferralTemplateGroupRatesEditor/);
  assert.match(templateSource, /referralTemplateIdentityRatePatch/);
  assert.match(templateSource, /referralTemplateInviteeShareDefaultPatch/);
  assert.match(templateSource, /referralTemplateIdentityRateBps/);
  assert.match(templateSource, /referralTemplateInviteeShareDefaultBps/);
  assert.match(templateSource, /inviteeShareDefaultBps:\s*rateBps/);
  assert.match(templateSource, /directCapBps:\s*rateBps/);
  assert.match(templateSource, /teamCapBps:\s*rateBps/);
});

test("referral settings page copy describes multi-group bundles and scoped uniqueness", () => {
  const templateSource = fs.readFileSync(
    "web/classic/src/pages/Setting/Referral/SettingsReferralTemplates.jsx",
    "utf8",
  );

  assert.match(templateSource, /一个模板组可以覆盖多个系统分组/);
  assert.match(templateSource, /模板名只需要在同一返佣类型 \+ 分组内保持唯一/);
  assert.match(templateSource, /确认删除该模板组及其覆盖的所有分组模板吗/);
  assert.doesNotMatch(templateSource, /模板名全局唯一/);
});

test("normalizeReferralTemplateItems normalizes bundle-shaped items", () => {
  const [item] = normalizeReferralTemplateItems([
    {
      bundle_key: " bundle-1 ",
      template_ids: ["9", "3", "9", 0, null],
      referral_type: " subscription_referral ",
      groups: [" vip ", "default", "vip", ""],
      name: " starter bundle ",
      level_type: " direct ",
      enabled: 1,
      direct_cap_bps: "1200",
      team_cap_bps: "0",
      invitee_share_default_bps: "300",
      group_rates: [
        {
          group: "vip",
          direct_cap_bps: "1500",
          invitee_share_default_bps: "500",
        },
        {
          group: "default",
          direct_cap_bps: "1200",
          invitee_share_default_bps: "300",
        },
      ],
    },
  ]);

  assert.equal(item.bundleKey, "bundle-1");
  assert.deepEqual(item.templateIds, [3, 9]);
  assert.deepEqual(item.groups, ["default", "vip"]);
  assert.equal(item.referralType, "subscription_referral");
  assert.equal(item.name, "starter bundle");
  assert.equal(item.levelType, "direct");
  assert.equal(item.directCapBps, 1200);
  assert.equal(item.inviteeShareDefaultBps, 300);
  assert.deepEqual(item.groupRates, [
    {
      group: "default",
      directCapBps: 1200,
      teamCapBps: 0,
      inviteeShareDefaultBps: 300,
    },
    {
      group: "vip",
      directCapBps: 1500,
      teamCapBps: 0,
      inviteeShareDefaultBps: 500,
    },
  ]);
});
